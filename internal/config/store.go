package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// Store owns the persisted config state: the in-memory copy, the overrides file,
// and the mutex that serializes all reads and writes. Both the HTTP API and the
// wifi manager share a single Store so concurrent saves cannot race.
type Store struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

// NewStore creates a Store seeded with initial config, writing to overridesPath.
func NewStore(initial Config, overridesPath string) *Store {
	return &Store{cfg: cloneConfig(initial), path: overridesPath}
}

// Snapshot returns a deep copy of the current config.
func (s *Store) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneConfig(s.cfg)
}

// PasswordHash returns the current bcrypt hash without copying the full config.
func (s *Store) PasswordHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Auth.PasswordHash
}

// Update applies fn to the config, persists it atomically, and rolls back on
// any error (from fn or from the write).
func (s *Store) Update(fn func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := cloneConfig(s.cfg)
	if err := fn(&s.cfg); err != nil {
		s.cfg = prev
		return err
	}

	data, err := toml.Marshal(&s.cfg)
	if err != nil {
		s.cfg = prev
		return err
	}
	if err := writeAtomic(s.path, data); err != nil {
		s.cfg = prev
		return err
	}
	return nil
}

func cloneConfig(c Config) Config {
	out := c
	if c.Sensors != nil {
		out.Sensors = make([]SensorConfig, len(c.Sensors))
		for i, s := range c.Sensors {
			out.Sensors[i] = s
			out.Sensors[i].Characteristics = append([]CharacteristicConfig(nil), s.Characteristics...)
			out.Sensors[i].MockReadings = append([]MockReadingConfig(nil), s.MockReadings...)
		}
	}
	return out
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	// fsync the dir so the rename survives power loss, not just the file's bytes.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		dir.Close()
		return err
	}
	return dir.Close()
}
