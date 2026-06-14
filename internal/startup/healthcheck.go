package startup

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/state"
)

// HealthCheck serves /healthz and / against a throwaway in-process server (no socket, no
// hardware), so the updater can run `--health-check` on a freshly-downloaded binary before
// swapping it, proving it boots, parses config, and renders the UI on this device.
func HealthCheck(log *slog.Logger) error {
	handler := httpapi.NewServer(httpapi.Config{Log: log, Production: true, Bus: state.NewBus()})
	checks := []struct {
		path string
		want int
	}{
		{"/healthz", http.StatusNoContent},
		{"/", http.StatusOK},
	}
	for _, c := range checks {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.want {
			return fmt.Errorf("health-check %s: got %d, want %d", c.path, rec.Code, c.want)
		}
	}
	return nil
}
