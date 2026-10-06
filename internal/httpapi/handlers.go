package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
	"github.com/MateEke/picture-frame/internal/redact"
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

// ImmichAlbum is one album offered by GET /api/immich/albums: the subset of
// Immich's album object the picker needs.
type ImmichAlbum struct {
	ID         string `json:"id" doc:"Immich album UUID; store in immich_api_key.album_ids"`
	Name       string `json:"name"`
	AssetCount int    `json:"asset_count"`
}

// ImmichAlbumsRequest carries unsaved connection details so the picker can
// list albums before the first save — saving requires an album selection, so a
// GET-only endpoint would deadlock. The key travels in the body rather than the
// query string so it stays out of access logs.
type ImmichAlbumsRequest struct {
	URL    string `json:"url,omitempty" doc:"Overrides the saved Immich URL when set"`
	APIKey string `json:"api_key,omitempty" doc:"Overrides the saved API key when set"`
}

// ImmichAlbumsInput is the POST /api/immich/albums input.
type ImmichAlbumsInput struct {
	Body ImmichAlbumsRequest
}

// ImmichAlbumsOutput is the POST /api/immich/albums output.
type ImmichAlbumsOutput struct {
	Body []ImmichAlbum
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

// registerImmichAlbumsRoute lists the albums an Immich API key can see, so the
// settings UI can offer a picker instead of asking for pasted UUIDs.
//
// POST with an optional body, not GET: saving api-key mode requires an album
// selection, so the picker must be able to use credentials the user has typed
// but not yet saved. Empty fields fall back to the persisted config, which is
// the common case for editing an existing connection.
func (s *server) registerImmichAlbumsRoute(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-immich-albums",
		Method:      http.MethodPost,
		Path:        "/api/immich/albums",
		Summary:     "List Immich albums for the album picker",
	}, func(ctx context.Context, input *ImmichAlbumsInput) (*ImmichAlbumsOutput, error) {
		im := s.savedConfig().Immich
		if v := strings.TrimSpace(input.Body.URL); v != "" {
			im.URL = v
		}
		if v := strings.TrimSpace(input.Body.APIKey); v != "" {
			im.APIKey = v
		}
		if im.URL == "" || im.APIKey == "" {
			return nil, huma.Error409Conflict("immich url and api_key are required to list albums")
		}
		albums, err := immich.ListAlbums(ctx, im.URL, im.APIKey, nil)
		if err != nil {
			return nil, albumListError(err)
		}
		out := make([]ImmichAlbum, 0, len(albums))
		for _, a := range albums {
			out = append(out, ImmichAlbum{ID: a.ID, Name: a.AlbumName, AssetCount: a.AssetCount})
		}
		return &ImmichAlbumsOutput{Body: out}, nil
	})
}

// albumListError maps an Immich failure to an HTTP status with a message the UI
// can act on. httpError renders as a bare "status 401", which tells an admin
// nothing about whether the key or the URL is at fault.
func albumListError(err error) error {
	var hErr interface{ Error() string }
	if errors.As(err, &hErr) && strings.HasPrefix(hErr.Error(), "status ") {
		switch strings.TrimPrefix(hErr.Error(), "status ") {
		case "401":
			return huma.Error401Unauthorized("Immich rejected the API key; create a full-access key in the Immich UI")
		case "403":
			return huma.Error403Forbidden("the API key cannot read albums on this Immich server")
		default:
			return huma.Error502BadGateway("Immich returned " + hErr.Error())
		}
	}
	return huma.Error502BadGateway(redact.Path(err.Error()))
}

// savedConfig snapshots the persisted config (what the settings form last
// wrote), falling back to the running snapshot when no store is wired.
func (s *server) savedConfig() config.Config {
	if s.store != nil {
		return s.store.Snapshot()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
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
