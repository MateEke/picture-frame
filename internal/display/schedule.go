package display

import (
	"context"
	"iter"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
)

const scheduleCheckInterval = 10 * time.Second

// catchUpWindow covers a full off-and-on window, whose latest boundary is
// always within the last day.
const catchUpWindow = 48 * time.Hour

// lateGrace caps how late a lone time fires (a missed off mustn't blank a noon
// plug-in) and how long a failed switch retries (it mustn't undo a later toggle).
const lateGrace = 5 * time.Minute

// Switcher is satisfied by *Screen.
type Switcher interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
}

// ScheduleStore persists the last check, so a restart can tell a missed
// boundary from an applied one.
type ScheduleStore interface {
	// LoadLastCheck returns the zero time when nothing is saved.
	LoadLastCheck() (time.Time, error)
	SaveLastCheck(t time.Time) error
}

type NextChange struct {
	At    time.Time
	On    bool
	Clock config.Clock
}

type edge struct {
	clock config.Clock
	on    bool
}

func (e edge) key() string {
	if e.on {
		return "on_at"
	}
	return "off_at"
}

// Scheduler flips the screen like a manual switch at fixed daily times, so
// Policy needs no notion of a schedule.
type Scheduler struct {
	log    *slog.Logger
	screen Switcher
	store  ScheduleStore // nil disables restart catch-up

	mu        sync.Mutex
	edges     []edge
	loc       *time.Location
	lastCheck time.Time
}

type SchedulerConfig struct {
	Log      *slog.Logger
	Screen   Switcher
	Store    ScheduleStore
	Schedule config.ScheduleConfig
	Location *time.Location
}

func NewScheduler(cfg SchedulerConfig) *Scheduler {
	return &Scheduler{
		log:    cfg.Log,
		screen: cfg.Screen,
		store:  cfg.Store,
		edges:  parseEdges(cfg.Schedule),
		loc:    cfg.Location,
	}
}

// parseEdges skips invalid times; config validation already rejects them.
func parseEdges(s config.ScheduleConfig) []edge {
	var edges []edge
	if c, err := config.ParseClock(s.OffAt); err == nil {
		edges = append(edges, edge{clock: c})
	}
	if c, err := config.ParseClock(s.OnAt); err == nil {
		edges = append(edges, edge{clock: c, on: true})
	}
	return edges
}

// Start applies a boundary missed while the frame was down. Call once before Run.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logScheduleLocked()
	now := time.Now()
	s.lastCheck = now
	if s.store == nil {
		return
	}
	last, err := s.store.LoadLastCheck()
	if err != nil {
		s.log.Warn("schedule: failed to load last check; skipping catch-up", "err", err)
		return
	}
	if !last.IsZero() {
		s.lastCheck = last
	}
	s.checkLocked(ctx, now)
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(scheduleCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.checkLocked(ctx, time.Now())
			s.mu.Unlock()
		}
	}
}

// Set takes effect from the next boundary, not at once. It reports whether the
// schedule changed; unchanged settings are a no-op.
func (s *Scheduler) Set(schedule config.ScheduleConfig, loc *time.Location) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	edges := parseEdges(schedule)
	if slices.Equal(edges, s.edges) && loc.String() == s.loc.String() {
		return false
	}
	s.edges = edges
	s.loc = loc
	s.logScheduleLocked()
	s.lastCheck = time.Now()
	s.saveLocked(s.lastCheck)
	return true
}

func (s *Scheduler) Next() (NextChange, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var next NextChange
	found := false
	for at, e := range boundaries(s.edges, s.loc, now, now.Add(catchUpWindow)) {
		if at.After(now) && (!found || at.Before(next.At)) {
			next, found = NextChange{At: at, On: e.on, Clock: e.clock}, true
		}
	}
	return next, found
}

// checkLocked applies the latest boundary since lastCheck; on failure lastCheck
// stays, so the next tick retries until lateGrace has passed.
func (s *Scheduler) checkLocked(ctx context.Context, now time.Time) {
	if now.Before(s.lastCheck) {
		// The clock went back, e.g. on a network time sync.
		s.lastCheck = now
		return
	}
	window := catchUpWindow
	if len(s.edges) < 2 {
		window = lateGrace
	}
	from := s.lastCheck
	if floor := now.Add(-window); from.Before(floor) {
		from = floor
	}
	e, at, ok := latestEdge(s.edges, s.loc, from, now)
	if !ok {
		s.lastCheck = now
		return
	}
	var err error
	if e.on {
		err = s.screen.On(ctx)
	} else {
		err = s.screen.Off(ctx)
	}
	if err != nil {
		if now.Sub(at) < lateGrace {
			s.log.Warn("schedule: failed to switch screen; retrying", "on", e.on, "err", err)
			return
		}
		s.log.Error("schedule: failed to switch screen; giving up until the next time", "on", e.on, "err", err)
		s.lastCheck = now
		s.saveLocked(now)
		return
	}
	s.log.Info("schedule: screen switched", "on", e.on, "at", e.clock)
	s.lastCheck = now
	s.saveLocked(now)
}

func (s *Scheduler) saveLocked(t time.Time) {
	if s.store == nil {
		return
	}
	if err := s.store.SaveLastCheck(t); err != nil {
		s.log.Warn("schedule: failed to persist last check", "err", err)
	}
}

func (s *Scheduler) logScheduleLocked() {
	attrs := []any{"tz", s.loc.String()}
	for _, e := range s.edges {
		attrs = append(attrs, e.key(), e.clock)
	}
	s.log.Info("schedule: active", attrs...)
}

// latestEdge returns the edge with the latest boundary in (from, to], and when it falls.
func latestEdge(edges []edge, loc *time.Location, from, to time.Time) (edge, time.Time, bool) {
	var best edge
	var bestAt time.Time
	found := false
	for at, e := range boundaries(edges, loc, from, to) {
		if at.After(from) && !at.After(to) && (!found || at.After(bestAt)) {
			best, bestAt, found = e, at, true
		}
	}
	return best, bestAt, found
}

// boundaries yields every edge's instant on each calendar day from from's date
// through to's, in loc.
func boundaries(edges []edge, loc *time.Location, from, to time.Time) iter.Seq2[time.Time, edge] {
	return func(yield func(time.Time, edge) bool) {
		y, m, d := from.In(loc).Date()
		for i := 0; !time.Date(y, m, d+i, 0, 0, 0, 0, loc).After(to); i++ {
			for _, e := range edges {
				if !yield(time.Date(y, m, d+i, e.clock.Hour, e.clock.Minute, 0, 0, loc), e) {
					return
				}
			}
		}
	}
}
