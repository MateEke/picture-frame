package display

import "context"

// Screen is the manual screen-control facade (HTTP API, MQTT) over Policy, which
// owns panel power.
type Screen struct {
	policy *Policy
}

func NewScreen(policy *Policy) *Screen {
	return &Screen{policy: policy}
}

// On turns the display on and re-enables motion auto-wake.
func (s *Screen) On(ctx context.Context) error { return s.policy.SetManual(ctx, true) }

// Off turns the display off and suppresses motion auto-wake until On.
func (s *Screen) Off(ctx context.Context) error { return s.policy.SetManual(ctx, false) }

// State reports last-known panel power from the policy (no hardware read).
func (s *Screen) State() bool { return s.policy.State() }

// Auto reports whether motion auto-wake is active (false after a manual Off).
func (s *Screen) Auto() bool { return s.policy.Auto() }

// Reconcile re-asserts desired power; called on kiosk reconnect.
func (s *Screen) Reconcile(ctx context.Context) { s.policy.Reconcile(ctx) }
