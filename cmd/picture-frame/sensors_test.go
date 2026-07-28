package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/hostmetrics"
	"github.com/MateEke/picture-frame/internal/version"
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

func TestDeviceMetaRaspberryPi(t *testing.T) {
	m := deviceMeta(hostmetrics.HostInfo{Model: "Raspberry Pi Zero 2 W Rev 1.0", Revision: "Rev 1.0"}, "192.168.2.99", ":8080")
	if m.Manufacturer != "Raspberry Pi Ltd" {
		t.Errorf("manufacturer: %q", m.Manufacturer)
	}
	if m.Model != "Raspberry Pi Zero 2 W Rev 1.0" || m.HwVersion != "Rev 1.0" {
		t.Errorf("model/hw: %+v", m)
	}
	if m.SwVersion != version.Version {
		t.Errorf("sw version: %q, want %q", m.SwVersion, version.Version)
	}
	if m.ConfigurationURL != "http://192.168.2.99:8080" {
		t.Errorf("url: %q", m.ConfigurationURL)
	}
}

func TestDeviceMetaNonPiOmitsManufacturer(t *testing.T) {
	m := deviceMeta(hostmetrics.HostInfo{Model: "Some SBC"}, "10.0.0.5", "127.0.0.1:9000")
	if m.Manufacturer != "" {
		t.Errorf("manufacturer should be empty for a non-Pi model: %q", m.Manufacturer)
	}
	if m.ConfigurationURL != "http://10.0.0.5:9000" {
		t.Errorf("url: %q", m.ConfigurationURL)
	}
}

func TestDeviceMetaNoURLWithoutIP(t *testing.T) {
	if got := deviceMeta(hostmetrics.HostInfo{}, "", ":8080").ConfigurationURL; got != "" {
		t.Errorf("url should be empty without an IP: %q", got)
	}
}

func TestDeviceMetaNoURLWhenAddrHasNoPort(t *testing.T) {
	if got := deviceMeta(hostmetrics.HostInfo{}, "10.0.0.5", "no-port").ConfigurationURL; got != "" {
		t.Errorf("url should be empty when addr has no port: %q", got)
	}
}
