package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MateEke/picture-frame/internal/display/adapter"
)

func TestBacklightScalesPercent(t *testing.T) {
	base := t.TempDir()
	dev := filepath.Join(base, "11-0045")
	if err := os.MkdirAll(dev, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "max_brightness"), []byte("31\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev, "brightness"), []byte("31"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := adapter.NewBacklight(base)
	if !b.Supported() {
		t.Fatal("device not found")
	}
	for pct, want := range map[int]string{50: "16", 100: "31", 1: "1", 150: "31"} {
		if err := b.Set(pct); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(filepath.Join(dev, "brightness"))
		if string(got) != want {
			t.Errorf("Set(%d) wrote %q, want %q", pct, got, want)
		}
	}
	if err := b.Set(0); err != nil {
		t.Fatal("0 must be a no-op")
	}
}

func TestBacklightMissing(t *testing.T) {
	b := adapter.NewBacklight(filepath.Join(t.TempDir(), "nope"))
	if b.Supported() {
		t.Fatal("unexpected device")
	}
	if b.Set(50) == nil {
		t.Fatal("expected error without a device")
	}
	if b.Set(0) != nil {
		t.Fatal("0 must be a no-op even without a device")
	}
}

func TestBacklightBadMax(t *testing.T) {
	base := t.TempDir()
	dev := filepath.Join(base, "x")
	_ = os.MkdirAll(dev, 0o750)
	_ = os.WriteFile(filepath.Join(dev, "max_brightness"), []byte("zero"), 0o600)
	if adapter.NewBacklight(base).Set(10) == nil {
		t.Fatal("expected error")
	}
}
