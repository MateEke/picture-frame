package adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Backlight sets panel brightness through /sys/class/backlight (DSI panels such
// as the Raspberry Pi Touch Display 2; HDMI monitors have no device here). The
// installer's udev rule gives the video group write access to the brightness file.
type Backlight struct {
	mu  sync.Mutex
	dir string // e.g. /sys/class/backlight/11-0045; "" when none found
}

// NewBacklight picks the first backlight device under base ("" = /sys/class/backlight).
func NewBacklight(base string) *Backlight {
	if base == "" {
		base = "/sys/class/backlight"
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return &Backlight{}
	}
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "max_brightness")); err == nil {
			return &Backlight{dir: dir}
		}
	}
	return &Backlight{}
}

// Supported reports whether a backlight device was found.
func (b *Backlight) Supported() bool { return b.dir != "" }

// Set applies percent (1–100) scaled to the device's range; 0 is a no-op so an
// unset config never touches the panel.
func (b *Backlight) Set(percent int) error {
	if percent <= 0 {
		return nil
	}
	if b.dir == "" {
		return errors.New("backlight: no device")
	}
	percent = min(percent, 100)
	b.mu.Lock()
	defer b.mu.Unlock()
	raw, err := os.ReadFile(filepath.Join(b.dir, "max_brightness"))
	if err != nil {
		return fmt.Errorf("backlight: read max: %w", err)
	}
	maxLevel, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || maxLevel <= 0 {
		return fmt.Errorf("backlight: bad max_brightness %q", raw)
	}
	level := max(1, (maxLevel*percent+50)/100)
	if err := os.WriteFile(filepath.Join(b.dir, "brightness"), []byte(strconv.Itoa(level)), 0); err != nil {
		return fmt.Errorf("backlight: write: %w", err)
	}
	return nil
}
