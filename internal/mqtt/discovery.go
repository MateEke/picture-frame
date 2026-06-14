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
}

// SensorSpec is one sensor and the kinds it emits, used to build HA entities
// before any reading arrives.
type SensorSpec struct {
	ID    string   // matches Reading.DeviceID and the config sensor id
	Role  string   // friendly label; falls back to ID
	Kinds []string // "temperature" | "humidity" | "motion"
}

const (
	availOnline  = "online"
	availOffline = "offline"
	payloadOn    = "ON"
	payloadOff   = "OFF"
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
	Identifiers []string `json:"identifiers"`
	Name        string   `json:"name"`
}

type haAvailability struct {
	Topic string `json:"topic"`
}

// discoveryConfig is the HA discovery payload; omitempty lets sensor and switch
// share one struct.
type discoveryConfig struct {
	UniqueID          string           `json:"unique_id"`
	Name              string           `json:"name"`
	StateTopic        string           `json:"state_topic"`
	CommandTopic      string           `json:"command_topic,omitempty"`
	DeviceClass       string           `json:"device_class,omitempty"`
	StateClass        string           `json:"state_class,omitempty"`
	UnitOfMeasurement string           `json:"unit_of_measurement,omitempty"`
	PayloadOn         string           `json:"payload_on,omitempty"`
	PayloadOff        string           `json:"payload_off,omitempty"`
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
	dev := haDevice{Identifiers: []string{s.NodeID}, Name: s.deviceName()}
	var msgs []message
	for _, spec := range specs {
		for _, kind := range spec.Kinds {
			msgs = append(msgs, s.sensorDiscovery(spec, kind, dev))
		}
	}
	msgs = append(msgs, s.switchDiscovery(dev), s.screenPowerDiscovery(dev))
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
