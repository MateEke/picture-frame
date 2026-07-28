package hostmetrics

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type fakeThrottle struct {
	bits uint64
	ok   bool
}

func (f fakeThrottle) Throttled(context.Context) (uint64, bool) { return f.bits, f.ok }

// writeTree lays out a fake /proc + /sys and returns a Config pointing at it.
func writeTree(t *testing.T, thermal, meminfo, stat string, thr ThrottleReader) Config {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	return Config{
		ThermalPath: write("temp", thermal),
		MeminfoPath: write("meminfo", meminfo),
		StatPath:    write("stat", stat),
		Throttle:    thr,
	}
}

const sampleMeminfo = "MemTotal:         435196 kB\nMemFree: 64272 kB\nMemAvailable:     161844 kB\n"

func TestReadParsesAllFields(t *testing.T) {
	boot := time.Now().Add(-3 * time.Hour).Unix()
	cfg := writeTree(t,
		"47236\n",
		sampleMeminfo,
		"cpu  1 2 3\nbtime "+strconv.FormatInt(boot, 10)+"\nprocesses 5\n",
		fakeThrottle{bits: 0x50000, ok: true}, // bit16 set: undervoltage occurred
	)
	m := New(cfg).Read(context.Background())

	if !m.HasCPUTemp || m.CPUTempC != 47.236 {
		t.Errorf("cpu temp: got %v (has=%v), want 47.236", m.CPUTempC, m.HasCPUTemp)
	}
	// used% = (1 - 161844/435196) * 100 = 62.81...
	if !m.HasMem || m.MemUsedPct < 62.8 || m.MemUsedPct > 62.82 {
		t.Errorf("mem used%%: got %v (has=%v), want ~62.81", m.MemUsedPct, m.HasMem)
	}
	if !m.HasBootTime || m.BootTime.Unix() != boot {
		t.Errorf("boot time: got %v (has=%v), want unix %d", m.BootTime, m.HasBootTime, boot)
	}
	if !m.HasThrottle || !m.Undervoltage {
		t.Errorf("undervoltage: got %v (has=%v), want true", m.Undervoltage, m.HasThrottle)
	}
}

func TestReadMissingFilesLeaveFieldsAbsent(t *testing.T) {
	cfg := Config{
		ThermalPath: "/nonexistent/temp",
		MeminfoPath: "/nonexistent/meminfo",
		StatPath:    "/nonexistent/stat",
		Throttle:    fakeThrottle{ok: false},
	}
	m := New(cfg).Read(context.Background())
	if m.HasCPUTemp || m.HasMem || m.HasBootTime || m.HasThrottle {
		t.Errorf("expected all fields absent, got %+v", m)
	}
}

func TestReadNilThrottleLeavesThrottleAbsent(t *testing.T) {
	cfg := writeTree(t, "40000\n", sampleMeminfo, "btime 100\n", nil)
	if m := New(cfg).Read(context.Background()); m.HasThrottle {
		t.Errorf("nil throttle reader should leave throttle absent, got %+v", m)
	}
}

func TestReadThrottleClearMeansNoUndervoltage(t *testing.T) {
	cfg := writeTree(t, "40000\n", sampleMeminfo, "btime 100\n", fakeThrottle{bits: 0x0, ok: true})
	m := New(cfg).Read(context.Background())
	if !m.HasThrottle || m.Undervoltage {
		t.Errorf("undervoltage: got %v (has=%v), want false", m.Undervoltage, m.HasThrottle)
	}
}

func TestReadCurrentUndervoltageBit(t *testing.T) {
	cfg := writeTree(t, "40000\n", sampleMeminfo, "btime 100\n", fakeThrottle{bits: 0x1, ok: true})
	if m := New(cfg).Read(context.Background()); !m.Undervoltage {
		t.Error("bit0 (under-voltage now) should set Undervoltage")
	}
}

func TestReadMalformedValuesAbsent(t *testing.T) {
	cfg := writeTree(t, "not-a-number\n", "MemTotal: x kB\n", "btime nope\n", fakeThrottle{ok: false})
	m := New(cfg).Read(context.Background())
	if m.HasCPUTemp || m.HasMem || m.HasBootTime {
		t.Errorf("malformed inputs should be absent, got %+v", m)
	}
}

func TestReadMemZeroTotalAbsent(t *testing.T) {
	cfg := writeTree(t, "40000\n", "MemTotal: 0 kB\nMemAvailable: 10 kB\n", "btime 1\n", nil)
	if m := New(cfg).Read(context.Background()); m.HasMem {
		t.Error("zero MemTotal should leave memory absent (no divide by zero)")
	}
}

func TestReadMemEmptyValueAbsent(t *testing.T) {
	cfg := writeTree(t, "40000\n", "MemTotal:\nMemAvailable: 100 kB\n", "btime 1\n", nil)
	if m := New(cfg).Read(context.Background()); m.HasMem {
		t.Error("MemTotal with an empty value should leave memory absent")
	}
}

func TestReadStatWithoutBtimeAbsent(t *testing.T) {
	cfg := writeTree(t, "40000\n", sampleMeminfo, "cpu 1 2 3\nprocesses 5\n", nil)
	if m := New(cfg).Read(context.Background()); m.HasBootTime {
		t.Error("stat without a btime line should leave boot time absent")
	}
}

func TestReadInfoParsesModelAndSerial(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model")
	cpu := filepath.Join(dir, "cpuinfo")
	mustWrite(t, model, "Raspberry Pi Zero 2 W Rev 1.0\x00")
	mustWrite(t, cpu, "Serial\t\t: 00000000150d684e\nModel\t\t: X\n")

	info := New(Config{ModelPath: model, CPUInfoPath: cpu}).ReadInfo()
	if info.Model != "Raspberry Pi Zero 2 W Rev 1.0" {
		t.Errorf("model: got %q", info.Model)
	}
	if info.Revision != "Rev 1.0" {
		t.Errorf("revision: got %q", info.Revision)
	}
	if info.Serial != "00000000150d684e" {
		t.Errorf("serial: got %q", info.Serial)
	}
}

func TestReadInfoFallsBackToCPUInfoModel(t *testing.T) {
	dir := t.TempDir()
	cpu := filepath.Join(dir, "cpuinfo")
	mustWrite(t, cpu, "Model\t\t: Raspberry Pi 4 Model B Rev 1.5\n")

	info := New(Config{ModelPath: "/nonexistent/model", CPUInfoPath: cpu}).ReadInfo()
	if info.Model != "Raspberry Pi 4 Model B Rev 1.5" || info.Revision != "Rev 1.5" {
		t.Errorf("cpuinfo fallback failed: %+v", info)
	}
}

func TestReadInfoNoRevisionWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model")
	mustWrite(t, model, "Some Board\x00")
	if info := New(Config{ModelPath: model, CPUInfoPath: "/nonexistent"}).ReadInfo(); info.Revision != "" {
		t.Errorf("revision should be empty, got %q", info.Revision)
	}
}

func TestRevisionSuffixAtStart(t *testing.T) {
	// A model that is only the revision keeps the match at index 0.
	if got := revisionSuffix("Rev 1.0"); got != "Rev 1.0" {
		t.Errorf("revisionSuffix(%q) = %q, want the whole string", "Rev 1.0", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
