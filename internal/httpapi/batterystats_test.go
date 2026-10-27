package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MateEke/picture-frame/internal/batterytracker"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestBatteryStatsEndpoint(t *testing.T) {
	tracker := batterytracker.New(nil, nil, nil, "")
	handler := httpapi.NewServer(httpapi.Config{
		Log:            testutil.NopLogger(),
		BatteryTracker: tracker,
	})

	req := httptest.NewRequest("GET", "/api/system/battery-stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var stats batterytracker.StatsSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
}
