package startup_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/startup"
	"github.com/MateEke/picture-frame/internal/testutil"
	weatheradapter "github.com/MateEke/picture-frame/internal/weather/adapter"
)

func TestNewDisplayController(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.DisplayConfig
		wantErr bool
	}{
		{"wlopm default requires output", config.DisplayConfig{Backend: "", Output: ""}, true},
		{"wlopm default with output", config.DisplayConfig{Backend: "", Output: "HDMI-A-1"}, false},
		{"wlopm explicit", config.DisplayConfig{Backend: config.DisplayBackendWlopm, Output: "HDMI-A-1"}, false},
		{"vcgencmd", config.DisplayConfig{Backend: config.DisplayBackendVcgencmd}, false},
		{"unknown backend", config.DisplayConfig{Backend: "bogus"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, err := startup.NewDisplayController(testutil.NopLogger(), tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ctrl == nil {
				t.Error("got a nil controller")
			}
		})
	}
}

func TestWeatherEnabled(t *testing.T) {
	tests := []struct {
		name       string
		weather    config.WeatherConfig
		production bool
		want       bool
	}{
		{"api key in prod", config.WeatherConfig{APIKey: "k"}, true, true},
		{"api key in dev", config.WeatherConfig{APIKey: "k"}, false, true},
		{"no key in dev uses mock", config.WeatherConfig{}, false, true},
		{"no key in prod disabled", config.WeatherConfig{}, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := startup.WeatherEnabled(&config.Config{Weather: tc.weather}, tc.production)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildWeatherFetcher(t *testing.T) {
	tests := []struct {
		name       string
		weather    config.WeatherConfig
		production bool
		wantNil    bool
		wantOWM    bool // non-nil result: true = real OWM client, false = dev mock
	}{
		{"api key uses owm", config.WeatherConfig{APIKey: "k", Units: "metric"}, true, false, true},
		{"dev without key uses mock", config.WeatherConfig{}, false, false, false},
		{"prod without key disables", config.WeatherConfig{}, true, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := startup.BuildWeatherFetcher(testutil.NopLogger(), &config.Config{Weather: tc.weather}, tc.production)
			if (got == nil) != tc.wantNil {
				t.Errorf("got nil = %v, want nil = %v", got == nil, tc.wantNil)
			}
			if tc.wantNil {
				return
			}
			if _, isOWM := got.(*weatheradapter.OWM); isOWM != tc.wantOWM {
				t.Errorf("OWM client = %v, want %v (got %T)", isOWM, tc.wantOWM, got)
			}
		})
	}
}
