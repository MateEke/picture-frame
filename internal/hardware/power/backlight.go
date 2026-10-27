package power

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// SysfsBacklight implements brightness control directly via Linux sysfs
// (/sys/class/backlight/<device>/brightness).
// It maps the abstract 0-100 percentage range to the hardware's 0-max_brightness.
type SysfsBacklight struct {
	mu            sync.Mutex
	backlightDir  string
	maxBrightness int
}

// NewSysfsBacklight searches for a backlight device in basePath (defaults to "/sys/class/backlight" if empty).
// If deviceName is non-empty, it selects that specific device (e.g. "intel_backlight"); otherwise it picks
// the first available device matching basePath/* (preferring "intel_backlight" if present).
func NewSysfsBacklight(basePath, deviceName string) (*SysfsBacklight, error) {
	if basePath == "" {
		basePath = "/sys/class/backlight"
	}
	var dir string
	if deviceName != "" {
		dir = filepath.Join(basePath, deviceName)
	} else {
		// Prefer intel_backlight if present
		candidate := filepath.Join(basePath, "intel_backlight")
		if _, err := os.Stat(candidate); err == nil {
			dir = candidate
		} else {
			matches, err := filepath.Glob(filepath.Join(basePath, "*"))
			if err != nil {
				return nil, fmt.Errorf("sysfs backlight glob: %w", err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no backlight devices found in %s", basePath)
			}
			dir = matches[0]
		}
	}

	maxRaw, err := os.ReadFile(filepath.Join(dir, "max_brightness"))
	if err != nil {
		return nil, fmt.Errorf("read max_brightness in %s: %w", dir, err)
	}
	maxVal, err := strconv.Atoi(strings.TrimSpace(string(maxRaw)))
	if err != nil || maxVal <= 0 {
		return nil, fmt.Errorf("invalid max_brightness %q in %s", string(maxRaw), dir)
	}

	return &SysfsBacklight{
		backlightDir:  dir,
		maxBrightness: maxVal,
	}, nil
}

// SetBrightness maps 0-100% to 0-max_brightness and writes it to sysfs.
func (s *SysfsBacklight) SetBrightness(_ context.Context, level int) error {
	if level < 0 || level > 100 {
		return fmt.Errorf("power: brightness %d out of range 0-100", level)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rawVal := (level * s.maxBrightness) / 100
	path := filepath.Join(s.backlightDir, "brightness")
	data := []byte(strconv.Itoa(rawVal) + "\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write brightness to %s: %w", path, err)
	}
	return nil
}

// Brightness reads the current hardware brightness and maps it back to 0-100%.
func (s *SysfsBacklight) Brightness(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.backlightDir, "brightness")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read brightness from %s: %w", path, err)
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("parse brightness %q: %w", string(raw), err)
	}

	pct := (val * 100) / s.maxBrightness
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	return pct, nil
}
