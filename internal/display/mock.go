package display

import (
	"context"
	"sync"
)

// Mock is a thread-safe in-memory Controller for use in tests.
type Mock struct {
	mu       sync.Mutex
	on       bool
	OnErr    error
	OffErr   error
	StateErr error
	onCalls  int
	offCalls int
}

func (m *Mock) On(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.OnErr != nil {
		return m.OnErr
	}
	m.on = true
	m.onCalls++
	return nil
}

func (m *Mock) Off(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.OffErr != nil {
		return m.OffErr
	}
	m.on = false
	m.offCalls++
	return nil
}

func (m *Mock) State(_ context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.StateErr != nil {
		return false, m.StateErr
	}
	return m.on, nil
}

// IsOn reports the current simulated power state.
func (m *Mock) IsOn() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.on
}

// SetOn sets the initial power state without affecting call counters.
// Use this in tests to match the state the policy assumes (on at startup).
func (m *Mock) SetOn(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.on = v
}

// Calls returns how many times On and Off have been called successfully.
func (m *Mock) Calls() (on, off int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onCalls, m.offCalls
}
