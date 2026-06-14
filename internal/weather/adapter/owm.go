// Package adapter implements weather.Fetcher backends for external providers.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
)

const (
	owmBaseURL     = "https://api.openweathermap.org/data/2.5/weather"
	defaultUnits   = "metric"
	requestTimeout = 15 * time.Second
)

// OWM fetches current conditions from OpenWeatherMap.
type OWM struct {
	apiKey  string
	lat     float64
	lon     float64
	units   string
	baseURL string
	client  *http.Client
}

// NewOWM builds an OWM fetcher; empty units defaults to "metric".
func NewOWM(apiKey string, lat, lon float64, units string) *OWM {
	if units == "" {
		units = defaultUnits
	}
	return &OWM{
		apiKey:  apiKey,
		lat:     lat,
		lon:     lon,
		units:   units,
		baseURL: owmBaseURL,
		client:  &http.Client{Timeout: requestTimeout},
	}
}

// owmResponse is the subset of the OWM payload we use.
type owmResponse struct {
	Weather []struct {
		Icon string `json:"icon"`
	} `json:"weather"`
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity float64 `json:"humidity"`
	} `json:"main"`
}

// Fetch retrieves current conditions for the configured coordinates.
func (o *OWM) Fetch(ctx context.Context) (state.WeatherPayload, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(o.lat, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(o.lon, 'f', -1, 64))
	q.Set("units", o.units)
	q.Set("appid", o.apiKey)
	reqURL := o.baseURL + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return state.WeatherPayload{}, fmt.Errorf("owm: build request: %w", err)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return state.WeatherPayload{}, fmt.Errorf("owm: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return state.WeatherPayload{}, fmt.Errorf("owm: unexpected status %d", resp.StatusCode)
	}

	return parseOWM(resp.Body)
}

// parseOWM decodes an OWM response into a WeatherPayload.
func parseOWM(r io.Reader) (state.WeatherPayload, error) {
	var body owmResponse
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return state.WeatherPayload{}, fmt.Errorf("owm: decode response: %w", err)
	}
	if len(body.Weather) == 0 {
		return state.WeatherPayload{}, fmt.Errorf("owm: response missing weather data")
	}
	return state.WeatherPayload{
		IconCode: body.Weather[0].Icon,
		Temp:     body.Main.Temp,
		Humidity: body.Main.Humidity,
	}, nil
}
