//go:build !linux

package adapter

import (
	"context"
	"fmt"
	"runtime"

	"github.com/MateEke/picture-frame/internal/sensors/ble"
)

// Bluetooth is unavailable on non-Linux platforms because the production
// adapter uses BlueZ through tinygo.org/x/bluetooth.
type Bluetooth struct{}

// New returns an unsupported-platform error on non-Linux systems.
func New() (*Bluetooth, error) {
	return NewWithID("hci0")
}

// NewWithID returns an unsupported-platform error on non-Linux systems.
func NewWithID(id string) (*Bluetooth, error) {
	return nil, fmt.Errorf("bluetooth adapter %s is only available on linux, not %s", id, runtime.GOOS)
}

// Connect satisfies ble.Adapter for code that compiles on non-Linux platforms.
func (b *Bluetooth) Connect(_ context.Context, _ string, _ string, _ []string) (ble.Device, error) {
	return nil, fmt.Errorf("bluetooth adapter is only available on linux, not %s", runtime.GOOS)
}

// Reset satisfies ble.Adapter for code that compiles on non-Linux platforms.
func (b *Bluetooth) Reset(_ context.Context) error {
	return fmt.Errorf("bluetooth adapter is only available on linux, not %s", runtime.GOOS)
}
