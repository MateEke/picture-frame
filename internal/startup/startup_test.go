package startup_test

import (
	"log/slog"
	"slices"
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestApplyConfiguredLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		env      string // "" means LOG_LEVEL unset
		cfgLevel string
		initial  slog.Level
		want     slog.Level
	}{
		{"env overrides valid config", "debug", "error", slog.LevelInfo, slog.LevelDebug},
		{"invalid env keeps current", "nonsense", "", slog.LevelWarn, slog.LevelWarn},
		{"config used when env empty", "", "error", slog.LevelInfo, slog.LevelError},
		{"invalid config keeps current", "", "nonsense", slog.LevelWarn, slog.LevelWarn},
		{"both empty is a no-op", "", "", slog.LevelWarn, slog.LevelWarn},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.env)
			var lv slog.LevelVar
			lv.Set(tc.initial)
			startup.ApplyConfiguredLogLevel(&lv, &config.Config{LogLevel: tc.cfgLevel}, testutil.NopLogger())
			if lv.Level() != tc.want {
				t.Errorf("level = %v, want %v", lv.Level(), tc.want)
			}
		})
	}
}

func TestBuildSensorSpecsOnlyBridgesBLE(t *testing.T) {
	cfg := &config.Config{Sensors: []config.SensorConfig{
		{ID: "ble1", Type: "ble", Role: "inside", Characteristics: []config.CharacteristicConfig{
			{Kind: "temperature"}, {Kind: "humidity"},
		}},
		{ID: "mock1", Type: "mock"},
		{ID: "sub1", Type: "mqtt-subscriber"},
	}}

	specs := startup.BuildSensorSpecs(cfg)

	if len(specs) != 1 {
		t.Fatalf("len = %d, want 1 (only BLE is bridged)", len(specs))
	}
	if specs[0].ID != "ble1" || specs[0].Role != "inside" {
		t.Errorf("spec = %+v, want ID=ble1 Role=inside", specs[0])
	}
	if want := []string{"temperature", "humidity"}; !slices.Equal(specs[0].Kinds, want) {
		t.Errorf("kinds = %v, want %v", specs[0].Kinds, want)
	}
}

func TestSyncerStatusNilReturnsNilInterface(t *testing.T) {
	if got := startup.SyncerStatus(nil); got != nil {
		t.Errorf("got %v, want a nil interface", got)
	}
}

func TestSyncerStatusNonNilPassesThrough(t *testing.T) {
	if got := startup.SyncerStatus(&library.Syncer{}); got == nil {
		t.Error("got nil, want the syncer")
	}
}
