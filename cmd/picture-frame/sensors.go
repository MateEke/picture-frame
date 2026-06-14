package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	displaypkg "github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/mqtt"
	mqttadapter "github.com/MateEke/picture-frame/internal/mqtt/adapter"
	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/sensors/ble"
	bleadapter "github.com/MateEke/picture-frame/internal/sensors/ble/adapter"
	mocksensor "github.com/MateEke/picture-frame/internal/sensors/mock"
	"github.com/MateEke/picture-frame/internal/sensors/mqttsubscriber"
	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/state"
)

func hasMqttSubscribers(cfg *config.Config) bool {
	for _, s := range cfg.Sensors {
		if s.Type == "mqtt-subscriber" {
			return true
		}
	}
	return false
}

// setupMQTT returns (nil, closed) when MQTT isn't needed. pubDone closes after
// the Publisher's offline publish so main can disconnect cleanly.
func setupMQTT(ctx context.Context, log *slog.Logger, cfg *config.Config, bus *state.Bus, screen *displaypkg.Screen) (*mqtt.Hub, <-chan struct{}) {
	closed := make(chan struct{})
	close(closed)
	if !cfg.Mqtt.Bridge.Enabled && !hasMqttSubscribers(cfg) {
		return nil, closed
	}
	mqttSet := mqtt.Settings{
		NodeID:          cfg.Mqtt.Bridge.NodeID,
		BaseTopic:       cfg.Mqtt.Bridge.BaseTopic,
		DiscoveryPrefix: cfg.Mqtt.Bridge.DiscoveryPrefix,
		StaleAfter:      cfg.Mqtt.Bridge.StaleAfter.Duration,
	}
	willTopic := ""
	if cfg.Mqtt.Bridge.Enabled {
		willTopic = mqttSet.BridgeAvailabilityTopic()
	}
	mqttClient := mqttadapter.New(mqttadapter.Config{
		Broker:    cfg.Mqtt.Broker,
		ClientID:  cfg.Mqtt.ClientID,
		Username:  cfg.Mqtt.Username,
		Password:  cfg.Mqtt.Password,
		WillTopic: willTopic,
	})
	hub := mqtt.NewHub(log, mqttClient)
	if !cfg.Mqtt.Bridge.Enabled {
		log.Info("mqtt: connection enabled for mqtt-subscriber sensors only", "broker", cfg.Mqtt.Broker)
		return hub, closed
	}
	pub := mqtt.New(log, hub, bus, screen, mqttSet, startup.BuildSensorSpecs(cfg))
	pubDone := make(chan struct{})
	go func() {
		defer close(pubDone)
		pub.Run(ctx)
	}()
	log.Info("mqtt: bridge enabled", "broker", cfg.Mqtt.Broker, "node_id", cfg.Mqtt.Bridge.NodeID)
	return hub, pubDone
}

// buildSources builds sensor sources from config (BLE adapter and Hub shared per
// type). An unbuildable source (no Bluetooth, no broker) is logged and skipped,
// never fatal, the frame shows photos regardless of sensors.
func buildSources(log *slog.Logger, cfg *config.Config, hub *mqtt.Hub) []sensors.Source {
	var sources []sensors.Source
	var bleAdapter *bleadapter.Bluetooth
	bleUnavailable := false

	for _, sensorCfg := range cfg.Sensors {
		switch sensorCfg.Type {
		case "ble":
			if bleUnavailable {
				continue
			}
			if bleAdapter == nil {
				var err error
				bleAdapter, err = bleadapter.NewWithID(cfg.BluetoothAdapter)
				if err != nil {
					log.Error("bluetooth adapter unavailable; skipping all BLE sensors",
						"adapter", cfg.BluetoothAdapter, "err", err)
					bleUnavailable = true
					continue
				}
			}
			src, err := ble.New(log, sensorCfg, bleAdapter)
			if err != nil {
				log.Error("skipping BLE sensor", "id", sensorCfg.ID, "err", err)
				continue
			}
			sources = append(sources, src)
		case "mqtt-subscriber":
			if hub == nil {
				log.Error("skipping mqtt-subscriber sensor: mqtt broker not configured", "id", sensorCfg.ID)
				continue
			}
			src, err := mqttsubscriber.New(log, sensorCfg, hub)
			if err != nil {
				log.Error("skipping mqtt-subscriber sensor", "id", sensorCfg.ID, "err", err)
				continue
			}
			sources = append(sources, src)
			log.Info("mqtt-subscriber configured", "id", sensorCfg.ID, "topic", sensorCfg.Topic, "kind", sensorCfg.Kind)
		case "mock":
			interval := sensorCfg.PollInterval.Duration
			if interval == 0 {
				interval = 5 * time.Second
			}
			readings := make([]mocksensor.Reading, 0, len(sensorCfg.MockReadings))
			for _, r := range sensorCfg.MockReadings {
				readings = append(readings, mocksensor.Reading{
					Kind:  sensors.Kind(r.Kind),
					Value: r.Value,
					Delta: r.Delta,
				})
			}
			sources = append(sources, mocksensor.New(sensorCfg.ID, interval, readings...))
			log.Info("mock sensor configured", "id", sensorCfg.ID, "readings", len(readings), "interval", interval)
		}
	}
	return sources
}
