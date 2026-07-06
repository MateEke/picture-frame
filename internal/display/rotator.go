package display

import "context"

// Rotator applies the output transform. Nil when the backend can't rotate
// (vcgencmd); callers must nil-check.
type Rotator interface {
	// Set applies the rotation now (0/90/180/270 counter-clockwise degrees).
	Set(ctx context.Context, degrees int) error
	// Reconcile re-applies only on drift (query-first), so it can run on
	// every SSE connect.
	Reconcile(ctx context.Context)
	// Supported re-checks per call, so a post-install of wlr-randr is
	// noticed without a restart.
	Supported() bool
}
