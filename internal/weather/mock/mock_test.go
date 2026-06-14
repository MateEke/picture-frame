package mock_test

import (
	"context"
	"testing"

	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/weather/mock"
)

func TestFetchReturnsConfiguredPayload(t *testing.T) {
	want := state.WeatherPayload{IconCode: "01d", Temp: 18.5, Humidity: 60}
	f := mock.New(want)

	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestNewDefaultReturnsNonEmptyPayload(t *testing.T) {
	got, err := mock.NewDefault().Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IconCode == "" || got.Temp == 0 {
		t.Errorf("default payload looks unseeded: %+v", got)
	}
}
