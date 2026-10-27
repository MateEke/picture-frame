package power

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

func TestSchedulerAppliesDayAndNightLevels(t *testing.T) {
	panel := &Noop{}
	cfg := config.DisplayConfig{BrightnessSchedule: config.BrightnessScheduleConfig{
		Enabled: true, DayLevel: 80, NightLevel: 15, DayStart: "07:00", NightStart: "22:00",
	}}
	s := NewScheduler(panel, slog.New(slog.DiscardHandler), cfg)
	s.Apply(context.Background(), time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local))
	if got := panel.Brightness(); got != 80 {
		t.Fatalf("day brightness = %d, want 80", got)
	}
	s.Apply(context.Background(), time.Date(2026, 1, 1, 23, 0, 0, 0, time.Local))
	if got := panel.Brightness(); got != 15 {
		t.Fatalf("night brightness = %d, want 15", got)
	}
}
