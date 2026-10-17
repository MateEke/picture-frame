package config_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
)

func TestParseClock(t *testing.T) {
	cases := []struct {
		in      string
		want    config.Clock
		wantErr bool
	}{
		{"00:00", config.Clock{}, false},
		{"07:05", config.Clock{Hour: 7, Minute: 5}, false},
		{"23:59", config.Clock{Hour: 23, Minute: 59}, false},
		{"7:00", config.Clock{}, true},
		{"24:00", config.Clock{}, true},
		{"12:60", config.Clock{}, true},
		{"07:00 ", config.Clock{}, true},
		{"aa:bb", config.Clock{}, true},
		{"", config.Clock{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := config.ParseClock(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestClockString(t *testing.T) {
	if got := (config.Clock{Hour: 7, Minute: 5}).String(); got != "07:05" {
		t.Errorf("got %q, want 07:05", got)
	}
}
