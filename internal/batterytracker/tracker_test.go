package batterytracker_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/batterytracker"
	"github.com/MateEke/picture-frame/internal/hostmetrics"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mockMetrics struct {
	m hostmetrics.Metrics
}

func (m mockMetrics) Read(context.Context) hostmetrics.Metrics {
	return m.m
}

type mockDisplay struct {
	on bool
}

func (m mockDisplay) State() bool {
	return m.on
}

func TestBatteryTrackerSamplingAndStats(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "battery.json")

	mm := &mockMetrics{m: hostmetrics.Metrics{HasBattery: true, BatteryPct: 80, BatteryCharging: false}}
	md := &mockDisplay{on: true}

	tracker := batterytracker.New(nil, mm, md, storePath)
	tracker.Sample(context.Background())

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected store file to exist: %v", err)
	}

	stats := tracker.GetStats()
	if stats.CurrentPct != 80 {
		t.Errorf("CurrentPct = %d, want 80", stats.CurrentPct)
	}
	if stats.ScreenOn.SufficientData {
		t.Errorf("expected insufficient data for single sample")
	}

	// Verify stats output formatting with empty/insufficient data
	if !stats.ScreenOn.SufficientData && stats.ScreenOn.Confidence != "insufficient" {
		t.Errorf("Confidence = %s, want insufficient", stats.ScreenOn.Confidence)
	}
}

// The store must grow by one line per sample, never rewrite the full history.
func TestStoreIsAppendOnlyAndReloads(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "battery-history.json")

	mm := &mockMetrics{m: hostmetrics.Metrics{HasBattery: true, BatteryPct: 80}}
	tr := batterytracker.New(testLogger(), mm, &mockDisplay{on: true}, storePath)
	for i := 0; i < 3; i++ {
		tr.Sample(context.Background())
	}

	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("store has %d lines, want 3 (one per sample)", len(lines))
	}
	for i, line := range lines {
		var s batterytracker.Sample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Errorf("line %d is not a single sample: %v", i+1, err)
		}
	}

	// A fresh tracker reloads the same history.
	reloaded := batterytracker.New(testLogger(), mm, &mockDisplay{on: true}, storePath)
	if got := len(reloaded.GetStats().History); got != 3 {
		t.Errorf("reloaded history = %d samples, want 3", got)
	}
}

// The original store was one JSON array; loading it must prune, migrate to
// JSONL, and ignore a torn final line.
func TestLoadMigratesLegacyArrayAndTornLine(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "battery-history.json")

	now := time.Now()
	legacy := []batterytracker.Sample{
		{Timestamp: now.Add(-8 * 24 * time.Hour), Pct: 50, ScreenOn: true},
		{Timestamp: now.Add(-time.Hour), Pct: 45, ScreenOn: true},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy store: %v", err)
	}
	if err := os.WriteFile(storePath, raw, 0o600); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}

	mm := &mockMetrics{m: hostmetrics.Metrics{HasBattery: true, BatteryPct: 45}}
	tr := batterytracker.New(testLogger(), mm, &mockDisplay{on: true}, storePath)
	if got := len(tr.GetStats().History); got != 1 {
		t.Fatalf("history = %d samples, want 1 (8-day-old sample pruned)", got)
	}

	migrated, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read migrated store: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(migrated)), "[") {
		t.Error("store was not migrated to JSONL")
	}
	if strings.Count(strings.TrimSpace(string(migrated)), "\n") != 0 {
		t.Errorf("migrated store = %q, want a single line", migrated)
	}

	// A torn last line (power loss mid-append) must not discard the good ones.
	torn := string(migrated) + `{"timestamp":"2026-10-27T21:00:00Z","pct":4`
	//nolint:gosec // G703 false positive: storePath is derived from t.TempDir().
	if err := os.WriteFile(storePath, []byte(torn), 0o600); err != nil {
		t.Fatalf("write torn store: %v", err)
	}
	tr2 := batterytracker.New(testLogger(), mm, &mockDisplay{on: true}, storePath)
	if got := len(tr2.GetStats().History); got != 1 {
		t.Errorf("history with torn line = %d samples, want 1", got)
	}
}
