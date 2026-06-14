package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
	"github.com/MateEke/picture-frame/internal/version"
)

func TestSystemInfoViaRoute(t *testing.T) {
	srv := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Screen:      &mockScreen{},
		Bus:         state.NewBus(),
		KioskBeater: &fakeBeater{},
	})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/info", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var body httpapi.SystemInfoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != version.Version {
		t.Errorf("version: got %q, want %q", body.Version, version.Version)
	}
	if body.Platform != version.Platform {
		t.Errorf("platform: got %q, want %q", body.Platform, version.Platform)
	}
}
