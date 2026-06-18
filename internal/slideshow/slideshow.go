package slideshow

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/slideplan"
	"github.com/MateEke/picture-frame/internal/state"
)

// Slideshow publishes slide events to the bus on a configurable interval. Slides
// come from the planner (a solo image, or a split-screen pair). Call Next to
// advance immediately and reset the timer.
type Slideshow struct {
	log      *slog.Logger
	lib      *library.Library
	planner  *slideplan.Planner
	bus      *state.Bus
	mu       sync.Mutex
	interval time.Duration
	advance  chan struct{}
}

func New(log *slog.Logger, lib *library.Library, planner *slideplan.Planner, bus *state.Bus, interval time.Duration) *Slideshow {
	return &Slideshow{
		log:      log,
		lib:      lib,
		planner:  planner,
		bus:      bus,
		interval: interval,
		advance:  make(chan struct{}, 1),
	}
}

// SetInterval updates the advance interval and triggers an immediate advance so the
// new duration takes effect right away rather than waiting out the remaining tick.
func (s *Slideshow) SetInterval(d time.Duration) {
	s.mu.Lock()
	s.interval = d
	s.mu.Unlock()
	select {
	case s.advance <- struct{}{}:
	default:
	}
}

// SetRandomize toggles random ordering and re-plans in place. Disable sorts now;
// enable shuffles at the next cycle wrap. The cursor is preserved, not reset.
func (s *Slideshow) SetRandomize(enabled bool) {
	s.lib.SetRandomize(enabled)
	s.planner.Invalidate()
	select {
	case s.advance <- struct{}{}:
	default:
	}
}

// SetSplitConfig updates split-screen pairing (enable + threshold) and rebuilds
// the slide plan. Used for live config reload.
func (s *Slideshow) SetSplitConfig(enabled bool, thr slideplan.Threshold) {
	s.planner.SetConfig(enabled, thr)
	select {
	case s.advance <- struct{}{}:
	default:
	}
}

// Next advances immediately and resets the timer. Safe from any goroutine.
func (s *Slideshow) Next() {
	select {
	case s.advance <- struct{}{}:
	default:
	}
}

// Run publishes the current slide immediately (so SSE clients don't wait for
// the first tick), then advances on the interval. The ticker is paused while
// the library is empty and resumed via the advance channel.
func (s *Slideshow) Run(ctx context.Context) {
	if slide := s.servable(s.planner.Current()); slide != nil {
		s.publish(slide)
	}

	s.mu.Lock()
	interval := s.interval
	s.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// A nil channel is never ready, so leaving tickerC nil "pauses" the ticker.
	var tickerC <-chan time.Time
	if s.lib.Len() > 0 {
		tickerC = ticker.C
	} else {
		ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerC:
			if slide := s.servable(s.planner.Next()); slide != nil {
				s.publish(slide)
				continue
			}
			// Library went empty between ticks; pause until next advance.
			ticker.Stop()
			tickerC = nil
		case <-s.advance:
			s.mu.Lock()
			interval = s.interval
			s.mu.Unlock()
			ticker.Reset(interval)
			tickerC = ticker.C
			if slide := s.servable(s.planner.Next()); slide != nil {
				s.publish(slide)
			}
		}
	}
}

// servable skips slides referencing an image deleted since the plan was built so
// the kiosk never 404s; skipping a whole plan forces a wrap, which rebuilds from
// the current library. Empty library yields nil to pause.
func (s *Slideshow) servable(slide *slideplan.Slide) *slideplan.Slide {
	for i, limit := 0, s.planner.SlideCount()+1; slide != nil && i < limit && !s.allPresent(slide); i++ {
		slide = s.planner.Next()
	}
	return slide
}

func (s *Slideshow) allPresent(slide *slideplan.Slide) bool {
	for _, name := range slide.Names {
		if !s.lib.Has(name) {
			return false
		}
	}
	return true
}

func (s *Slideshow) publish(slide *slideplan.Slide) {
	s.bus.Publish(state.Event{
		Kind:    state.KindImage,
		Payload: state.ImagePayload{Names: slide.Names},
	})
	s.log.Debug("slideshow: displaying slide", "names", slide.Names)
}
