package httpapi

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/MateEke/picture-frame/internal/batterytracker"
)

type BatteryStatsOutput struct {
	Body batterytracker.StatsSummary
}

func (s *server) registerBatteryStatsRoute(api huma.API) {
	if s.batteryTracker == nil {
		return
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-battery-stats",
		Method:      "GET",
		Path:        "/api/system/battery-stats",
		Summary:     "Get battery life analysis for screen on vs screen off",
		Description: "Returns observed discharge rates, estimated autonomy, 7-day daily summaries, and sample history.",
		Tags:        []string{"system"},
	}, func(ctx context.Context, input *struct{}) (*BatteryStatsOutput, error) {
		stats := s.batteryTracker.GetStats()
		return &BatteryStatsOutput{Body: stats}, nil
	})
}
