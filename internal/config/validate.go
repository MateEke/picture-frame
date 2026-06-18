package config

import "fmt"

// Validate checks structural invariants; backends validate external references
// (e.g. decoder names) at startup.
func (c *Config) Validate() error {
	seenID := map[string]bool{}
	// A reused overlay key means two sensors would clobber each other on the kiosk.
	keyOwner := map[string]string{}
	hasSubscriber := false
	for _, sensor := range c.Sensors {
		if sensor.ID == "" {
			return fmt.Errorf("sensor missing id")
		}
		if seenID[sensor.ID] {
			return fmt.Errorf("duplicate sensor id %q", sensor.ID)
		}
		seenID[sensor.ID] = true
		if err := sensor.validate(); err != nil {
			return fmt.Errorf("sensor %q: %w", sensor.ID, err)
		}
		if sensor.Type == "mqtt-subscriber" {
			hasSubscriber = true
		}
		roleKey := sensor.roleKey()
		for _, kind := range sensor.kinds() {
			key := overlayKey(roleKey, kind)
			if prior, ok := keyOwner[key]; ok {
				return fmt.Errorf(
					"sensors %q and %q both publish (role=%q, kind=%q); they would collide on the overlay",
					prior, sensor.ID, roleKey, kind)
			}
			keyOwner[key] = sensor.ID
		}
	}
	if err := c.Weather.validate(); err != nil {
		return fmt.Errorf("weather: %w", err)
	}
	if err := c.Mqtt.validate(hasSubscriber); err != nil {
		return fmt.Errorf("mqtt: %w", err)
	}
	if err := c.Library.validate(); err != nil {
		return fmt.Errorf("library: %w", err)
	}
	if err := c.Display.validate(); err != nil {
		return fmt.Errorf("display: %w", err)
	}
	if err := c.WiFi.validate(); err != nil {
		return fmt.Errorf("wifi: %w", err)
	}
	if err := c.Updater.validate(); err != nil {
		return fmt.Errorf("updater: %w", err)
	}
	if err := c.Slideshow.validate(); err != nil {
		return fmt.Errorf("slideshow: %w", err)
	}
	return nil
}

func (s SlideshowConfig) validate() error {
	// A factor <= 1 would classify every image as an outlier, pairing everything.
	if s.SplitScreen && s.PairThreshold <= 1.0 {
		return fmt.Errorf("pair_threshold must be > 1 when split_screen is enabled, got %v", s.PairThreshold)
	}
	return nil
}

func (u UpdaterConfig) validate() error {
	if u.UpdateHour < 0 || u.UpdateHour > 23 {
		return fmt.Errorf("update_hour %d out of range 0–23", u.UpdateHour)
	}
	return nil
}

func (s SensorConfig) validate() error {
	switch s.Type {
	case "ble":
		if s.MAC == "" {
			return fmt.Errorf("ble sensor missing mac")
		}
		for i, char := range s.Characteristics {
			if char.UUID == "" {
				return fmt.Errorf("characteristic[%d] missing uuid", i)
			}
			if char.Kind == "" {
				return fmt.Errorf("characteristic[%d] missing kind", i)
			}
			if char.Decoder == "" {
				return fmt.Errorf("characteristic[%d] missing decoder", i)
			}
		}
	case "mqtt-subscriber":
		if s.Topic == "" {
			return fmt.Errorf("mqtt-subscriber missing topic")
		}
		if s.Kind == "" {
			return fmt.Errorf("mqtt-subscriber missing kind")
		}
		if s.Parser == "" {
			return fmt.Errorf("mqtt-subscriber missing parser")
		}
	case "mock":
		// no required fields
	case "":
		return fmt.Errorf("missing type")
	default:
		return fmt.Errorf("unknown type %q (valid: ble, mqtt-subscriber, mock)", s.Type)
	}
	return nil
}

func (w WiFiConfig) validate() error {
	if w.APSSID == "" {
		return nil
	}
	if w.APTimeoutMinutes <= 0 {
		return fmt.Errorf("ap_timeout_minutes must be positive when ap_ssid is set")
	}
	if w.ScanIntervalMinutes <= 0 {
		return fmt.Errorf("scan_interval_minutes must be positive when ap_ssid is set")
	}
	return nil
}

func (d DisplayConfig) validate() error {
	switch d.Backend {
	case "", DisplayBackendWlopm, DisplayBackendVcgencmd:
		return nil
	default:
		return fmt.Errorf("unknown backend %q (valid: %s, %s)", d.Backend, DisplayBackendWlopm, DisplayBackendVcgencmd)
	}
}

func (l LibraryConfig) validate() error {
	switch l.Backend {
	case "", BackendFS:
		return nil
	case BackendImmich:
		if l.Immich.ShareURL == "" {
			return fmt.Errorf("immich.share_url required when backend is immich")
		}
		return nil
	default:
		return fmt.Errorf("unknown backend %q (valid: %s, %s)", l.Backend, BackendFS, BackendImmich)
	}
}

func (m MqttConfig) validate(hasSubscriber bool) error {
	if !m.Bridge.Enabled && !hasSubscriber {
		return nil
	}
	if m.Broker == "" {
		return fmt.Errorf("broker required (bridge enabled or mqtt-subscriber sensors present)")
	}
	if !m.Bridge.Enabled {
		return nil
	}
	switch {
	case m.Bridge.NodeID == "":
		return fmt.Errorf("bridge.node_id required when bridge enabled")
	case m.Bridge.BaseTopic == "":
		return fmt.Errorf("bridge.base_topic required when bridge enabled")
	case m.Bridge.DiscoveryPrefix == "":
		return fmt.Errorf("bridge.discovery_prefix required when bridge enabled")
	default:
		return nil
	}
}

func (w WeatherConfig) validate() error {
	switch w.Units {
	case "", "standard", "metric", "imperial":
		return nil
	default:
		return fmt.Errorf("invalid units %q (valid: standard, metric, imperial)", w.Units)
	}
}
