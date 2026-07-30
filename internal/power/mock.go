package power

import (
	"context"
	"log/slog"
)

// MockLogind stands in for logind in dev and on hosts with no system bus. It
// never touches the machine, so a developer pressing Shutdown loses nothing.
type MockLogind struct {
	log       *slog.Logger
	permitted bool
}

func NewMockLogind(log *slog.Logger, permitted bool) *MockLogind {
	return &MockLogind{log: log, permitted: permitted}
}

func (m *MockLogind) Can(context.Context, Action) (string, error) {
	if m.permitted {
		return verdictYes, nil
	}
	return "na", nil
}

func (m *MockLogind) Do(_ context.Context, a Action) error {
	m.log.Warn("power: mock logind, host untouched", "action", a)
	return nil
}
