package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestTouchSettingsRoundTripAppliesLive(t *testing.T) {
	saved := defaultSaved()
	saved.Sleep = config.SleepConfig{IdleAfter: config.Duration{Duration: 2 * time.Minute}, OffFrom: "23:00", OffUntil: "07:00"}
	srv, lc, _ := makeConfigServer(t, saved)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/touch/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d: %s", rec.Code, rec.Body)
	}
	var got httpapi.TouchSettingsBody
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Interval != "2m0s" || got.Sleep.IdleAfter != "2m0s" || got.Sleep.OffFrom != "23:00" || got.BrightnessSupported {
		t.Fatalf("get = %+v", got)
	}

	body := `{"sleep":{"idle_after":"5m","schedule":true,"off_from":"22:30","off_until":"06:45","wake_for":"1m"},` +
		`"interval":"30s","randomize":true,"split_screen":true,"brightness":70,"rotation":90}`
	req := httptest.NewRequest(http.MethodPut, "/api/touch/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put %d: %s", rec.Code, rec.Body)
	}
	if lc.calls.Load() != 1 {
		t.Fatal("ApplyLive not called")
	}
	c := lc.last
	if c.Sleep.IdleAfter.Duration != 5*time.Minute || !c.Sleep.Schedule || c.Sleep.OffFrom != "22:30" ||
		c.Slideshow.Interval.Duration != 30*time.Second || !c.Slideshow.Randomize ||
		c.Display.Brightness != 70 || c.Display.Rotation != 90 {
		t.Fatalf("applied = %+v", c)
	}
	if c.Mqtt.ClientID != "frame" || c.Display.Output != "HDMI-A-1" {
		t.Fatal("fields outside the touch subset must be preserved")
	}

	// A touch edit never needs a restart.
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var full struct {
		RestartPending bool `json:"restart_pending"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&full)
	if full.RestartPending {
		t.Fatal("touch settings must all be live-applied")
	}
}

func TestTouchSettingsRejectsInvalid(t *testing.T) {
	srv, lc, _ := makeConfigServer(t, defaultSaved())
	for _, body := range []string{
		`{"sleep":{"idle_after":"x","schedule":false,"off_from":"","off_until":"","wake_for":"1m"},"interval":"1m","randomize":false,"split_screen":false,"brightness":0,"rotation":0}`,
		`{"sleep":{"idle_after":"1m","schedule":false,"off_from":"","off_until":"","wake_for":"x"},"interval":"1m","randomize":false,"split_screen":false,"brightness":0,"rotation":0}`,
		`{"sleep":{"idle_after":"1m","schedule":true,"off_from":"22:00","off_until":"22:00","wake_for":"1m"},"interval":"1m","randomize":false,"split_screen":false,"brightness":0,"rotation":0}`,
		`{"sleep":{"idle_after":"1m","schedule":false,"off_from":"","off_until":"","wake_for":"1m"},"interval":"0s","randomize":false,"split_screen":false,"brightness":0,"rotation":0}`,
		`{"sleep":{"idle_after":"1m","schedule":false,"off_from":"","off_until":"","wake_for":"1m"},"interval":"bad","randomize":false,"split_screen":false,"brightness":0,"rotation":0}`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/touch/settings", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("body %s: got %d", body, rec.Code)
		}
	}
	if lc.calls.Load() != 0 {
		t.Fatal("invalid settings applied")
	}
}

// Touch-UI routes open to the on-device kiosk over loopback, never to the LAN.
func TestTouchUIRoutesGating(t *testing.T) {
	hash := hashFor(t, "pw")
	cfg := config.Config{Auth: config.AuthConfig{PasswordHash: hash}}
	srv := httpapi.NewServer(httpapi.Config{
		Log:           testutil.NopLogger(),
		Screen:        &mockScreen{},
		Bus:           state.NewBus(),
		Library:       library.New(nil, false),
		KioskBeater:   &fakeBeater{},
		Store:         config.NewStore(cfg, filepath.Join(t.TempDir(), "overrides.toml")),
		RunningConfig: cfg,
	})
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/touch/settings"},
		{http.MethodGet, "/api/images"},
		{http.MethodPut, "/api/images/slideshow"},
		{http.MethodGet, "/api/files"},
		{http.MethodGet, "/api/system/info"},
		{http.MethodGet, "/api/screen"},
		{http.MethodGet, "/thumb/a.jpg"},
		{http.MethodGet, "/files/a.txt"},
	} {
		for _, remote := range []string{"127.0.0.1:5000", "192.168.1.5:5000"} {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = remote
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			loopback := strings.HasPrefix(remote, "127.")
			if loopback && rec.Code == http.StatusUnauthorized {
				t.Errorf("loopback %s %s = 401", c.method, c.path)
			}
			if !loopback && rec.Code != http.StatusUnauthorized {
				t.Errorf("remote %s %s = %d, want 401", c.method, c.path, rec.Code)
			}
		}
	}
	// /api/config stays admin-only even from the device.
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("loopback /api/config = %d, want 401", rec.Code)
	}
}
