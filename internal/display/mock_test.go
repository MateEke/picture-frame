package display_test

import (
	"context"
	"testing"

	"github.com/MateEke/picture-frame/internal/display"
)

func TestMockRotatorRecordsSetAndReconcile(t *testing.T) {
	m := display.NewMockRotator(true)
	if !m.Supported() {
		t.Error("Supported() = false, want true")
	}
	if err := m.Set(context.Background(), 90); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := m.Degrees(); got != 90 {
		t.Errorf("Degrees() = %d, want 90", got)
	}
	m.Reconcile(context.Background())
	m.Reconcile(context.Background())
	if got := m.Reconciles(); got != 2 {
		t.Errorf("Reconciles() = %d, want 2", got)
	}
}

func TestMockRotatorUnsupported(t *testing.T) {
	if display.NewMockRotator(false).Supported() {
		t.Error("Supported() = true, want false")
	}
}
