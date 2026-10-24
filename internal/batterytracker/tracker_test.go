package batterytracker_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MateEke/picture-frame/internal/batterytracker"
	"github.com/MateEke/picture-frame/internal/hostmetrics"
)

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
