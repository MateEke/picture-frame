package display_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/display"
)

func TestFileScheduleStoreRoundTrip(t *testing.T) {
	s := display.NewFileScheduleStore(filepath.Join(t.TempDir(), "schedule-state"))

	got, err := s.LoadLastCheck()
	if err != nil || !got.IsZero() {
		t.Fatalf("missing file: got (%v, %v), want (zero, nil)", got, err)
	}

	want := time.Date(2026, 10, 17, 23, 0, 5, 0, time.UTC)
	if err := s.SaveLastCheck(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err = s.LoadLastCheck()
	if err != nil || !got.Equal(want) {
		t.Fatalf("got (%v, %v), want (%v, nil)", got, err, want)
	}
}

func TestFileScheduleStoreLoadError(t *testing.T) {
	s := display.NewFileScheduleStore(t.TempDir())
	if _, err := s.LoadLastCheck(); err == nil {
		t.Fatal("expected a read error when the path is a directory")
	}
}

func TestFileScheduleStoreLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedule-state")
	if err := os.WriteFile(path, []byte("yesterday\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := display.NewFileScheduleStore(path).LoadLastCheck(); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestFileScheduleStoreSaveError(t *testing.T) {
	s := display.NewFileScheduleStore(filepath.Join(t.TempDir(), "missing-dir", "state"))
	if err := s.SaveLastCheck(time.Now()); err == nil {
		t.Fatal("expected a write error when the parent directory is missing")
	}
}
