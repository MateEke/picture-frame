package mqtt

import (
	"encoding/json"
	"strings"
	"time"
)

// Settings configures the MQTT bridge. NodeID identifies this frame, it forms
// the HA device id, prefixes every unique_id, and groups discovery topics, so
// multiple frames need only distinct NodeIDs (no hardcoded MACs).
type Settings struct {
	NodeID          string
	BaseTopic       string
	DiscoveryPrefix string
	DeviceName      string // HA device friendly name; defaults to a title-cased NodeID
	StaleAfter      time.Duration
	Device          DeviceMeta // hardware/software metadata for the HA device card
	Undervoltage    bool       // advertise the undervoltage entity (vcgencmd available)
	Hostname        string     // OS hostname for the diagnostic sensor
	CanReboot       bool       // logind permits reboot (polkit rule present)
	CanPowerOff     bool       // logind permits power-off (polkit rule present)
}

// DeviceMeta is the optional hardware/software metadata shown on the HA device card.
type DeviceMeta struct {
	Manufacturer     string
	Model            string
	HwVersion        string
	SwVersion        string
	ConfigurationURL string
}

// SensorSpec is one sensor and the kinds it emits, used to build HA entities
// before any reading arrives.
type SensorSpec struct {
	ID    string   // matches Reading.DeviceID and the config sensor id
	Role  string   // friendly label; falls back to ID
	Kinds []string // "temperature" | "humidity" | "motion"
}

const (
	availOnline        = "online"
	availOffline       = "offline"
	payloadOn          = "ON"
	payloadOff         = "OFF"
	diagnosticCategory = "diagnostic"
)

// BridgeAvailabilityTopic is the bridge availability topic; callers set it as
// the MQTT Last Will so an unclean disconnect marks the frame offline.
func (s Settings) BridgeAvailabilityTopic() string { return s.bridgeAvailTopic() }

func (s Settings) bridgeAvailTopic() string { return s.BaseTopic + "/availability" }
func (s Settings) switchStateTopic() string { return s.BaseTopic + "/switch/screen/state" }
func (s Settings) switchSetTopic() string   { return s.BaseTopic + "/switch/screen/set" }

// screenPowerStateTopic carries live panel power for the read-only binary_sensor
// (distinct from the intent switch, which idle-blank/motion must not move).
func (s Settings) screenPowerStateTopic() string {
	return s.BaseTopic + "/binary_sensor/screen_power/state"
}
func (s Settings) cpuTempStateTopic() string { return s.BaseTopic + "/sensor/cpu_temperature/state" }
func (s Settings) memoryStateTopic() string  { return s.BaseTopic + "/sensor/memory_usage/state" }
func (s Settings) uptimeStateTopic() string  { return s.BaseTopic + "/sensor/uptime/state" }
func (s Settings) undervoltageStateTopic() string {
	return s.BaseTopic + "/binary_sensor/undervoltage/state"
}
func (s Settings) lastTouchStateTopic() string { return s.BaseTopic + "/sensor/last_touch/state" }
func (s Settings) hostnameStateTopic() string  { return s.BaseTopic + "/sensor/hostname/state" }
func (s Settings) ipStateTopic() string        { return s.BaseTopic + "/sensor/ip_address/state" }

func (s Settings) rebootCommandTopic() string {
	return s.BaseTopic + "/button/reboot/set"
}

func (s Settings) shutdownCommandTopic() string {
	return s.BaseTopic + "/button/shutdown/set"
}

func (s Settings) sensorAvailTopic(id string) string {
	return s.BaseTopic + "/sensor/" + id + "/availability"
}

func (s Settings) stateTopic(id, kind string) string {
	return s.BaseTopic + "/sensor/" + id + "/" + kind
}

func (s Settings) discoveryTopic(component, objectID string) string {
	return s.DiscoveryPrefix + "/" + component + "/" + s.NodeID + "/" + objectID + "/config"
}

func (s Settings) deviceName() string {
	if s.DeviceName != "" {
		return s.DeviceName
	}
	return titleCase(s.NodeID)
}

func component(kind string) string {
	if kind == "motion" {
		return "binary_sensor"
	}
	return "sensor"
}

// haDevice groups all entities under one HA device.
type haDevice struct {
	Identifiers      []string `json:"identifiers"`
	Name             string   `json:"name"`
	Manufacturer     string   `json:"manufacturer,omitempty"`
	Model            string   `json:"model,omitempty"`
	HwVersion        string   `json:"hw_version,omitempty"`
	SwVersion        string   `json:"sw_version,omitempty"`
	ConfigurationURL string   `json:"configuration_url,omitempty"`
}

type haAvailability struct {
	Topic string `json:"topic"`
}

// discoveryConfig is the HA discovery payload; omitempty lets sensor and switch
// share one struct.
type discoveryConfig struct {
	UniqueID          string           `json:"unique_id"`
	Name              string           `json:"name"`
	StateTopic        string           `json:"state_topic,omitempty"` // omitted for command-only buttons
	CommandTopic      string           `json:"command_topic,omitempty"`
	Icon              string           `json:"icon,omitempty"`
	DeviceClass       string           `json:"device_class,omitempty"`
	StateClass        string           `json:"state_class,omitempty"`
	UnitOfMeasurement string           `json:"unit_of_measurement,omitempty"`
	PayloadOn         string           `json:"payload_on,omitempty"`
	PayloadOff        string           `json:"payload_off,omitempty"`
	EntityCategory    string           `json:"entity_category,omitempty"`
	Availability      []haAvailability `json:"availability"`
	AvailabilityMode  string           `json:"availability_mode"`
	Device            haDevice         `json:"device"`
}

type message struct {
	topic   string
	payload []byte
	qos     byte
	retain  bool
}

// discoveryMessages builds the discovery config for every entity and the switch.
func (s Settings) discoveryMessages(specs []SensorSpec) []message {
	dev := haDevice{
		Identifiers:      []string{s.NodeID},
		Name:             s.deviceName(),
		Manufacturer:     s.Device.Manufacturer,
		Model:            s.Device.Model,
		HwVersion:        s.Device.HwVersion,
		SwVersion:        s.Device.SwVersion,
		ConfigurationURL: s.Device.ConfigurationURL,
	}
	var msgs []message
	for _, spec := range specs {
		for _, kind := range spec.Kinds {
			msgs = append(msgs, s.sensorDiscovery(spec, kind, dev))
		}
	}
	msgs = append(msgs, s.switchDiscovery(dev), s.screenPowerDiscovery(dev))
	msgs = append(msgs, s.hostMetricDiscoveries(dev)...)
	msgs = append(msgs, s.hostInfoDiscoveries(dev)...)
	msgs = append(msgs, s.powerDiscoveries(dev)...)
	return msgs
}

// Hostname and IP are the two facts you need to reach the frame, and HA's device
// card has nowhere to show them.
func (s Settings) hostInfoDiscoveries(dev haDevice) []message {
	avail := []haAvailability{{Topic: s.bridgeAvailTopic()}}
	host := discoveryConfig{
		UniqueID: s.NodeID + "_hostname", Name: "Hostname",
		StateTopic: s.hostnameStateTopic(), Icon: "mdi:server",
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	ip := discoveryConfig{
		UniqueID: s.NodeID + "_ip_address", Name: "IP Address",
		StateTopic: s.ipStateTopic(), Icon: "mdi:ip-network",
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	return []message{
		discoveryMessage(s.discoveryTopic("sensor", "hostname"), host),
		discoveryMessage(s.discoveryTopic("sensor", "ip_address"), ip),
	}
}

func (s Settings) powerDiscoveries(dev haDevice) []message {
	avail := []haAvailability{{Topic: s.bridgeAvailTopic()}}
	reboot := discoveryConfig{
		UniqueID: s.NodeID + "_reboot", Name: "Reboot",
		CommandTopic: s.rebootCommandTopic(), DeviceClass: "restart", Icon: "mdi:restart",
		Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	shutdown := discoveryConfig{
		UniqueID: s.NodeID + "_shutdown", Name: "Shutdown",
		CommandTopic: s.shutdownCommandTopic(), Icon: "mdi:power",
		Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	return []message{
		buttonMessage(s.discoveryTopic("button", "reboot"), reboot, s.CanReboot),
		buttonMessage(s.discoveryTopic("button", "shutdown"), shutdown, s.CanPowerOff),
	}
}

// An empty retained payload makes HA drop the entity, so a frame that loses the
// polkit rule stops advertising a button that could only fail.
func buttonMessage(topic string, cfg discoveryConfig, permitted bool) message {
	if !permitted {
		return message{topic: topic, payload: nil, qos: 1, retain: true}
	}
	return discoveryMessage(topic, cfg)
}

// hostMetricDiscoveries advertises the Pi telemetry, plus undervoltage when vcgencmd
// is available. Bridge availability alone gates them: a live frame means live values.
func (s Settings) hostMetricDiscoveries(dev haDevice) []message {
	avail := []haAvailability{{Topic: s.bridgeAvailTopic()}}

	temp := discoveryConfig{
		UniqueID: s.NodeID + "_cpu_temperature", Name: "CPU Temperature",
		StateTopic: s.cpuTempStateTopic(), DeviceClass: "temperature",
		UnitOfMeasurement: sensorUnits["temperature"], StateClass: "measurement",
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	mem := discoveryConfig{
		UniqueID: s.NodeID + "_memory_usage", Name: "Memory Usage",
		StateTopic: s.memoryStateTopic(), UnitOfMeasurement: "%", StateClass: "measurement",
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	uptime := discoveryConfig{
		UniqueID: s.NodeID + "_uptime", Name: "Uptime",
		StateTopic: s.uptimeStateTopic(), DeviceClass: "timestamp", // boot time; HA renders "x days"
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	touch := discoveryConfig{
		UniqueID: s.NodeID + "_last_touch", Name: "Last Touch",
		StateTopic: s.lastTouchStateTopic(), DeviceClass: "timestamp",
		EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
	}
	msgs := []message{
		discoveryMessage(s.discoveryTopic("sensor", "cpu_temperature"), temp),
		discoveryMessage(s.discoveryTopic("sensor", "memory_usage"), mem),
		discoveryMessage(s.discoveryTopic("sensor", "uptime"), uptime),
		discoveryMessage(s.discoveryTopic("sensor", "last_touch"), touch),
	}
	if s.Undervoltage {
		uv := discoveryConfig{
			UniqueID: s.NodeID + "_undervoltage", Name: "Undervoltage",
			StateTopic: s.undervoltageStateTopic(), DeviceClass: "problem",
			PayloadOn: payloadOn, PayloadOff: payloadOff,
			EntityCategory: diagnosticCategory, Availability: avail, AvailabilityMode: "all", Device: dev,
		}
		msgs = append(msgs, discoveryMessage(s.discoveryTopic("binary_sensor", "undervoltage"), uv))
	}
	return msgs
}

func (s Settings) sensorDiscovery(spec SensorSpec, kind string, dev haDevice) message {
	cfg := discoveryConfig{
		UniqueID:    s.NodeID + "_" + spec.ID + "_" + kind,
		Name:        entityName(spec.Role, spec.ID, kind),
		StateTopic:  s.stateTopic(spec.ID, kind),
		DeviceClass: kind,
		// availability_mode "all": a dead frame (bridge LWT) or a stale sensor
		// (freshness topic) flips the entity unavailable instead of showing stale data.
		Availability: []haAvailability{
			{Topic: s.bridgeAvailTopic()},
			{Topic: s.sensorAvailTopic(spec.ID)},
		},
		AvailabilityMode: "all",
		Device:           dev,
	}
	switch kind {
	case "motion":
		cfg.PayloadOn, cfg.PayloadOff = payloadOn, payloadOff
	default:
		cfg.StateClass = "measurement"
		cfg.UnitOfMeasurement = sensorUnits[kind]
	}
	return discoveryMessage(s.discoveryTopic(component(kind), spec.ID+"_"+kind), cfg)
}

func (s Settings) switchDiscovery(dev haDevice) message {
	cfg := discoveryConfig{
		UniqueID:         s.NodeID + "_screen",
		Name:             "Screen",
		StateTopic:       s.switchStateTopic(),
		CommandTopic:     s.switchSetTopic(),
		PayloadOn:        payloadOn,
		PayloadOff:       payloadOff,
		Availability:     []haAvailability{{Topic: s.bridgeAvailTopic()}},
		AvailabilityMode: "all",
		Device:           dev,
	}
	return discoveryMessage(s.discoveryTopic("switch", "screen"), cfg)
}

// screenPowerDiscovery is the read-only live-power companion to the intent switch.
// device_class "running" renders as Running/Not running; bridge availability only.
func (s Settings) screenPowerDiscovery(dev haDevice) message {
	cfg := discoveryConfig{
		UniqueID:         s.NodeID + "_screen_power",
		Name:             "Screen Power",
		StateTopic:       s.screenPowerStateTopic(),
		DeviceClass:      "running",
		PayloadOn:        payloadOn,
		PayloadOff:       payloadOff,
		Availability:     []haAvailability{{Topic: s.bridgeAvailTopic()}},
		AvailabilityMode: "all",
		Device:           dev,
	}
	return discoveryMessage(s.discoveryTopic("binary_sensor", "screen_power"), cfg)
}

func discoveryMessage(topic string, cfg discoveryConfig) message {
	payload, _ := json.Marshal(cfg) // discoveryConfig has no unmarshalable fields
	return message{topic: topic, payload: payload, qos: 1, retain: true}
}

// sensorUnits gives each measurement kind its HA unit.
var sensorUnits = map[string]string{
	"temperature": "°C",
	"humidity":    "%",
}

func entityName(role, id, kind string) string {
	label := role
	if label == "" {
		label = id
	}
	return titleCase(label + " " + kind)
}

// titleCase capitalises each word, splitting on spaces and underscores
// ("living_room temperature" → "Living Room Temperature").
func titleCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '_' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
