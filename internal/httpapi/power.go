package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/power"
)

// Lets the 202 reach the browser before logind takes the host down, as with
// POST /api/system/restart.
const powerResponseDelay = 200 * time.Millisecond

func (s *server) registerPowerRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "system-reboot",
		Method:        http.MethodPost,
		Path:          "/api/system/reboot",
		Summary:       "Reboot the host (requires the picture-frame polkit rule)",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, s.schedulePower("reboot", s.powerCapabilities().Reboot, func(ctx context.Context) error {
			return s.power.Reboot(ctx)
		})
	})

	huma.Register(api, huma.Operation{
		OperationID:   "system-shutdown",
		Method:        http.MethodPost,
		Path:          "/api/system/shutdown",
		Summary:       "Power off the host (requires the picture-frame polkit rule)",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, s.schedulePower("shutdown", s.powerCapabilities().PowerOff, func(ctx context.Context) error {
			return s.power.PowerOff(ctx)
		})
	})
}

// Rejects up front so the caller gets a 503 rather than an accepted request that
// never happens. action runs only when allowed, so it may assume a wired controller.
func (s *server) schedulePower(name string, allowed bool, action func(context.Context) error) error {
	if !allowed {
		return huma.Error503ServiceUnavailable(name + " not available on this host")
	}
	go func() {
		time.Sleep(powerResponseDelay)
		if err := action(context.Background()); err != nil {
			s.log.Error("power action failed", "action", name, "err", err)
		}
	}()
	return nil
}

// Reports nothing when power control is unwired (dev and tests).
func (s *server) powerCapabilities() power.Capabilities {
	if s.power == nil {
		return power.Capabilities{}
	}
	return s.power.Capabilities()
}
