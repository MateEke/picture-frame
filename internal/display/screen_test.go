package display_test

import (
	"context"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/state"
)

// Screen is a thin facade over Policy; these tests verify the delegation. The
// behaviour itself is covered in policy_test.go.

func TestScreenOnOffDelegates(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	bus := state.NewBus()
	pol := newPolicy(ctrl, bus, nil, time.Minute)
	scr := display.NewScreen(pol)

	ch, unsub := bus.Subscribe()
	defer unsub()

	if err := scr.Off(context.Background()); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if ctrl.IsOn() {
		t.Error("expected display off")
	}
	if scr.Auto() {
		t.Error("expected Auto false after Off")
	}
	assertScreenEvent(t, ch, false, false)

	if err := scr.On(context.Background()); err != nil {
		t.Fatalf("On: %v", err)
	}
	if !ctrl.IsOn() {
		t.Error("expected display on")
	}
	if !scr.Auto() {
		t.Error("expected Auto true after On")
	}
	assertScreenEvent(t, ch, true, true)
}

func TestScreenStateDelegates(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)
	scr := display.NewScreen(pol)

	if !scr.State() {
		t.Error("expected State true")
	}

	if err := scr.Off(context.Background()); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if scr.State() {
		t.Error("expected State false after Off")
	}
}

func TestScreenReconcileCorrectsDrift(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)
	scr := display.NewScreen(pol)

	ctrl.SetOn(false) // simulate drift
	scr.Reconcile(context.Background())
	if !ctrl.IsOn() {
		t.Error("reconcile should have corrected drift back to on")
	}
}
