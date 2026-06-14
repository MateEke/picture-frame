package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
	"github.com/MateEke/picture-frame/web"
)

// mockScreen is a minimal ScreenController for handler tests. On/Off flip both
// power and auto, mirroring display.Screen's coupling.
type mockScreen struct {
	on         bool
	auto       bool
	onErr      error
	offErr     error
	reconciles atomic.Int32
}

func (m *mockScreen) On(context.Context) error {
	if m.onErr != nil {
		return m.onErr
	}
	m.on, m.auto = true, true
	return nil
}

func (m *mockScreen) Off(context.Context) error {
	if m.offErr != nil {
		return m.offErr
	}
	m.on, m.auto = false, false
	return nil
}

func (m *mockScreen) State() bool { return m.on }

func (m *mockScreen) Auto() bool { return m.auto }

func (m *mockScreen) Reconcile(context.Context) { m.reconciles.Add(1) }

func newTestServer(t *testing.T) (http.Handler, *mockScreen) {
	t.Helper()
	m := &mockScreen{}
	srv := httpapi.NewServer(httpapi.Config{
		Log:    testutil.NopLogger(),
		Screen: m,
		Bus:    state.NewBus(),
	})
	return srv, m
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
}

func TestScreenEndpoint(t *testing.T) {
	cases := []struct {
		body   string
		status int
	}{
		{`{"state":"on"}`, http.StatusNoContent},
		{`{"state":"off"}`, http.StatusNoContent},
		{`{"state":"invalid"}`, http.StatusUnprocessableEntity},
		{`not json`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		srv, _ := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/api/screen",
			bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("body=%q: got %d, want %d", tc.body, rec.Code, tc.status)
		}
	}
}

func TestGetScreenState(t *testing.T) {
	srv, m := newTestServer(t)

	// Initial state: off
	req := httptest.NewRequest(http.MethodGet, "/api/screen", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"off"`) {
		t.Errorf("expected off in body, got %s", rec.Body.String())
	}

	// Turn on, re-check
	_ = m.On(context.Background())
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
	if !strings.Contains(rec.Body.String(), `"on"`) {
		t.Errorf("expected on in body, got %s", rec.Body.String())
	}
}

func TestScreenPostTogglesAuto(t *testing.T) {
	srv, m := newTestServer(t)
	m.auto = true

	post := func(body string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/screen",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(httptest.NewRecorder(), req)
	}

	post(`{"state":"off"}`)
	if m.auto {
		t.Fatal("auto must be off after POST off")
	}

	post(`{"state":"on"}`)
	if !m.auto {
		t.Fatal("auto must be on after POST on")
	}
}

func TestGetScreenStateIncludesAuto(t *testing.T) {
	srv, m := newTestServer(t)
	m.auto = true

	check := func(wantAuto string) {
		t.Helper()
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/screen", nil))
		if !strings.Contains(rec.Body.String(), wantAuto) {
			t.Errorf("expected %q in body, got: %s", wantAuto, rec.Body.String())
		}
	}

	check(`"auto":true`)
	m.auto = false
	check(`"auto":false`)
}

func TestPostScreenDisplayError(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:    testutil.NopLogger(),
		Screen: &mockScreen{onErr: errors.New("vcgencmd failed")},
		Bus:    state.NewBus(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/screen",
		bytes.NewBufferString(`{"state":"on"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
}

func TestScreenStateChanges(t *testing.T) {
	srv, m := newTestServer(t)

	post := func(body string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/screen",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(httptest.NewRecorder(), req)
	}

	post(`{"state":"on"}`)
	if !m.on {
		t.Fatal("expected display on")
	}
	post(`{"state":"off"}`)
	if m.on {
		t.Fatal("expected display off")
	}
}

// fakeBeater counts Beat() calls for handler tests.
type fakeBeater struct {
	calls       atomic.Int32
	lastVersion atomic.Value // string
}

func (b *fakeBeater) Beat(version string) {
	b.calls.Add(1)
	b.lastVersion.Store(version)
}

func TestHeartbeatEndpoint(t *testing.T) {
	beater := &fakeBeater{}
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: beater,
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/heartbeat?version=v1.2.3", nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204", rec.Code)
	}
	if got := beater.calls.Load(); got != 1 {
		t.Errorf("expected Beat to be called once, got %d", got)
	}
	if got := beater.lastVersion.Load(); got != "v1.2.3" {
		t.Errorf("expected the frontend version to reach Beat, got %v", got)
	}
}

type fakeSyncer struct {
	st       library.Status
	triggers int
}

func (f *fakeSyncer) Status() library.Status { return f.st }
func (f *fakeSyncer) Trigger()               { f.triggers++ }

func TestLibraryEndpointFS(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/library", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"backend":"fs"`) {
		t.Errorf("body: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"sync"`) {
		t.Errorf("fs backend should not include sync: %s", rec.Body.String())
	}
}

func TestLibraryEndpointImmichWithSyncerStatus(t *testing.T) {
	st := library.Status{
		LastSync:   time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
		AssetCount: 47,
		LastError:  "",
	}
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		Backend:     "immich",
		Syncer:      &fakeSyncer{st: st},
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/library", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"backend":"immich"`, `"asset_count":47`, `"last_sync":"2026-05-28T12:00:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q: %s", want, body)
		}
	}
}

func TestSyncLibraryTriggersSyncer(t *testing.T) {
	syncer := &fakeSyncer{}
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
		Backend:     "immich",
		Syncer:      syncer,
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/library/sync", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d", rec.Code)
	}
	if syncer.triggers != 1 {
		t.Errorf("expected Trigger to be called once, got %d", syncer.triggers)
	}
}

func TestSyncLibraryWithoutSyncerConflicts(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/library/sync", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

// Hashed /_app/immutable/ files cache hard; everything else, including the
// SPA fallback, revalidates.
func TestSPACacheHeaders(t *testing.T) {
	h := httpapi.NewServer(httpapi.Config{
		Log:        testutil.NopLogger(),
		Screen:     &mockScreen{},
		Bus:        state.NewBus(),
		Production: true,
	})

	var asset string
	err := fs.WalkDir(web.BuildFS, "build/_app/immutable", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if asset == "" && !d.IsDir() {
			asset = strings.TrimPrefix(p, "build")
		}
		return nil
	})
	if err != nil || asset == "" {
		t.Fatalf("no immutable asset in embedded build (err=%v), run make build-ui", err)
	}

	cases := []struct {
		name, path, want string
	}{
		{"immutable asset", asset, "public, max-age=31536000, immutable"},
		{"index", "/", "no-cache"},
		{"spa fallback", "/admin/settings", "no-cache"},
		{"stale immutable hash falls back to index", "/_app/immutable/gone.js", "no-cache"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: got %d, want 200", tc.path, rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("%s: Cache-Control %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
