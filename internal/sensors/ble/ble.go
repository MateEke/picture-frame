package ble

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/sensors"
)

const defaultPollInterval = 80 * time.Second

// Adapter connects to BLE peripherals and can power-cycle the host adapter.
type Adapter interface {
	Connect(ctx context.Context, mac string, addressType string, uuids []string) (Device, error)
	// Reset power-cycles the adapter; called after connect fails for resetAfter.
	Reset(ctx context.Context) error
}

// Device is a live BLE connection with pre-discovered characteristics.
type Device interface {
	Subscribe(uuid string, handler func(data []byte)) error
	Read(uuid string) ([]byte, error)
	Disconnect() error
}

// Source implements sensors.Source for one BLE peripheral: GATT notifications
// for low latency, timer polling as fallback and disconnect detection.
type Source struct {
	cfg          config.SensorConfig
	adapter      Adapter
	log          *slog.Logger
	pollInterval time.Duration
	resetAfter   time.Duration
	decoders     map[string]sensors.Decoder
	firstFailure time.Time // zero when last connect attempt succeeded
}

// New builds a Source, pre-validating all decoder names from cfg.
func New(log *slog.Logger, cfg config.SensorConfig, adapter Adapter) (*Source, error) {
	decoders := make(map[string]sensors.Decoder, len(cfg.Characteristics))
	for _, c := range cfg.Characteristics {
		dec, ok := sensors.LookupDecoder(c.Decoder)
		if !ok {
			return nil, fmt.Errorf("ble %q: unknown decoder %q for uuid %s", cfg.ID, c.Decoder, c.UUID)
		}
		decoders[c.UUID] = dec
	}

	s := &Source{
		cfg:          cfg,
		adapter:      adapter,
		log:          log,
		pollInterval: defaultPollInterval,
		decoders:     decoders,
	}
	// resetAfter defaults to 0 (disabled); both intervals are overridable via config.
	if cfg.PollInterval.Duration > 0 {
		s.pollInterval = cfg.PollInterval.Duration
	}
	if cfg.ResetAfter.Duration > 0 {
		s.resetAfter = cfg.ResetAfter.Duration
	}
	return s, nil
}

func (s *Source) ID() string { return s.cfg.ID }

func (s *Source) Start(ctx context.Context, out chan<- sensors.Reading) error {
	// BlueZ can wedge after repeated failures; a power cycle is the only reliable
	// recovery. resetAfter == 0 (the default) disables it.
	if s.resetAfter > 0 && !s.firstFailure.IsZero() && time.Since(s.firstFailure) >= s.resetAfter {
		s.log.Warn("ble: adapter unresponsive, power cycling",
			"id", s.cfg.ID,
			"failing_for", time.Since(s.firstFailure).Round(time.Second))
		if err := s.adapter.Reset(ctx); err != nil {
			s.log.Error("ble: adapter reset failed", "id", s.cfg.ID, "err", err)
		}
		s.firstFailure = time.Now() // restart the window after reset
	}

	uuids := make([]string, len(s.cfg.Characteristics))
	for i, c := range s.cfg.Characteristics {
		uuids[i] = c.UUID
	}

	s.log.Info("ble: scanning then connecting", "id", s.cfg.ID, "mac", s.cfg.MAC)
	dev, err := s.adapter.Connect(ctx, s.cfg.MAC, s.cfg.AddressType, uuids)
	if err != nil {
		if s.firstFailure.IsZero() {
			s.firstFailure = time.Now()
		}
		return fmt.Errorf("ble %s: connect: %w", s.cfg.ID, err)
	}
	s.firstFailure = time.Time{} // clear on successful connect

	defer func() {
		s.log.Info("ble: disconnecting", "id", s.cfg.ID)
		_ = dev.Disconnect()
	}()
	s.log.Info("ble: connected", "id", s.cfg.ID)

	// GATT notifications: the low-latency path for motion.
	for _, c := range s.cfg.Characteristics {
		dec := s.decoders[c.UUID]
		if err := dev.Subscribe(c.UUID, func(data []byte) {
			s.emit(out, c, dec, data)
		}); err != nil {
			s.log.Warn("ble: subscribe failed, relying on poll", "id", s.cfg.ID, "uuid", c.UUID, "err", err)
		}
	}

	// Poll on a timer: scheduled readings plus disconnect detection via Read
	// errors (BlueZ doesn't always surface disconnects).
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.poll(dev, out); err != nil {
				return fmt.Errorf("ble %s: %w", s.cfg.ID, err)
			}
		}
	}
}

func (s *Source) poll(dev Device, out chan<- sensors.Reading) error {
	for _, c := range s.cfg.Characteristics {
		data, err := dev.Read(c.UUID)
		if err != nil {
			return fmt.Errorf("poll read %s: %w", c.UUID, err)
		}
		s.emit(out, c, s.decoders[c.UUID], data)
	}
	return nil
}

func (s *Source) emit(out chan<- sensors.Reading, c config.CharacteristicConfig, dec sensors.Decoder, data []byte) {
	v, err := dec(data)
	if err != nil {
		s.log.Warn("ble: decode error", "id", s.cfg.ID, "uuid", c.UUID, "err", err)
		return
	}
	select {
	case out <- sensors.Reading{
		DeviceID:  s.cfg.ID,
		Kind:      sensors.Kind(c.Kind),
		Value:     v,
		Timestamp: time.Now(),
	}:
	default:
		s.log.Warn("ble: reading dropped, channel full", "id", s.cfg.ID)
	}
}
