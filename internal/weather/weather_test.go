package weather_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
	"github.com/MateEke/picture-frame/internal/weather"
)

type stubFetcher struct {
	mu      sync.Mutex
	calls   int
	payload state.WeatherPayload
	err     error
}

func (s *stubFetcher) Fetch(context.Context) (state.WeatherPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.payload, s.err
}

func (s *stubFetcher) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubFetcher) set(payload state.WeatherPayload, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payload = payload
	s.err = err
}

func TestRunPublishesImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		ch, unsub := bus.Subscribe()
		defer unsub()

		f := &stubFetcher{payload: state.WeatherPayload{IconCode: "10d", Temp: 12.5, Humidity: 80}}
		p := weather.New(testutil.NopLogger(), f, bus, time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait()
		select {
		case e := <-ch:
			w := e.Payload.(state.WeatherPayload)
			if w.IconCode != "10d" || w.Temp != 12.5 || w.Humidity != 80 {
				t.Errorf("unexpected payload: %+v", w)
			}
		default:
			t.Fatal("expected an immediate weather event")
		}
	})
}

func TestRunPublishesOnInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{payload: state.WeatherPayload{IconCode: "01d"}}
		p := weather.New(testutil.NopLogger(), f, bus, time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait()
		if got := f.Calls(); got != 1 {
			t.Fatalf("after start: got %d fetches, want 1", got)
		}

		time.Sleep(3 * time.Minute)
		synctest.Wait()
		if got := f.Calls(); got != 4 {
			t.Fatalf("after 3 intervals: got %d fetches, want 4", got)
		}
	})
}

func TestRunPublishesEmptyOnFetchError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		ch, unsub := bus.Subscribe()
		defer unsub()

		f := &stubFetcher{err: errors.New("boom")}
		p := weather.New(testutil.NopLogger(), f, bus, time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait()
		select {
		case e := <-ch:
			if w := e.Payload.(state.WeatherPayload); w.IconCode != "" {
				t.Errorf("expected empty payload on fetch error, got %+v", w)
			}
		default:
			t.Fatal("expected a placeholder weather event on fetch error")
		}
	})
}

func TestRunStopsOnContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{payload: state.WeatherPayload{IconCode: "01d"}}
		p := weather.New(testutil.NopLogger(), f, bus, time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			p.Run(ctx)
			close(done)
		}()

		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("Run did not return after context cancel")
		}
	})
}

// A failing poll retries on backoff, doubling the gap each failure.
func TestRunBacksOffExponentiallyOnFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{err: errors.New("boom")}
		p := weather.New(testutil.NopLogger(), f, bus, 10*time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait() // t=0: immediate poll fails, next retry at +30s
		if got := f.Calls(); got != 1 {
			t.Fatalf("after start: got %d fetches, want 1", got)
		}

		time.Sleep(30 * time.Second) // t=30s
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("after first retry: got %d fetches, want 2", got)
		}

		// Second retry waits 60s (doubled), not 30s.
		time.Sleep(31 * time.Second) // t=61s
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("retry fired too early: got %d fetches, want 2", got)
		}

		time.Sleep(29 * time.Second) // t=90s
		synctest.Wait()
		if got := f.Calls(); got != 3 {
			t.Fatalf("after second retry: got %d fetches, want 3", got)
		}
	})
}

// Backoff must never exceed the normal poll interval.
func TestRunBackoffCapsAtInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{err: errors.New("boom")}
		// 40s doubles to 80s, which is capped to the 60s interval.
		p := weather.New(testutil.NopLogger(), f, bus, time.Minute, 40*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait()              // t=0: poll #1, retry at +40s
		time.Sleep(40 * time.Second) // t=40s
		synctest.Wait()              // poll #2, retry capped at +60s
		if got := f.Calls(); got != 2 {
			t.Fatalf("after first retry: got %d fetches, want 2", got)
		}

		time.Sleep(59 * time.Second) // t=99s, before the 60s cap
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("retry fired before the interval cap: got %d fetches, want 2", got)
		}

		time.Sleep(time.Second) // t=100s
		synctest.Wait()
		if got := f.Calls(); got != 3 {
			t.Fatalf("after capped retry: got %d fetches, want 3", got)
		}
	})
}

// After a failure recovers, polling must return to the full interval cadence.
func TestRunResumesIntervalAfterRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{err: errors.New("boom")}
		p := weather.New(testutil.NopLogger(), f, bus, 10*time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait() // t=0: poll #1 fails, retry at +30s
		f.set(state.WeatherPayload{IconCode: "01d"}, nil)

		time.Sleep(30 * time.Second) // t=30s: poll #2 succeeds, back to full interval
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("after recovery: got %d fetches, want 2", got)
		}

		// Fast-retry timer is gone; nothing fires 31s later.
		time.Sleep(31 * time.Second)
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("retried after recovery: got %d fetches, want 2", got)
		}

		time.Sleep(10 * time.Minute) // next interval tick
		synctest.Wait()
		if got := f.Calls(); got != 3 {
			t.Fatalf("after interval resumed: got %d fetches, want 3", got)
		}
	})
}

func TestSetIntervalsTakeEffect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{payload: state.WeatherPayload{IconCode: "01d"}}
		p := weather.New(testutil.NopLogger(), f, bus, 10*time.Minute, 30*time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait() // t=0: first poll fires, timer reset to 10min
		if got := f.Calls(); got != 1 {
			t.Fatalf("expected 1 call after startup, got %d", got)
		}

		// Shorten interval to 1 minute; takes effect on next timer reset.
		p.SetIntervals(time.Minute, 5*time.Second)

		// 10 minutes later the original timer fires; next reset uses 1min.
		time.Sleep(10 * time.Minute)
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("expected 2 calls after 10min, got %d", got)
		}

		// 1 minute later the new shorter interval fires.
		time.Sleep(time.Minute)
		synctest.Wait()
		if got := f.Calls(); got != 3 {
			t.Fatalf("expected 3 calls after 1min with new interval, got %d", got)
		}
	})
}

// A non-positive retryInterval disables fast retry: failures wait the full interval.
func TestRunWithoutRetryUsesInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bus := state.NewBus()
		f := &stubFetcher{err: errors.New("boom")}
		p := weather.New(testutil.NopLogger(), f, bus, 10*time.Minute, 0)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.Run(ctx)

		synctest.Wait() // t=0: poll #1 fails
		time.Sleep(31 * time.Second)
		synctest.Wait()
		if got := f.Calls(); got != 1 {
			t.Fatalf("fast-retried despite disabled retry: got %d fetches, want 1", got)
		}

		time.Sleep(10 * time.Minute) // only the interval triggers the next poll
		synctest.Wait()
		if got := f.Calls(); got != 2 {
			t.Fatalf("after interval: got %d fetches, want 2", got)
		}
	})
}
