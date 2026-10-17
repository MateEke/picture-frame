package display

import (
	"errors"
	"os"
	"strings"
	"time"
)

// FileScheduleStore stores the last check as one RFC 3339 line.
type FileScheduleStore struct{ path string }

func NewFileScheduleStore(path string) *FileScheduleStore { return &FileScheduleStore{path: path} }

func (s *FileScheduleStore) LoadLastCheck() (time.Time, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
}

// SaveLastCheck writes via rename so a power cut can't leave a truncated file.
func (s *FileScheduleStore) SaveLastCheck(t time.Time) error {
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(t.Format(time.RFC3339)+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
