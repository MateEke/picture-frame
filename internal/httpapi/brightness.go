package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// --- Brightness ---

// BrightnessRequest carries a manual brightness level (0-100) or -1 to clear
// any override and resume the schedule.
type BrightnessRequest struct {
	// Level sets panel brightness as a percentage 0-100, or -1 to clear.
	Level int `json:"level" minimum:"-1" maximum:"100" doc:"Brightness 0-100, or -1 to clear override"`
}

type BrightnessInput struct {
	Body BrightnessRequest
}

// GetBrightnessResponse carries the current brightness state.
type GetBrightnessResponse struct {
	// Available is true when the host has a writable backlight device.
	Available bool `json:"available"`
	// Level is the last-applied brightness percentage (0-100), or -1 when unavailable.
	Level int `json:"level"`
}

type GetBrightnessOutput struct {
	Body GetBrightnessResponse
}

func (s *server) registerBrightnessRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-brightness",
		Method:      http.MethodGet,
		Path:        "/api/brightness",
		Summary:     "Get brightness availability",
	}, func(_ context.Context, _ *struct{}) (*GetBrightnessOutput, error) {
		return &GetBrightnessOutput{Body: GetBrightnessResponse{
			Available: s.brightness != nil,
			Level:     -1,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "set-brightness",
		Method:        http.MethodPost,
		Path:          "/api/brightness",
		Summary:       "Set manual brightness override (0-100) or -1 to clear",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *BrightnessInput) (*struct{}, error) {
		if s.brightness == nil {
			return nil, huma.Error503ServiceUnavailable("no backlight device available on this host")
		}
		if err := s.brightness.SetOverride(ctx, input.Body.Level); err != nil {
			s.log.Error("brightness override failed", "level", input.Body.Level, "err", err)
			return nil, huma.Error500InternalServerError("failed to set brightness")
		}
		return nil, nil
	})
}
