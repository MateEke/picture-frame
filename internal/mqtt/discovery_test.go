package mqtt

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func testSettings() Settings {
	return Settings{
		NodeID:          "picture_frame",
		BaseTopic:       "picture-frame",
		DiscoveryPrefix: "homeassistant",
		StaleAfter:      10 * time.Minute,
	}
}

func testSpecs() []SensorSpec {
	return []SensorSpec{{
		ID:    "living_room",
		Role:  "inside",
		Kinds: []string{"temperature", "humidity", "motion"},
	}}
}

func findMessage(t *testing.T, msgs []message, topic string) message {
	t.Helper()
	for _, m := range msgs {
		if m.topic == topic {
			return m
		}
	}
	t.Fatalf("no message for topic %q", topic)
	return message{}
}

func decodeConfig(t *testing.T, m message) discoveryConfig {
	t.Helper()
	var c discoveryConfig
	if err := json.Unmarshal(m.payload, &c); err != nil {
		t.Fatalf("unmarshal %s: %v", m.topic, err)
	}
	return c
}

func TestDiscoveryMessageCount(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	// 3 sensor entities + 1 switch + 1 screen-power + 3 always-on host metrics.
	if len(msgs) != 8 {
		t.Fatalf("got %d messages, want 8", len(msgs))
	}
	for _, m := range msgs {
		if m.qos != 1 || !m.retain {
			t.Errorf("%s: discovery must be qos1 retained, got qos=%d retain=%v", m.topic, m.qos, m.retain)
		}
	}
}

func TestSensorDiscoveryTemperature(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	m := findMessage(t, msgs, "homeassistant/sensor/picture_frame/living_room_temperature/config")
	c := decodeConfig(t, m)

	if c.UniqueID != "picture_frame_living_room_temperature" {
		t.Errorf("unique_id: %q", c.UniqueID)
	}
	if c.Name != "Inside Temperature" {
		t.Errorf("name: %q", c.Name)
	}
	if c.StateTopic != "picture-frame/sensor/living_room/temperature" {
		t.Errorf("state_topic: %q", c.StateTopic)
	}
	if c.DeviceClass != "temperature" || c.StateClass != "measurement" || c.UnitOfMeasurement != "°C" {
		t.Errorf("class/unit: %+v", c)
	}
	if c.PayloadOn != "" || c.PayloadOff != "" {
		t.Errorf("sensor must not carry payload_on/off: %+v", c)
	}
	if c.AvailabilityMode != "all" || len(c.Availability) != 2 {
		t.Fatalf("availability: mode=%q topics=%v", c.AvailabilityMode, c.Availability)
	}
	if c.Availability[0].Topic != "picture-frame/availability" ||
		c.Availability[1].Topic != "picture-frame/sensor/living_room/availability" {
		t.Errorf("availability topics: %v", c.Availability)
	}
	if len(c.Device.Identifiers) != 1 || c.Device.Identifiers[0] != "picture_frame" {
		t.Errorf("device identifiers: %v", c.Device.Identifiers)
	}
	if c.Device.Name != "Picture Frame" {
		t.Errorf("device name: %q", c.Device.Name)
	}
}

func TestSensorDiscoveryHumidityUnit(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	c := decodeConfig(t, findMessage(t, msgs, "homeassistant/sensor/picture_frame/living_room_humidity/config"))
	if c.UnitOfMeasurement != "%" || c.DeviceClass != "humidity" {
		t.Errorf("humidity: unit=%q class=%q", c.UnitOfMeasurement, c.DeviceClass)
	}
}

func TestMotionDiscoveryIsBinarySensor(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	m := findMessage(t, msgs, "homeassistant/binary_sensor/picture_frame/living_room_motion/config")
	c := decodeConfig(t, m)
	if c.PayloadOn != "ON" || c.PayloadOff != "OFF" {
		t.Errorf("motion payloads: on=%q off=%q", c.PayloadOn, c.PayloadOff)
	}
	if c.StateClass != "" || c.UnitOfMeasurement != "" {
		t.Errorf("motion must not have state_class/unit: %+v", c)
	}
	if c.DeviceClass != "motion" {
		t.Errorf("device_class: %q", c.DeviceClass)
	}
}

func TestSwitchDiscovery(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	m := findMessage(t, msgs, "homeassistant/switch/picture_frame/screen/config")
	c := decodeConfig(t, m)
	if c.UniqueID != "picture_frame_screen" || c.Name != "Screen" {
		t.Errorf("switch identity: id=%q name=%q", c.UniqueID, c.Name)
	}
	if c.StateTopic != "picture-frame/switch/screen/state" || c.CommandTopic != "picture-frame/switch/screen/set" {
		t.Errorf("switch topics: state=%q cmd=%q", c.StateTopic, c.CommandTopic)
	}
	// The switch tracks the frame itself, so only the bridge availability applies.
	if len(c.Availability) != 1 || c.Availability[0].Topic != "picture-frame/availability" {
		t.Errorf("switch availability: %v", c.Availability)
	}
	if c.PayloadOn != "ON" || c.PayloadOff != "OFF" {
		t.Errorf("switch payloads: on=%q off=%q", c.PayloadOn, c.PayloadOff)
	}
}

func TestScreenPowerDiscoveryIsBinarySensor(t *testing.T) {
	msgs := testSettings().discoveryMessages(testSpecs())
	c := decodeConfig(t, findMessage(t, msgs, "homeassistant/binary_sensor/picture_frame/screen_power/config"))
	if c.UniqueID != "picture_frame_screen_power" || c.Name != "Screen Power" {
		t.Errorf("screen-power identity: id=%q name=%q", c.UniqueID, c.Name)
	}
	if c.StateTopic != "picture-frame/binary_sensor/screen_power/state" {
		t.Errorf("state_topic: %q", c.StateTopic)
	}
	if c.DeviceClass != "running" || c.PayloadOn != "ON" || c.PayloadOff != "OFF" {
		t.Errorf("class/payloads: class=%q on=%q off=%q", c.DeviceClass, c.PayloadOn, c.PayloadOff)
	}
	if c.CommandTopic != "" {
		t.Errorf("live-power sensor must be read-only, got command_topic=%q", c.CommandTopic)
	}
	// Read-only live state tracks the frame, so only bridge availability applies.
	if len(c.Availability) != 1 || c.Availability[0].Topic != "picture-frame/availability" {
		t.Errorf("availability: %v", c.Availability)
	}
}

func TestDeviceNameOverride(t *testing.T) {
	set := testSettings()
	set.DeviceName = "Hallway Frame"
	c := decodeConfig(t, findMessage(t, set.discoveryMessages(testSpecs()),
		"homeassistant/switch/picture_frame/screen/config"))
	if c.Device.Name != "Hallway Frame" {
		t.Errorf("device name override: %q", c.Device.Name)
	}
}

func TestEntityNameFallsBackToID(t *testing.T) {
	specs := []SensorSpec{{ID: "balcony", Kinds: []string{"temperature"}}}
	c := decodeConfig(t, findMessage(t, testSettings().discoveryMessages(specs),
		"homeassistant/sensor/picture_frame/balcony_temperature/config"))
	if c.Name != "Balcony Temperature" {
		t.Errorf("name should fall back to id: %q", c.Name)
	}
}

func TestBridgeAvailabilityTopic(t *testing.T) {
	if got := testSettings().BridgeAvailabilityTopic(); got != "picture-frame/availability" {
		t.Errorf("BridgeAvailabilityTopic() = %q", got)
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"picture_frame":        "Picture Frame",
		"inside temperature":   "Inside Temperature",
		"living_room humidity": "Living Room Humidity",
		"a":                    "A",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceBlockCarriesMetadata(t *testing.T) {
	s := testSettings()
	s.Device = DeviceMeta{
		Manufacturer:     "Raspberry Pi Ltd",
		Model:            "Raspberry Pi Zero 2 W Rev 1.0",
		HwVersion:        "Rev 1.0",
		SwVersion:        "1.4.2",
		ConfigurationURL: "http://10.0.2.21:8080",
	}
	msgs := s.discoveryMessages(testSpecs())
	cfg := decodeConfig(t, findMessage(t, msgs, s.discoveryTopic("switch", "screen")))
	if cfg.Device.Manufacturer != "Raspberry Pi Ltd" || cfg.Device.Model != "Raspberry Pi Zero 2 W Rev 1.0" {
		t.Errorf("device metadata missing: %+v", cfg.Device)
	}
	if cfg.Device.HwVersion != "Rev 1.0" || cfg.Device.SwVersion != "1.4.2" {
		t.Errorf("hw/sw version missing: %+v", cfg.Device)
	}
	if cfg.Device.ConfigurationURL != "http://10.0.2.21:8080" {
		t.Errorf("configuration_url missing: %+v", cfg.Device)
	}
}

func TestDeviceBlockOmitsEmptyMetadata(t *testing.T) {
	s := testSettings() // no Device set
	raw := findMessage(t, s.discoveryMessages(testSpecs()), s.discoveryTopic("switch", "screen")).payload
	if bytes.Contains(raw, []byte("manufacturer")) || bytes.Contains(raw, []byte("configuration_url")) {
		t.Errorf("empty metadata must be omitted from JSON: %s", raw)
	}
}

func TestHostMetricDiscovery(t *testing.T) {
	s := testSettings()
	s.Undervoltage = true
	msgs := s.discoveryMessages(testSpecs())

	temp := decodeConfig(t, findMessage(t, msgs, s.discoveryTopic("sensor", "cpu_temperature")))
	if temp.DeviceClass != "temperature" || temp.UnitOfMeasurement != "°C" || temp.EntityCategory != "diagnostic" {
		t.Errorf("cpu_temperature config wrong: %+v", temp)
	}
	if temp.StateTopic != "picture-frame/sensor/cpu_temperature/state" {
		t.Errorf("cpu_temperature state_topic: %q", temp.StateTopic)
	}
	mem := decodeConfig(t, findMessage(t, msgs, s.discoveryTopic("sensor", "memory_usage")))
	if mem.UnitOfMeasurement != "%" || mem.StateClass != "measurement" || mem.EntityCategory != "diagnostic" {
		t.Errorf("memory_usage config wrong: %+v", mem)
	}
	up := decodeConfig(t, findMessage(t, msgs, s.discoveryTopic("sensor", "uptime")))
	if up.DeviceClass != "timestamp" || up.StateClass != "" {
		t.Errorf("uptime config wrong: %+v", up)
	}
	uv := decodeConfig(t, findMessage(t, msgs, s.discoveryTopic("binary_sensor", "undervoltage")))
	if uv.DeviceClass != "problem" || uv.PayloadOn != payloadOn || uv.PayloadOff != payloadOff {
		t.Errorf("undervoltage config wrong: %+v", uv)
	}
	// Host metrics gate on bridge availability only (no per-sensor freshness topic).
	if len(uv.Availability) != 1 || uv.Availability[0].Topic != s.bridgeAvailTopic() {
		t.Errorf("undervoltage availability wrong: %+v", uv.Availability)
	}
}

func TestUndervoltageAdvertisedOnlyWhenAvailable(t *testing.T) {
	off := testSettings().discoveryMessages(testSpecs())
	if len(off) != 8 {
		t.Fatalf("without undervoltage: got %d messages, want 8", len(off))
	}
	s := testSettings()
	s.Undervoltage = true
	if on := s.discoveryMessages(testSpecs()); len(on) != 9 {
		t.Fatalf("with undervoltage: got %d messages, want 9", len(on))
	}
	for _, m := range off {
		if m.topic == testSettings().discoveryTopic("binary_sensor", "undervoltage") {
			t.Fatal("undervoltage must not be advertised when throttle is unavailable")
		}
	}
}
