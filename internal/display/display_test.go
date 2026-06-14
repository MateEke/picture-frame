package display_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MateEke/picture-frame/internal/display"
)

func TestMock(t *testing.T) {
	ctx := context.Background()
	m := &display.Mock{}

	if m.IsOn() {
		t.Fatal("expected off initially")
	}
	if on, err := m.State(ctx); err != nil || on {
		t.Fatalf("State: got (%v, %v), want (false, nil)", on, err)
	}
	if err := m.On(ctx); err != nil {
		t.Fatalf("On: %v", err)
	}
	if !m.IsOn() {
		t.Fatal("expected on after On()")
	}
	if on, err := m.State(ctx); err != nil || !on {
		t.Fatalf("State after On: got (%v, %v), want (true, nil)", on, err)
	}
	if err := m.Off(ctx); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if m.IsOn() {
		t.Fatal("expected off after Off()")
	}
}

func TestMockErrors(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("boom")

	on := &display.Mock{OnErr: sentinel}
	if err := on.On(ctx); !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error from On, got %v", err)
	}

	off := &display.Mock{OffErr: sentinel}
	if err := off.Off(ctx); !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error from Off, got %v", err)
	}
}
