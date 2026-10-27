package power

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

// Scheduler applies a two-period backlight schedule. It is deliberately small:
// the system clock is authoritative and configuration changes take effect now.
type Scheduler struct {
	mu        sync.Mutex
	backlight interface {
		SetBrightness(context.Context, int) error
	}
	log           *slog.Logger
	cfg           config.DisplayConfig
	last          int
	manualLevel   int  // -1 = no override
	manualEnabled bool
}

func NewScheduler(backlight interface {
	SetBrightness(context.Context, int) error
}, log *slog.Logger, cfg config.DisplayConfig) *Scheduler {
	return &Scheduler{backlight: backlight, log: log, cfg: cfg, last: -1, manualLevel: -1}
}

// SetOverride immediately applies level (0-100) and holds it until the next
// SetConfig call or Apply tick resets the schedule. level=-1 clears the override.
func (s *Scheduler) SetOverride(ctx context.Context, level int) error {
	if level != -1 && (level < 0 || level > 100) {
		return fmt.Errorf("power: brightness override %d out of range 0-100", level)
	}
	s.mu.Lock()
	s.manualLevel = level
	s.manualEnabled = level != -1
	s.mu.Unlock()
	if level == -1 {
		// Clear override: force re-apply on next Apply tick.
		s.mu.Lock()
		s.last = -1
		s.mu.Unlock()
		return nil
	}
	if err := s.backlight.SetBrightness(ctx, level); err != nil {
		return fmt.Errorf("brightness override write: %w", err)
	}
	s.mu.Lock()
	s.last = level
	s.mu.Unlock()
	s.log.Info("brightness override applied", "level", level)
	return nil
}

func (s *Scheduler) SetConfig(cfg config.DisplayConfig) {
	s.mu.Lock()
	s.cfg = cfg
	s.last = -1
	s.mu.Unlock()
	s.Apply(context.Background(), time.Now())
}

func (s *Scheduler) Run(ctx context.Context) {
	s.Apply(ctx, time.Now())
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.Apply(ctx, now)
		}
	}
}

func (s *Scheduler) Apply(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A manual override holds until explicitly cleared or config changes.
	if s.manualEnabled {
		return
	}
	b := s.cfg.BrightnessSchedule
	if !b.Enabled {
		return
	}
	loc := time.Local
	if s.cfg.Timezone != "" {
		if loaded, err := time.LoadLocation(s.cfg.Timezone); err == nil {
			loc = loaded
		}
	}
	now = now.In(loc)
	day, _ := time.Parse("15:04", b.DayStart)
	night, _ := time.Parse("15:04", b.NightStart)
	minute := now.Hour()*60 + now.Minute()
	dayMinute := day.Hour()*60 + day.Minute()
	nightMinute := night.Hour()*60 + night.Minute()
	daytime := (dayMinute < nightMinute && minute >= dayMinute && minute < nightMinute) || (dayMinute > nightMinute && (minute >= dayMinute || minute < nightMinute))
	level := b.NightLevel
	if daytime {
		level = b.DayLevel
	}
	if level == s.last {
		return
	}
	if err := s.backlight.SetBrightness(ctx, level); err != nil {
		s.log.Warn("brightness apply failed", "err", err)
		return
	}
	s.last = level
	s.log.Info("brightness applied", "level", level)
}
