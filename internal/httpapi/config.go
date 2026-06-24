package httpapi

import (
	"context"
	"net/http"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/sensors"
	"github.com/MateEke/picture-frame/internal/state"
)

// LiveConfig applies Tier-1 config changes to running subsystems without a restart.
type LiveConfig interface {
	ApplyLive(cfg config.Config)
}

// copyTier1 overwrites dst's Tier-1 fields with src's: the single definition of
// which config fields apply live (no restart). Both needsRestart and
// applyTier1ToRunning derive from it; keep it in sync with ApplyLive, which does
// the actual live application. Auth.PasswordHash is excluded (mutated via
// /api/auth/password, handled separately in needsRestart).
func copyTier1(dst *config.Config, src config.Config) {
	dst.LogLevel = src.LogLevel // applied live via the slog LevelVar
	dst.Display.BlankAfter = src.Display.BlankAfter
	dst.Display.Locale = src.Display.Locale // re-published on the kiosk SSE event
	dst.Display.HideClockDate = src.Display.HideClockDate
	dst.Display.Timezone = src.Display.Timezone
	dst.Display.Labels = src.Display.Labels
	dst.Slideshow.Interval = src.Slideshow.Interval
	dst.Slideshow.Randomize = src.Slideshow.Randomize
	dst.Slideshow.SplitScreen = src.Slideshow.SplitScreen // slideshow.SetSplitConfig
	dst.Slideshow.PairThreshold = src.Slideshow.PairThreshold
	dst.Weather.PollInterval = src.Weather.PollInterval
	dst.Weather.RetryInterval = src.Weather.RetryInterval
}

// needsRestart reports whether any non-Tier-1 field differs between the running
// and saved configs. Nil and empty slices are normalised so a freshly-loaded
// config (nil Sensors) compares equal to a round-tripped one (empty Sensors).
func needsRestart(running, saved config.Config) bool {
	normalizeSlices := func(c *config.Config) {
		if c.Sensors == nil {
			c.Sensors = []config.SensorConfig{}
		}
		for i := range c.Sensors {
			if c.Sensors[i].Characteristics == nil {
				c.Sensors[i].Characteristics = []config.CharacteristicConfig{}
			}
			if c.Sensors[i].MockReadings == nil {
				c.Sensors[i].MockReadings = []config.MockReadingConfig{}
			}
		}
	}
	r := running
	copyTier1(&r, saved)     // ignore Tier-1 differences
	r.Auth.PasswordHash = "" // mutated via /api/auth/password, never a restart
	s := saved
	s.Auth.PasswordHash = ""
	normalizeSlices(&r)
	normalizeSlices(&s)
	return !reflect.DeepEqual(r, s)
}

// applyTier1ToRunning syncs the in-process running config to what was just
// applied live, so the next needsRestart comparison is correct.
func applyTier1ToRunning(running *config.Config, saved config.Config) {
	copyTier1(running, saved)
}

type getConfigOutput struct {
	Body ConfigResponseBody
}

type getConfigMetaOutput struct {
	Body ConfigMetaBody
}

type putConfigInput struct {
	Body ConfigDTO
}

type putConfigOutput struct {
	Body PutConfigResponseBody
}

func (s *server) registerConfigRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-config",
		Method:      http.MethodGet,
		Path:        "/api/config",
		Summary:     "Get current configuration",
	}, func(_ context.Context, _ *struct{}) (*getConfigOutput, error) {
		saved := s.store.Snapshot()
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()
		return &getConfigOutput{Body: ConfigResponseBody{
			ConfigDTO:      toDTO(saved),
			RestartPending: needsRestart(running, saved),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-config-meta",
		Method:      http.MethodGet,
		Path:        "/api/config/meta",
		Summary:     "Get configuration metadata (enum values for UI selects)",
	}, func(_ context.Context, _ *struct{}) (*getConfigMetaOutput, error) {
		return &getConfigMetaOutput{Body: ConfigMetaBody{
			Decoders:     sensors.DecoderNames(),
			Kinds:        []string{"temperature", "humidity", "motion"},
			Units:        []string{"standard", "metric", "imperial"},
			Backends:     []string{config.BackendFS, config.BackendImmich},
			SensorTypes:  []string{"ble", "mqtt-subscriber", "mock"},
			AddressTypes: []string{"random", "public"},
			LogLevels:    []string{"debug", "info", "warn", "error"},
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:  "put-config",
		Method:       http.MethodPut,
		Path:         "/api/config",
		Summary:      "Save configuration (Tier-1 fields apply live; others require restart)",
		MaxBodyBytes: 64 * 1024,
	}, func(_ context.Context, input *putConfigInput) (*putConfigOutput, error) {
		return s.handlePutConfig(input)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "system-restart",
		Method:        http.MethodPost,
		Path:          "/api/system/restart",
		Summary:       "Reload the frame backend in place (re-exec; systemd sees no crash)",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		if s.restart == nil {
			return nil, huma.Error503ServiceUnavailable("restart not configured")
		}
		go func() {
			// Brief delay so the 202 response flushes to the client before we re-exec.
			time.Sleep(200 * time.Millisecond)
			if err := s.restart(); err != nil {
				s.log.Error("restart failed", "err", err)
			}
		}()
		return nil, nil
	})
}

func (s *server) handlePutConfig(input *putConfigInput) (*putConfigOutput, error) {
	var newCfg config.Config
	var validationErr error
	if err := s.store.Update(func(c *config.Config) error {
		applied, err := applyDTO(input.Body, *c)
		if err != nil {
			validationErr = err
			return err
		}
		if err := applied.Validate(); err != nil {
			validationErr = err
			return err
		}
		*c = applied
		newCfg = applied
		return nil
	}); err != nil {
		if validationErr != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return nil, huma.Error500InternalServerError("failed to save config: " + err.Error())
	}

	s.mu.Lock()
	if s.liveConfig != nil {
		s.liveConfig.ApplyLive(newCfg)
	}
	applyTier1ToRunning(&s.running, newCfg)
	running := s.running
	s.mu.Unlock()

	s.bus.Publish(state.Event{Kind: state.KindKiosk, Payload: KioskEventPayload(newCfg, s.weatherActive)})

	return &putConfigOutput{Body: PutConfigResponseBody{
		RestartPending: needsRestart(running, newCfg),
	}}, nil
}
