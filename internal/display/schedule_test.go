package display_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/display"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func TestScheduleContains(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 1, 1, h, m, 0, 0, time.UTC) }
	wrap := display.Schedule{Enabled: true, From: 23 * 60, Until: 7 * 60, Loc: time.UTC}
	for _, c := range []struct {
		t    time.Time
		want bool
	}{
		{at(22, 59), false}, {at(23, 0), true}, {at(2, 0), true}, {at(6, 59), true}, {at(7, 0), false}, {at(12, 0), false},
	} {
		if got := wrap.Contains(c.t); got != c.want {
			t.Errorf("wrap %v = %v", c.t, got)
		}
	}
	day := display.Schedule{Enabled: true, From: 9 * 60, Until: 17 * 60}
	if !day.Contains(at(12, 0)) || day.Contains(at(18, 0)) {
		t.Error("daytime window wrong")
	}
	if (display.Schedule{From: 0, Until: 60}).Contains(at(0, 30)) {
		t.Error("disabled schedule must never contain")
	}
	helsinki, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Skip("no tzdata")
	}
	// 21:30 UTC is 23:30 in Helsinki in winter.
	if !(display.Schedule{Enabled: true, From: 23 * 60, Until: 7 * 60, Loc: helsinki}).Contains(at(21, 30)) {
		t.Error("location not applied")
	}
}

// synctest's clock starts at 2000-01-01 00:00 UTC; the window is 00:01–00:04.
func newScheduledPolicy(ctrl display.Controller, bus *state.Bus, motion bool) *display.Policy {
	p := display.NewPolicy(display.PolicyConfig{
		Log:             testutil.NopLogger(),
		Display:         ctrl,
		Bus:             bus,
		MotionAvailable: motion,
		Schedule:        display.Schedule{Enabled: true, From: 1, Until: 4, Loc: time.UTC},
		WakeFor:         30 * time.Second,
	})
	p.Start(context.Background())
	return p
}

func TestScheduleTurnsOffAndBackOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		time.Sleep(50 * time.Second)
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("off before the window")
		}
		time.Sleep(20 * time.Second) // 00:01:10
		synctest.Wait()
		if ctrl.IsOn() || !pol.NightOff() {
			t.Fatal("expected off inside the window")
		}

		// A touch lights it for WakeFor, then the window takes over again.
		if err := pol.Wake(ctx); err != nil {
			t.Fatal(err)
		}
		if !ctrl.IsOn() {
			t.Fatal("touch must wake at night")
		}
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("off before WakeFor elapsed")
		}
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if ctrl.IsOn() {
			t.Fatal("expected off after WakeFor")
		}

		time.Sleep(3 * time.Minute) // past 00:04
		synctest.Wait()
		if !ctrl.IsOn() || pol.NightOff() {
			t.Fatal("expected on after the window")
		}
	})
}

func TestScheduleMotionDoesNotWake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newScheduledPolicy(ctrl, bus, true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		time.Sleep(70 * time.Second)
		synctest.Wait()
		if ctrl.IsOn() {
			t.Fatal("expected off")
		}
		bus.Publish(motionEvent(1))
		synctest.Wait()
		if ctrl.IsOn() {
			t.Fatal("motion must not wake inside the window")
		}
	})
}

func TestScheduleManualOnAtNightActsLikeTouch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		time.Sleep(70 * time.Second)
		synctest.Wait()
		if err := pol.SetManual(ctx, true); err != nil {
			t.Fatal(err)
		}
		if !ctrl.IsOn() {
			t.Fatal("manual on must light the panel")
		}
		time.Sleep(40 * time.Second)
		synctest.Wait()
		if ctrl.IsOn() {
			t.Fatal("schedule must resume after WakeFor")
		}
	})
}

func TestSetScheduleDisables(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newScheduledPolicy(ctrl, state.NewBus(), false)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		time.Sleep(70 * time.Second)
		synctest.Wait()
		pol.SetSchedule(display.Schedule{}, 0)
		time.Sleep(15 * time.Second)
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("disabling the schedule must turn the panel back on")
		}
	})
}
