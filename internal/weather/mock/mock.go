// Package mock provides a static weather.Fetcher for development and tests.
package mock

import (
	"context"

	"github.com/MateEke/picture-frame/internal/state"
)

// Fetcher returns a fixed payload on every Fetch.
type Fetcher struct {
	payload state.WeatherPayload
}

// New returns a Fetcher that always reports payload.
func New(payload state.WeatherPayload) *Fetcher {
	return &Fetcher{payload: payload}
}

// NewDefault returns a Fetcher with a development payload.
func NewDefault() *Fetcher {
	return New(state.WeatherPayload{IconCode: "01d", Temp: 18.5, Humidity: 60})
}

// Fetch returns the configured payload and never errors.
func (f *Fetcher) Fetch(context.Context) (state.WeatherPayload, error) {
	return f.payload, nil
}
