package sensors_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// fixedSource emits a list of readings then stops cleanly.
type fixedSource struct {
	id       string
	readings []sensors.Reading
}

func (f *fixedSource) ID() string { return f.id }
func (f *fixedSource) Start(ctx context.Context, out chan<- sensors.Reading) error {
	for _, r := range f.readings {
		select {
		case out <- r:
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

// errSource fails a fixed number of times, then emits one reading and stops.
type errSource struct {
	mu        sync.Mutex
	id        string
	failsLeft int
	reading   sensors.Reading
}

func (e *errSource) ID() string { return e.id }
func (e *errSource) Start(ctx context.Context, out chan<- sensors.Reading) error {
	e.mu.Lock()
	fail := e.failsLeft > 0
	if fail {
		e.failsLeft--
	}
	e.mu.Unlock()

	if fail {
		return errors.New("transient error")
	}
	select {
	case out <- e.reading:
	case <-ctx.Done():
	}
	return nil
}

func TestRegistryDeliverReadings(t *testing.T) {
	want := []sensors.Reading{
		{DeviceID: "a", Kind: sensors.KindTemperature, Value: 21.0},
		{DeviceID: "a", Kind: sensors.KindHumidity, Value: 55.0},
	}

	var mu sync.Mutex
	var got []sensors.Reading

	reg := sensors.NewRegistry(
		testutil.NopLogger(),
		[]sensors.Source{&fixedSource{id: "a", readings: want}},
		func(r sensors.Reading) {
			mu.Lock()
			got = append(got, r)
			mu.Unlock()
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("got %d readings, want %d", len(got), len(want))
	}
	for i, r := range got {
		if r.DeviceID != want[i].DeviceID || r.Kind != want[i].Kind || r.Value != want[i].Value {
			t.Errorf("reading[%d]: got %+v, want %+v", i, r, want[i])
		}
	}
}

func TestRegistryMultipleSources(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]bool{}

	sources := []sensors.Source{
		&fixedSource{id: "s1", readings: []sensors.Reading{{DeviceID: "s1"}}},
		&fixedSource{id: "s2", readings: []sensors.Reading{{DeviceID: "s2"}}},
	}

	reg := sensors.NewRegistry(testutil.NopLogger(), sources, func(r sensors.Reading) {
		mu.Lock()
		seen[r.DeviceID] = true
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reg.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if !seen["s1"] || !seen["s2"] {
		t.Fatalf("expected both sources, got %v", seen)
	}
}

func TestRegistryRetriesOnError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := sensors.Reading{DeviceID: "x", Kind: sensors.KindMotion, Value: 1}
		src := &errSource{id: "x", failsLeft: 2, reading: want}

		var got []sensors.Reading
		reg := sensors.NewRegistry(
			testutil.NopLogger(),
			[]sensors.Source{src},
			func(r sensors.Reading) { got = append(got, r) },
		)

		// Run returns once the source stops failing and succeeds; the exponential
		// backoff between attempts elapses on synctest's virtual clock. Run's
		// WaitGroup join happens-before the read below, so no lock is needed.
		reg.Run(context.Background())

		if len(got) != 1 || got[0].DeviceID != want.DeviceID {
			t.Fatalf("got %v, want one reading from %q", got, want.DeviceID)
		}
	})
}

func TestRegistryContextCancellation(t *testing.T) {
	synctest.Test(t, func(*testing.T) {
		done := make(chan struct{})
		reg := sensors.NewRegistry(
			testutil.NopLogger(),
			[]sensors.Source{&blockingSource{}},
			func(sensors.Reading) {},
		)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			reg.Run(ctx)
			close(done)
		}()
		synctest.Wait() // the source is started and blocked on ctx.Done

		cancel()
		<-done // synctest fails the test if Run never returns
	})
}

// blockingSource blocks until ctx is cancelled, never emitting.
type blockingSource struct{}

func (b *blockingSource) ID() string { return "blocking" }
func (b *blockingSource) Start(ctx context.Context, _ chan<- sensors.Reading) error {
	<-ctx.Done()
	return nil
}

// schedSource fails instantly 3 times, then "runs" 60s before failing (long
// enough to reset the backoff), then blocks until cancelled.
type schedSource struct {
	mu     sync.Mutex
	id     string
	starts []time.Time
}

func (s *schedSource) ID() string { return s.id }
func (s *schedSource) Start(ctx context.Context, _ chan<- sensors.Reading) error {
	s.mu.Lock()
	s.starts = append(s.starts, time.Now())
	n := len(s.starts)
	s.mu.Unlock()
	switch {
	case n <= 3:
		return errors.New("instant failure")
	case n == 4:
		select {
		case <-time.After(60 * time.Second):
			return errors.New("late failure")
		case <-ctx.Done():
			return nil
		}
	default:
		<-ctx.Done()
		return nil
	}
}

func (s *schedSource) offsets(from time.Time) []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.starts))
	for i, ts := range s.starts {
		out[i] = ts.Sub(from)
	}
	return out
}

// Pins the restart schedule: 1s backoff doubling between instant failures, and
// a reset back to 1s after an attempt that ran for the 60s health threshold.
func TestRegistryBackoffScheduleAndReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &schedSource{id: "x"}
		reg := sensors.NewRegistry(testutil.NopLogger(), []sensors.Source{src}, func(sensors.Reading) {})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		begin := time.Now()
		go func() { reg.Run(ctx); close(done) }()
		time.Sleep(2 * time.Minute) // past the full schedule; the 5th attempt sits on ctx
		synctest.Wait()

		want := []time.Duration{0, 1 * time.Second, 3 * time.Second, 7 * time.Second, 68 * time.Second}
		if got := src.offsets(begin); !slices.Equal(got, want) {
			t.Fatalf("restart schedule: got %v, want %v", got, want)
		}
		cancel()
		<-done
	})
}
