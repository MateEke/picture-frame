// Package power reboots and powers off the host through systemd-logind. PID 1
// does the work, so the backend needs no capability of its own, only the polkit
// grant in deploy/polkit/50-pictureframe-power.rules.
package power

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrUnsupported means logind will not run the action unattended, almost always
// a missing polkit rule.
var ErrUnsupported = errors.New("power action not permitted")

// Action is named for its logind D-Bus method.
type Action string

const (
	Reboot   Action = "Reboot"
	PowerOff Action = "PowerOff"
)

// The only Can* reply that succeeds without an auth agent; "challenge" would
// block a sessionless service forever.
const verdictYes = "yes"

// logind calls return as soon as the job is queued, so a stall means D-Bus is
// wedged. Unbounded, that would hang boot (probe) or the MQTT callback (action).
const callTimeout = 10 * time.Second

type Logind interface {
	Can(ctx context.Context, a Action) (string, error)
	Do(ctx context.Context, a Action) error
}

// Capabilities records which actions the polkit policy permits.
type Capabilities struct {
	Reboot   bool
	PowerOff bool
}

// Manager gates logind calls on a capability probe taken once at startup.
type Manager struct {
	log    *slog.Logger
	logind Logind
	caps   Capabilities
}

func New(log *slog.Logger, logind Logind) *Manager {
	return &Manager{log: log, logind: logind}
}

// Probe caches what logind will allow. Entities and endpoints key off the result.
func (m *Manager) Probe(ctx context.Context) Capabilities {
	m.caps = Capabilities{
		Reboot:   m.permitted(ctx, Reboot),
		PowerOff: m.permitted(ctx, PowerOff),
	}
	return m.caps
}

func (m *Manager) Capabilities() Capabilities { return m.caps }

func (m *Manager) Reboot(ctx context.Context) error   { return m.do(ctx, Reboot) }
func (m *Manager) PowerOff(ctx context.Context) error { return m.do(ctx, PowerOff) }

func (m *Manager) permitted(ctx context.Context, a Action) bool {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	verdict, err := m.logind.Can(ctx, a)
	if err != nil {
		m.log.Warn("power: capability probe failed", "action", a, "err", err)
		return false
	}
	// The verdict separates a missing polkit rule ("challenge") from no logind
	// at all ("na"): the first thing to check when the buttons do not appear.
	m.log.Info("power: capability probe", "action", a, "verdict", verdict)
	return verdict == verdictYes
}

func (m *Manager) do(ctx context.Context, a Action) error {
	if !m.allows(a) {
		return fmt.Errorf("%s: %w", a, ErrUnsupported)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	m.log.Info("power: invoking logind", "action", a)
	return m.logind.Do(ctx, a)
}

func (m *Manager) allows(a Action) bool {
	if a == Reboot {
		return m.caps.Reboot
	}
	return m.caps.PowerOff
}
