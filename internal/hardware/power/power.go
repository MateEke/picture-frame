// Package power defines the panel-power seam for the future dedicated frame:
// on/off, state readout, and brightness. It covers the PANEL only; host
// reboot/shutdown lives in internal/power (systemd-logind), and the
// motion/idle policy owning power decisions lives in internal/display.
//
// The production backend for this interface is internal/display.Controller
// (wlopm/vcgencmd) driven by display.Policy; Noop below is the test/dev
// double, following the display.Mock precedent. Night mode schedules, sleep
// states, and battery status are future interfaces, not methods here.
package power

import (
	"context"
	"fmt"
	"sync"
)

// Controller drives panel power. Implementations must be safe for concurrent
// use; callers serialize policy decisions above this interface.
type Controller interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
	// State reports whether the panel is currently on.
	State(ctx context.Context) (bool, error)
	// SetBrightness sets panel brightness, 0-100. Out-of-range values are an
	// error; backends without hardware support return an error naming that.
	SetBrightness(ctx context.Context, level int) error
}

// Noop is an in-memory Controller: it records the last requested state and
// brightness without touching hardware. The zero value is off, brightness 0.
type Noop struct {
	mu         sync.Mutex
	on         bool
	brightness int
}

var _ Controller = (*Noop)(nil)

// On powers the panel on.
func (n *Noop) On(_ context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.on = true
	return nil
}

// Off powers the panel off.
func (n *Noop) Off(_ context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.on = false
	return nil
}

// State reports the last requested power state.
func (n *Noop) State(_ context.Context) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.on, nil
}

// SetBrightness records level, rejecting values outside 0-100.
func (n *Noop) SetBrightness(_ context.Context, level int) error {
	if level < 0 || level > 100 {
		return fmt.Errorf("power: brightness %d out of range 0-100", level)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.brightness = level
	return nil
}

// Brightness reports the last set level. Test/dev helper, not part of Controller.
func (n *Noop) Brightness() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.brightness
}
