package adapter

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestStateSysfs(t *testing.T) {
	dir := t.TempDir()
	cardDir := filepath.Join(dir, "card0-HDMI-A-1")
	if err := os.MkdirAll(cardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cardDir, "dpms"), []byte("On\n"), 0600); err != nil {
		t.Fatal(err)
	}

	w := &Wlopm{
		output:    "HDMI-A-1",
		log:       slog.New(slog.NewTextHandler(os.Stderr, nil)),
		sysfsBase: dir,
	}

	on, err := w.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if !on {
		t.Error("expected on")
	}

	if err := os.WriteFile(filepath.Join(cardDir, "dpms"), []byte("Off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	on, err = w.State(context.Background())
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if on {
		t.Error("expected off")
	}
}

func TestStateOutputNotFound(t *testing.T) {
	w := &Wlopm{
		output:    "DSI-1",
		log:       slog.New(slog.NewTextHandler(os.Stderr, nil)),
		sysfsBase: t.TempDir(),
	}
	_, err := w.State(context.Background())
	if err == nil {
		t.Fatal("expected error for missing output")
	}
}
