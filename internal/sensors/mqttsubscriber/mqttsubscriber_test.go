package mqttsubscriber_test

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/sensors/mqttsubscriber"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// fakeHub captures the registered handler so tests can deliver payloads.
type fakeHub struct {
	mu      sync.Mutex
	topic   string
	qos     byte
	handler func([]byte)
}

func (h *fakeHub) Subscribe(topic string, qos byte, handler func([]byte)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.topic, h.qos, h.handler = topic, qos, handler
}

func (h *fakeHub) deliver(payload []byte) {
	h.mu.Lock()
	hh := h.handler
	h.mu.Unlock()
	if hh != nil {
		hh(payload)
	}
}

func validCfg() config.SensorConfig {
	return config.SensorConfig{
		ID:     "ha_inside_temp",
		Type:   "mqtt-subscriber",
		Role:   "inside",
		Topic:  "homeassistant/sensor/inside/temperature/state",
		Kind:   "temperature",
		Parser: "raw_float",
	}
}

func TestNewRejectsUnknownParser(t *testing.T) {
	cfg := validCfg()
	cfg.Parser = "nonexistent"
	if _, err := mqttsubscriber.New(testutil.NopLogger(), cfg, &fakeHub{}); err == nil {
		t.Fatal("expected error for unknown parser")
	}
}

func TestNewSubscribesToConfiguredTopic(t *testing.T) {
	hub := &fakeHub{}
	if _, err := mqttsubscriber.New(testutil.NopLogger(), validCfg(), hub); err != nil {
		t.Fatal(err)
	}
	if hub.topic != "homeassistant/sensor/inside/temperature/state" {
		t.Errorf("subscribed topic: %q", hub.topic)
	}
}

func TestEmitsReadingForRawPayload(t *testing.T) {
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), validCfg(), hub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan sensors.Reading, 1)
	go func() { _ = src.Start(ctx, out) }()

	hub.deliver([]byte("21.5"))
	select {
	case r := <-out:
		if r.DeviceID != "ha_inside_temp" || r.Kind != sensors.KindTemperature || r.Value != 21.5 {
			t.Errorf("reading: %+v", r)
		}
		if r.Timestamp.IsZero() {
			t.Error("timestamp not set")
		}
	case <-time.After(time.Second):
		t.Fatal("no reading emitted")
	}
}

func TestEmitsReadingForJSONPayload(t *testing.T) {
	cfg := validCfg()
	cfg.JSONField = "main.temp"
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), cfg, hub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan sensors.Reading, 1)
	go func() { _ = src.Start(ctx, out) }()

	hub.deliver([]byte(`{"main": {"temp": 22.7, "humidity": 48}}`))
	select {
	case r := <-out:
		if r.Value != 22.7 {
			t.Errorf("dotted-path extract failed: %v", r.Value)
		}
	case <-time.After(time.Second):
		t.Fatal("no reading emitted")
	}
}

func TestEmitsMotionFromOnOff(t *testing.T) {
	cfg := validCfg()
	cfg.Kind = "motion"
	cfg.Parser = "onoff_to_bool"
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), cfg, hub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan sensors.Reading, 2)
	go func() { _ = src.Start(ctx, out) }()

	hub.deliver([]byte("ON"))
	r := mustReceive(t, out)
	if r.Value != 1 {
		t.Errorf("ON: got %v, want 1", r.Value)
	}
	hub.deliver([]byte("OFF"))
	r = mustReceive(t, out)
	if r.Value != 0 {
		t.Errorf("OFF: got %v, want 0", r.Value)
	}
}

func TestEmitsBoolFromJSONField(t *testing.T) {
	cfg := validCfg()
	cfg.Kind = "motion"
	cfg.Parser = "onoff_to_bool"
	cfg.JSONField = "motion"
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), cfg, hub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan sensors.Reading, 1)
	go func() { _ = src.Start(ctx, out) }()

	hub.deliver([]byte(`{"motion": true}`))
	r := mustReceive(t, out)
	if r.Value != 1 {
		t.Errorf("JSON true: got %v, want 1", r.Value)
	}
}

func TestDropsOnDecodeError(t *testing.T) {
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), validCfg(), hub)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(chan sensors.Reading, 1)
	go func() { _ = src.Start(ctx, out) }()

	hub.deliver([]byte("not-a-number"))
	select {
	case r := <-out:
		t.Errorf("expected drop, got %+v", r)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDropsOnJSONExtractError(t *testing.T) {
	cases := []struct {
		name    string
		field   string
		payload string
	}{
		{"invalid json", "main.temp", "not-json"},
		{"missing field", "main.missing", `{"main": {"temp": 1}}`},
		{"non-object at path", "main.temp.x", `{"main": {"temp": 1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validCfg()
			cfg.JSONField = tc.field
			hub := &fakeHub{}
			src, _ := mqttsubscriber.New(testutil.NopLogger(), cfg, hub)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			out := make(chan sensors.Reading, 1)
			go func() { _ = src.Start(ctx, out) }()

			hub.deliver([]byte(tc.payload))
			select {
			case r := <-out:
				t.Errorf("expected drop, got %+v", r)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}

// Reaches the formatLeaf branches via the public API.
func TestLeafFormatsForJSONScalars(t *testing.T) {
	cases := []struct {
		name     string
		field    string
		parser   string
		payload  string
		expected float64
	}{
		{"string scalar", "v", "raw_float", `{"v": "9.5"}`, 9.5},
		{"bool false", "v", "onoff_to_bool", `{"v": false}`, 0},
		{"array via remarshal", "v", "raw_int", `{"v": 7}`, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validCfg()
			cfg.JSONField = tc.field
			cfg.Parser = tc.parser
			hub := &fakeHub{}
			src, _ := mqttsubscriber.New(testutil.NopLogger(), cfg, hub)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			out := make(chan sensors.Reading, 1)
			go func() { _ = src.Start(ctx, out) }()

			hub.deliver([]byte(tc.payload))
			r := mustReceive(t, out)
			if r.Value != tc.expected {
				t.Errorf("value: got %v, want %v", r.Value, tc.expected)
			}
		})
	}
}

func TestStartReturnsOnCtxCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := &fakeHub{}
		src, _ := mqttsubscriber.New(testutil.NopLogger(), validCfg(), hub)

		ctx, cancel := context.WithCancel(t.Context())
		out := make(chan sensors.Reading, 1)
		done := make(chan struct{})
		go func() {
			_ = src.Start(ctx, out)
			close(done)
		}()
		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Error("Start did not return after ctx cancel")
		}
	})
}

// The handler must only touch the Source-owned internal channel; closing `out`
// after Start returns would panic if it ever wrote there.
func TestHandlerNeverWritesToDownstream(t *testing.T) {
	hub := &fakeHub{}
	src, _ := mqttsubscriber.New(testutil.NopLogger(), validCfg(), hub)
	ctx, cancel := context.WithCancel(t.Context())
	out := make(chan sensors.Reading, 1)
	done := make(chan struct{})
	go func() {
		_ = src.Start(ctx, out)
		close(done)
	}()
	cancel()
	<-done
	close(out)
	hub.deliver([]byte("21.5"))
}

func mustReceive(t *testing.T, out <-chan sensors.Reading) sensors.Reading {
	t.Helper()
	select {
	case r := <-out:
		return r
	case <-time.After(time.Second):
		t.Fatal("no reading received")
		return sensors.Reading{}
	}
}
