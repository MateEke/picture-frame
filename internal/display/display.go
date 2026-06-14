package display

import "context"

// Controller manages screen power state.
type Controller interface {
	On(ctx context.Context) error
	Off(ctx context.Context) error
	// State reports whether the display is currently on.
	State(ctx context.Context) (bool, error)
}
