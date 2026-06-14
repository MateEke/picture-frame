// Package weather polls a provider and publishes conditions to the state bus.
package weather

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
)

// Fetcher retrieves the current weather. Implementations live in subpackages
// (adapter, mock).
type Fetcher interface {
	Fetch(ctx context.Context) (state.WeatherPayload, error)
}

// Poller fetches weather on an interval and publishes each reading. Failures
// retry with exponential backoff, capped at the interval.
type Poller struct {
	log           *slog.Logger
	fetcher       Fetcher
	bus           *state.Bus
	mu            sync.Mutex
	interval      time.Duration
	retryInterval time.Duration
}

// New constructs a Poller. retryInterval is the first delay after a failure,
// doubling up to interval; a non-positive value disables fast retry.
func New(log *slog.Logger, fetcher Fetcher, bus *state.Bus, interval, retryInterval time.Duration) *Poller {
	return &Poller{log: log, fetcher: fetcher, bus: bus, interval: interval, retryInterval: retryInterval}
}

// SetIntervals updates the poll and retry intervals. Takes effect on the next timer expiry.
func (p *Poller) SetIntervals(interval, retryInterval time.Duration) {
	p.mu.Lock()
	p.interval = interval
	p.retryInterval = retryInterval
	p.mu.Unlock()
}

// Run polls immediately, then re-polls until ctx is cancelled: a full interval
// after success, or with backoff after failure.
func (p *Poller) Run(ctx context.Context) {
	var failures int
	timer := time.NewTimer(0) // fire immediately for the first poll
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := p.poll(ctx); err != nil {
				failures++
				next := p.backoff(failures)
				p.log.Warn("weather: fetch failed, retrying", "err", err, "in", next, "attempt", failures)
				timer.Reset(next)
			} else {
				failures = 0
				p.mu.Lock()
				iv := p.interval
				p.mu.Unlock()
				timer.Reset(iv)
			}
		}
	}
}

// poll fetches once and publishes the result, returning any error so Run can
// schedule the retry.
func (p *Poller) poll(ctx context.Context) error {
	w, err := p.fetcher.Fetch(ctx)
	if err != nil {
		// Zero payload drops the overlay to placeholders rather than stale data.
		p.bus.Publish(state.Event{Kind: state.KindWeather, Payload: state.WeatherPayload{}})
		return err
	}
	p.bus.Publish(state.Event{Kind: state.KindWeather, Payload: w})
	p.log.Debug("weather: published", "icon", w.IconCode, "temp", w.Temp)
	return nil
}

// backoff is retryInterval doubled per failure, capped at interval; a
// non-positive retryInterval falls back to interval.
func (p *Poller) backoff(failures int) time.Duration {
	p.mu.Lock()
	interval := p.interval
	retry := p.retryInterval
	p.mu.Unlock()

	delay := retry
	if delay <= 0 {
		return interval
	}
	for i := 1; i < failures && delay < interval; i++ {
		delay *= 2
	}
	if delay > interval {
		return interval
	}
	return delay
}
