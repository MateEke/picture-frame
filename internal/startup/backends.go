package startup

import (
	"fmt"
	"log/slog"

	"github.com/MateEke/picture-frame/internal/config"
	displaypkg "github.com/MateEke/picture-frame/internal/display"
	displayadapter "github.com/MateEke/picture-frame/internal/display/adapter"
	"github.com/MateEke/picture-frame/internal/weather"
	weatheradapter "github.com/MateEke/picture-frame/internal/weather/adapter"
	weathermock "github.com/MateEke/picture-frame/internal/weather/mock"
)

// NewDisplayController builds the production controller: wlopm (default) or
// vcgencmd (legacy fkms).
func NewDisplayController(log *slog.Logger, cfg config.DisplayConfig) (displaypkg.Controller, error) {
	switch cfg.Backend {
	case "", config.DisplayBackendWlopm:
		if cfg.Output == "" {
			return nil, fmt.Errorf("display.output is required for the %s backend", config.DisplayBackendWlopm)
		}
		log.Info("display: using wlopm", "output", cfg.Output)
		return displayadapter.NewWlopm(cfg.Output, log), nil
	case config.DisplayBackendVcgencmd:
		log.Info("display: using vcgencmd (legacy fkms)")
		return displayadapter.NewVcgencmd(), nil
	default:
		return nil, fmt.Errorf("unknown display backend %q", cfg.Backend)
	}
}

// WeatherEnabled reports whether a weather fetcher runs (real with an api_key,
// the dev mock otherwise). Only prod-without-key disables it.
func WeatherEnabled(cfg *config.Config, production bool) bool {
	return cfg.Weather.APIKey != "" || !production
}

// BuildWeatherFetcher returns the OWM client when an API key is set, a static
// mock in dev, or nil to disable weather in prod.
func BuildWeatherFetcher(log *slog.Logger, cfg *config.Config, production bool) weather.Fetcher {
	if !WeatherEnabled(cfg, production) {
		log.Info("weather: disabled (no api_key configured)")
		return nil
	}
	if cfg.Weather.APIKey != "" {
		log.Info("weather: using OpenWeatherMap", "lat", cfg.Weather.Lat, "lon", cfg.Weather.Lon, "units", cfg.Weather.Units)
		return weatheradapter.NewOWM(cfg.Weather.APIKey, cfg.Weather.Lat, cfg.Weather.Lon, cfg.Weather.Units)
	}
	log.Info("weather: using static mock (development mode)")
	return weathermock.NewDefault()
}
