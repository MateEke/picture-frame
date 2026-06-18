package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// fakeLiveConfig records ApplyLive calls for assertion in tests.
type fakeLiveConfig struct {
	calls atomic.Int32
	last  config.Config
}

func (f *fakeLiveConfig) ApplyLive(cfg config.Config) {
	f.last = cfg
	f.calls.Add(1)
}

func makeConfigServer(t *testing.T, saved config.Config) (http.Handler, *fakeLiveConfig, *atomic.Int32) {
	t.Helper()
	lc := &fakeLiveConfig{}
	restartCalls := &atomic.Int32{}
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		LiveConfig:    lc,
		Restart: func() error {
			restartCalls.Add(1)
			return nil
		},
	})
	return srv, lc, restartCalls
}

func defaultSaved() config.Config {
	return config.Config{
		Addr:             ":8080",
		BluetoothAdapter: "hci0",
		Display: config.DisplayConfig{
			BlankAfter: config.Duration{Duration: 20 * time.Minute},
			Backend:    config.DisplayBackendWlopm,
			Output:     "HDMI-A-1",
		},
		Slideshow: config.SlideshowConfig{
			Interval:      config.Duration{Duration: 2 * time.Minute},
			ImagesDir:     "images",
			PairThreshold: 1.5,
		},
		Library: config.LibraryConfig{Backend: config.BackendFS},
		Weather: config.WeatherConfig{
			PollInterval:  config.Duration{Duration: 10 * time.Minute},
			RetryInterval: config.Duration{Duration: 30 * time.Second},
			Units:         "metric",
		},
		Mqtt: config.MqttConfig{
			ClientID: "frame",
			Bridge: config.MqttBridgeConfig{
				NodeID:          "pf",
				BaseTopic:       "picture-frame",
				DiscoveryPrefix: "homeassistant",
				StaleAfter:      config.Duration{Duration: 10 * time.Minute},
			},
		},
	}
}

func TestGetConfigReturnsDTO(t *testing.T) {
	saved := defaultSaved()
	saved.Weather.APIKey = "mykey"
	srv, _, _ := makeConfigServer(t, saved)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	weather := body["weather"].(map[string]any)
	if weather["api_key"] != nil && weather["api_key"] != "" {
		t.Error("api_key must not be returned in GET response")
	}
	if weather["api_key_set"] != true {
		t.Errorf("api_key_set must be true, got %v", weather["api_key_set"])
	}
	if _, ok := body["restart_pending"]; !ok {
		t.Error("restart_pending field missing from GET response")
	}
}

func TestGetConfigRestartPendingFalseInitially(t *testing.T) {
	srv, _, _ := makeConfigServer(t, defaultSaved())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["restart_pending"] != false {
		t.Errorf("expected restart_pending=false initially, got %v", body["restart_pending"])
	}
}

func TestPutConfigAppliesLiveForTier1(t *testing.T) {
	saved := defaultSaved()
	srv, lc, restartCalls := makeConfigServer(t, saved)

	// Change only a Tier-1 field (slideshow interval).
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["slideshow"].(map[string]any)["interval"] = "5m"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
	if lc.calls.Load() != 1 {
		t.Errorf("ApplyLive: called %d times, want 1", lc.calls.Load())
	}
	if restartCalls.Load() != 0 {
		t.Error("Restart must not be called on PUT")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["restart_pending"] != false {
		t.Errorf("Tier-1 only change: restart_pending must be false, got %v", resp["restart_pending"])
	}
}

func TestPutConfigSplitScreenIsLive(t *testing.T) {
	saved := defaultSaved()
	srv, lc, restartCalls := makeConfigServer(t, saved)

	// Toggling split-screen re-plans live (ApplyLive → SetSplitConfig); never a restart.
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["slideshow"].(map[string]any)["split_screen"] = true
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
	if lc.calls.Load() != 1 {
		t.Errorf("ApplyLive: called %d times, want 1", lc.calls.Load())
	}
	if restartCalls.Load() != 0 {
		t.Error("Restart must not be called on PUT")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["restart_pending"] != false {
		t.Errorf("split_screen change: restart_pending must be false, got %v", resp["restart_pending"])
	}
}

func TestPutConfigRestartPendingForNonTier1(t *testing.T) {
	saved := defaultSaved()
	srv, _, _ := makeConfigServer(t, saved)

	// Change a restart-tier field (display backend).
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["display"].(map[string]any)["backend"] = config.DisplayBackendVcgencmd
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["restart_pending"] != true {
		t.Errorf("restart-tier change: restart_pending must be true, got %v", resp["restart_pending"])
	}
}

func TestPutConfigUpdaterNeedsRestart(t *testing.T) {
	saved := defaultSaved()
	srv, _, _ := makeConfigServer(t, saved)

	// The updater snapshots its schedule at construction, so a change is restart-tier.
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["updater"].(map[string]any)["update_hour"] = 5
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["restart_pending"] != true {
		t.Errorf("updater change: restart_pending must be true, got %v", resp["restart_pending"])
	}
}

func TestPutConfigLocaleIsLive(t *testing.T) {
	saved := defaultSaved()
	srv, _, restartCalls := makeConfigServer(t, saved)

	// Changing only the locale must apply live, never flag a restart.
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["display"].(map[string]any)["locale"] = "hu-HU"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
	if restartCalls.Load() != 0 {
		t.Error("Restart must not be called on PUT")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["restart_pending"] != false {
		t.Errorf("locale-only change: restart_pending must be false, got %v", resp["restart_pending"])
	}
}

func TestPutConfigPublishesKioskEvent(t *testing.T) {
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	saved := defaultSaved()
	bus := state.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           bus,
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		LiveConfig:    &fakeLiveConfig{},
		Restart:       func() error { return nil },
	})

	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["display"].(map[string]any)["locale"] = "hu-HU"
	})
	if rec := doJSONPut(srv, "/api/config", body); rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}

	select {
	case ev := <-ch:
		if ev.Kind != state.KindKiosk {
			t.Fatalf("event kind: got %q, want %q", ev.Kind, state.KindKiosk)
		}
		kp, ok := ev.Payload.(state.KioskPayload)
		if !ok {
			t.Fatalf("payload type: got %T, want state.KioskPayload", ev.Payload)
		}
		if kp.Locale != "hu-HU" {
			t.Errorf("kiosk locale: got %q, want hu-HU", kp.Locale)
		}
	default:
		t.Fatal("expected a kiosk event to be published on PUT")
	}
}

func TestPutConfigInvalidDuration(t *testing.T) {
	saved := defaultSaved()
	srv, _, _ := makeConfigServer(t, saved)

	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["display"].(map[string]any)["blank_after"] = "notaduration"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want 422", rec.Code)
	}
}

func TestPutConfigValidationError(t *testing.T) {
	saved := defaultSaved()
	srv, _, _ := makeConfigServer(t, saved)

	// Send duplicate sensor IDs to trigger Validate() failure.
	body := putBody(t, saved, func(dto *map[string]any) {
		dupSensor := map[string]any{"id": "dup", "type": "mock", "role": "inside"}
		(*dto)["sensors"] = []any{dupSensor, dupSensor}
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want 422; body: %s", rec.Code, rec.Body)
	}
}

func TestPutConfigSaveOverridesError(t *testing.T) {
	saved := defaultSaved()
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, t.TempDir()),
		RunningConfig: saved,
	})
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["bluetooth_adapter"] = "hci1"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestPutConfigNilLiveConfigDoesNotPanic(t *testing.T) {
	saved := defaultSaved()
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		LiveConfig:    nil, // should be a no-op, not a panic
	})
	body := putBody(t, saved, func(dto *map[string]any) {
		(*dto)["slideshow"].(map[string]any)["interval"] = "5m"
	})
	rec := doJSONPut(srv, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d; body: %s", rec.Code, rec.Body)
	}
}

func TestGetConfigMetaIncludesDecoders(t *testing.T) {
	srv, _, _ := makeConfigServer(t, defaultSaved())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config/meta", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"decoders", "kinds", "units", "backends", "sensor_types", "address_types", "log_levels"} {
		if !strings.Contains(body, want) {
			t.Errorf("meta missing field %q", want)
		}
	}
	// At least one known decoder name must appear.
	if !strings.Contains(body, "raw_float") {
		t.Error("meta decoders missing raw_float")
	}
}

func TestSystemRestartCalls202AndRestartSpy(t *testing.T) {
	saved := defaultSaved()
	_, _, restartCalls := makeConfigServer(t, saved)

	lc := &fakeLiveConfig{}
	calls := &atomic.Int32{}
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		LiveConfig:    lc,
		Restart: func() error {
			calls.Add(1)
			return nil
		},
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/system/restart", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", rec.Code)
	}
	// The restart goroutine sleeps 200ms before calling Restart.
	time.Sleep(400 * time.Millisecond)
	if calls.Load() != 1 {
		t.Errorf("Restart spy: called %d times, want 1", calls.Load())
	}
	_ = restartCalls // suppress unused warning
}

func TestSystemRestartTriggersReexec(t *testing.T) {
	saved := defaultSaved()
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	called := &atomic.Int32{}
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		Restart:       func() error { called.Add(1); return nil },
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/system/restart", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", rec.Code)
	}
	time.Sleep(400 * time.Millisecond) // the restart goroutine sleeps 200ms first
	if called.Load() != 1 {
		t.Errorf("Restart spy: called %d times, want 1", called.Load())
	}
}

func TestSystemRestartErrorIsLogged(t *testing.T) {
	saved := defaultSaved()
	overridesPath := filepath.Join(t.TempDir(), "overrides.toml")
	restartErr := errors.New("re-exec failed")
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(saved, overridesPath),
		RunningConfig: saved,
		Restart:       func() error { return restartErr },
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/system/restart", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", rec.Code)
	}
	// The restart goroutine sleeps 200ms before calling Restart which returns an error.
	// The error is logged (side effect); we just verify the goroutine completes without panic.
	time.Sleep(400 * time.Millisecond)
}

func TestSystemRestartNilRestartReturns503(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		// Restart is nil
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/system/restart", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", rec.Code)
	}
}

// putBody builds a PUT body by first GET-ting the current config as a map,
// then applying a mutation function. This avoids spelling out every field.
func putBody(t *testing.T, saved config.Config, mutate func(*map[string]any)) []byte {
	t.Helper()
	// Use a temporary server just to GET the current config as a map.
	tmpSrv, _, _ := makeConfigServer(t, saved)
	rec := httptest.NewRecorder()
	tmpSrv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("putBody decode: %v", err)
	}
	delete(dto, "restart_pending") // not a config field
	mutate(&dto)
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("putBody marshal: %v", err)
	}
	return data
}

func doJSONPut(srv http.Handler, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestPutConfigBodyLimitRejectsOversized(t *testing.T) {
	srv, _, _ := makeConfigServer(t, defaultSaved())
	big := bytes.NewBufferString(`{"log_level":"` + strings.Repeat("x", 70*1024) + `"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", big)
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}
}
