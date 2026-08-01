package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/state"
)

// --- Screen ---

type ScreenStateResponse struct {
	State string `json:"state" enum:"on,off" doc:"Current screen power state"`
	Auto  bool   `json:"auto" doc:"Whether automatic power management is active"`
}

type GetScreenOutput struct {
	Body ScreenStateResponse
}

type SetScreenRequest struct {
	State string `json:"state" enum:"on,off" doc:"Desired screen power state"`
}

type SetScreenInput struct {
	Body SetScreenRequest
}

func (s *server) registerScreenRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-screen",
		Method:      http.MethodGet,
		Path:        "/api/screen",
		Summary:     "Get screen state",
	}, func(_ context.Context, _ *struct{}) (*GetScreenOutput, error) {
		screenState := "off"
		if s.screen.State() {
			screenState = "on"
		}
		return &GetScreenOutput{Body: ScreenStateResponse{State: screenState, Auto: s.screen.Auto()}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "set-screen",
		Method:        http.MethodPost,
		Path:          "/api/screen",
		Summary:       "Set screen state",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *SetScreenInput) (*struct{}, error) {
		var err error
		switch input.Body.State {
		case "on":
			err = s.screen.On(ctx)
		case "off":
			err = s.screen.Off(ctx)
		}
		if err != nil {
			s.log.Error("screen toggle failed", "state", input.Body.State, "err", err)
			return nil, huma.Error500InternalServerError("failed to toggle screen")
		}
		return nil, nil
	})

	// Exempt so the kiosk can post it per tap. Wake-only: off stays gated.
	s.kioskExempt("/api/screen/wake")
	huma.Register(api, huma.Operation{
		OperationID:   "screen-wake",
		Method:        http.MethodPost,
		Path:          "/api/screen/wake",
		Summary:       "Wake the screen after a kiosk touch",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   huma.Middlewares{markKioskOrigin},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		s.recordTouch(ctx)
		if err := s.screen.Wake(ctx); err != nil {
			s.log.Error("screen wake failed", "err", err)
			return nil, huma.Error500InternalServerError("failed to wake screen")
		}
		return nil, nil
	})
}

// --- Kiosk origin ---

type ctxKey int

const kioskOriginKey ctxKey = iota

// The frame's own screen: loopback with no forwarding headers, since a phone
// behind a reverse proxy is loopback too.
func markKioskOrigin(ctx huma.Context, next func(huma.Context)) {
	onDevice := isLoopback(ctx.RemoteAddr()) &&
		ctx.Header("X-Forwarded-For") == "" && ctx.Header("X-Real-IP") == ""
	next(huma.WithValue(ctx, kioskOriginKey, onDevice))
}

// False on routes without markKioskOrigin.
func onDeviceKiosk(ctx context.Context) bool {
	onDevice, _ := ctx.Value(kioskOriginKey).(bool)
	return onDevice
}

// The admin UI drives the same routes, so only on-device taps count.
func (s *server) recordTouch(ctx context.Context) {
	if !onDeviceKiosk(ctx) {
		return
	}
	s.bus.Publish(state.Event{Kind: state.KindTouch, Payload: state.TouchPayload{At: time.Now()}})
}

// --- Heartbeat ---

type heartbeatInput struct {
	// Reporting frontend's build; the update commit gate fires only when the new build beats.
	Version string `query:"version"`
	// Screen aspect (width/height) for split-screen pairing; see markKioskOrigin.
	Aspect float64 `query:"aspect"`
}

func (s *server) registerHeartbeatRoutes(api huma.API) {
	s.kioskExempt("/api/heartbeat")
	huma.Register(api, huma.Operation{
		OperationID:   "heartbeat",
		Method:        http.MethodPost,
		Path:          "/api/heartbeat",
		Summary:       "Record kiosk heartbeat",
		DefaultStatus: http.StatusNoContent,
		Middlewares:   huma.Middlewares{markKioskOrigin},
	}, func(ctx context.Context, input *heartbeatInput) (*struct{}, error) {
		if s.planner != nil && input.Aspect > 0 && input.Aspect < 100 && onDeviceKiosk(ctx) {
			if s.planner.SetScreenAspect(input.Aspect) {
				s.bus.Publish(state.Event{
					Kind:    state.KindScreenAspect,
					Payload: state.ScreenAspectPayload{Aspect: input.Aspect},
				})
				// Re-plan now (e.g. on rotation) instead of waiting out the dwell.
				if s.slideshow != nil {
					s.slideshow.Next()
				}
			}
		}
		s.kioskBeater.Beat(input.Version)
		return nil, nil
	})
}

// --- Library ---

type LibraryResponse struct {
	Backend string       `json:"backend" doc:"Active library backend (fs or immich)"`
	Sync    *LibrarySync `json:"sync,omitempty" doc:"Sync status for remote backends"`
}

type LibrarySync struct {
	LastSync   string `json:"last_sync,omitempty" doc:"Timestamp of last successful sync"`
	AssetCount int    `json:"asset_count" doc:"Number of synced assets"`
	LastError  string `json:"last_error,omitempty" doc:"Error from last sync attempt"`
}

type GetLibraryOutput struct {
	Body LibraryResponse
}

func (s *server) registerLibraryRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-library",
		Method:      http.MethodGet,
		Path:        "/api/library",
		Summary:     "Get library info",
	}, func(_ context.Context, _ *struct{}) (*GetLibraryOutput, error) {
		resp := LibraryResponse{Backend: s.backend}
		if s.syncer != nil {
			st := s.syncer.Status()
			sync := &LibrarySync{AssetCount: st.AssetCount, LastError: st.LastError}
			if !st.LastSync.IsZero() {
				sync.LastSync = st.LastSync.UTC().Format(time.RFC3339)
			}
			resp.Sync = sync
		}
		return &GetLibraryOutput{Body: resp}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "sync-library",
		Method:        http.MethodPost,
		Path:          "/api/library/sync",
		Summary:       "Trigger a remote library sync",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		if s.syncer == nil {
			return nil, huma.Error409Conflict("no remote library backend is active")
		}
		s.syncer.Trigger()
		return nil, nil
	})
}

// --- Health ---

func (s *server) registerHealthRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "healthz",
		Method:        http.MethodGet,
		Path:          "/healthz",
		Summary:       "Health check",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		return nil, nil
	})
}
