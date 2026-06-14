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

// needsRestart reports whether any non-Tier-1 field differs between the running
// and saved configs. Tier-1 fields (live-applied on PUT) are zeroed before the
// comparison. Nil and empty slices are normalised so a freshly-loaded config
// (nil Sensors) compares equal to a round-tripped one (empty Sensors).
func needsRestart(running, saved config.Config) bool {
	zeroTier1 := func(c *config.Config) {
		c.LogLevel = "" // applied live via the slog LevelVar
		c.Display.BlankAfter = config.Duration{}
		c.Display.Locale = ""                         // applied live by re-publishing the kiosk SSE event
		c.Display.Labels = config.KioskLabelsConfig{} // applied live the same way
		c.Slideshow.Interval = config.Duration{}
		c.Slideshow.Randomize = false
		c.Weather.PollInterval = config.Duration{}
		c.Weather.RetryInterval = config.Duration{}
		c.Auth.PasswordHash = "" // mutated live via /api/auth/password, never needs a restart
	}
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
	zeroTier1(&r)
	normalizeSlices(&r)
	s := saved
	zeroTier1(&s)
	normalizeSlices(&s)
	return !reflect.DeepEqual(r, s)
}

// applyTier1ToRunning copies only the Tier-1 fields from saved into running so
// the in-process state matches what was just applied live.
func applyTier1ToRunning(running *config.Config, saved config.Config) {
	running.LogLevel = saved.LogLevel
	running.Display.BlankAfter = saved.Display.BlankAfter
	running.Display.Locale = saved.Display.Locale
	running.Display.Labels = saved.Display.Labels
	running.Slideshow.Interval = saved.Slideshow.Interval
	running.Slideshow.Randomize = saved.Slideshow.Randomize
	running.Weather.PollInterval = saved.Weather.PollInterval
	running.Weather.RetryInterval = saved.Weather.RetryInterval
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
