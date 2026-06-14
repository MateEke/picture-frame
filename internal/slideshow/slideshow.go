package slideshow

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
)

// Slideshow publishes image events to the bus on a configurable interval.
// Call Next to advance immediately and reset the timer.
type Slideshow struct {
	log      *slog.Logger
	lib      *library.Library
	bus      *state.Bus
	mu       sync.Mutex
	interval time.Duration
	advance  chan struct{}
}

func New(log *slog.Logger, lib *library.Library, bus *state.Bus, interval time.Duration) *Slideshow {
	return &Slideshow{
		log:      log,
		lib:      lib,
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

// SetRandomize enables or disables random ordering. Restarts the slideshow
// from the beginning of the new order immediately.
func (s *Slideshow) SetRandomize(enabled bool) {
	s.lib.SetRandomize(enabled)
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

// Run publishes the current image immediately (so SSE clients don't wait for
// the first tick), then advances on the interval. The ticker is paused while
// the library is empty and resumed via the advance channel.
func (s *Slideshow) Run(ctx context.Context) {
	if img := s.lib.Current(); img != nil {
		s.publish(img)
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
			if img := s.lib.Next(); img != nil {
				s.publish(img)
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
			if img := s.lib.Next(); img != nil {
				s.publish(img)
			}
		}
	}
}

func (s *Slideshow) publish(img *library.Image) {
	s.bus.Publish(state.Event{
		Kind:    state.KindImage,
		Payload: state.ImagePayload{Name: img.Name},
	})
	s.log.Debug("slideshow: displaying image", "name", img.Name)
}
