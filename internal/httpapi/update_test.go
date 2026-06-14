package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
	"github.com/MateEke/picture-frame/internal/updater"
)

type fakeUpdater struct {
	status   updater.Status
	triggers atomic.Int32
	checks   atomic.Int32
}

func (f *fakeUpdater) Status() updater.Status { return f.status }
func (f *fakeUpdater) Trigger()               { f.triggers.Add(1) }
func (f *fakeUpdater) Check()                 { f.checks.Add(1) }

func updaterServer(u httpapi.UpdaterStatus) http.Handler {
	return httpapi.NewServer(httpapi.Config{
		Log: testutil.NopLogger(), Screen: &mockScreen{}, Bus: state.NewBus(),
		KioskBeater: &fakeBeater{}, Updater: u,
	})
}

func TestGetUpdateStatus(t *testing.T) {
	last := time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC)
	srv := updaterServer(&fakeUpdater{status: updater.Status{
		Current: "v1.2.0", Platform: "linux_armv6", Latest: "v1.3.1", Available: true,
		LastCheck: last, LastCheckOK: true, Phase: updater.PhaseDownloading, LastResult: "rolled back from v1.1.0",
	}})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/update", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var body httpapi.UpdateStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Available || body.Latest != "v1.3.1" || body.Phase != "downloading" {
		t.Errorf("body: %+v", body)
	}
	if body.LastCheck != "2026-06-11T02:00:00Z" || body.LastResult != "rolled back from v1.1.0" {
		t.Errorf("last_check/result: %+v", body)
	}
}

func TestGetUpdateStatusNilUpdater(t *testing.T) {
	rec := httptest.NewRecorder()
	updaterServer(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/update", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var body httpapi.UpdateStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Available || body.Phase != "idle" {
		t.Errorf("nil updater should read idle/unavailable: %+v", body)
	}
}

func TestApplyAndCheckUpdate(t *testing.T) {
	u := &fakeUpdater{}
	srv := updaterServer(u)

	for _, tc := range []struct {
		path        string
		wantTrigger int32
		wantCheck   int32
	}{
		{"/api/system/update", 1, 0},
		{"/api/system/update/check", 1, 1},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, nil))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("POST %s: got %d, want 202; body: %s", tc.path, rec.Code, rec.Body)
		}
		if u.triggers.Load() != tc.wantTrigger || u.checks.Load() != tc.wantCheck {
			t.Errorf("POST %s: triggers=%d checks=%d", tc.path, u.triggers.Load(), u.checks.Load())
		}
	}
}

func TestUpdateActionsNilUpdater(t *testing.T) {
	srv := updaterServer(nil)
	for _, path := range []string{"/api/system/update", "/api/system/update/check"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != http.StatusConflict {
			t.Errorf("POST %s with nil updater: got %d, want 409", path, rec.Code)
		}
	}
}

func TestGetLicenses(t *testing.T) {
	rec := httptest.NewRecorder()
	updaterServer(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/licenses", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected embedded notices, got empty body")
	}
}
