package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/power"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakePower struct {
	caps     power.Capabilities
	reboots  atomic.Int32
	poweroff atomic.Int32
	err      error
}

func (p *fakePower) Capabilities() power.Capabilities { return p.caps }

func (p *fakePower) Reboot(context.Context) error {
	if p.err != nil {
		return p.err
	}
	p.reboots.Add(1)
	return nil
}

func (p *fakePower) PowerOff(context.Context) error {
	if p.err != nil {
		return p.err
	}
	p.poweroff.Add(1)
	return nil
}

func powerServer(pw httpapi.PowerController) http.Handler {
	return httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		Power:       pw,
	})
}

func postPower(t *testing.T, srv http.Handler, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	return rec.Code
}

func TestRebootEndpointInvokesPowerController(t *testing.T) {
	pw := &fakePower{caps: power.Capabilities{Reboot: true, PowerOff: true}}
	srv := powerServer(pw)

	if code := postPower(t, srv, "/api/system/reboot"); code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", code)
	}
	time.Sleep(400 * time.Millisecond) // the action goroutine sleeps 200ms first
	if pw.reboots.Load() != 1 {
		t.Errorf("Reboot called %d times, want 1", pw.reboots.Load())
	}
}

func TestShutdownEndpointInvokesPowerController(t *testing.T) {
	pw := &fakePower{caps: power.Capabilities{Reboot: true, PowerOff: true}}
	srv := powerServer(pw)

	if code := postPower(t, srv, "/api/system/shutdown"); code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", code)
	}
	time.Sleep(400 * time.Millisecond)
	if pw.poweroff.Load() != 1 {
		t.Errorf("PowerOff called %d times, want 1", pw.poweroff.Load())
	}
}

// Refusing up front beats accepting a request that could only fail later.
func TestPowerEndpointsRejectDeniedActions(t *testing.T) {
	pw := &fakePower{caps: power.Capabilities{Reboot: true}}
	srv := powerServer(pw)

	if code := postPower(t, srv, "/api/system/shutdown"); code != http.StatusServiceUnavailable {
		t.Errorf("denied shutdown: got %d, want 503", code)
	}
	if code := postPower(t, srv, "/api/system/reboot"); code != http.StatusAccepted {
		t.Errorf("permitted reboot: got %d, want 202", code)
	}
	time.Sleep(400 * time.Millisecond)
	if pw.poweroff.Load() != 0 {
		t.Errorf("denied action must not run, got %d calls", pw.poweroff.Load())
	}
}

func TestPowerEndpointsUnavailableWithoutController(t *testing.T) {
	srv := powerServer(nil)

	for _, path := range []string{"/api/system/reboot", "/api/system/shutdown"} {
		if code := postPower(t, srv, path); code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d, want 503", path, code)
		}
	}
}

func TestPowerActionErrorIsLogged(t *testing.T) {
	pw := &fakePower{caps: power.Capabilities{Reboot: true}, err: errors.New("logind refused")}
	srv := powerServer(pw)

	if code := postPower(t, srv, "/api/system/reboot"); code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202", code)
	}
	time.Sleep(400 * time.Millisecond) // failure is logged, not surfaced; must not panic
	if pw.reboots.Load() != 0 {
		t.Errorf("failed action must not count: %d", pw.reboots.Load())
	}
}

func TestSystemInfoReportsPowerCapabilities(t *testing.T) {
	pw := &fakePower{caps: power.Capabilities{Reboot: true}}
	rec := httptest.NewRecorder()
	powerServer(pw).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/info", nil))

	var body httpapi.SystemInfoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.CanReboot || body.CanPowerOff {
		t.Errorf("capabilities: got reboot=%v poweroff=%v, want true/false", body.CanReboot, body.CanPowerOff)
	}
}

func TestSystemInfoReportsNoPowerWithoutController(t *testing.T) {
	rec := httptest.NewRecorder()
	powerServer(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/info", nil))

	var body httpapi.SystemInfoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.CanReboot || body.CanPowerOff {
		t.Errorf("unwired power must report nothing: %+v", body)
	}
}
