package mock_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/sensors/mock"
)

func TestMockID(t *testing.T) {
	src := mock.New("my-sensor", time.Millisecond)
	if src.ID() != "my-sensor" {
		t.Errorf("ID: got %q, want my-sensor", src.ID())
	}
}

func TestMockEmitsReadings(t *testing.T) {
	readings := []mock.Reading{
		{Kind: sensors.KindTemperature, Value: 21.5},
		{Kind: sensors.KindHumidity, Value: 60.0},
	}
	src := mock.New("dev", time.Millisecond, readings...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out := make(chan sensors.Reading, 10)
	go func() { _ = src.Start(ctx, out) }()

	got := make([]sensors.Reading, 0, len(readings))
	for range readings {
		select {
		case r := <-out:
			got = append(got, r)
		case <-ctx.Done():
			t.Fatal("timeout waiting for readings")
		}
	}

	for i, r := range got {
		if r.Kind != readings[i].Kind || r.Value != readings[i].Value {
			t.Errorf("reading[%d]: got %+v, want %+v", i, r, readings[i])
		}
		if r.Timestamp.IsZero() {
			t.Errorf("reading[%d]: timestamp not set", i)
		}
	}
}

func TestMockDeltaApplied(t *testing.T) {
	src := mock.New("dev", time.Millisecond,
		mock.Reading{Kind: sensors.KindTemperature, Value: 20.0, Delta: 1.0},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out := make(chan sensors.Reading, 10)
	go func() { _ = src.Start(ctx, out) }()

	want := []float64{20.0, 21.0, 22.0}
	for _, wantVal := range want {
		select {
		case r := <-out:
			if r.Value != wantVal {
				t.Errorf("got value %v, want %v", r.Value, wantVal)
			}
		case <-ctx.Done():
			t.Fatal("timeout")
		}
	}
}

func TestMockContextCancellation(t *testing.T) {
	src := mock.New("dev", time.Hour, mock.Reading{Kind: sensors.KindMotion, Value: 1})

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan sensors.Reading, 1)

	done := make(chan error, 1)
	go func() { done <- src.Start(ctx, out) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil on cancel, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestMockEmptyReadingsWaitsForCancel(t *testing.T) {
	src := mock.New("dev", time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan sensors.Reading, 1)

	done := make(chan error, 1)
	go func() { done <- src.Start(ctx, out) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return")
	}
}

// A non-positive interval makes time.After fire at once and spin the loop.
func TestMockNonPositiveIntervalUsesDefault(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				src := mock.New("dev", interval, mock.Reading{Kind: sensors.KindTemperature, Value: 20})
				out := make(chan sensors.Reading, 10)
				go func() { _ = src.Start(t.Context(), out) }()

				synctest.Wait()
				<-out // the first reading is immediate
				time.Sleep(mock.DefaultInterval - time.Nanosecond)
				synctest.Wait()
				if len(out) != 0 {
					t.Fatalf("emitted %d readings before the default interval", len(out))
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				if len(out) != 1 {
					t.Fatalf("expected one reading at the default interval, got %d", len(out))
				}
			})
		})
	}
}
