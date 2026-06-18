package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/MateEke/picture-frame/internal/state"
)

const pingInterval = 30 * time.Second

// ReadyEvent is an empty event signalling snapshot completion.
type ReadyEvent struct{}

// PingEvent is an empty keep-alive event.
type PingEvent struct{}

func (s *server) registerSSERoutes(api huma.API) {
	s.kioskExempt("/events")
	sse.Register(api, huma.Operation{
		OperationID: "events",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "SSE event stream",
		Middlewares: huma.Middlewares{
			func(ctx huma.Context, next func(huma.Context)) {
				ctx.SetHeader("Cache-Control", "no-cache")
				ctx.SetHeader("X-Accel-Buffering", "no")
				next(ctx)
			},
		},
	}, map[string]any{
		"sensor":        state.SensorPayload{},
		"weather":       state.WeatherPayload{},
		"image":         state.ImagePayload{},
		"screen":        state.ScreenPayload{},
		"screen_aspect": state.ScreenAspectPayload{},
		"kiosk":         state.KioskPayload{},
		"ready":         ReadyEvent{},
		"ping":          PingEvent{},
	}, func(ctx context.Context, _ *struct{}, send sse.Sender) {
		s.streamEvents(ctx, send)
	})
}

func (s *server) streamEvents(ctx context.Context, send sse.Sender) {
	ch, unsub := s.bus.Subscribe()
	defer unsub()

	s.screen.Reconcile(ctx)

	maxSnapshotID := s.sendSnapshot(send)
	if maxSnapshotID == 0 && ctx.Err() != nil {
		return
	}
	if err := send.Data(ReadyEvent{}); err != nil {
		s.log.Warn("sse: ready write failed", "err", err)
		return
	}

	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if event.ID <= maxSnapshotID {
				continue
			}
			if _, isImage := event.Payload.(state.ImagePayload); isImage && !s.screen.State() {
				continue
			}
			if err := send(sse.Message{ID: int(event.ID), Data: event.Payload}); err != nil { //nolint:gosec // ID won't overflow int on 64-bit
				s.log.Warn("sse: write failed", "err", err)
				return
			}
		case <-ping.C:
			if err := send.Data(PingEvent{}); err != nil {
				s.log.Warn("sse: ping write failed", "err", err)
				return
			}
		}
	}
}

func (s *server) sendSnapshot(send sse.Sender) uint64 {
	var maxID uint64
	for _, e := range s.bus.Snapshot() {
		if e.ID > maxID {
			maxID = e.ID
		}
		if err := send(sse.Message{ID: int(e.ID), Data: e.Payload}); err != nil { //nolint:gosec // ID won't overflow int on 64-bit
			s.log.Warn("sse: snapshot write failed", "err", err)
			return 0
		}
	}
	return maxID
}
