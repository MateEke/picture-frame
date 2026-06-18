package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/config.toml", "/nonexistent/overrides.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("addr: got %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.Display.BlankAfter.Duration != 20*time.Minute {
		t.Errorf("blank_after: got %v, want 20m", cfg.Display.BlankAfter)
	}
	if cfg.Display.Backend != config.DisplayBackendWlopm {
		t.Errorf("display backend: got %q, want %q", cfg.Display.Backend, config.DisplayBackendWlopm)
	}
	if cfg.Display.Output != "HDMI-A-1" {
		t.Errorf("display output: got %q, want HDMI-A-1", cfg.Display.Output)
	}
	if cfg.Weather.Units != "metric" {
		t.Errorf("weather units: got %q, want metric", cfg.Weather.Units)
	}
	if cfg.Weather.PollInterval.Duration != 10*time.Minute {
		t.Errorf("weather poll_interval: got %v, want 10m", cfg.Weather.PollInterval)
	}
	if cfg.Weather.RetryInterval.Duration != 30*time.Second {
		t.Errorf("weather retry_interval: got %v, want 30s", cfg.Weather.RetryInterval)
	}
	if cfg.Mqtt.Bridge.Enabled {
		t.Error("mqtt bridge should default to disabled")
	}
	if cfg.Mqtt.Bridge.NodeID != "picture_frame" {
		t.Errorf("mqtt bridge.node_id: got %q, want picture_frame", cfg.Mqtt.Bridge.NodeID)
	}
	if cfg.Mqtt.Bridge.BaseTopic != "picture-frame" {
		t.Errorf("mqtt bridge.base_topic: got %q, want picture-frame", cfg.Mqtt.Bridge.BaseTopic)
	}
	if cfg.Mqtt.Bridge.DiscoveryPrefix != "homeassistant" {
		t.Errorf("mqtt bridge.discovery_prefix: got %q, want homeassistant", cfg.Mqtt.Bridge.DiscoveryPrefix)
	}
	if cfg.Mqtt.Bridge.StaleAfter.Duration != 10*time.Minute {
		t.Errorf("mqtt bridge.stale_after: got %v, want 10m", cfg.Mqtt.Bridge.StaleAfter)
	}
	if cfg.Slideshow.Interval.Duration != 120*time.Second {
		t.Errorf("slideshow interval: got %v, want 2m", cfg.Slideshow.Interval)
	}
	if !cfg.Slideshow.SplitScreen {
		t.Error("slideshow split_screen should default to true")
	}
	if cfg.Slideshow.PairThreshold != 1.5 {
		t.Errorf("slideshow pair_threshold: got %v, want 1.5", cfg.Slideshow.PairThreshold)
	}
	if cfg.Library.Immich.SyncInterval.Duration != 15*time.Minute {
		t.Errorf("immich sync_interval: got %v, want 15m", cfg.Library.Immich.SyncInterval)
	}
}

func TestLoadUserConfig(t *testing.T) {
	dir := t.TempDir()
	userPath := write(t, dir, "config.toml", `
addr = ":9090"

[display]
blank_after = "30m"
`)
	cfg, err := config.Load(userPath, filepath.Join(dir, "overrides.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("addr: got %q, want :9090", cfg.Addr)
	}
	if cfg.Display.BlankAfter.Duration != 30*time.Minute {
		t.Errorf("blank_after: got %v, want 30m", cfg.Display.BlankAfter)
	}
}

func TestLoadAuthPasswordHash(t *testing.T) {
	dir := t.TempDir()
	userPath := write(t, dir, "config.toml", `
[auth]
password_hash = "$2a$10$abcdefghijklmnopqrstuv"
`)
	cfg, err := config.Load(userPath, filepath.Join(dir, "overrides.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.PasswordHash != "$2a$10$abcdefghijklmnopqrstuv" {
		t.Errorf("auth.password_hash: got %q", cfg.Auth.PasswordHash)
	}
}

func TestLoadOverridesWin(t *testing.T) {
	dir := t.TempDir()
	userPath := write(t, dir, "config.toml", `addr = ":9090"`)
	overridesPath := write(t, dir, "overrides.toml", `addr = ":7070"`)

	cfg, err := config.Load(userPath, overridesPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != ":7070" {
		t.Errorf("overrides should win: got %q, want :7070", cfg.Addr)
	}
}

func TestLoadOverridesDoNotZeroUserFields(t *testing.T) {
	dir := t.TempDir()
	userPath := write(t, dir, "config.toml", `addr = ":9090"`)
	// overrides only sets display, not addr
	overridesPath := write(t, dir, "overrides.toml", `
[display]
blank_after = "5m"
`)
	cfg, err := config.Load(userPath, overridesPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("user addr should be preserved: got %q", cfg.Addr)
	}
	if cfg.Display.BlankAfter.Duration != 5*time.Minute {
		t.Errorf("override blank_after not applied: got %v", cfg.Display.BlankAfter)
	}
}

func TestStoreUpdateAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.toml")

	store := config.NewStore(config.Config{}, path)
	if err := store.Update(func(c *config.Config) error {
		c.Addr = ":1234"
		c.Display.BlankAfter = config.Duration{Duration: 15 * time.Minute}
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	cfg, err := config.Load("/nonexistent", path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":1234" {
		t.Errorf("addr: got %q, want :1234", cfg.Addr)
	}
	if cfg.Display.BlankAfter.Duration != 15*time.Minute {
		t.Errorf("blank_after: got %v, want 15m", cfg.Display.BlankAfter)
	}
}

func TestDurationParsing(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config.toml", `
[display]
blank_after = "1h30m"
`)
	cfg, err := config.Load(path, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Display.BlankAfter.Duration != 90*time.Minute {
		t.Errorf("got %v, want 1h30m", cfg.Display.BlankAfter)
	}
}

func TestDurationInvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config.toml", `
[display]
blank_after = "not-a-duration"
`)
	_, err := config.Load(path, "/nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestSensorRolesParsed(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "config.toml", `
[[sensor]]
id   = "s1"
type = "ble"
role = "inside"
mac  = "AA:BB:CC:DD:EE:FF"

[[sensor]]
id   = "s2"
type = "mock"

[[sensor]]
id   = "s3"
type = "mqtt-subscriber"
role = "outside"
topic = "ha/temp"
kind  = "temperature"
`)
	cfg, err := config.Load(path, "/nonexistent")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	roles := config.SensorRoles(cfg.Sensors)
	if roles["s1"] != "inside" {
		t.Errorf("s1: got %q, want inside", roles["s1"])
	}
	if roles["s2"] != "" {
		t.Errorf("s2: expected no role, got %q", roles["s2"])
	}
	if roles["s3"] != "outside" {
		t.Errorf("s3: got %q, want outside", roles["s3"])
	}
}

func TestSensorRolesEmpty(t *testing.T) {
	roles := config.SensorRoles(nil)
	if len(roles) != 0 {
		t.Fatalf("expected empty map, got %v", roles)
	}
}

func TestValidateWeatherUnits(t *testing.T) {
	cases := []struct {
		name    string
		units   string
		wantErr bool
	}{
		{"empty defaults ok", "", false},
		{"standard", "standard", false},
		{"metric", "metric", false},
		{"imperial", "imperial", false},
		{"unknown", "kelvin", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Weather: config.WeatherConfig{Units: tc.units}}
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSlideshow(t *testing.T) {
	cases := []struct {
		name    string
		split   bool
		thr     float64
		wantErr bool
	}{
		{"disabled ignores threshold", false, 0, false},
		{"enabled needs threshold above 1", true, 1.0, true},
		{"enabled threshold below 1", true, 0.5, true},
		{"enabled valid threshold", true, 1.5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Slideshow: config.SlideshowConfig{SplitScreen: tc.split, PairThreshold: tc.thr}}
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBLESensors(t *testing.T) {
	sensors := []config.SensorConfig{
		{
			ID: "living_room", Type: "ble", Role: "inside",
			Characteristics: []config.CharacteristicConfig{
				{UUID: "u1", Kind: "temperature", Decoder: "d"},
				{UUID: "u2", Kind: "humidity", Decoder: "d"},
				{UUID: "u3", Kind: "temperature", Decoder: "d"}, // duplicate kind
				{UUID: "u4", Kind: "", Decoder: "d"},            // empty kind skipped
			},
		},
		{ID: "mock1", Type: "mock", Role: "inside"},                            // non-BLE skipped
		{ID: "sub1", Type: "mqtt-subscriber", Topic: "t", Kind: "temperature"}, // non-BLE skipped
		{ID: "bare", Type: "ble", MAC: "AA:BB:CC:DD:EE:FF"},                    // no characteristics skipped
	}

	got := config.BLESensors(sensors)
	if len(got) != 1 {
		t.Fatalf("got %d BLE sensors, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.ID != "living_room" || s.Role != "inside" {
		t.Errorf("identity: id=%q role=%q", s.ID, s.Role)
	}
	want := []string{"temperature", "humidity"} // distinct, declaration order
	if len(s.Kinds) != len(want) || s.Kinds[0] != want[0] || s.Kinds[1] != want[1] {
		t.Errorf("kinds: got %v, want %v", s.Kinds, want)
	}
}

func TestBLESensorsEmpty(t *testing.T) {
	if got := config.BLESensors(nil); got != nil {
		t.Errorf("expected nil for no sensors, got %v", got)
	}
}

func TestSensorKeys(t *testing.T) {
	sensors := []config.SensorConfig{
		{
			ID: "s1", Type: "mock", Role: "inside",
			MockReadings: []config.MockReadingConfig{
				{Kind: "temperature"},
				{Kind: "humidity"},
				{Kind: "temperature"}, // duplicate collapses
			},
		},
		{ID: "s2", Type: "mqtt-subscriber", Role: "outside", Topic: "t", Kind: "temperature"},
		{ // no role → falls back to id
			ID: "s3", Type: "ble", MAC: "AA:BB:CC:DD:EE:FF",
			Characteristics: []config.CharacteristicConfig{{UUID: "u", Kind: "motion", Decoder: "bool_nonzero"}},
		},
		// Cross-sensor duplicate role:kind collapses.
		{ID: "s4", Type: "mqtt-subscriber", Role: "inside", Topic: "t2", Kind: "temperature"},
	}

	got := config.SensorKeys(sensors)
	want := []string{"inside:humidity", "inside:temperature", "outside:temperature", "s3:motion"}
	if !slices.Equal(got, want) {
		t.Errorf("SensorKeys: got %v, want %v", got, want)
	}
}

func TestSensorKeysEmpty(t *testing.T) {
	if got := config.SensorKeys(nil); got != nil {
		t.Errorf("expected nil for no sensors, got %v", got)
	}
}

func TestValidateMqtt(t *testing.T) {
	full := config.MqttConfig{
		Broker: "tcp://broker:1883",
		Bridge: config.MqttBridgeConfig{
			Enabled: true, NodeID: "n", BaseTopic: "b", DiscoveryPrefix: "homeassistant",
		},
	}
	without := func(mut func(*config.MqttConfig)) config.MqttConfig {
		m := full
		mut(&m)
		return m
	}
	cases := []struct {
		name    string
		mqtt    config.MqttConfig
		sensors []config.SensorConfig
		wantErr string
	}{
		{"bridge disabled and no subscribers skips checks", config.MqttConfig{}, nil, ""},
		{"valid bridge", full, nil, ""},
		{"bridge missing broker", without(func(m *config.MqttConfig) { m.Broker = "" }), nil, "broker required"},
		{"bridge missing node_id", without(func(m *config.MqttConfig) { m.Bridge.NodeID = "" }), nil, "node_id required"},
		{"bridge missing base_topic", without(func(m *config.MqttConfig) { m.Bridge.BaseTopic = "" }), nil, "base_topic required"},
		{"bridge missing discovery_prefix", without(func(m *config.MqttConfig) { m.Bridge.DiscoveryPrefix = "" }), nil, "discovery_prefix required"},
		{
			"subscriber without broker",
			config.MqttConfig{},
			[]config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "ha/t", Kind: "temperature", Parser: "raw_float"}},
			"broker required",
		},
		{
			"subscriber with broker (no bridge)",
			config.MqttConfig{Broker: "tcp://broker:1883"},
			[]config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "ha/t", Kind: "temperature", Parser: "raw_float"}},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Mqtt: tc.mqtt, Sensors: tc.sensors}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateLibrary(t *testing.T) {
	cases := []struct {
		name    string
		lib     config.LibraryConfig
		wantErr string
	}{
		{"empty defaults to fs", config.LibraryConfig{}, ""},
		{"fs", config.LibraryConfig{Backend: "fs"}, ""},
		{
			"immich with share_url",
			config.LibraryConfig{Backend: "immich", Immich: config.ImmichLibraryConfig{ShareURL: "https://host/share/x"}},
			"",
		},
		{"immich missing share_url", config.LibraryConfig{Backend: "immich"}, "share_url required"},
		{"unknown backend", config.LibraryConfig{Backend: "icloud"}, "unknown backend"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Library: tc.lib}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestWiFiDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/config.toml", "/nonexistent/overrides.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WiFi.APTimeoutMinutes != 3 {
		t.Errorf("ap_timeout_minutes: got %d, want 3", cfg.WiFi.APTimeoutMinutes)
	}
	if cfg.WiFi.ScanIntervalMinutes != 5 {
		t.Errorf("scan_interval_minutes: got %d, want 5", cfg.WiFi.ScanIntervalMinutes)
	}
	if cfg.WiFi.APSSID != "PictureFrame" {
		t.Errorf("ap_ssid: got %q, want PictureFrame", cfg.WiFi.APSSID)
	}
	if cfg.WiFi.APPassword != "" {
		t.Errorf("ap_password: got %q, want empty", cfg.WiFi.APPassword)
	}
}

func TestValidateWiFi(t *testing.T) {
	cases := []struct {
		name    string
		wifi    config.WiFiConfig
		wantErr string
	}{
		{"empty ssid (dormant) skips interval checks", config.WiFiConfig{}, ""},
		{"valid active config", config.WiFiConfig{APSSID: "PF", APTimeoutMinutes: 1, ScanIntervalMinutes: 1}, ""},
		{
			"zero timeout with ssid",
			config.WiFiConfig{APSSID: "PF", APTimeoutMinutes: 0, ScanIntervalMinutes: 5},
			"ap_timeout_minutes must be positive",
		},
		{
			"zero scan interval with ssid",
			config.WiFiConfig{APSSID: "PF", APTimeoutMinutes: 3, ScanIntervalMinutes: 0},
			"scan_interval_minutes must be positive",
		},
		{
			"negative timeout",
			config.WiFiConfig{APSSID: "PF", APTimeoutMinutes: -1, ScanIntervalMinutes: 5},
			"ap_timeout_minutes must be positive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{WiFi: tc.wifi}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestStoreUpdateWiFiPreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overrides.toml")

	initial := config.Config{
		Weather: config.WeatherConfig{APIKey: "abc123", Units: "metric"},
		WiFi:    config.WiFiConfig{APSSID: "OldName", APTimeoutMinutes: 10},
	}
	store := config.NewStore(initial, path)

	if err := store.Update(func(c *config.Config) error {
		c.WiFi = config.WiFiConfig{APSSID: "NewFrame", APTimeoutMinutes: 3, ScanIntervalMinutes: 5, APPassword: "secret"}
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	cfg, err := config.Load("/nonexistent", path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Weather.APIKey != "abc123" {
		t.Errorf("weather.api_key: got %q, want abc123", cfg.Weather.APIKey)
	}
	if cfg.WiFi.APSSID != "NewFrame" {
		t.Errorf("wifi.ap_ssid: got %q, want NewFrame", cfg.WiFi.APSSID)
	}
	if cfg.WiFi.APPassword != "secret" {
		t.Errorf("wifi.ap_password: got %q, want secret", cfg.WiFi.APPassword)
	}
}

func TestStoreUpdateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	store := config.NewStore(config.Config{}, path)
	if err := store.Update(func(c *config.Config) error {
		c.WiFi.APSSID = "PF"
		c.WiFi.APTimeoutMinutes = 3
		c.WiFi.ScanIntervalMinutes = 5
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	cfg, err := config.Load("/nonexistent", path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.WiFi.APSSID != "PF" {
		t.Errorf("ap_ssid: got %q, want PF", cfg.WiFi.APSSID)
	}
}

func TestValidateDisplay(t *testing.T) {
	cases := []struct {
		name    string
		display config.DisplayConfig
		wantErr string
	}{
		{"empty defaults ok", config.DisplayConfig{}, ""},
		{"wlopm", config.DisplayConfig{Backend: config.DisplayBackendWlopm, Output: "HDMI-A-1"}, ""},
		{"vcgencmd", config.DisplayConfig{Backend: config.DisplayBackendVcgencmd}, ""},
		{"unknown backend", config.DisplayConfig{Backend: "x11"}, "unknown backend"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Display: tc.display}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestUpdaterConfigDefaultsAndParse(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load("/nonexistent", filepath.Join(dir, "o.toml"))
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if !cfg.Updater.AutoUpdate {
		t.Errorf("auto_update default: got false, want true")
	}
	if cfg.Updater.UpdateHour != 2 {
		t.Errorf("update_hour default: got %d, want 2", cfg.Updater.UpdateHour)
	}

	userPath := write(t, dir, "config.toml", `
[updater]
auto_update = false
update_hour = 3
github_repo = "me/fork"
`)
	cfg, err = config.Load(userPath, filepath.Join(dir, "o.toml"))
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	if cfg.Updater.AutoUpdate {
		t.Errorf("auto_update: got true, want false (user override)")
	}
	if cfg.Updater.UpdateHour != 3 {
		t.Errorf("update_hour: got %d, want 3", cfg.Updater.UpdateHour)
	}
	if cfg.Updater.GithubRepo != "me/fork" {
		t.Errorf("github_repo: got %q, want me/fork", cfg.Updater.GithubRepo)
	}
}

func TestUpdaterConfigRejectsBadHour(t *testing.T) {
	cases := []struct {
		name string
		hour int
		ok   bool
	}{
		{"low bound", 0, true},
		{"high bound", 23, true},
		{"negative", -1, false},
		{"too big", 24, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Updater: config.UpdaterConfig{UpdateHour: tc.hour}}
			err := cfg.Validate()
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "update_hour")) {
				t.Errorf("err = %v, want update_hour error", err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	withBroker := config.MqttConfig{Broker: "tcp://broker:1883"}
	cases := []struct {
		name    string
		sensors []config.SensorConfig
		mqtt    config.MqttConfig
		wantErr string
	}{
		{
			name: "valid ble",
			sensors: []config.SensorConfig{{
				ID: "s1", Type: "ble", MAC: "AA:BB:CC:DD:EE:FF",
				Characteristics: []config.CharacteristicConfig{
					{UUID: "uuid-1", Kind: "temperature", Decoder: "int16le_div100"},
				},
			}},
		},
		{
			name: "valid mqtt-subscriber",
			sensors: []config.SensorConfig{{
				ID: "s2", Type: "mqtt-subscriber",
				Topic: "ha/sensor/temp/state", Kind: "temperature", Parser: "raw_float",
			}},
			mqtt: withBroker,
		},
		{
			name:    "valid mock",
			sensors: []config.SensorConfig{{ID: "s3", Type: "mock"}},
		},
		{
			name:    "missing id",
			sensors: []config.SensorConfig{{Type: "mock"}},
			wantErr: "missing id",
		},
		{
			name: "duplicate id",
			sensors: []config.SensorConfig{
				{ID: "dup", Type: "mock"},
				{ID: "dup", Type: "mock"},
			},
			wantErr: "duplicate sensor id",
		},
		{
			name:    "missing type",
			sensors: []config.SensorConfig{{ID: "s"}},
			wantErr: "missing type",
		},
		{
			name:    "unknown type",
			sensors: []config.SensorConfig{{ID: "s", Type: "zigbee"}},
			wantErr: "unknown type",
		},
		{
			name:    "ble missing mac",
			sensors: []config.SensorConfig{{ID: "s", Type: "ble"}},
			wantErr: "missing mac",
		},
		{
			name: "ble characteristic missing uuid",
			sensors: []config.SensorConfig{{
				ID: "s", Type: "ble", MAC: "AA:BB:CC:DD:EE:FF",
				Characteristics: []config.CharacteristicConfig{{Kind: "temperature", Decoder: "int16le_div100"}},
			}},
			wantErr: "missing uuid",
		},
		{
			name:    "mqtt missing topic",
			sensors: []config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Kind: "temperature"}},
			wantErr: "missing topic",
		},
		{
			name:    "mqtt missing kind",
			sensors: []config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "ha/temp", Parser: "raw_float"}},
			wantErr: "missing kind",
		},
		{
			name:    "mqtt missing parser",
			sensors: []config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "ha/temp", Kind: "temperature"}},
			wantErr: "missing parser",
		},
		{
			name: "role+kind collision across types",
			sensors: []config.SensorConfig{
				{
					ID: "ble1", Type: "ble", Role: "inside", MAC: "AA:BB:CC:DD:EE:FF",
					Characteristics: []config.CharacteristicConfig{
						{UUID: "u", Kind: "temperature", Decoder: "int16le_div100"},
					},
				},
				{ID: "sub1", Type: "mqtt-subscriber", Role: "inside", Topic: "ha/t", Kind: "temperature", Parser: "raw_float"},
			},
			wantErr: `both publish (role="inside", kind="temperature")`,
		},
		{
			name: "role+kind distinct kinds same role ok",
			sensors: []config.SensorConfig{
				{
					ID: "ble1", Type: "ble", Role: "inside", MAC: "AA:BB:CC:DD:EE:FF",
					Characteristics: []config.CharacteristicConfig{
						{UUID: "u", Kind: "temperature", Decoder: "int16le_div100"},
					},
				},
				{ID: "sub1", Type: "mqtt-subscriber", Role: "inside", Topic: "ha/h", Kind: "humidity", Parser: "raw_float"},
			},
			mqtt: withBroker,
		},
		{
			name: "role+kind distinct roles same kind ok",
			sensors: []config.SensorConfig{
				{
					ID: "ble1", Type: "ble", Role: "inside", MAC: "AA:BB:CC:DD:EE:FF",
					Characteristics: []config.CharacteristicConfig{
						{UUID: "u", Kind: "temperature", Decoder: "int16le_div100"},
					},
				},
				{ID: "sub1", Type: "mqtt-subscriber", Role: "outside", Topic: "ha/t", Kind: "temperature", Parser: "raw_float"},
			},
			mqtt: withBroker,
		},
		{
			name: "role+kind missing role falls back to id",
			sensors: []config.SensorConfig{
				{ID: "a", Type: "mock", MockReadings: []config.MockReadingConfig{{Kind: "temperature", Value: 1}}},
				{ID: "b", Type: "mock", MockReadings: []config.MockReadingConfig{{Kind: "temperature", Value: 2}}},
			},
		},
		{
			name: "role+kind mock vs ble collision",
			sensors: []config.SensorConfig{
				{
					ID: "ble1", Type: "ble", Role: "inside", MAC: "AA:BB:CC:DD:EE:FF",
					Characteristics: []config.CharacteristicConfig{
						{UUID: "u", Kind: "motion", Decoder: "bool_nonzero"},
					},
				},
				{
					ID: "mock1", Type: "mock", Role: "inside",
					MockReadings: []config.MockReadingConfig{{Kind: "motion", Value: 1}},
				},
			},
			wantErr: `both publish (role="inside", kind="motion")`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Sensors: tc.sensors, Mqtt: tc.mqtt}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestHasMotionSensor(t *testing.T) {
	cases := []struct {
		name    string
		sensors []config.SensorConfig
		want    bool
	}{
		{"none", nil, false},
		{
			"ble motion characteristic",
			[]config.SensorConfig{{ID: "s", Type: "ble", Characteristics: []config.CharacteristicConfig{
				{UUID: "u", Kind: "motion", Decoder: "d"},
			}}},
			true,
		},
		{
			"mqtt-subscriber motion",
			[]config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "t", Kind: "motion"}},
			true,
		},
		{
			"mock motion reading",
			[]config.SensorConfig{{ID: "s", Type: "mock", MockReadings: []config.MockReadingConfig{{Kind: "motion"}}}},
			true,
		},
		{
			"only temperature",
			[]config.SensorConfig{{ID: "s", Type: "mqtt-subscriber", Topic: "t", Kind: "temperature"}},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.HasMotionSensor(tc.sensors); got != tc.want {
				t.Errorf("HasMotionSensor = %v, want %v", got, tc.want)
			}
		})
	}
}
