package httpapi

import (
	"context"

	"github.com/MateEke/picture-frame/internal/batterytracker"
	"github.com/danielgtaylor/huma/v2"
)

type BatteryStatsOutput struct {
	Body batterytracker.StatsSummary
}

func (s *server) registerBatteryStatsRoute(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-battery-stats",
		Method:      "GET",
		Path:        "/api/system/battery-stats",
		Summary:     "Get battery life analysis for screen on vs screen off",
		Description: "Returns observed discharge rates, estimated autonomy, 7-day daily summaries, and sample history.",
		Tags:        []string{"system"},
	}, func(_ context.Context, _ *struct{}) (*BatteryStatsOutput, error) {
		if s.batteryTracker == nil {
			return nil, huma.Error503ServiceUnavailable("battery tracking unavailable on this host")
		}
		stats := s.batteryTracker.GetStats()
		return &BatteryStatsOutput{Body: stats}, nil
	})
}
