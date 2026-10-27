package display_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

// synctest bubbles start at 2000-01-01 00:00:00 UTC, so windows below are
// written against midnight UTC.

// shortWindow opens one minute into the bubble and closes two minutes later.
var shortWindow = display.Schedule{Enabled: true, Off: "00:01", On: "00:03", Location: time.UTC}

// nightWindow crosses midnight and contains the bubble's start time.
var nightWindow = display.Schedule{Enabled: true, Off: "23:00", On: "07:00", Location: time.UTC}

func newScheduledPolicy(ctrl display.Controller, bus *state.Bus, blankAfter time.Duration, s display.Schedule) *display.Policy {
	p := display.NewPolicy(display.PolicyConfig{
		Log:             testutil.NopLogger(),
		Display:         ctrl,
		Bus:             bus,
		BlankAfter:      blankAfter,
		MotionAvailable: true,
		Schedule:        s,
	})
	p.Start(context.Background())
	return p
}

// advance moves the virtual clock by d and lets the policy's ticks settle.
func advance(d time.Duration) {
	time.Sleep(d)
	synctest.Wait()
}

func TestScheduleSwitchesPanelOffThenOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		// blank_after=0 keeps idle-blank out of the way.
		pol := newScheduledPolicy(ctrl, bus, 0, shortWindow)
		ch, unsub := bus.Subscribe()
		defer unsub()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		advance(30 * time.Second)
		if !ctrl.IsOn() {
			t.Fatal("panel must stay on before the window opens")
		}

		advance(50 * time.Second) // 00:01:20
		if ctrl.IsOn() {
			t.Fatal("expected panel off once the window opened")
		}
		assertScreenEvent(t, ch, false, true) // live off, intent still auto

		advance(2 * time.Minute) // 00:03:20
		if !ctrl.IsOn() {
			t.Fatal("expected panel on once the window closed")
		}
		assertScreenEvent(t, ch, true, true)
		if on, off := ctrl.Calls(); on != 1 || off != 1 {
			t.Errorf("calls on=%d off=%d, want 1/1", on, off)
		}
	})
}

func TestScheduleStartInsideMidnightWindowTurnsOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		newScheduledPolicy(ctrl, state.NewBus(), 0, nightWindow)

		if ctrl.IsOn() {
			t.Fatal("starting inside a window that crosses midnight must turn the panel off")
		}
	})
}

func TestScheduleIgnoresMotion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newScheduledPolicy(ctrl, bus, testBlankAfter, nightWindow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		bus.Publish(motionEvent(1))
		advance(10 * time.Second)

		if ctrl.IsOn() {
			t.Fatal("motion must not light the panel inside the window")
		}
	})
}

// A touch lights the panel at night; it re-blanks blank_after after the touch,
// even while presence motion keeps arriving.
func TestScheduleTouchLightsThenReblanks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newScheduledPolicy(ctrl, bus, testBlankAfter, nightWindow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		if err := pol.Wake(ctx); err != nil {
			t.Fatalf("Wake: %v", err)
		}
		if !ctrl.IsOn() {
			t.Fatal("expected a touch to light the panel inside the window")
		}

		for range 3 {
			bus.Publish(motionEvent(1))
			advance(10 * time.Second)
		}
		advance(10 * time.Second)

		if ctrl.IsOn() {
			t.Fatal("expected the panel off again blank_after after the touch")
		}
	})
}

func TestScheduleTouchUsesDefaultWhenBlankAfterZero(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), 0, nightWindow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		if err := pol.Wake(ctx); err != nil {
			t.Fatalf("Wake: %v", err)
		}
		advance(4 * time.Minute)
		if !ctrl.IsOn() {
			t.Fatal("panel must stay lit for the default relock period")
		}
		advance(90 * time.Second)
		if ctrl.IsOn() {
			t.Fatal("expected the panel off after the default relock period")
		}
	})
}

func TestScheduleKeepsManualOffAtWindowEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), 0, shortWindow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		if err := pol.SetManual(ctx, false); err != nil {
			t.Fatalf("SetManual: %v", err)
		}
		advance(4 * time.Minute)

		if ctrl.IsOn() {
			t.Fatal("the window closing must not override a manual off")
		}
	})
}

func TestSetScheduleTakesEffectLive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), 0, display.Schedule{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		advance(10 * time.Second)
		if !ctrl.IsOn() {
			t.Fatal("a disabled schedule must leave the panel on")
		}

		pol.SetSchedule(nightWindow)
		advance(10 * time.Second)
		if ctrl.IsOn() {
			t.Fatal("enabling a window that contains now must turn the panel off")
		}

		pol.SetSchedule(display.Schedule{})
		advance(10 * time.Second)
		if !ctrl.IsOn() {
			t.Fatal("disabling the schedule inside the window must turn the panel on")
		}
	})
}

func TestScheduleApplyFailureRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		ctrl.OffErr = errors.New("off boom")
		pol := newScheduledPolicy(ctrl, state.NewBus(), 0, shortWindow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		advance(80 * time.Second)
		if !ctrl.IsOn() {
			t.Fatal("precondition: the failed Off leaves the panel on")
		}

		ctrl.SetOffErr(nil)
		advance(10 * time.Second)
		if ctrl.IsOn() {
			t.Fatal("expected the next tick to retry the window edge")
		}
	})
}

func TestScheduleWindowBoundaries(t *testing.T) {
	cases := []struct {
		name string
		s    display.Schedule
		want bool // panel off at the bubble's 00:00 UTC start
	}{
		{"disabled", display.Schedule{Off: "23:00", On: "07:00", Location: time.UTC}, false},
		{"crosses midnight", nightWindow, true},
		{"same day, outside", display.Schedule{Enabled: true, Off: "09:00", On: "17:00", Location: time.UTC}, false},
		{"opens at now", display.Schedule{Enabled: true, Off: "00:00", On: "06:00", Location: time.UTC}, true},
		{"closes at now", display.Schedule{Enabled: true, Off: "22:00", On: "00:00", Location: time.UTC}, false},
		{"empty window", display.Schedule{Enabled: true, Off: "07:00", On: "07:00", Location: time.UTC}, false},
		{"unparseable", display.Schedule{Enabled: true, Off: "late", On: "07:00", Location: time.UTC}, false},
		// 00:00 UTC is 01:00 in Rome (CET): inside 00:30-02:00 there, outside in UTC.
		{"location applied", display.Schedule{Enabled: true, Off: "00:30", On: "02:00", Location: mustLoad(t, "Europe/Rome")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := &display.Mock{}
				ctrl.SetOn(true)
				newScheduledPolicy(ctrl, state.NewBus(), 0, tc.s)
				if off := !ctrl.IsOn(); off != tc.want {
					t.Errorf("panel off = %v, want %v", off, tc.want)
				}
			})
		})
	}
}

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	return loc
}
