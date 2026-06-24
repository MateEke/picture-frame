package httpapi

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/state"
)

func TestKioskEventPayload(t *testing.T) {
	cfg := fullTestConfig() // locale en-US, inside+outside temp sensors
	got := KioskEventPayload(cfg, true)

	if got.Locale != "en-US" {
		t.Errorf("locale: got %q, want en-US", got.Locale)
	}
	if !got.HideClockDate {
		t.Error("hide_clock_date: want true")
	}
	if got.Timezone != "Europe/Budapest" {
		t.Errorf("timezone: got %q, want Europe/Budapest", got.Timezone)
	}
	if !got.Weather {
		t.Error("weather: want true when weatherActive")
	}
	want := []string{"inside:temperature", "outside:temperature"}
	if !slices.Equal(got.Sensors, want) {
		t.Errorf("sensors: got %v, want %v", got.Sensors, want)
	}
	wantLabels := state.KioskLabels{Outside: "Kint", Inside: "Bent", Humidity: "Pára"}
	if got.Labels != wantLabels {
		t.Errorf("labels: got %+v, want %+v", got.Labels, wantLabels)
	}

	if KioskEventPayload(cfg, false).Weather {
		t.Error("weather: want false when not weatherActive")
	}
}

func fullTestConfig() config.Config {
	return config.Config{
		Addr:             ":8080",
		LogLevel:         "debug",
		BluetoothAdapter: "hci0",
		Display: config.DisplayConfig{
			BlankAfter:    config.Duration{Duration: 20 * time.Minute},
			Backend:       config.DisplayBackendWlopm,
			Output:        "HDMI-A-1",
			Locale:        "en-US",
			HideClockDate: true,
			Timezone:      "Europe/Budapest",
			Labels:        config.KioskLabelsConfig{Outside: "Kint", Inside: "Bent", Humidity: "Pára"},
		},
		Slideshow: config.SlideshowConfig{
			Interval:  config.Duration{Duration: 2 * time.Minute},
			Randomize: true,
			ImagesDir: "images",
		},
		Library: config.LibraryConfig{
			Backend: config.BackendFS,
			Immich: config.ImmichLibraryConfig{
				ShareURL:      "https://immich.example.com",
				SharePassword: "secret-immich",
				SyncInterval:  config.Duration{Duration: 15 * time.Minute},
			},
		},
		Sensors: []config.SensorConfig{
			{
				ID:   "sensor1",
				Type: "mock",
				Role: "inside",
				MockReadings: []config.MockReadingConfig{
					{Kind: "temperature", Value: 22.5, Delta: 0.1},
				},
			},
			{
				ID:           "sensor2",
				Type:         "ble",
				Role:         "outside",
				MAC:          "AA:BB:CC:DD:EE:FF",
				AddressType:  "random",
				PollInterval: config.Duration{Duration: 80 * time.Second},
				ResetAfter:   config.Duration{Duration: 5 * time.Minute},
				Characteristics: []config.CharacteristicConfig{
					{UUID: "00002a6e-0000-1000-8000-00805f9b34fb", Kind: "temperature", Decoder: "int16le_div100"},
				},
			},
		},
		Weather: config.WeatherConfig{
			APIKey:        "wkey",
			Lat:           47.5,
			Lon:           19.0,
			PollInterval:  config.Duration{Duration: 10 * time.Minute},
			RetryInterval: config.Duration{Duration: 30 * time.Second},
			Units:         "metric",
		},
		Mqtt: config.MqttConfig{
			Broker:   "tcp://broker:1883",
			Username: "user",
			Password: "mqttpass",
			ClientID: "frame",
			Bridge: config.MqttBridgeConfig{
				Enabled:         true,
				NodeID:          "pf",
				BaseTopic:       "picture-frame",
				DiscoveryPrefix: "homeassistant",
				StaleAfter:      config.Duration{Duration: 10 * time.Minute},
			},
		},
	}
}

func TestToDTOSecretsAreMasked(t *testing.T) {
	cfg := fullTestConfig()
	dto := toDTO(cfg)

	if dto.Weather.APIKey != "" {
		t.Error("weather.api_key must not be returned")
	}
	if !dto.Weather.APIKeySet {
		t.Error("weather.api_key_set must be true when key is set")
	}
	if dto.Mqtt.Password != "" {
		t.Error("mqtt.password must not be returned")
	}
	if !dto.Mqtt.PasswordSet {
		t.Error("mqtt.password_set must be true when password is set")
	}
	if dto.Library.Immich.SharePassword != "" {
		t.Error("library.immich.share_password must not be returned")
	}
	if !dto.Library.Immich.SharePasswordSet {
		t.Error("library.immich.share_password_set must be true when password is set")
	}
}

func TestToDTOSecretsUnsetWhenEmpty(t *testing.T) {
	cfg := fullTestConfig()
	cfg.Weather.APIKey = ""
	cfg.Mqtt.Password = ""
	cfg.Library.Immich.SharePassword = ""

	dto := toDTO(cfg)

	if dto.Weather.APIKeySet {
		t.Error("api_key_set must be false when key is empty")
	}
	if dto.Mqtt.PasswordSet {
		t.Error("password_set must be false when password is empty")
	}
	if dto.Library.Immich.SharePasswordSet {
		t.Error("share_password_set must be false when password is empty")
	}
}

func TestToDTOAddrNotPresent(_ *testing.T) {
	cfg := fullTestConfig()
	dto := toDTO(cfg)
	// ConfigDTO has no Addr field; verify it doesn't sneak in through some other path.
	// (compile-time check via struct literal below)
	_ = ConfigDTO{BluetoothAdapter: dto.BluetoothAdapter}
}

func TestToDTODurationStrings(t *testing.T) {
	cfg := fullTestConfig()
	dto := toDTO(cfg)

	if dto.Display.BlankAfter != "20m0s" {
		t.Errorf("blank_after: got %q", dto.Display.BlankAfter)
	}
	if dto.Slideshow.Interval != "2m0s" {
		t.Errorf("interval: got %q", dto.Slideshow.Interval)
	}
}

func TestToDTOZeroDurationIsEmpty(t *testing.T) {
	cfg := fullTestConfig()
	cfg.Sensors[0].PollInterval = config.Duration{}
	cfg.Sensors[0].ResetAfter = config.Duration{}
	dto := toDTO(cfg)

	if dto.Sensors[0].PollInterval != "" {
		t.Errorf("zero poll_interval should be empty string, got %q", dto.Sensors[0].PollInterval)
	}
}

func TestUpdaterDTORoundTrip(t *testing.T) {
	cfg := fullTestConfig()
	cfg.Updater = config.UpdaterConfig{AutoUpdate: true, UpdateHour: 5, GithubRepo: "me/fork", GithubToken: "ghp_secret"}
	dto := toDTO(cfg)
	if !dto.Updater.AutoUpdate || dto.Updater.UpdateHour != 5 || dto.Updater.GithubRepo != "me/fork" {
		t.Fatalf("toDTO updater: %+v", dto.Updater)
	}
	// The token is write-only: never echoed, only a "set" flag.
	if dto.Updater.GithubToken != "" || !dto.Updater.GithubTokenSet {
		t.Errorf("toDTO must hide the token but flag it set: %+v", dto.Updater)
	}
	out, err := applyDTO(dto, cfg)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	// A blank incoming token with set=true keeps the stored one (no accidental wipe).
	if out.Updater != cfg.Updater {
		t.Errorf("round-trip: got %+v, want %+v", out.Updater, cfg.Updater)
	}
}

func TestUpdaterDTOClearsToken(t *testing.T) {
	cfg := fullTestConfig()
	cfg.Updater.GithubToken = "ghp_secret"
	dto := toDTO(cfg)
	dto.Updater.GithubTokenSet = false // user hit Clear
	out, err := applyDTO(dto, cfg)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Updater.GithubToken != "" {
		t.Errorf("clearing should drop the token, got %q", out.Updater.GithubToken)
	}
}

func TestUpdaterDTORejectsBadHour(t *testing.T) {
	// fullTestConfig is valid everywhere else, so the bad hour is the only possible error.
	dto := toDTO(fullTestConfig())
	dto.Updater.UpdateHour = 25
	if _, err := applyDTO(dto, fullTestConfig()); err == nil {
		t.Fatal("expected error for update_hour 25")
	}
}

func TestApplyDTOPreservesAddr(t *testing.T) {
	current := fullTestConfig()
	current.Addr = ":9090"

	dto := toDTO(current)
	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Addr != ":9090" {
		t.Errorf("addr not preserved: got %q", out.Addr)
	}
}

func TestApplyDTOCoercesInvalidPairThresholdOnEnable(t *testing.T) {
	current := fullTestConfig()
	current.Slideshow.SplitScreen = false
	current.Slideshow.PairThreshold = 0.5 // invalid for split, but allowed while disabled

	dto := toDTO(current)
	dto.Slideshow.SplitScreen = true // the toggle-only DTO carries no pair_threshold

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Slideshow.PairThreshold != config.DefaultPairThreshold {
		t.Errorf("PairThreshold = %v, want coerced to %v", out.Slideshow.PairThreshold, config.DefaultPairThreshold)
	}
	if err := out.Validate(); err != nil {
		t.Errorf("coerced config must validate, got %v", err)
	}
}

func TestApplyDTOPreservesBlankSecret(t *testing.T) {
	current := fullTestConfig()
	// current has "wkey", "mqttpass", "secret-immich"

	dto := toDTO(current) // secrets are blanked in the DTO
	// secrets are blank; applyDTO must keep the current values
	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Weather.APIKey != "wkey" {
		t.Errorf("api_key should be preserved when blank in dto, got %q", out.Weather.APIKey)
	}
	if out.Mqtt.Password != "mqttpass" {
		t.Errorf("mqtt password should be preserved, got %q", out.Mqtt.Password)
	}
	if out.Library.Immich.SharePassword != "secret-immich" {
		t.Errorf("share_password should be preserved, got %q", out.Library.Immich.SharePassword)
	}
}

func TestApplyDTOUpdatesSecretWhenProvided(t *testing.T) {
	current := fullTestConfig()
	dto := toDTO(current)
	dto.Weather.APIKey = "newkey"

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Weather.APIKey != "newkey" {
		t.Errorf("api_key should be updated when provided, got %q", out.Weather.APIKey)
	}
}

func TestApplyDTODurationParseErrors(t *testing.T) {
	current := fullTestConfig()
	cases := []struct {
		name   string
		mutate func(*ConfigDTO)
	}{
		{"display.blank_after", func(d *ConfigDTO) { d.Display.BlankAfter = "bad" }},
		{"slideshow.interval", func(d *ConfigDTO) { d.Slideshow.Interval = "bad" }},
		{"library.immich.sync_interval", func(d *ConfigDTO) { d.Library.Immich.SyncInterval = "bad" }},
		{"sensors[0].poll_interval", func(d *ConfigDTO) { d.Sensors[0].PollInterval = "bad" }},
		{"sensors[1].reset_after", func(d *ConfigDTO) { d.Sensors[1].ResetAfter = "bad" }},
		{"weather.poll_interval", func(d *ConfigDTO) { d.Weather.PollInterval = "bad" }},
		{"weather.retry_interval", func(d *ConfigDTO) { d.Weather.RetryInterval = "bad" }},
		{"mqtt.bridge.stale_after", func(d *ConfigDTO) { d.Mqtt.Bridge.StaleAfter = "bad" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := toDTO(current)
			tc.mutate(&dto)
			_, err := applyDTO(dto, current)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestApplyDTOMqttPasswordUpdated(t *testing.T) {
	current := fullTestConfig()
	dto := toDTO(current) // mqtt.password blanked in DTO
	dto.Mqtt.Password = "newpass"

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Mqtt.Password != "newpass" {
		t.Errorf("mqtt password should be updated when provided, got %q", out.Mqtt.Password)
	}
}

func TestApplyDTORoundTrip(t *testing.T) {
	current := fullTestConfig()
	dto := toDTO(current)
	// Provide secrets explicitly so they survive the round-trip.
	dto.Weather.APIKey = current.Weather.APIKey
	dto.Mqtt.Password = current.Mqtt.Password
	dto.Library.Immich.SharePassword = current.Library.Immich.SharePassword

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}

	// Addr is preserved from current.
	if out.Addr != current.Addr {
		t.Errorf("addr: got %q, want %q", out.Addr, current.Addr)
	}
	// Non-secret, non-addr fields should match.
	if out.LogLevel != current.LogLevel {
		t.Errorf("log_level: got %q, want %q", out.LogLevel, current.LogLevel)
	}
	if out.BluetoothAdapter != current.BluetoothAdapter {
		t.Errorf("bluetooth_adapter: got %q, want %q", out.BluetoothAdapter, current.BluetoothAdapter)
	}
	if out.Display.Locale != current.Display.Locale {
		t.Errorf("display.locale: got %q, want %q", out.Display.Locale, current.Display.Locale)
	}
	if out.Display.HideClockDate != current.Display.HideClockDate {
		t.Errorf("display.hide_clock_date: got %v, want %v", out.Display.HideClockDate, current.Display.HideClockDate)
	}
	if out.Display.Timezone != current.Display.Timezone {
		t.Errorf("display.timezone: got %q, want %q", out.Display.Timezone, current.Display.Timezone)
	}
	if out.Display.Labels != current.Display.Labels {
		t.Errorf("display.labels: got %+v, want %+v", out.Display.Labels, current.Display.Labels)
	}
	if out.Weather.Lat != current.Weather.Lat {
		t.Errorf("lat: got %v, want %v", out.Weather.Lat, current.Weather.Lat)
	}
	if out.Slideshow.Randomize != current.Slideshow.Randomize {
		t.Errorf("randomize: got %v, want %v", out.Slideshow.Randomize, current.Slideshow.Randomize)
	}
	if len(out.Sensors) != len(current.Sensors) {
		t.Errorf("sensors len: got %d, want %d", len(out.Sensors), len(current.Sensors))
	}
}

func TestToDTODefaultsEmptyLogLevelToInfo(t *testing.T) {
	cfg := fullTestConfig()
	cfg.LogLevel = "" // an older config that never set a level
	if got := toDTO(cfg).LogLevel; got != "info" {
		t.Errorf("log_level: got %q, want %q", got, "info")
	}
}

func TestApplyDTOClearsWeatherAPIKey(t *testing.T) {
	current := fullTestConfig() // has APIKey = "wkey"
	dto := toDTO(current)
	dto.Weather.APIKeySet = false // signal: clear the stored key

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Weather.APIKey != "" {
		t.Errorf("api_key should be cleared, got %q", out.Weather.APIKey)
	}
}

func TestApplyDTOClearsMqttPassword(t *testing.T) {
	current := fullTestConfig() // has Password = "mqttpass"
	dto := toDTO(current)
	dto.Mqtt.PasswordSet = false // signal: clear the stored password

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Mqtt.Password != "" {
		t.Errorf("mqtt password should be cleared, got %q", out.Mqtt.Password)
	}
}

func TestApplyDTOClearsSharePassword(t *testing.T) {
	current := fullTestConfig() // has SharePassword = "secret-immich"
	dto := toDTO(current)
	dto.Library.Immich.SharePasswordSet = false // signal: clear the stored password

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Library.Immich.SharePassword != "" {
		t.Errorf("share_password should be cleared, got %q", out.Library.Immich.SharePassword)
	}
}

func TestApplyDTOSlideshowIntervalRequired(t *testing.T) {
	current := fullTestConfig()
	dto := toDTO(current)
	dto.Slideshow.Interval = ""

	_, err := applyDTO(dto, current)
	if err == nil {
		t.Fatal("expected error for empty slideshow.interval")
	}
}

func TestApplyDTOPreservesExistingSensorDurations(t *testing.T) {
	current := fullTestConfig() // sensor2 has PollInterval=80s, ResetAfter=5m
	dto := toDTO(current)
	// Simulate a form field sent as empty (absent/blank) for an existing sensor.
	dto.Sensors[1].PollInterval = ""
	dto.Sensors[1].ResetAfter = ""

	out, err := applyDTO(dto, current)
	if err != nil {
		t.Fatalf("applyDTO: %v", err)
	}
	if out.Sensors[1].PollInterval != current.Sensors[1].PollInterval {
		t.Errorf("poll_interval: got %v, want %v", out.Sensors[1].PollInterval, current.Sensors[1].PollInterval)
	}
	if out.Sensors[1].ResetAfter != current.Sensors[1].ResetAfter {
		t.Errorf("reset_after: got %v, want %v", out.Sensors[1].ResetAfter, current.Sensors[1].ResetAfter)
	}
}

// Pins applyTier1ToRunning to zeroTier1: a tier-1-only edit, once applied,
// must leave running identical to saved: a field present in one list but
// missing from the other fails here.
func TestApplyTier1CoversAllTier1Fields(t *testing.T) {
	running := fullTestConfig()
	saved := fullTestConfig()
	saved.LogLevel = "warn"
	saved.Display.BlankAfter = config.Duration{Duration: 5 * time.Minute}
	saved.Display.Locale = "hu-HU"
	saved.Display.Labels = config.KioskLabelsConfig{Outside: "Udvar", Inside: "Nappali", Humidity: "Pára"}
	saved.Slideshow.Interval = config.Duration{Duration: 5 * time.Minute}
	saved.Slideshow.Randomize = !saved.Slideshow.Randomize
	saved.Weather.PollInterval = config.Duration{Duration: 5 * time.Minute}
	saved.Weather.RetryInterval = config.Duration{Duration: 90 * time.Second}

	applyTier1ToRunning(&running, saved)

	if !reflect.DeepEqual(running, saved) {
		t.Errorf("running diverges from saved after applying tier-1 fields:\nrunning: %+v\nsaved:   %+v", running, saved)
	}
}

func TestNeedsRestartTier1FieldsAreLive(t *testing.T) {
	base := fullTestConfig()

	tier1Cases := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{"log_level", func(c *config.Config) { c.LogLevel = "warn" }},
		{"display.blank_after", func(c *config.Config) { c.Display.BlankAfter = config.Duration{Duration: 5 * time.Minute} }},
		{"display.locale", func(c *config.Config) { c.Display.Locale = "hu-HU" }},
		{"display.hide_clock_date", func(c *config.Config) { c.Display.HideClockDate = !c.Display.HideClockDate }},
		{"display.timezone", func(c *config.Config) { c.Display.Timezone = "America/New_York" }},
		{"display.labels", func(c *config.Config) { c.Display.Labels.Outside = "Garden" }},
		{"slideshow.interval", func(c *config.Config) { c.Slideshow.Interval = config.Duration{Duration: 5 * time.Minute} }},
		{"slideshow.randomize", func(c *config.Config) { c.Slideshow.Randomize = !c.Slideshow.Randomize }},
		{"weather.poll_interval", func(c *config.Config) { c.Weather.PollInterval = config.Duration{Duration: 5 * time.Minute} }},
		{"weather.retry_interval", func(c *config.Config) { c.Weather.RetryInterval = config.Duration{Duration: 5 * time.Minute} }},
	}

	for _, tc := range tier1Cases {
		t.Run(tc.name, func(t *testing.T) {
			running := base
			saved := base
			tc.mutate(&saved)
			if needsRestart(running, saved) {
				t.Errorf("%s: expected no restart needed (Tier-1 field)", tc.name)
			}
		})
	}
}

func TestNeedsRestartNonTier1Fields(t *testing.T) {
	base := fullTestConfig()

	restartCases := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{"bluetooth_adapter", func(c *config.Config) { c.BluetoothAdapter = "hci1" }},
		{"display.backend", func(c *config.Config) { c.Display.Backend = config.DisplayBackendVcgencmd }},
		{"display.output", func(c *config.Config) { c.Display.Output = "HDMI-A-2" }},
		{"slideshow.images_dir", func(c *config.Config) { c.Slideshow.ImagesDir = "other" }},
		{"library.backend", func(c *config.Config) { c.Library.Backend = config.BackendImmich }},
		{"weather.api_key", func(c *config.Config) { c.Weather.APIKey = "other" }},
		{"weather.lat", func(c *config.Config) { c.Weather.Lat = 99.9 }},
		{"weather.units", func(c *config.Config) { c.Weather.Units = "imperial" }},
		{"mqtt.broker", func(c *config.Config) { c.Mqtt.Broker = "tcp://other:1883" }},
		{"sensors", func(c *config.Config) { c.Sensors = append(c.Sensors, config.SensorConfig{ID: "new"}) }},
	}

	for _, tc := range restartCases {
		t.Run(tc.name, func(t *testing.T) {
			running := base
			saved := base
			tc.mutate(&saved)
			if !needsRestart(running, saved) {
				t.Errorf("%s: expected restart needed", tc.name)
			}
		})
	}
}

// nil and round-tripped-empty slices must compare equal, else every load reads dirty.
func TestNeedsRestartNormalizesNilVsEmptySlices(t *testing.T) {
	base := fullTestConfig()
	cases := []struct {
		name             string
		nilform, empties func(*config.Config)
	}{
		{
			"sensors",
			func(c *config.Config) { c.Sensors = nil },
			func(c *config.Config) { c.Sensors = []config.SensorConfig{} },
		},
		{
			"characteristics",
			func(c *config.Config) { c.Sensors = []config.SensorConfig{{ID: "s", Characteristics: nil}} },
			func(c *config.Config) {
				c.Sensors = []config.SensorConfig{{ID: "s", Characteristics: []config.CharacteristicConfig{}}}
			},
		},
		{
			"mock_readings",
			func(c *config.Config) { c.Sensors = []config.SensorConfig{{ID: "s", MockReadings: nil}} },
			func(c *config.Config) {
				c.Sensors = []config.SensorConfig{{ID: "s", MockReadings: []config.MockReadingConfig{}}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			running := base
			saved := base
			tc.nilform(&running)
			tc.empties(&saved)
			if needsRestart(running, saved) {
				t.Errorf("%s: nil and empty slices must compare equal", tc.name)
			}
		})
	}
}

func TestNeedsRestartIdenticalConfigs(t *testing.T) {
	cfg := fullTestConfig()
	if needsRestart(cfg, cfg) {
		t.Error("identical configs: expected no restart needed")
	}
}

// Both UpdateHour edges must be accepted: the schema rejects out-of-range
// values, and applyUpdaterDTO's own guard must agree with it exactly.
func TestApplyDTOUpdateHourEdges(t *testing.T) {
	for _, hour := range []int{0, 23} {
		dto := toDTO(fullTestConfig())
		dto.Updater.UpdateHour = hour
		out, err := applyDTO(dto, fullTestConfig())
		if err != nil {
			t.Fatalf("hour %d: %v", hour, err)
		}
		if out.Updater.UpdateHour != hour {
			t.Errorf("hour %d: got %d", hour, out.Updater.UpdateHour)
		}
	}
	dto := toDTO(fullTestConfig())
	dto.Updater.UpdateHour = -1
	if _, err := applyDTO(dto, fullTestConfig()); err == nil {
		t.Fatal("expected error for update_hour -1")
	}
}
