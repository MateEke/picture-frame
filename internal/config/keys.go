package config

import "slices"

// roleKey is the identity the kiosk overlay groups a sensor's readings under:
// its role, or its ID when no role is set.
func (s SensorConfig) roleKey() string {
	if s.Role != "" {
		return s.Role
	}
	return s.ID
}

// overlayKey is the "role:kind" key the kiosk overlay indexes a single reading
// by. Two sensors yielding the same key would collide on the overlay; Validate
// rejects that.
func overlayKey(roleKey, kind string) string {
	return roleKey + ":" + kind
}

// SensorRoles maps sensor ID → role for sensors with a role set, so bus events
// can be tagged by role instead of device ID.
func SensorRoles(sensors []SensorConfig) map[string]string {
	roles := make(map[string]string, len(sensors))
	for _, s := range sensors {
		if s.Role != "" {
			roles[s.ID] = s.Role
		}
	}
	return roles
}

// BLESensor is the identity and distinct reading kinds of one BLE sensor.
type BLESensor struct {
	ID    string
	Role  string
	Kinds []string // distinct, non-empty characteristic kinds, in declaration order
}

// BLESensors returns each BLE sensor with the distinct kinds it reports; non-BLE
// and characteristic-less sensors are skipped. Only BLE is bridged to MQTT.
func BLESensors(sensors []SensorConfig) []BLESensor {
	var out []BLESensor
	for _, sensor := range sensors {
		if sensor.Type != "ble" {
			continue
		}
		kinds := distinctKinds(sensor.Characteristics)
		if len(kinds) == 0 {
			continue
		}
		out = append(out, BLESensor{ID: sensor.ID, Role: sensor.Role, Kinds: kinds})
	}
	return out
}

func distinctKinds(chars []CharacteristicConfig) []string {
	seen := make(map[string]bool)
	var kinds []string
	for _, char := range chars {
		if char.Kind == "" || seen[char.Kind] {
			continue
		}
		seen[char.Kind] = true
		kinds = append(kinds, char.Kind)
	}
	return kinds
}

// SensorKeys returns the sorted, distinct overlay keys the kiosk indexes
// readings by (see overlayKey).
func SensorKeys(sensors []SensorConfig) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, sensor := range sensors {
		for _, kind := range sensor.kinds() {
			key := overlayKey(sensor.roleKey(), kind)
			if seen[key] {
				continue
			}
			seen[key] = true
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func (s SensorConfig) kinds() []string {
	switch s.Type {
	case "ble":
		return distinctKinds(s.Characteristics)
	case "mqtt-subscriber":
		if s.Kind == "" {
			return nil
		}
		return []string{s.Kind}
	case "mock":
		seen := make(map[string]bool)
		var kinds []string
		for _, reading := range s.MockReadings {
			if reading.Kind == "" || seen[reading.Kind] {
				continue
			}
			seen[reading.Kind] = true
			kinds = append(kinds, reading.Kind)
		}
		return kinds
	}
	return nil
}

// HasMotionSensor reports whether any configured sensor publishes motion readings.
func HasMotionSensor(sensors []SensorConfig) bool {
	for _, sensor := range sensors {
		if slices.Contains(sensor.kinds(), "motion") {
			return true
		}
	}
	return false
}
