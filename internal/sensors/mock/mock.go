package mock

import (
	"context"
	"time"

	"github.com/MateEke/picture-frame/internal/sensors"
)

// Reading describes one value the mock source emits each cycle.
// Delta is added to Value after every full cycle; zero means constant.
type Reading struct {
	Kind  sensors.Kind
	Value float64
	Delta float64
}

// Source emits a fixed list of readings on a configurable interval,
// cycling until ctx is cancelled. Used in tests and local development.
type Source struct {
	id       string
	interval time.Duration
	readings []Reading
}

func New(id string, interval time.Duration, readings ...Reading) *Source {
	return &Source{id: id, interval: interval, readings: readings}
}

func (s *Source) ID() string { return s.id }

func (s *Source) Start(ctx context.Context, out chan<- sensors.Reading) error {
	if len(s.readings) == 0 {
		<-ctx.Done()
		return nil
	}
	values := make([]float64, len(s.readings))
	for i, r := range s.readings {
		values[i] = r.Value
	}
	for {
		for i, r := range s.readings {
			select {
			case out <- sensors.Reading{
				DeviceID:  s.id,
				Kind:      r.Kind,
				Value:     values[i],
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
				return nil
			}
			select {
			case <-time.After(s.interval):
			case <-ctx.Done():
				return nil
			}
		}
		for i := range s.readings {
			values[i] += s.readings[i].Delta
		}
	}
}
