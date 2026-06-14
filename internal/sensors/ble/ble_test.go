package ble_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/sensors/ble"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type mockAdapter struct {
	mu         sync.Mutex
	dev        *mockDevice
	err        error
	resetCalls int
}

func (m *mockAdapter) Connect(_ context.Context, _, _ string, _ []string) (ble.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.dev, nil
}

func (m *mockAdapter) Reset(_ context.Context) error {
	m.mu.Lock()
	m.resetCalls++
	m.mu.Unlock()
	return nil
}

func (m *mockAdapter) resets() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resetCalls
}

type mockDevice struct {
	mu           sync.Mutex
	readings     map[string][]byte
	readErr      error
	subs         map[string]func([]byte)
	disconnected bool
}

func newMockDevice(readings map[string][]byte) *mockDevice {
	return &mockDevice{
		readings: readings,
		subs:     map[string]func([]byte){},
	}
}

func (m *mockDevice) Read(uuid string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	data, ok := m.readings[uuid]
	if !ok {
		return nil, errors.New("uuid not found: " + uuid)
	}
	return data, nil
}

func (m *mockDevice) Subscribe(uuid string, handler func([]byte)) error {
	m.mu.Lock()
	m.subs[uuid] = handler
	m.mu.Unlock()
	return nil
}

func (m *mockDevice) Disconnect() error {
	m.mu.Lock()
	m.disconnected = true
	m.mu.Unlock()
	return nil
}

// notify simulates an inbound GATT notification.
func (m *mockDevice) notify(uuid string, data []byte) {
	m.mu.Lock()
	h := m.subs[uuid]
	m.mu.Unlock()
	if h != nil {
		h(data)
	}
}

const tempUUID = "00002a6e-0000-1000-8000-00805f9b34fb"
const motionUUID = "0000a001-0000-1000-8000-00805f9b34fb"

var testCfg = config.SensorConfig{
	ID:          "living_room",
	Type:        "ble",
	MAC:         "D5:CE:E6:61:5B:1D",
	AddressType: "random",
	Characteristics: []config.CharacteristicConfig{
		{UUID: tempUUID, Kind: "temperature", Decoder: "int16le_div100"},
		{UUID: motionUUID, Kind: "motion", Decoder: "bool_nonzero"},
	},
}

func newSource(t *testing.T, cfg config.SensorConfig, adapter ble.Adapter) *ble.Source {
	t.Helper()
	src, err := ble.New(testutil.NopLogger(), cfg, adapter)
	if err != nil {
		t.Fatalf("ble.New: %v", err)
	}
	return src
}

func TestSourceID(t *testing.T) {
	src := newSource(t, testCfg, &mockAdapter{})
	if src.ID() != "living_room" {
		t.Errorf("ID: got %q, want living_room", src.ID())
	}
}

func TestNewUnknownDecoder(t *testing.T) {
	cfg := config.SensorConfig{
		ID: "x", Type: "ble", MAC: "AA:BB:CC:DD:EE:FF",
		Characteristics: []config.CharacteristicConfig{
			{UUID: tempUUID, Kind: "temperature", Decoder: "nonexistent"},
		},
	}
	_, err := ble.New(testutil.NopLogger(), cfg, &mockAdapter{})
	if err == nil {
		t.Fatal("expected error for unknown decoder")
	}
}

func TestStartConnectError(t *testing.T) {
	src := newSource(t, testCfg, &mockAdapter{err: errors.New("no device")})
	err := src.Start(context.Background(), make(chan sensors.Reading, 8))
	if err == nil {
		t.Fatal("expected error from connect failure")
	}
}

func TestStartContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dev := newMockDevice(map[string][]byte{
			tempUUID:   {0x29, 0x09}, // 23.45°C
			motionUUID: {0x00},
		})
		src := newSource(t, testCfg, &mockAdapter{dev: dev})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- src.Start(ctx, make(chan sensors.Reading, 8)) }()
		synctest.Wait() // connected, subscribed, blocked on the poll ticker

		cancel()
		if err := <-done; err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
		if !dev.disconnected {
			t.Error("expected Disconnect to be called on exit")
		}
	})
}

func TestStartPollDeliversReadings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dev := newMockDevice(map[string][]byte{
			tempUUID:   {0x29, 0x09}, // 23.45°C as little-endian int16 2345
			motionUUID: {0x01},
		})
		cfg := testCfg
		cfg.PollInterval = config.Duration{Duration: time.Minute}
		out := make(chan sensors.Reading, 8)
		src := newSource(t, cfg, &mockAdapter{dev: dev})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { _ = src.Start(ctx, out) }()
		synctest.Wait()

		time.Sleep(cfg.PollInterval.Duration) // advance to the first poll tick
		synctest.Wait()

		got := map[sensors.Kind]float64{}
		for len(got) < 2 {
			r := <-out
			got[r.Kind] = r.Value
		}

		if got[sensors.KindTemperature] != 23.45 {
			t.Errorf("temperature: got %v, want 23.45", got[sensors.KindTemperature])
		}
		if got[sensors.KindMotion] != 1 {
			t.Errorf("motion: got %v, want 1", got[sensors.KindMotion])
		}
	})
}

func TestStartSubscriptionDeliversReadings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dev := newMockDevice(map[string][]byte{
			tempUUID:   {0x00, 0x00},
			motionUUID: {0x00},
		})
		out := make(chan sensors.Reading, 8)
		src := newSource(t, testCfg, &mockAdapter{dev: dev})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() { _ = src.Start(ctx, out) }()
		synctest.Wait() // subscribed to every characteristic

		dev.notify(motionUUID, []byte{0x01})

		r := <-out
		if r.Kind != sensors.KindMotion || r.Value != 1 {
			t.Errorf("got %+v, want motion=1", r)
		}
	})
}

func TestAdapterResetAfterProlongedFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		adapter := &mockAdapter{err: errors.New("no device")}
		cfg := testCfg
		cfg.ResetAfter = config.Duration{Duration: 20 * time.Second}
		src := newSource(t, cfg, adapter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out := make(chan sensors.Reading, 8)

		_ = src.Start(ctx, out)      // records firstFailure; no reset yet
		time.Sleep(30 * time.Second) // cross the reset threshold
		_ = src.Start(ctx, out)      // now Reset() fires

		if adapter.resets() < 1 {
			t.Fatalf("expected at least one adapter reset, got %d", adapter.resets())
		}
	})
}

func TestStartPollReadErrorSignalsDisconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dev := newMockDevice(map[string][]byte{
			tempUUID:   {0x00, 0x00},
			motionUUID: {0x00},
		})
		cfg := testCfg
		cfg.PollInterval = config.Duration{Duration: time.Minute}
		src := newSource(t, cfg, &mockAdapter{dev: dev})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() { done <- src.Start(ctx, make(chan sensors.Reading, 8)) }()
		synctest.Wait()

		dev.mu.Lock()
		dev.readErr = errors.New("device disconnected")
		dev.mu.Unlock()
		time.Sleep(cfg.PollInterval.Duration) // advance to the failing poll

		if err := <-done; err == nil {
			t.Fatal("expected error from poll read failure")
		}
	})
}

// resetAfter == 0 (the default) disables power cycling: no amount of failure
// may trigger a reset.
func TestAdapterResetDisabledByDefault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		adapter := &mockAdapter{err: errors.New("no device")}
		src := newSource(t, testCfg, adapter) // testCfg has no ResetAfter

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out := make(chan sensors.Reading, 8)

		_ = src.Start(ctx, out)
		time.Sleep(time.Hour) // far beyond any threshold
		_ = src.Start(ctx, out)

		if adapter.resets() != 0 {
			t.Fatalf("reset fired with resetAfter disabled: %d", adapter.resets())
		}
	})
}
