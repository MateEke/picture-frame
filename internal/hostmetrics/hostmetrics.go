// Package hostmetrics reads Raspberry Pi telemetry from /proc, /sys, and vcgencmd.
// Every field is best-effort: a failed read leaves that field absent (Has* = false)
// rather than failing the whole snapshot, so a non-Pi dev box degrades cleanly.
package hostmetrics

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// undervoltageBits is get_throttled bit0 (under-voltage now) | bit16 (occurred
// since boot). Occurred-since-boot is included so 60s sampling does not miss a dip.
const undervoltageBits = 0x1 | 0x10000

// Metrics carries a presence flag per value so one failed read does not poison the rest.
type Metrics struct {
	CPUTempC     float64
	HasCPUTemp   bool
	MemUsedPct   float64
	HasMem       bool
	BootTime     time.Time
	HasBootTime  bool
	Undervoltage bool
	HasThrottle  bool
}

type HostInfo struct {
	Model    string
	Revision string
	Serial   string
}

// ThrottleReader reads the get_throttled bitmask; ok=false when vcgencmd is unavailable.
type ThrottleReader interface {
	Throttled(ctx context.Context) (bits uint64, ok bool)
}

// Config overrides the source paths (for tests) and supplies the throttle reader.
type Config struct {
	ThermalPath string // default /sys/class/thermal/thermal_zone0/temp
	MeminfoPath string // default /proc/meminfo
	StatPath    string // default /proc/stat
	ModelPath   string // default /proc/device-tree/model
	CPUInfoPath string // default /proc/cpuinfo
	Throttle    ThrottleReader
}

type Reader struct{ cfg Config }

func New(cfg Config) *Reader {
	def := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	cfg.ThermalPath = def(cfg.ThermalPath, "/sys/class/thermal/thermal_zone0/temp")
	cfg.MeminfoPath = def(cfg.MeminfoPath, "/proc/meminfo")
	cfg.StatPath = def(cfg.StatPath, "/proc/stat")
	cfg.ModelPath = def(cfg.ModelPath, "/proc/device-tree/model")
	cfg.CPUInfoPath = def(cfg.CPUInfoPath, "/proc/cpuinfo")
	return &Reader{cfg: cfg}
}

func (r *Reader) Read(ctx context.Context) Metrics {
	var m Metrics
	if c, ok := readCPUTemp(r.cfg.ThermalPath); ok {
		m.CPUTempC, m.HasCPUTemp = c, true
	}
	if p, ok := readMemUsedPct(r.cfg.MeminfoPath); ok {
		m.MemUsedPct, m.HasMem = p, true
	}
	if b, ok := readBootTime(r.cfg.StatPath); ok {
		m.BootTime, m.HasBootTime = b, true
	}
	if r.cfg.Throttle != nil {
		if bits, ok := r.cfg.Throttle.Throttled(ctx); ok {
			m.Undervoltage, m.HasThrottle = bits&undervoltageBits != 0, true
		}
	}
	return m
}

func (r *Reader) ReadInfo() HostInfo {
	var info HostInfo
	if raw, err := os.ReadFile(r.cfg.ModelPath); err == nil {
		info.Model = strings.TrimSpace(string(bytes.TrimRight(raw, "\x00")))
	}
	if info.Model == "" {
		info.Model = cpuinfoValue(r.cfg.CPUInfoPath, "Model")
	}
	info.Revision = revisionSuffix(info.Model)
	info.Serial = cpuinfoValue(r.cfg.CPUInfoPath, "Serial")
	return info
}

func readCPUTemp(path string) (float64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	milli, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, false
	}
	return float64(milli) / 1000, true
}

func readMemUsedPct(path string) (float64, bool) {
	fields := readColonFields(path, "MemTotal", "MemAvailable")
	total, okT := fields["MemTotal"]
	avail, okA := fields["MemAvailable"]
	if !okT || !okA || total == 0 {
		return 0, false
	}
	return (1 - float64(avail)/float64(total)) * 100, true
}

func readBootTime(path string) (time.Time, bool) {
	for _, line := range readLines(path) {
		rest, ok := strings.CutPrefix(line, "btime ")
		if !ok {
			continue
		}
		sec, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(sec, 0), true
	}
	return time.Time{}, false
}

// revisionSuffix extracts a trailing "Rev x.y" from a model string, "" if none.
func revisionSuffix(model string) string {
	if i := strings.Index(model, "Rev "); i >= 0 {
		return model[i:]
	}
	return ""
}

func cpuinfoValue(path, key string) string {
	for _, line := range readLines(path) {
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// readColonFields returns the integer value of each requested "Key: value kB" line present.
func readColonFields(path string, keys ...string) map[string]int64 {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	out := make(map[string]int64, len(keys))
	for _, line := range readLines(path) {
		k, v, ok := strings.Cut(line, ":")
		if !ok || !want[strings.TrimSpace(k)] {
			continue
		}
		num := strings.Fields(strings.TrimSpace(v)) // "161844 kB" -> "161844"
		if len(num) == 0 {
			continue
		}
		if n, err := strconv.ParseInt(num[0], 10, 64); err == nil {
			out[strings.TrimSpace(k)] = n
		}
	}
	return out
}

// readLines whole-reads path (these /proc and /sys files are tiny), nil if unreadable.
func readLines(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(raw), "\n")
}
