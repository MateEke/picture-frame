package display_test

import (
	"path/filepath"
	"testing"

	"github.com/MateEke/picture-frame/internal/display"
)

func TestFileIntentStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen-state")
	s := display.NewFileIntentStore(path)

	// Missing file → auto (false), no error.
	off, err := s.LoadManualOff()
	if err != nil || off {
		t.Fatalf("missing file: got (%v, %v), want (false, nil)", off, err)
	}

	if err := s.SaveManualOff(true); err != nil {
		t.Fatalf("Save off: %v", err)
	}
	off, err = s.LoadManualOff()
	if err != nil || !off {
		t.Fatalf("after Save(true): got (%v, %v), want (true, nil)", off, err)
	}

	if err := s.SaveManualOff(false); err != nil {
		t.Fatalf("Save auto: %v", err)
	}
	off, err = s.LoadManualOff()
	if err != nil || off {
		t.Fatalf("after Save(false): got (%v, %v), want (false, nil)", off, err)
	}
}

func TestFileIntentStoreLoadError(t *testing.T) {
	// A directory at the path makes ReadFile fail with something other than
	// ErrNotExist, exercising the error branch.
	s := display.NewFileIntentStore(t.TempDir())
	if _, err := s.LoadManualOff(); err == nil {
		t.Fatal("expected a read error when the path is a directory")
	}
}

func TestFileIntentStoreSaveError(t *testing.T) {
	s := display.NewFileIntentStore(filepath.Join(t.TempDir(), "missing-dir", "state"))
	if err := s.SaveManualOff(true); err == nil {
		t.Fatal("expected a write error when the parent directory is missing")
	}
}
