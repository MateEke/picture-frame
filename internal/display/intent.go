package display

import (
	"errors"
	"os"
	"strings"
)

// IntentStore persists the manual screen intent across restarts, so a manually-off
// screen stays off when the process comes back.
type IntentStore interface {
	LoadManualOff() (bool, error)
	SaveManualOff(off bool) error
}

const (
	intentOff  = "off"
	intentAuto = "auto"
)

// FileIntentStore persists the intent as a one-line file ("off" or "auto").
type FileIntentStore struct{ path string }

func NewFileIntentStore(path string) *FileIntentStore { return &FileIntentStore{path: path} }

// LoadManualOff treats a missing file as no saved intent, auto (false).
func (s *FileIntentStore) LoadManualOff() (bool, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(b)) == intentOff, nil
}

func (s *FileIntentStore) SaveManualOff(off bool) error {
	v := intentAuto
	if off {
		v = intentOff
	}
	return os.WriteFile(s.path, []byte(v+"\n"), 0o600)
}
