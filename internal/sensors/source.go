package sensors

import (
	"context"
	"time"
)

// Kind identifies what a sensor is measuring.
type Kind string

const (
	KindTemperature Kind = "temperature"
	KindHumidity    Kind = "humidity"
	KindMotion      Kind = "motion"
)

// Reading is a single observation from a sensor source.
type Reading struct {
	DeviceID  string
	Kind      Kind
	Value     float64 // motion: 0 = no motion, 1 = motion detected
	Timestamp time.Time
}

// Source is a pluggable sensor backend. Start blocks until ctx is cancelled or a
// fatal error, sending readings on out (the caller owns and never closes it).
type Source interface {
	ID() string
	Start(ctx context.Context, out chan<- Reading) error
}
