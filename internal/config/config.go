package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type WiFiConfig struct {
	APTimeoutMinutes    int    `toml:"ap_timeout_minutes"`
	ScanIntervalMinutes int    `toml:"scan_interval_minutes"`
	APSSID              string `toml:"ap_ssid"`
	APPassword          string `toml:"ap_password"`
}

// AuthConfig holds the admin-UI credential; empty PasswordHash disables the gate
// (opt-in). PasswordHash is bcrypt, never plaintext.
type AuthConfig struct {
	PasswordHash string `toml:"password_hash"`
}

// Config holds all runtime configuration for the application.
type Config struct {
	Addr             string          `toml:"addr"`
	LogLevel         string          `toml:"log_level"`         // "debug", "info", "warn", "error"; default "info"
	BluetoothAdapter string          `toml:"bluetooth_adapter"` // HCI device ID, e.g. "hci0" or "hci1"
	Display          DisplayConfig   `toml:"display"`
	Slideshow        SlideshowConfig `toml:"slideshow"`
	Library          LibraryConfig   `toml:"library"`
	Sensors          []SensorConfig  `toml:"sensor"`
	Weather          WeatherConfig   `toml:"weather"`
	Mqtt             MqttConfig      `toml:"mqtt"`
	WiFi             WiFiConfig      `toml:"wifi"`
	Auth             AuthConfig      `toml:"auth"`
	Updater          UpdaterConfig   `toml:"updater"`
}

// UpdaterConfig controls the in-app updater. Checking always runs; AutoUpdate
// gates only the nightly auto-apply at UpdateHour (device-local time).
type UpdaterConfig struct {
	AutoUpdate  bool   `toml:"auto_update"`
	UpdateHour  int    `toml:"update_hour"`  // local hour 0–23 for the scheduled check+apply
	GithubRepo  string `toml:"github_repo"`  // override release source for forks; empty = built-in default
	GithubToken string `toml:"github_token"` // optional auth for a private source; PF_GITHUB_TOKEN env is the fallback
}

// Known library backend names. Use these instead of string literals when
// branching on the active backend.
const (
	BackendFS     = "fs"
	BackendImmich = "immich"
)

// LibraryConfig selects the image library backend. "fs" reads local uploads
// from Slideshow.ImagesDir; "immich" syncs from a shared Immich album into
// Slideshow.ImagesDir/immich/.
type LibraryConfig struct {
	Backend string              `toml:"backend"` // BackendFS (default) | BackendImmich
	Immich  ImmichLibraryConfig `toml:"immich"`
}

// ImmichLibraryConfig configures the Immich shared-link backend.
type ImmichLibraryConfig struct {
	ShareURL      string   `toml:"share_url"`
	SharePassword string   `toml:"share_password"`
	SyncInterval  Duration `toml:"sync_interval"`
}

// MqttConfig is the broker connection shared by bridge and subscriber sources.
type MqttConfig struct {
	Broker   string           `toml:"broker"` // e.g. "tcp://192.168.1.10:1883"
	Username string           `toml:"username"`
	Password string           `toml:"password"`
	ClientID string           `toml:"client_id"`
	Bridge   MqttBridgeConfig `toml:"bridge"`
}

// MqttBridgeConfig is the outbound Home Assistant bridge.
type MqttBridgeConfig struct {
	Enabled bool `toml:"enabled"`
	// NodeID identifies this frame: HA device id, unique_id prefix, and discovery
	// group. Distinct per frame; no hardcoded MACs.
	NodeID string `toml:"node_id"`
	// BaseTopic namespaces all state, availability, and command topics.
	BaseTopic string `toml:"base_topic"`
	// DiscoveryPrefix is the HA discovery prefix (config topics only).
	DiscoveryPrefix string `toml:"discovery_prefix"`
	// StaleAfter: report a sensor offline in HA if no reading arrives within it.
	StaleAfter Duration `toml:"stale_after"`
}

// DefaultPairThreshold is the split-screen aspect-deviation factor used when none
// is configured. A value <= 1 would classify every image as an outlier.
const DefaultPairThreshold = 1.5

type SlideshowConfig struct {
	Interval  Duration `toml:"interval"`
	Randomize bool     `toml:"randomize"`
	ImagesDir string   `toml:"images_dir"`
	// SplitScreen pairs same-orientation outliers instead of cropping; PairThreshold
	// is the aspect deviation factor (>1) at which a photo counts as an outlier.
	SplitScreen   bool    `toml:"split_screen"`
	PairThreshold float64 `toml:"pair_threshold"`
}

// Known display backend names.
const (
	DisplayBackendWlopm    = "wlopm"    // full KMS DPMS via the Wayland compositor (default)
	DisplayBackendVcgencmd = "vcgencmd" // legacy fkms firmware path
)

type DisplayConfig struct {
	BlankAfter Duration `toml:"blank_after"`
	// Backend: DisplayBackendWlopm (default) or DisplayBackendVcgencmd.
	Backend string `toml:"backend"`
	// Output is the wlopm connector, e.g. "HDMI-A-1" (run `wlopm` to list);
	// required for wlopm, ignored by vcgencmd.
	Output string `toml:"output"`
	// Locale is the BCP-47 tag the kiosk formats the clock and date with; default "en-US".
	Locale string `toml:"locale"`
	// Labels are the owner-provided captions under the kiosk readings;
	// an empty string hides that caption.
	Labels KioskLabelsConfig `toml:"labels"`
}

// KioskLabelsConfig holds free-text captions, owner wording, not translations.
type KioskLabelsConfig struct {
	Outside  string `toml:"outside"`
	Inside   string `toml:"inside"`
	Humidity string `toml:"humidity"`
}

// SensorConfig describes a single sensor source.
// Fields used depend on Type: see each backend's documentation.
type SensorConfig struct {
	ID   string `toml:"id"`
	Type string `toml:"type"` // "ble" | "mqtt-subscriber" | "mock"
	// Role tags this sensor's readings (e.g. "inside"); the kiosk indexes by role,
	// not device ID, so any sensor type can fill any display position.
	Role string `toml:"role"`

	// BLE
	MAC             string                 `toml:"mac"`
	AddressType     string                 `toml:"address_type"` // "random" | "public"
	Characteristics []CharacteristicConfig `toml:"characteristic"`
	PollInterval    Duration               `toml:"poll_interval"` // default 80s
	ResetAfter      Duration               `toml:"reset_after"`   // 0 disables adapter power-cycling (default)

	// MQTT-subscriber
	Topic  string `toml:"topic"`
	Kind   string `toml:"kind"`
	Parser string `toml:"parser"`
	// JSONField extracts a value by dotted path (e.g. "main.temp"); empty = raw payload.
	JSONField string `toml:"json_field"`

	// Mock
	MockReadings []MockReadingConfig `toml:"mock_reading"`
}

type MockReadingConfig struct {
	Kind  string  `toml:"kind"`
	Value float64 `toml:"value"`
	Delta float64 `toml:"delta"` // added to value after every full cycle; 0 means constant
}

type CharacteristicConfig struct {
	UUID    string `toml:"uuid"`
	Kind    string `toml:"kind"`    // "temperature" | "humidity" | "motion"
	Decoder string `toml:"decoder"` // decoder registry name
}

type WeatherConfig struct {
	APIKey       string   `toml:"api_key"`
	Lat          float64  `toml:"lat"`
	Lon          float64  `toml:"lon"`
	PollInterval Duration `toml:"poll_interval"`
	// First retry delay after a failed poll; doubles up to PollInterval (0 = none).
	RetryInterval Duration `toml:"retry_interval"`
	Units         string   `toml:"units"` // "standard" | "metric" | "imperial"; default "metric"
}

// Duration is a time.Duration that marshals to/from a TOML string (e.g. "20m").
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(b []byte) error {
	dur, err := time.ParseDuration(string(b))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(b), err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// defaults returns the baseline Config applied before any file is loaded.
func defaults() Config {
	return Config{
		Addr:             ":8080",
		BluetoothAdapter: "hci0",
		Display: DisplayConfig{
			BlankAfter: Duration{20 * time.Minute},
			Backend:    DisplayBackendWlopm,
			Output:     "HDMI-A-1",
			Locale:     "en-US",
		},
		Slideshow: SlideshowConfig{
			Interval:      Duration{120 * time.Second},
			ImagesDir:     "images",
			Randomize:     false,
			SplitScreen:   true,
			PairThreshold: DefaultPairThreshold,
		},
		Library: LibraryConfig{
			Backend: BackendFS,
			Immich:  ImmichLibraryConfig{SyncInterval: Duration{15 * time.Minute}},
		},
		Weather: WeatherConfig{
			PollInterval:  Duration{10 * time.Minute},
			RetryInterval: Duration{30 * time.Second},
			Units:         "metric",
		},
		Mqtt: MqttConfig{
			ClientID: "picture-frame",
			Bridge: MqttBridgeConfig{
				NodeID:          "picture_frame",
				BaseTopic:       "picture-frame",
				DiscoveryPrefix: "homeassistant",
				StaleAfter:      Duration{10 * time.Minute},
			},
		},
		WiFi: WiFiConfig{
			APTimeoutMinutes:    3,
			ScanIntervalMinutes: 5,
			APSSID:              "PictureFrame",
		},
		// AutoUpdate on by default; the SameMajor gate keeps it to minor/patch.
		Updater: UpdaterConfig{AutoUpdate: true, UpdateHour: 2},
	}
}

// Load reads userPath then merges overridesPath on top. A missing file is
// skipped; any other read or parse error is returned.
func Load(userPath, overridesPath string) (*Config, error) {
	cfg := defaults()
	if err := loadFile(userPath, &cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", userPath, err)
	}
	if err := loadFile(overridesPath, &cfg); err != nil {
		return nil, fmt.Errorf("overrides %s: %w", overridesPath, err)
	}
	return &cfg, nil
}

func loadFile(path string, dst *Config) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, dst)
}
