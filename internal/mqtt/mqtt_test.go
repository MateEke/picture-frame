package mqtt

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type pubRec struct {
	topic   string
	qos     byte
	retain  bool
	payload string
}

type fakeClient struct {
	mu               sync.Mutex
	pubs             []pubRec
	onConnect        func()
	onConnectionLost func(err error)
	subs             map[string]func([]byte)
	connectFails     int // transient failures before the next Connect succeeds
	subErr           error
	pubErr           error
	disconnected     bool
}

func (c *fakeClient) OnConnect(f func())             { c.onConnect = f }
func (c *fakeClient) OnConnectionLost(f func(error)) { c.onConnectionLost = f }

func (c *fakeClient) Connect() error {
	c.mu.Lock()
	if c.connectFails > 0 {
		c.connectFails--
		c.mu.Unlock()
		return errors.New("broker unreachable")
	}
	c.mu.Unlock()
	if c.onConnect != nil {
		c.onConnect()
	}
	return nil
}

func (c *fakeClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnected = true
}

func (c *fakeClient) Publish(topic string, qos byte, retain bool, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pubErr != nil {
		return c.pubErr
	}
	c.pubs = append(c.pubs, pubRec{topic, qos, retain, string(payload)})
	return nil
}

func (c *fakeClient) Subscribe(topic string, _ byte, h func([]byte)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subErr != nil {
		return c.subErr
	}
	if c.subs == nil {
		c.subs = make(map[string]func([]byte))
	}
	c.subs[topic] = h
	return nil
}

// lastPayload returns the most recent payload published to topic.
func (c *fakeClient) lastPayload(topic string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range slices.Backward(c.pubs) {
		if v.topic == topic {
			return v.payload, true
		}
	}
	return "", false
}

func (c *fakeClient) countTopic(topic string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, p := range c.pubs {
		if p.topic == topic {
			n++
		}
	}
	return n
}

// fakeScreen mirrors display.Screen: On/Off flip both intent (auto) and live
// power (on) and publish a KindScreen event so both echo back through the bus.
type fakeScreen struct {
	bus  *state.Bus
	auto bool
	on   bool
	ons  int
	offs int
	err  error
}

func (s *fakeScreen) On(context.Context) error {
	if s.err != nil {
		return s.err
	}
	s.ons++
	s.auto, s.on = true, true
	s.bus.Publish(state.Event{Kind: state.KindScreen, Payload: state.ScreenPayload{On: true, Auto: true}})
	return nil
}

func (s *fakeScreen) Off(context.Context) error {
	if s.err != nil {
		return s.err
	}
	s.offs++
	s.auto, s.on = false, false
	s.bus.Publish(state.Event{Kind: state.KindScreen, Payload: state.ScreenPayload{On: false, Auto: false}})
	return nil
}

func (s *fakeScreen) Auto() bool  { return s.auto }
func (s *fakeScreen) State() bool { return s.on }

func newTestPublisher(client *fakeClient, screen Screen) *Publisher {
	hub := NewHub(testutil.NopLogger(), client)
	return New(testutil.NopLogger(), hub, state.NewBus(), screen, testSettings(), testSpecs())
}

// newTestPublisherWithHub returns both for tests that drive the connection explicitly.
func newTestPublisherWithHub(client *fakeClient, screen Screen, bus *state.Bus) (*Publisher, *Hub) {
	hub := NewHub(testutil.NopLogger(), client)
	return New(testutil.NopLogger(), hub, bus, screen, testSettings(), testSpecs()), hub
}

func sensorEvent(id, kind string, value float64) state.Event {
	return state.Event{
		Kind:    state.KindSensor,
		Payload: state.SensorPayload{DeviceID: id, Kind: kind, Value: value},
	}
}

func TestRunReturnsIfCancelledBeforeConnecting(t *testing.T) {
	client := &fakeClient{connectFails: 1_000_000} // never connects
	p := newTestPublisher(client, &fakeScreen{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	p.Run(ctx)

	if len(client.pubs) != 0 {
		t.Errorf("nothing should be published before connecting, got %v", client.pubs)
	}
	if client.disconnected {
		t.Error("Disconnect must not run when never connected")
	}
}

func TestRunRetriesInitialConnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{connectFails: 2}
		bus := state.NewBus()
		p, hub := newTestPublisherWithHub(client, &fakeScreen{bus: bus}, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)
		go hub.Connect(ctx)

		// Two failures, then connect succeeds after the retry backoffs elapse.
		time.Sleep(2*connectRetry + time.Second)
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/availability"); v != "online" {
			t.Errorf("expected bridge online after retries, got %q", v)
		}
	})
}

func TestRepublishPublishesDiscoveryAvailabilityAndSwitch(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{auto: true, on: true})
	p.republish()

	if _, ok := client.lastPayload("homeassistant/sensor/picture_frame/living_room_temperature/config"); !ok {
		t.Error("temperature discovery not published")
	}
	if v, _ := client.lastPayload("picture-frame/availability"); v != "online" {
		t.Errorf("bridge availability: %q, want online", v)
	}
	if v, _ := client.lastPayload("picture-frame/switch/screen/state"); v != "ON" {
		t.Errorf("switch state: %q, want ON (auto)", v)
	}
	if v, _ := client.lastPayload("picture-frame/binary_sensor/screen_power/state"); v != "ON" {
		t.Errorf("live power state: %q, want ON", v)
	}
	// Sensors start offline until a reading arrives.
	if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "offline" {
		t.Errorf("per-sensor availability: %q, want offline initially", v)
	}
}

func TestSubscribesToScreenCommandOnConnect(t *testing.T) {
	client := &fakeClient{}
	_ = newTestPublisher(client, &fakeScreen{})
	client.onConnect()
	if client.subs["picture-frame/switch/screen/set"] == nil {
		t.Error("did not subscribe to screen command topic on connect")
	}
}

func TestRepublishSwitchOffWhenNotAuto(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{auto: false})
	p.republish()
	if v, _ := client.lastPayload("picture-frame/switch/screen/state"); v != "OFF" {
		t.Errorf("switch state: %q, want OFF", v)
	}
}

func TestSubscribeReplayErrorIsLogged(t *testing.T) {
	client := &fakeClient{subErr: errors.New("sub failed")}
	_ = newTestPublisher(client, &fakeScreen{})
	client.onConnect()
	if client.subs != nil {
		t.Error("subscription should not be recorded on error")
	}
}

func TestHandleSensorPublishesStateAndBringsOnline(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)

	p.handleEvent(context.Background(), sensorEvent("living_room", "temperature", 21.5), stale)

	if v, _ := client.lastPayload("picture-frame/sensor/living_room/temperature"); v != "21.5" {
		t.Errorf("state: %q, want 21.5", v)
	}
	if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
		t.Errorf("availability: %q, want online", v)
	}
}

func TestHandleSensorMotionFormatsOnOff(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)

	p.handleEvent(context.Background(), sensorEvent("living_room", "motion", 1), stale)
	if v, _ := client.lastPayload("picture-frame/sensor/living_room/motion"); v != "ON" {
		t.Errorf("motion: %q, want ON", v)
	}
	p.handleEvent(context.Background(), sensorEvent("living_room", "motion", 0), stale)
	if v, _ := client.lastPayload("picture-frame/sensor/living_room/motion"); v != "OFF" {
		t.Errorf("motion: %q, want OFF", v)
	}
}

func TestHandleScreenEventPublishesSwitchAndPower(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)
	screenEvent := func(on, auto bool) state.Event {
		return state.Event{Kind: state.KindScreen, Payload: state.ScreenPayload{On: on, Auto: auto}}
	}
	assertState := func(wantSwitch, wantPower string) {
		t.Helper()
		if v, _ := client.lastPayload("picture-frame/switch/screen/state"); v != wantSwitch {
			t.Errorf("switch: %q, want %q", v, wantSwitch)
		}
		if v, _ := client.lastPayload("picture-frame/binary_sensor/screen_power/state"); v != wantPower {
			t.Errorf("power: %q, want %q", v, wantPower)
		}
	}

	// Manual on: intent and live power both on.
	p.handleEvent(context.Background(), screenEvent(true, true), stale)
	assertState("ON", "ON")
	// Idle-blank: live power off, but intent (auto) stays on; the switch must NOT move.
	p.handleEvent(context.Background(), screenEvent(false, true), stale)
	assertState("ON", "OFF")
	// Manual off: both off.
	p.handleEvent(context.Background(), screenEvent(false, false), stale)
	assertState("OFF", "OFF")
}

func TestHandleSensorDropsUnadvertisedDevice(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{}) // specs cover "living_room" only
	stale := make(chan string, 1)

	// A mock/other sensor not in the spec set must not reach the broker.
	p.handleEvent(context.Background(), sensorEvent("mock_inside", "temperature", 22), stale)
	if len(client.pubs) != 0 {
		t.Errorf("unadvertised sensor must not publish, got %v", client.pubs)
	}
}

func TestHandleEventIgnoresOtherKinds(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)
	p.handleEvent(context.Background(), state.Event{Kind: state.KindImage, Payload: state.ImagePayload{Names: []string{"x.jpg"}}}, stale)
	if len(client.pubs) != 0 {
		t.Errorf("non sensor/screen events must not publish, got %v", client.pubs)
	}
}

func TestMarkFreshOnlineOnlyOnTransition(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)

	p.markFresh(context.Background(), "living_room", stale)
	p.markFresh(context.Background(), "living_room", stale)

	if n := client.countTopic("picture-frame/sensor/living_room/availability"); n != 1 {
		t.Errorf("online published %d times, want 1 (only on transition)", n)
	}
}

func TestMarkFreshStaleAfterDisabledSkipsTimer(t *testing.T) {
	client := &fakeClient{}
	set := testSettings()
	set.StaleAfter = 0
	hub := NewHub(testutil.NopLogger(), client)
	p := New(testutil.NopLogger(), hub, state.NewBus(), &fakeScreen{}, set, testSpecs())
	stale := make(chan string, 1)

	p.markFresh(context.Background(), "living_room", stale)
	if len(p.timers) != 0 {
		t.Errorf("no freshness timer expected when StaleAfter<=0, got %d", len(p.timers))
	}
}

func TestMarkStaleIgnoresUnknownAndFreshSensors(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{})
	stale := make(chan string, 1)

	// Never seen → no publish.
	p.markStale("living_room")
	if n := client.countTopic("picture-frame/sensor/living_room/availability"); n != 0 {
		t.Fatalf("offline should not publish for a never-seen sensor, got %d", n)
	}

	// A fresh reading, then a stale timer that fired against it (the reset race):
	// lastSeen is recent, so the sensor must stay online, not flap offline.
	p.markFresh(context.Background(), "living_room", stale)
	p.markStale("living_room")
	if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
		t.Errorf("availability: %q, want online (fresh reading supersedes the stale check)", v)
	}
	if n := client.countTopic("picture-frame/sensor/living_room/availability"); n != 1 {
		t.Errorf("availability published %d times, want 1 (online only)", n)
	}
}

func TestReconnectRepublishesCurrentAvailability(t *testing.T) {
	client := &fakeClient{}
	p := newTestPublisher(client, &fakeScreen{auto: true})
	stale := make(chan string, 1)

	// Sensor comes online, then the broker reconnects: republish must reflect the
	// sensor's *current* state (online), not reset it to offline.
	p.markFresh(context.Background(), "living_room", stale)
	p.republish()

	if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
		t.Errorf("reconnect availability: %q, want online", v)
	}
}

func TestHandleCommand(t *testing.T) {
	bus := state.NewBus()
	screen := &fakeScreen{bus: bus}
	client := &fakeClient{}
	hub := NewHub(testutil.NopLogger(), client)
	p := New(testutil.NopLogger(), hub, bus, screen, testSettings(), testSpecs())

	p.handleCommand([]byte("ON"))
	if screen.ons != 1 {
		t.Errorf("On calls: %d, want 1", screen.ons)
	}
	p.handleCommand([]byte(" off ")) // trimmed + case-insensitive
	if screen.offs != 1 {
		t.Errorf("Off calls: %d, want 1", screen.offs)
	}
	p.handleCommand([]byte("garbage")) // ignored, no panic
	if screen.ons != 1 || screen.offs != 1 {
		t.Errorf("unknown command must not actuate: ons=%d offs=%d", screen.ons, screen.offs)
	}
}

func TestHandleCommandScreenErrorIsLogged(t *testing.T) {
	bus := state.NewBus()
	screen := &fakeScreen{bus: bus, err: errors.New("vcgencmd failed")}
	hub := NewHub(testutil.NopLogger(), &fakeClient{})
	p := New(testutil.NopLogger(), hub, bus, screen, testSettings(), testSpecs())

	p.handleCommand([]byte("ON"))
	p.handleCommand([]byte("OFF"))
	if screen.ons != 0 || screen.offs != 0 {
		t.Errorf("failed actuation must not count: ons=%d offs=%d", screen.ons, screen.offs)
	}
}

func TestPublishErrorIsLogged(t *testing.T) {
	client := &fakeClient{pubErr: errors.New("broker down")}
	p := newTestPublisher(client, &fakeScreen{})
	p.publish("picture-frame/x", "y", 0, false) // must not panic
	if len(client.pubs) != 0 {
		t.Error("nothing recorded when publish errors")
	}
}

func TestRunLifecycleAndStaleness(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{}
		bus := state.NewBus()
		p, hub := newTestPublisherWithHub(client, &fakeScreen{bus: bus}, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)
		go hub.Connect(ctx)
		synctest.Wait()

		// Connected: discovery + bridge online published.
		if v, _ := client.lastPayload("picture-frame/availability"); v != "online" {
			t.Fatalf("bridge availability after connect: %q", v)
		}

		// A reading brings the sensor online.
		bus.Publish(sensorEvent("living_room", "temperature", 20))
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
			t.Fatalf("sensor availability after reading: %q", v)
		}

		// No further readings: after StaleAfter the sensor goes offline.
		time.Sleep(testSettings().StaleAfter + time.Second)
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "offline" {
			t.Fatalf("sensor availability after staleness: %q", v)
		}

		cancel()
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/availability"); v != "offline" {
			t.Errorf("bridge availability on shutdown: %q", v)
		}
		hub.Disconnect()
		if !client.disconnected {
			t.Error("expected Disconnect after Hub.Disconnect")
		}
	})
}

func TestRunReconnectRepublishesLiveState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{}
		bus := state.NewBus()
		p, hub := newTestPublisherWithHub(client, &fakeScreen{bus: bus}, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)
		go hub.Connect(ctx)
		synctest.Wait()

		bus.Publish(sensorEvent("living_room", "temperature", 20)) // sensor → online
		synctest.Wait()

		// Reconnect: republish must reflect live (online) state, not reset.
		client.onConnect()
		synctest.Wait()

		if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
			t.Errorf("after reconnect: %q, want online", v)
		}
	})
}

func TestRunStaleThenRecover(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{}
		bus := state.NewBus()
		p, hub := newTestPublisherWithHub(client, &fakeScreen{bus: bus}, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)
		go hub.Connect(ctx)
		synctest.Wait()

		bus.Publish(sensorEvent("living_room", "humidity", 50))
		synctest.Wait()
		time.Sleep(testSettings().StaleAfter + time.Second)
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "offline" {
			t.Fatalf("expected offline after staleness, got %q", v)
		}

		// A fresh reading brings it back online.
		bus.Publish(sensorEvent("living_room", "humidity", 51))
		synctest.Wait()
		if v, _ := client.lastPayload("picture-frame/sensor/living_room/availability"); v != "online" {
			t.Errorf("expected online after recovery, got %q", v)
		}
	})
}

func TestFormatValueFullPrecision(t *testing.T) {
	if got := formatValue("temperature", 21.25); got != "21.25" {
		t.Errorf("got %q, want 21.25 (precision must not be truncated)", got)
	}
	if got := formatValue("motion", 1); got != payloadOn {
		t.Errorf("motion 1: got %q, want %q", got, payloadOn)
	}
	if got := formatValue("motion", 0); got != payloadOff {
		t.Errorf("motion 0: got %q, want %q", got, payloadOff)
	}
}
