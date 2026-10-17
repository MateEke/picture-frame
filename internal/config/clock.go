package config

import (
	"fmt"
	"time"
)

type Clock struct {
	Hour, Minute int
}

// ParseClock parses a strict 24-hour "HH:MM".
func ParseClock(s string) (Clock, error) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != len("15:04") {
		return Clock{}, fmt.Errorf("invalid time %q, want HH:MM", s)
	}
	return Clock{Hour: t.Hour(), Minute: t.Minute()}, nil
}

func (c Clock) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }
