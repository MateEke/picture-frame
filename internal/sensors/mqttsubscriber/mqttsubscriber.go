// Package mqttsubscriber implements sensors.Source over MQTT state topics.
package mqttsubscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/sensors"
)

// Hub is the subset of *mqtt.Hub the subscriber needs.
type Hub interface {
	Subscribe(topic string, qos byte, handler func([]byte))
}

const defaultQoS byte = 0

// Source is one mqtt-subscriber sensor.
type Source struct {
	log       *slog.Logger
	id        string
	topic     string
	kind      sensors.Kind
	decoder   sensors.Decoder
	jsonField string
	// internal decouples paho's callback goroutine from Start so a late
	// delivery after Start returned drops instead of writing to a closed out.
	internal chan sensors.Reading
}

// New registers the subscription with the Hub up front so it's in place before
// Hub.Connect fires.
func New(log *slog.Logger, cfg config.SensorConfig, hub Hub) (*Source, error) {
	dec, ok := sensors.LookupDecoder(cfg.Parser)
	if !ok {
		return nil, fmt.Errorf("mqtt-subscriber %q: unknown parser %q", cfg.ID, cfg.Parser)
	}
	s := &Source{
		log:       log,
		id:        cfg.ID,
		topic:     cfg.Topic,
		kind:      sensors.Kind(cfg.Kind),
		decoder:   dec,
		jsonField: cfg.JSONField,
		internal:  make(chan sensors.Reading, 8),
	}
	hub.Subscribe(cfg.Topic, defaultQoS, func(payload []byte) {
		r, ok := s.decode(payload)
		if !ok {
			return
		}
		select {
		case s.internal <- r:
		default:
			s.log.Warn("mqtt-subscriber: reading dropped, internal full", "id", s.id)
		}
	})
	return s, nil
}

func (s *Source) ID() string { return s.id }

// Start drains the internal queue to out until ctx is cancelled.
func (s *Source) Start(ctx context.Context, out chan<- sensors.Reading) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-s.internal:
			select {
			case out <- r:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (s *Source) decode(payload []byte) (sensors.Reading, bool) {
	data := payload
	if s.jsonField != "" {
		extracted, err := extractJSONField(payload, s.jsonField)
		if err != nil {
			s.log.Warn("mqtt-subscriber: json extract failed",
				"id", s.id, "field", s.jsonField, "err", err)
			return sensors.Reading{}, false
		}
		data = extracted
	}
	v, err := s.decoder(data)
	if err != nil {
		s.log.Warn("mqtt-subscriber: decode failed", "id", s.id, "err", err)
		return sensors.Reading{}, false
	}
	return sensors.Reading{
		DeviceID:  s.id,
		Kind:      s.kind,
		Value:     v,
		Timestamp: time.Now(),
	}, true
}

// extractJSONField walks payload by dotted path and returns the leaf as text.
func extractJSONField(payload []byte, path string) ([]byte, error) {
	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	parts := strings.Split(path, ".")
	cur := root
	for i, p := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: %q is not an object", path, strings.Join(parts[:i], "."))
		}
		cur, ok = obj[p]
		if !ok {
			return nil, fmt.Errorf("path %q: field %q missing", path, strings.Join(parts[:i+1], "."))
		}
	}
	return formatLeaf(cur), nil
}

func formatLeaf(v any) []byte {
	switch x := v.(type) {
	case string:
		return []byte(x)
	case float64:
		return []byte(strconv.FormatFloat(x, 'f', -1, 64))
	case bool:
		if x {
			return []byte("true")
		}
		return []byte("false")
	case nil:
		return []byte("null")
	default:
		// Object/array → raw JSON; lossless but rarely useful.
		b, _ := json.Marshal(v)
		return b
	}
}
