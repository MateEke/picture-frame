// Package mqtt bridges the state bus to MQTT with Home Assistant auto-discovery:
// it publishes BLE sensors, a screen intent switch plus a live-power binary_sensor,
// and routes screen commands back.
package mqtt

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
)

// Client is the minimal MQTT transport the Hub needs (paho in the adapter
// subpackage). OnConnect must fire on every (re)connect.
type Client interface {
	OnConnect(func())
	OnConnectionLost(func(err error))
	Connect() error
	Disconnect()
	Publish(topic string, qos byte, retain bool, payload []byte) error
	Subscribe(topic string, qos byte, handler func(payload []byte)) error
}

// Screen is the manual screen-control surface; implemented by *display.Screen.
type Screen interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
	Auto() bool  // intent: motion auto-wake enabled (the switch)
	State() bool // live panel power (the binary_sensor)
}

// Publisher mirrors bus state to MQTT and applies inbound screen commands.
type Publisher struct {
	log    *slog.Logger
	hub    *Hub
	bus    *state.Bus
	screen Screen
	set    Settings
	specs  []SensorSpec
	disc   []message

	known map[string]bool // advertised sensor IDs; readings for others are dropped

	// reconnected hops paho's onConnect goroutine to Run's; buffer 1 collapses
	// in-flight connects to a single republish.
	reconnected chan struct{}

	// Touched only by the Run goroutine (readings, stale timers, and the
	// reconnect signal are all funnelled through its select), so no locking.
	timers   map[string]*time.Timer // per-sensor staleness timers
	online   map[string]bool        // availability currently published, per sensor
	lastSeen map[string]time.Time   // last reading time; guards the stale check
}

// New registers with the Hub at construction so a connect that beats Run still
// queues a republish.
func New(log *slog.Logger, hub *Hub, bus *state.Bus, screen Screen, set Settings, specs []SensorSpec) *Publisher {
	known := make(map[string]bool, len(specs))
	for _, s := range specs {
		known[s.ID] = true
	}
	p := &Publisher{
		log:         log,
		hub:         hub,
		bus:         bus,
		screen:      screen,
		set:         set,
		specs:       specs,
		disc:        set.discoveryMessages(specs),
		known:       known,
		reconnected: make(chan struct{}, 1),
		timers:      make(map[string]*time.Timer),
		online:      make(map[string]bool),
		lastSeen:    make(map[string]time.Time),
	}
	hub.AddOnConnect(func() {
		select {
		case p.reconnected <- struct{}{}:
		default: // a republish is already pending; one covers this connect
		}
	})
	hub.Subscribe(set.switchSetTopic(), 1, p.handleCommand)
	return p
}

// Run mirrors bus events to MQTT until ctx is cancelled. Bridge-offline is
// published on exit only if a connection was ever established.
func (p *Publisher) Run(ctx context.Context) {
	stale := make(chan string)

	ch, unsub := p.bus.Subscribe()
	defer unsub()

	var everConnected bool
	for {
		select {
		case <-ctx.Done():
			if everConnected {
				p.shutdown()
			}
			return
		case <-p.reconnected:
			everConnected = true
			p.republish()
		case id := <-stale:
			p.markStale(id)
		case e, ok := <-ch:
			if !ok {
				return
			}
			p.handleEvent(ctx, e, stale)
		}
	}
}

// republish restores retained state after a (re)connect. Sensors stay offline
// until a reading arrives so HA never shows a value before one exists.
func (p *Publisher) republish() {
	for _, m := range p.disc {
		p.pub(m)
	}
	p.publish(p.set.bridgeAvailTopic(), availOnline, 1, true)
	p.publishSwitch(p.screen.Auto())
	p.publishScreenPower(p.screen.State())
	for _, spec := range p.specs {
		p.publish(p.set.sensorAvailTopic(spec.ID), availabilityWord(p.online[spec.ID]), 1, true)
	}
}

func (p *Publisher) handleEvent(ctx context.Context, e state.Event, stale chan<- string) {
	switch pl := e.Payload.(type) {
	case state.SensorPayload:
		if !p.known[pl.DeviceID] {
			return // only bridge advertised (BLE) sensors
		}
		p.publish(p.set.stateTopic(pl.DeviceID, pl.Kind), formatValue(pl.Kind, pl.Value), 0, false)
		p.markFresh(ctx, pl.DeviceID, stale)
	case state.ScreenPayload:
		// Auto drives the intent switch; On drives the live-power binary_sensor.
		p.publishSwitch(pl.Auto)
		p.publishScreenPower(pl.On)
	}
}

// markFresh brings a sensor online and re-arms its staleness timer. A
// non-positive StaleAfter disables expiry (stays online once seen).
func (p *Publisher) markFresh(ctx context.Context, id string, stale chan<- string) {
	p.lastSeen[id] = time.Now()
	if !p.online[id] {
		p.online[id] = true
		p.publish(p.set.sensorAvailTopic(id), availOnline, 1, true)
	}
	if p.set.StaleAfter <= 0 {
		return
	}
	if t := p.timers[id]; t != nil {
		t.Stop()
	}
	p.timers[id] = time.AfterFunc(p.set.StaleAfter, func() {
		select {
		case stale <- id:
		case <-ctx.Done():
		}
	})
}

// markStale takes a sensor offline once stale. The lastSeen check ignores a
// timer that a fresh reading just superseded, avoiding an offline flap.
func (p *Publisher) markStale(id string) {
	if p.online[id] && time.Since(p.lastSeen[id]) >= p.set.StaleAfter {
		p.online[id] = false
		p.publish(p.set.sensorAvailTopic(id), availOffline, 1, true)
	}
}

// handleCommand actuates the screen; the resulting KindScreen bus event echoes
// the new state back to the switch topic, so HA updates exactly once.
func (p *Publisher) handleCommand(payload []byte) {
	switch strings.ToUpper(strings.TrimSpace(string(payload))) {
	case payloadOn:
		if err := p.screen.On(context.Background()); err != nil {
			p.log.Warn("mqtt: screen on failed", "err", err)
		}
	case payloadOff:
		if err := p.screen.Off(context.Background()); err != nil {
			p.log.Warn("mqtt: screen off failed", "err", err)
		}
	default:
		p.log.Warn("mqtt: unknown screen command", "payload", string(payload))
	}
}

// shutdown publishes bridge-offline and stops staleness timers; the Hub owns
// disconnect.
func (p *Publisher) shutdown() {
	p.publish(p.set.bridgeAvailTopic(), availOffline, 1, true)
	for _, t := range p.timers {
		t.Stop()
	}
}

func (p *Publisher) publishSwitch(on bool) {
	p.publish(p.set.switchStateTopic(), boolWord(on), 1, true)
}

func (p *Publisher) publishScreenPower(on bool) {
	p.publish(p.set.screenPowerStateTopic(), boolWord(on), 1, true)
}

func (p *Publisher) pub(m message) {
	p.publishBytes(m.topic, m.payload, m.qos, m.retain)
}

func (p *Publisher) publish(topic, payload string, qos byte, retain bool) {
	p.publishBytes(topic, []byte(payload), qos, retain)
}

func (p *Publisher) publishBytes(topic string, payload []byte, qos byte, retain bool) {
	if err := p.hub.Publish(topic, qos, retain, payload); err != nil {
		p.log.Warn("mqtt: publish failed", "topic", topic, "err", err)
	}
}

func formatValue(kind string, v float64) string {
	if kind == "motion" {
		return boolWord(v != 0)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func boolWord(on bool) string {
	if on {
		return payloadOn
	}
	return payloadOff
}

func availabilityWord(online bool) string {
	if online {
		return availOnline
	}
	return availOffline
}
