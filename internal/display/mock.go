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

// MockRotator is the dev/test Rotator: records calls, never execs.
type MockRotator struct {
	mu         sync.Mutex
	supported  bool
	degrees    int
	reconciles int
}

func NewMockRotator(supported bool) *MockRotator { return &MockRotator{supported: supported} }

func (m *MockRotator) Set(_ context.Context, degrees int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.degrees = degrees
	return nil
}

func (m *MockRotator) Reconcile(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconciles++
}

func (m *MockRotator) Supported() bool { return m.supported }

// Degrees is the last Set value.
func (m *MockRotator) Degrees() int { m.mu.Lock(); defer m.mu.Unlock(); return m.degrees }

// Reconciles counts Reconcile calls.
func (m *MockRotator) Reconciles() int { m.mu.Lock(); defer m.mu.Unlock(); return m.reconciles }
