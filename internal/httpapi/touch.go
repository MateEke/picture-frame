package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/config"
)

// TouchSettingsDTO is the subset of config the on-device touch UI may change.
// It gets its own loopback-exempt route instead of exempting /api/config, which
// also carries the updater source, MQTT broker and other admin-only settings.
type TouchSettingsDTO struct {
	Sleep       SleepDTO `json:"sleep"`
	Interval    string   `json:"interval" doc:"Slideshow advance interval, e.g. \"2m\""`
	Randomize   bool     `json:"randomize"`
	SplitScreen bool     `json:"split_screen"`
	Brightness  int      `json:"brightness" minimum:"0" maximum:"100"`
	Rotation    int      `json:"rotation" enum:"0,90,180,270"`
}

// TouchSettingsBody adds read-only capability flags for the settings view.
type TouchSettingsBody struct {
	TouchSettingsDTO
	BrightnessSupported bool `json:"brightness_supported" doc:"true if the panel has a controllable backlight"`
	RotationSupported   bool `json:"rotation_supported" doc:"true if the display backend can rotate"`
}

type getTouchSettingsOutput struct {
	Body TouchSettingsBody
}

type putTouchSettingsInput struct {
	Body TouchSettingsDTO
}

func (s *server) registerTouchRoutes(api huma.API) {
	s.kioskExempt("/api/touch/settings")

	huma.Register(api, huma.Operation{
		OperationID: "get-touch-settings",
		Method:      http.MethodGet,
		Path:        "/api/touch/settings",
		Summary:     "Get the settings the touch UI can edit",
	}, func(_ context.Context, _ *struct{}) (*getTouchSettingsOutput, error) {
		if s.store == nil {
			return nil, huma.Error503ServiceUnavailable("config store unavailable")
		}
		c := s.store.Snapshot()
		dto := toDTO(c)
		return &getTouchSettingsOutput{Body: TouchSettingsBody{
			TouchSettingsDTO: TouchSettingsDTO{
				Sleep:       dto.Sleep,
				Interval:    dto.Slideshow.Interval,
				Randomize:   c.Slideshow.Randomize,
				SplitScreen: c.Slideshow.SplitScreen,
				Brightness:  c.Display.Brightness,
				Rotation:    c.Display.Rotation,
			},
			BrightnessSupported: s.backlight,
			RotationSupported:   s.rotator != nil && s.rotator.Supported(),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "put-touch-settings",
		Method:        http.MethodPut,
		Path:          "/api/touch/settings",
		Summary:       "Save the settings the touch UI can edit (all apply live)",
		DefaultStatus: http.StatusNoContent,
		MaxBodyBytes:  8 * 1024,
	}, func(_ context.Context, input *putTouchSettingsInput) (*struct{}, error) {
		if s.store == nil {
			return nil, huma.Error503ServiceUnavailable("config store unavailable")
		}
		_, err := s.saveConfig(func(c config.Config) (config.Config, error) {
			return applyTouchSettings(input.Body, c)
		})
		return nil, err
	})
}

func applyTouchSettings(dto TouchSettingsDTO, c config.Config) (config.Config, error) {
	if err := applySleepDTO(&c.Sleep, dto.Sleep); err != nil {
		return config.Config{}, err
	}
	interval, err := parseDuration(dto.Interval, "interval")
	if err != nil {
		return config.Config{}, err
	}
	if interval.Duration <= 0 {
		return config.Config{}, fmt.Errorf("interval: must be a positive duration")
	}
	c.Slideshow.Interval = interval
	c.Slideshow.Randomize = dto.Randomize
	c.Slideshow.SplitScreen = dto.SplitScreen
	if c.Slideshow.SplitScreen && c.Slideshow.PairThreshold <= 1 {
		c.Slideshow.PairThreshold = config.DefaultPairThreshold
	}
	c.Display.Brightness = dto.Brightness
	c.Display.Rotation = dto.Rotation
	return c, nil
}
