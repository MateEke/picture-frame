// Package startup holds the logic-bearing startup helpers extracted from the
// picture-frame command: config transforms and backend selection that are worth
// testing in isolation from the binary's process-level wiring.
package startup

import (
	"log/slog"
	"os"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/mqtt"
)

// ApplyConfiguredLogLevel sets the running log level from the LOG_LEVEL env var,
// falling back to config; an invalid value is warned about and left unchanged.
// The env var always wins, even over a valid config value.
func ApplyConfiguredLogLevel(levelVar *slog.LevelVar, cfg *config.Config, log *slog.Logger) {
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if err := levelVar.UnmarshalText([]byte(lvl)); err != nil {
			log.Warn("invalid LOG_LEVEL env var, ignoring", "value", lvl, "err", err)
		}
		return
	}
	if cfg.LogLevel != "" {
		if err := levelVar.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
			log.Warn("invalid log_level in config, using info", "value", cfg.LogLevel, "err", err)
		}
	}
}

// BuildSensorSpecs adapts BLE sensors to MQTT specs. Bridging mqtt-subscriber
// readings would loop them back through the same broker.
func BuildSensorSpecs(cfg *config.Config) []mqtt.SensorSpec {
	ble := config.BLESensors(cfg.Sensors)
	specs := make([]mqtt.SensorSpec, len(ble))
	for i, s := range ble {
		specs[i] = mqtt.SensorSpec{ID: s.ID, Role: s.Role, Kinds: s.Kinds}
	}
	return specs
}

// SyncerStatus avoids the typed-nil-in-interface trap: returning a nil
// *library.Syncer straight into the interface field would yield a non-nil
// interface value, so callers must funnel through this guard.
func SyncerStatus(s *library.Syncer) library.SyncerStatus {
	if s == nil {
		return nil
	}
	return s
}
