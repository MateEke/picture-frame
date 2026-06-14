package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
)

// An unbuildable source (here: mqtt-subscriber with no hub) is skipped, not fatal.
func TestBuildSourcesSkipsUnavailable(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{Sensors: []config.SensorConfig{
		{ID: "sub", Type: "mqtt-subscriber", Topic: "x", Kind: "temperature"},
	}}

	sources := buildSources(log, cfg, nil) // nil hub: mqtt-subscriber can't be built
	if len(sources) != 0 {
		t.Fatalf("expected the unbuildable source to be skipped, got %d sources", len(sources))
	}
}
