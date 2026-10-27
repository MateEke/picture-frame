package power_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	hardwarepower "github.com/MateEke/picture-frame/internal/hardware/power"
)

func TestSysfsBacklight(t *testing.T) {
	dir := t.TempDir()
	deviceDir := filepath.Join(dir, "intel_backlight")
	if err := os.MkdirAll(deviceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	maxPath := filepath.Join(deviceDir, "max_brightness")
	if err := os.WriteFile(maxPath, []byte("1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	brightPath := filepath.Join(deviceDir, "brightness")
	if err := os.WriteFile(brightPath, []byte("500\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	bl, err := hardwarepower.NewSysfsBacklight(dir, "")
	if err != nil {
		t.Fatalf("NewSysfsBacklight failed: %v", err)
	}

	val, err := bl.Brightness(ctx)
	if err != nil {
		t.Fatalf("Brightness failed: %v", err)
	}
	if val != 50 {
		t.Errorf("Brightness got %d, want 50", val)
	}

	if err := bl.SetBrightness(ctx, 75); err != nil {
		t.Fatalf("SetBrightness(75) failed: %v", err)
	}

	// Verify written file
	raw, err := os.ReadFile(brightPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "750\n" {
		t.Errorf("brightness file = %q, want %q", string(raw), "750\n")
	}

	val, err = bl.Brightness(ctx)
	if err != nil {
		t.Fatalf("Brightness after set failed: %v", err)
	}
	if val != 75 {
		t.Errorf("Brightness got %d, want 75", val)
	}

	// Range checks
	if err := bl.SetBrightness(ctx, -1); err == nil {
		t.Error("expected error for brightness -1")
	}
	if err := bl.SetBrightness(ctx, 101); err == nil {
		t.Error("expected error for brightness 101")
	}
}

func TestSysfsBacklightNoDevice(t *testing.T) {
	dir := t.TempDir()
	_, err := hardwarepower.NewSysfsBacklight(dir, "")
	if err == nil {
		t.Error("expected error when no devices exist in sysfs backlight")
	}
}
