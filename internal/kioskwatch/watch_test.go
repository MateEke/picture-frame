package kioskwatch_test

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/kioskwatch"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func newWatch(timeout time.Duration) (*kioskwatch.Watch, *atomic.Int32) {
	var calls atomic.Int32
	w := kioskwatch.New(testutil.NopLogger(), timeout, func() { calls.Add(1) })
	return w, &calls
}

// runWatch starts w.Run in a goroutine and returns a channel closed when it returns.
func runWatch(ctx context.Context, w *kioskwatch.Watch) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	return done
}

func TestWatchBeatResetsLastSeen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, calls := newWatch(15 * time.Second)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := runWatch(ctx, w)

		time.Sleep(9 * time.Second)
		w.Beat("v1")
		time.Sleep(13 * time.Second)

		select {
		case <-done:
			t.Fatal("Run returned at t=20 tick despite Beat resetting the deadline")
		default:
		}
		if got := calls.Load(); got != 0 {
			t.Errorf("onTimeout fired %d times despite Beat", got)
		}

		cancel()
		<-done
	})
}

func TestWatchRunReturnsAfterTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, calls := newWatch(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 11*time.Second)
		defer cancel()
		done := runWatch(ctx, w)

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Run did not return after timeout was crossed")
		}

		if got := calls.Load(); got != 1 {
			t.Errorf("expected onTimeout to fire exactly once via Run, got %d", got)
		}
	})
}

func TestWatchRunReturnsOnContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, calls := newWatch(100 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		done := runWatch(ctx, w)

		cancel()

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Run did not return after context cancellation")
		}

		if got := calls.Load(); got != 0 {
			t.Errorf("onTimeout should not fire on context cancellation, got %d", got)
		}
	})
}

func TestOnBeatObservesUntilHandled(t *testing.T) {
	w := kioskwatch.New(testutil.NopLogger(), time.Minute, func() {})
	var seen []string
	w.OnBeat(func(v string) bool {
		seen = append(seen, v)
		return v == "new" // handled (and cleared) only once the new build beats
	})
	w.Beat("old") // mismatch → keep observing
	w.Beat("new") // handled → cleared
	w.Beat("new") // already cleared → not observed
	if want := []string{"old", "new"}; !slices.Equal(seen, want) {
		t.Errorf("observed %v, want %v", seen, want)
	}
}

// Concurrent beats must fire the handler exactly once; a double-fire would, e.g., close the
// commit channel twice and panic. -race also catches the underlying data race.
func TestOnBeatHandledOnceUnderConcurrentBeats(t *testing.T) {
	w := kioskwatch.New(testutil.NopLogger(), time.Minute, func() {})
	var fired atomic.Int32
	w.OnBeat(func(string) bool {
		fired.Add(1)
		return true
	})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { w.Beat("v1") })
	}
	wg.Wait()

	if got := fired.Load(); got != 1 {
		t.Errorf("handler fired %d times under concurrent beats, want exactly 1", got)
	}
}

// A tick landing at exactly the timeout must not fire; only past it.
func TestWatchExactTimeoutBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, calls := newWatch(30 * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		runWatch(ctx, w)

		time.Sleep(30 * time.Second) // tick at t+30: elapsed == timeout
		synctest.Wait()
		if calls.Load() != 0 {
			t.Fatal("watchdog fired at exactly the timeout boundary")
		}
		time.Sleep(10 * time.Second) // tick at t+40: past the timeout
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("watchdog should fire once past the timeout, got %d", calls.Load())
		}
	})
}
