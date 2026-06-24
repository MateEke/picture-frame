package httpapi

// ConfigDTO and its mapping are the single place to touch when adding a config
// option: add the field to config.Config, mirror it in the nested DTO here plus
// toDTO/applyDTO, and add a form input to the matching settings card.
//
// Conventions:
//   - Duration fields are strings (e.g. "20m"), parsed by time.ParseDuration.
//   - Secrets: write-only string in, output-only *_set bool out; GET never
//     returns the stored value.
//   - addr and WiFi config are excluded (owned elsewhere).

import (
	"fmt"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/version"
)

// ConfigDTO mirrors config.Config for the settings API.
type ConfigDTO struct {
	LogLevel         string       `json:"log_level" enum:"debug,info,warn,error" doc:"Logging verbosity; applied live"`
	BluetoothAdapter string       `json:"bluetooth_adapter" doc:"HCI device ID, e.g. hci0"`
	Display          DisplayDTO   `json:"display"`
	Slideshow        SlideshowDTO `json:"slideshow"`
	Library          LibraryDTO   `json:"library"`
	Sensors          []SensorDTO  `json:"sensors"`
	Weather          WeatherDTO   `json:"weather"`
	Mqtt             MqttDTO      `json:"mqtt"`
	Updater          UpdaterDTO   `json:"updater"`
}

// UpdaterDTO mirrors config.UpdaterConfig for the settings UI.
type UpdaterDTO struct {
	AutoUpdate     bool   `json:"auto_update"`
	UpdateHour     int    `json:"update_hour" minimum:"0" maximum:"23"`
	GithubRepo     string `json:"github_repo"`
	GithubToken    string `json:"github_token,omitempty" doc:"Write-only; leave blank to keep current"`
	GithubTokenSet bool   `json:"github_token_set" doc:"true if a token is stored; set false on PUT to clear"`
}

// ConfigResponseBody extends ConfigDTO with the restart-pending flag for GET responses.
type ConfigResponseBody struct {
	ConfigDTO
	RestartPending bool `json:"restart_pending"`
}

// PutConfigResponseBody is the PUT /api/config response.
type PutConfigResponseBody struct {
	RestartPending bool `json:"restart_pending"`
}

// ConfigMetaBody carries the enumerated options that drive UI <select> elements.
type ConfigMetaBody struct {
	Decoders     []string `json:"decoders" doc:"Decoder/parser names from the sensor registry"`
	Kinds        []string `json:"kinds"`
	Units        []string `json:"units"`
	Backends     []string `json:"backends"`
	SensorTypes  []string `json:"sensor_types"`
	AddressTypes []string `json:"address_types"`
	LogLevels    []string `json:"log_levels"`
}

// DisplayDTO maps config.DisplayConfig.
type DisplayDTO struct {
	BlankAfter    string         `json:"blank_after" doc:"Idle duration before screen blanks, e.g. \"20m\""`
	Backend       string         `json:"backend" enum:"wlopm,vcgencmd"`
	Output        string         `json:"output" doc:"Wayland connector name, e.g. \"HDMI-A-1\" (wlopm only)"`
	Locale        string         `json:"locale" doc:"BCP-47 date/time locale for the kiosk clock, e.g. en-US"`
	HideClockDate bool           `json:"hide_clock_date" doc:"Hide the clock and date block on the kiosk overlay"`
	Timezone      string         `json:"timezone" doc:"IANA timezone for the kiosk clock/date, e.g. Europe/Budapest; empty uses the browser timezone"`
	Labels        KioskLabelsDTO `json:"labels"`
}

// KioskLabelsDTO maps config.KioskLabelsConfig.
type KioskLabelsDTO struct {
	Outside  string `json:"outside" doc:"Caption under the outside reading; empty hides it"`
	Inside   string `json:"inside" doc:"Caption under the inside reading; empty hides it"`
	Humidity string `json:"humidity" doc:"Caption under the humidity reading; empty hides it"`
}

// The label conversions live together so a new label field is added here, not
// hunted across toDTO/applyDTO/KioskEventPayload.
func labelsToDTO(c config.KioskLabelsConfig) KioskLabelsDTO {
	return KioskLabelsDTO{Outside: c.Outside, Inside: c.Inside, Humidity: c.Humidity}
}

func labelsFromDTO(d KioskLabelsDTO) config.KioskLabelsConfig {
	return config.KioskLabelsConfig{Outside: d.Outside, Inside: d.Inside, Humidity: d.Humidity}
}

func labelsToState(c config.KioskLabelsConfig) state.KioskLabels {
	return state.KioskLabels{Outside: c.Outside, Inside: c.Inside, Humidity: c.Humidity}
}

// SlideshowDTO maps config.SlideshowConfig.
type SlideshowDTO struct {
	Interval    string `json:"interval" doc:"Image advance interval, e.g. \"2m\""`
	Randomize   bool   `json:"randomize"`
	SplitScreen bool   `json:"split_screen" doc:"Pair mismatched-orientation photos side-by-side"`
	ImagesDir   string `json:"images_dir"`
}

// LibraryDTO maps config.LibraryConfig.
type LibraryDTO struct {
	Backend string           `json:"backend" enum:"fs,immich"`
	Immich  ImmichLibraryDTO `json:"immich"`
}

// ImmichLibraryDTO maps config.ImmichLibraryConfig with the password as write-only.
type ImmichLibraryDTO struct {
	ShareURL         string `json:"share_url"`
	SharePassword    string `json:"share_password,omitempty" doc:"Write-only; leave blank to keep current"`
	SharePasswordSet bool   `json:"share_password_set" doc:"true if a password is stored; set false on PUT to clear"`
	SyncInterval     string `json:"sync_interval"`
}

// WeatherDTO maps config.WeatherConfig with the API key as write-only.
type WeatherDTO struct {
	APIKey        string  `json:"api_key,omitempty" doc:"Write-only; leave blank to keep current"`
	APIKeySet     bool    `json:"api_key_set" doc:"true if an API key is stored; set false on PUT to clear"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	PollInterval  string  `json:"poll_interval"`
	RetryInterval string  `json:"retry_interval"`
	Units         string  `json:"units" enum:"standard,metric,imperial"`
}

// MqttDTO maps config.MqttConfig with the password as write-only.
type MqttDTO struct {
	Broker      string        `json:"broker"`
	Username    string        `json:"username"`
	Password    string        `json:"password,omitempty" doc:"Write-only; leave blank to keep current"`
	PasswordSet bool          `json:"password_set" doc:"true if a password is stored; set false on PUT to clear"`
	ClientID    string        `json:"client_id"`
	Bridge      MqttBridgeDTO `json:"bridge"`
}

// MqttBridgeDTO maps config.MqttBridgeConfig.
type MqttBridgeDTO struct {
	Enabled         bool   `json:"enabled"`
	NodeID          string `json:"node_id"`
	BaseTopic       string `json:"base_topic"`
	DiscoveryPrefix string `json:"discovery_prefix"`
	StaleAfter      string `json:"stale_after"`
}

// SensorDTO maps config.SensorConfig.
type SensorDTO struct {
	ID              string              `json:"id"`
	Type            string              `json:"type" enum:"ble,mqtt-subscriber,mock"`
	Role            string              `json:"role"`
	MAC             string              `json:"mac,omitempty"`
	AddressType     string              `json:"address_type,omitempty" enum:"random,public"`
	Characteristics []CharacteristicDTO `json:"characteristics,omitempty"`
	PollInterval    string              `json:"poll_interval,omitempty"`
	ResetAfter      string              `json:"reset_after,omitempty"`
	Topic           string              `json:"topic,omitempty"`
	Kind            string              `json:"kind,omitempty"`
	Parser          string              `json:"parser,omitempty"`
	JSONField       string              `json:"json_field,omitempty"`
	MockReadings    []MockReadingDTO    `json:"mock_readings,omitempty"`
}

// CharacteristicDTO maps config.CharacteristicConfig.
type CharacteristicDTO struct {
	UUID    string `json:"uuid"`
	Kind    string `json:"kind"`
	Decoder string `json:"decoder"`
}

// MockReadingDTO maps config.MockReadingConfig.
type MockReadingDTO struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
	Delta float64 `json:"delta"`
}

// toDTO converts a config.Config to ConfigDTO, masking secrets as *_set booleans.
func toDTO(cfg config.Config) ConfigDTO {
	sensors := make([]SensorDTO, len(cfg.Sensors))
	for i, s := range cfg.Sensors {
		sensors[i] = sensorToDTO(s)
	}
	return ConfigDTO{
		LogLevel:         logLevelOrDefault(cfg.LogLevel),
		BluetoothAdapter: cfg.BluetoothAdapter,
		Display: DisplayDTO{
			BlankAfter:    durString(cfg.Display.BlankAfter.Duration),
			Backend:       cfg.Display.Backend,
			Output:        cfg.Display.Output,
			Locale:        cfg.Display.Locale,
			HideClockDate: cfg.Display.HideClockDate,
			Timezone:      cfg.Display.Timezone,
			Labels:        labelsToDTO(cfg.Display.Labels),
		},
		Slideshow: SlideshowDTO{
			Interval:    durString(cfg.Slideshow.Interval.Duration),
			Randomize:   cfg.Slideshow.Randomize,
			SplitScreen: cfg.Slideshow.SplitScreen,
			ImagesDir:   cfg.Slideshow.ImagesDir,
		},
		Library: LibraryDTO{
			Backend: cfg.Library.Backend,
			Immich: ImmichLibraryDTO{
				ShareURL:         cfg.Library.Immich.ShareURL,
				SharePasswordSet: cfg.Library.Immich.SharePassword != "",
				SyncInterval:     durString(cfg.Library.Immich.SyncInterval.Duration),
			},
		},
		Sensors: sensors,
		Weather: WeatherDTO{
			APIKeySet:     cfg.Weather.APIKey != "",
			Lat:           cfg.Weather.Lat,
			Lon:           cfg.Weather.Lon,
			PollInterval:  durString(cfg.Weather.PollInterval.Duration),
			RetryInterval: durString(cfg.Weather.RetryInterval.Duration),
			Units:         cfg.Weather.Units,
		},
		Mqtt: MqttDTO{
			Broker:      cfg.Mqtt.Broker,
			Username:    cfg.Mqtt.Username,
			PasswordSet: cfg.Mqtt.Password != "",
			ClientID:    cfg.Mqtt.ClientID,
			Bridge: MqttBridgeDTO{
				Enabled:         cfg.Mqtt.Bridge.Enabled,
				NodeID:          cfg.Mqtt.Bridge.NodeID,
				BaseTopic:       cfg.Mqtt.Bridge.BaseTopic,
				DiscoveryPrefix: cfg.Mqtt.Bridge.DiscoveryPrefix,
				StaleAfter:      durString(cfg.Mqtt.Bridge.StaleAfter.Duration),
			},
		},
		Updater: UpdaterDTO{
			AutoUpdate:     cfg.Updater.AutoUpdate,
			UpdateHour:     cfg.Updater.UpdateHour,
			GithubRepo:     cfg.Updater.GithubRepo,
			GithubTokenSet: cfg.Updater.GithubToken != "",
		},
	}
}

// sensorToDTO converts one sensor to its DTO, formatting durations as strings.
func sensorToDTO(s config.SensorConfig) SensorDTO {
	chars := make([]CharacteristicDTO, len(s.Characteristics))
	for j, c := range s.Characteristics {
		chars[j] = CharacteristicDTO{UUID: c.UUID, Kind: c.Kind, Decoder: c.Decoder}
	}
	readings := make([]MockReadingDTO, len(s.MockReadings))
	for j, r := range s.MockReadings {
		readings[j] = MockReadingDTO{Kind: r.Kind, Value: r.Value, Delta: r.Delta}
	}
	return SensorDTO{
		ID:              s.ID,
		Type:            s.Type,
		Role:            s.Role,
		MAC:             s.MAC,
		AddressType:     s.AddressType,
		Characteristics: chars,
		PollInterval:    durString(s.PollInterval.Duration),
		ResetAfter:      durString(s.ResetAfter.Duration),
		Topic:           s.Topic,
		Kind:            s.Kind,
		Parser:          s.Parser,
		JSONField:       s.JSONField,
		MockReadings:    readings,
	}
}

// applyDTO merges dto onto current, preserving Addr, blank secrets, and any
// fields not exposed by the DTO. Returns an error only for malformed durations.
func applyDTO(dto ConfigDTO, current config.Config) (config.Config, error) {
	out := current // copy: preserves Addr, WiFi, and other unexposed fields
	out.LogLevel = dto.LogLevel
	out.BluetoothAdapter = dto.BluetoothAdapter

	if err := applyDisplayDTO(&out.Display, dto.Display); err != nil {
		return config.Config{}, err
	}
	if err := applySlideshowDTO(&out.Slideshow, dto.Slideshow); err != nil {
		return config.Config{}, err
	}
	if err := applyLibraryDTO(&out.Library, dto.Library); err != nil {
		return config.Config{}, err
	}
	sensors, err := applySensorsDTO(dto.Sensors, current.Sensors)
	if err != nil {
		return config.Config{}, err
	}
	out.Sensors = sensors
	if err := applyWeatherDTO(&out.Weather, dto.Weather); err != nil {
		return config.Config{}, err
	}
	if err := applyMqttDTO(&out.Mqtt, dto.Mqtt); err != nil {
		return config.Config{}, err
	}
	if err := applyUpdaterDTO(&out.Updater, dto.Updater); err != nil {
		return config.Config{}, err
	}
	return out, nil
}

func applyUpdaterDTO(dst *config.UpdaterConfig, dto UpdaterDTO) error {
	if dto.UpdateHour < 0 || dto.UpdateHour > 23 {
		return fmt.Errorf("updater.update_hour: must be 0–23")
	}
	dst.AutoUpdate = dto.AutoUpdate
	dst.UpdateHour = dto.UpdateHour
	dst.GithubRepo = dto.GithubRepo
	dst.GithubToken = applySecret(dst.GithubToken, dto.GithubToken, dto.GithubTokenSet)
	return nil
}

func applyDisplayDTO(dst *config.DisplayConfig, dto DisplayDTO) error {
	blankAfter, err := parseDuration(dto.BlankAfter, "display.blank_after")
	if err != nil {
		return err
	}
	dst.BlankAfter = blankAfter
	dst.Backend = dto.Backend
	dst.Output = dto.Output
	dst.Locale = dto.Locale
	dst.HideClockDate = dto.HideClockDate
	dst.Timezone = dto.Timezone
	dst.Labels = labelsFromDTO(dto.Labels)
	return nil
}

func applySlideshowDTO(dst *config.SlideshowConfig, dto SlideshowDTO) error {
	interval, err := parseDuration(dto.Interval, "slideshow.interval")
	if err != nil {
		return err
	}
	if interval.Duration <= 0 {
		return fmt.Errorf("slideshow.interval: must be a positive duration")
	}
	dst.Interval = interval
	dst.Randomize = dto.Randomize
	dst.SplitScreen = dto.SplitScreen
	// The toggle-only DTO omits pair_threshold; coerce an invalid running value so
	// enabling split-screen can't produce a config that fails Validate (>1 required).
	if dst.SplitScreen && dst.PairThreshold <= 1 {
		dst.PairThreshold = config.DefaultPairThreshold
	}
	dst.ImagesDir = dto.ImagesDir
	return nil
}

func applyLibraryDTO(dst *config.LibraryConfig, dto LibraryDTO) error {
	dst.Backend = dto.Backend
	dst.Immich.ShareURL = dto.Immich.ShareURL
	dst.Immich.SharePassword = applySecret(dst.Immich.SharePassword, dto.Immich.SharePassword, dto.Immich.SharePasswordSet)
	syncInterval, err := parseDuration(dto.Immich.SyncInterval, "library.immich.sync_interval")
	if err != nil {
		return err
	}
	dst.Immich.SyncInterval = syncInterval
	return nil
}

func applySensorsDTO(dtos []SensorDTO, prev []config.SensorConfig) ([]config.SensorConfig, error) {
	// Optional sensor durations the DTO omits fall back to the stored value, so a
	// UI form that leaves a field untouched can't zero it.
	prevByID := make(map[string]config.SensorConfig, len(prev))
	for _, s := range prev {
		prevByID[s.ID] = s
	}
	sensors := make([]config.SensorConfig, len(dtos))
	for i, dto := range dtos {
		sensor, err := sensorFromDTO(dto, prevByID[dto.ID], fmt.Sprintf("sensors[%d]", i))
		if err != nil {
			return nil, err
		}
		sensors[i] = sensor
	}
	return sensors, nil
}

func sensorFromDTO(dto SensorDTO, prev config.SensorConfig, field string) (config.SensorConfig, error) {
	chars := make([]config.CharacteristicConfig, len(dto.Characteristics))
	for j, c := range dto.Characteristics {
		chars[j] = config.CharacteristicConfig{UUID: c.UUID, Kind: c.Kind, Decoder: c.Decoder}
	}
	readings := make([]config.MockReadingConfig, len(dto.MockReadings))
	for j, r := range dto.MockReadings {
		readings[j] = config.MockReadingConfig{Kind: r.Kind, Value: r.Value, Delta: r.Delta}
	}
	pollInterval, err := parseDurationOr(dto.PollInterval, field+".poll_interval", prev.PollInterval)
	if err != nil {
		return config.SensorConfig{}, err
	}
	resetAfter, err := parseDurationOr(dto.ResetAfter, field+".reset_after", prev.ResetAfter)
	if err != nil {
		return config.SensorConfig{}, err
	}
	return config.SensorConfig{
		ID:              dto.ID,
		Type:            dto.Type,
		Role:            dto.Role,
		MAC:             dto.MAC,
		AddressType:     dto.AddressType,
		Characteristics: chars,
		PollInterval:    pollInterval,
		ResetAfter:      resetAfter,
		Topic:           dto.Topic,
		Kind:            dto.Kind,
		Parser:          dto.Parser,
		JSONField:       dto.JSONField,
		MockReadings:    readings,
	}, nil
}

func applyWeatherDTO(dst *config.WeatherConfig, dto WeatherDTO) error {
	dst.APIKey = applySecret(dst.APIKey, dto.APIKey, dto.APIKeySet)
	dst.Lat = dto.Lat
	dst.Lon = dto.Lon
	pollInterval, err := parseDuration(dto.PollInterval, "weather.poll_interval")
	if err != nil {
		return err
	}
	dst.PollInterval = pollInterval
	retryInterval, err := parseDuration(dto.RetryInterval, "weather.retry_interval")
	if err != nil {
		return err
	}
	dst.RetryInterval = retryInterval
	dst.Units = dto.Units
	return nil
}

func applyMqttDTO(dst *config.MqttConfig, dto MqttDTO) error {
	dst.Broker = dto.Broker
	dst.Username = dto.Username
	dst.Password = applySecret(dst.Password, dto.Password, dto.PasswordSet)
	dst.ClientID = dto.ClientID
	dst.Bridge.Enabled = dto.Bridge.Enabled
	dst.Bridge.NodeID = dto.Bridge.NodeID
	dst.Bridge.BaseTopic = dto.Bridge.BaseTopic
	dst.Bridge.DiscoveryPrefix = dto.Bridge.DiscoveryPrefix
	staleAfter, err := parseDuration(dto.Bridge.StaleAfter, "mqtt.bridge.stale_after")
	if err != nil {
		return err
	}
	dst.Bridge.StaleAfter = staleAfter
	return nil
}

// applySecret returns the secret to store: the incoming value if non-empty, the
// existing one if the field was omitted (set), or "" to clear it. GET masks the
// stored value, so a blank incoming value with set=true means "keep current".
func applySecret(current, incoming string, set bool) string {
	switch {
	case incoming != "":
		return incoming
	case !set:
		return ""
	default:
		return current
	}
}

// parseDuration parses an optional duration field; "" yields the zero Duration.
func parseDuration(s, field string) (config.Duration, error) {
	if s == "" {
		return config.Duration{}, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return config.Duration{}, fmt.Errorf("%s: %w", field, err)
	}
	return config.Duration{Duration: d}, nil
}

func parseDurationOr(s, field string, fallback config.Duration) (config.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return parseDuration(s, field)
}

// KioskEventPayload derives the kiosk overlay inputs from cfg. weatherActive
// (see startup.WeatherEnabled) gates the weather UI.
func KioskEventPayload(cfg config.Config, weatherActive bool) state.KioskPayload {
	return state.KioskPayload{
		Version:       version.Version,
		Locale:        cfg.Display.Locale,
		HideClockDate: cfg.Display.HideClockDate,
		Timezone:      cfg.Display.Timezone,
		Sensors:       config.SensorKeys(cfg.Sensors),
		Weather:       weatherActive,
		Labels:        labelsToState(cfg.Display.Labels),
	}
}

// logLevelOrDefault normalises an empty stored level to "info" so the DTO always
// carries a value that satisfies the log_level enum on a subsequent PUT.
func logLevelOrDefault(level string) string {
	if level == "" {
		return "info"
	}
	return level
}

// durString converts a time.Duration to its string form; returns "" for zero.
func durString(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}
