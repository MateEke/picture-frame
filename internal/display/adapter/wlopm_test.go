package adapter

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// wlopm exits 0 for an unknown output and reports the problem only on stderr.
func TestSetReportsErrorTextDespiteZeroExit(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"ERROR: Output '$2' does not exist.\" >&2\n"
	if err := os.WriteFile(filepath.Join(dir, "wlopm"), []byte(script), 0o700); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := NewWlopm("eDP-1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := w.Off(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Off: got %v, want unknown-output error", err)
	}
}
