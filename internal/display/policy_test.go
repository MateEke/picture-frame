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

// testBlankAfter is a whole number of idle-check intervals so the blank lands on
// a clean tick under synctest's virtual clock.
const testBlankAfter = 30 * time.Second

func motionEvent(value float64) state.Event {
	return state.Event{
		Kind:    state.KindSensor,
		Payload: state.SensorPayload{Kind: "motion", Value: value},
	}
}

// mockStore is an in-memory IntentStore with error injection.
type mockStore struct {
	manualOff bool
	loadErr   error
	saveErr   error
	saved     []bool
}

func (m *mockStore) LoadManualOff() (bool, error) { return m.manualOff, m.loadErr }

func (m *mockStore) SaveManualOff(off bool) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.manualOff = off
	m.saved = append(m.saved, off)
	return nil
}

// newPolicy builds and Starts a policy with the display already on (as at boot)
// unless the caller pre-sets ctrl. store may be nil; a nil bus is replaced with a
// throwaway one (Policy requires a bus; callers that don't assert published
// events just pass nil).
func newPolicy(ctrl display.Controller, bus *state.Bus, store display.IntentStore, blankAfter time.Duration) *display.Policy {
	if bus == nil {
		bus = state.NewBus()
	}
	p := display.NewPolicy(display.PolicyConfig{
		Log:             testutil.NopLogger(),
		Display:         ctrl,
		Bus:             bus,
		Store:           store,
		BlankAfter:      blankAfter,
		MotionAvailable: true,
	})
	p.Start(context.Background())
	return p
}

// idle advances past blank_after so the next idle check blanks the display.
func idle() {
	time.Sleep(2 * testBlankAfter)
	synctest.Wait()
}

func TestPolicyBlanksAfterIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()

		if ctrl.IsOn() {
			t.Fatal("expected display off after idle timeout")
		}
		if _, off := ctrl.Calls(); off != 1 {
			t.Errorf("Off called %d times, want 1", off)
		}
	})
}

func TestPolicyNonPositiveBlankAfterNeverBlanks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newPolicy(ctrl, state.NewBus(), nil, 0)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()

		if !ctrl.IsOn() {
			t.Fatal("blank_after=0 must disable idle-blank; display should stay on")
		}
		if _, off := ctrl.Calls(); off != 0 {
			t.Errorf("Off called %d times, want 0", off)
		}
	})
}

func TestPolicyNoMotionSensorNeverBlanks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		// MotionAvailable=false: nothing could wake an idle-blanked screen, so it
		// must never blank even with a positive blank_after.
		pol := display.NewPolicy(display.PolicyConfig{
			Log:             testutil.NopLogger(),
			Display:         ctrl,
			Bus:             state.NewBus(),
			BlankAfter:      testBlankAfter,
			MotionAvailable: false,
		})
		pol.Start(context.Background())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()

		if !ctrl.IsOn() {
			t.Fatal("without a motion sensor, idle-blank must be disabled; display should stay on")
		}
		if _, off := ctrl.Calls(); off != 0 {
			t.Errorf("Off called %d times, want 0", off)
		}
	})
}

// The crux of the intent/live split: idle-blank moves live power but NOT intent
// (the switch stays on), and a subsequent motion wake publishes live-on again.
func TestIdleBlankPublishesLivePowerNotIntent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ch, unsub := bus.Subscribe()
		defer unsub()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		idle()
		assertScreenEvent(t, ch, false, true) // live off, intent still auto

		bus.Publish(motionEvent(1))
		synctest.Wait()
		assertScreenEvent(t, ch, true, true) // live on again, intent unchanged
	})
}

func TestPolicyWakesOnMotion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()
		if ctrl.IsOn() {
			t.Fatal("precondition: expected display off before motion")
		}

		bus.Publish(motionEvent(1))
		synctest.Wait()

		if !ctrl.IsOn() {
			t.Fatal("expected display on after motion event")
		}
		if on, _ := ctrl.Calls(); on != 1 {
			t.Errorf("On called %d times, want 1", on)
		}
	})
}

func TestPolicyMotionResetsIdleTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		time.Sleep(testBlankAfter - 10*time.Second)
		synctest.Wait()
		bus.Publish(motionEvent(1))
		synctest.Wait()
		time.Sleep(testBlankAfter - 10*time.Second)
		synctest.Wait()

		if !ctrl.IsOn() {
			t.Fatal("expected display still on: last motion was < blank_after ago")
		}
	})
}

func TestPolicyDoesNotCallOnWhenAlreadyOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait()

		bus.Publish(motionEvent(1))
		bus.Publish(motionEvent(1))
		synctest.Wait()

		if on, _ := ctrl.Calls(); on != 0 {
			t.Errorf("On called %d times on already-on display, want 0", on)
		}
	})
}

func TestPolicyZeroValueMotionIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()
		if ctrl.IsOn() {
			t.Fatal("precondition: expected display off")
		}

		bus.Publish(motionEvent(0))
		synctest.Wait()

		if ctrl.IsOn() {
			t.Fatal("value=0 motion event must not wake the display")
		}
	})
}

func TestPolicyManualOffBlocksMotionWake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		if err := pol.SetManual(ctx, false); err != nil {
			t.Fatalf("SetManual off: %v", err)
		}
		if ctrl.IsOn() {
			t.Fatal("precondition: expected display off after manual off")
		}

		bus.Publish(motionEvent(1))
		synctest.Wait()

		if ctrl.IsOn() {
			t.Fatal("motion must not wake the display when manually off")
		}
	})
}

func TestPolicyManualOnRestoresAutoWake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		// Manual off, then manual on should re-enable motion auto-wake.
		if err := pol.SetManual(ctx, false); err != nil {
			t.Fatalf("SetManual off: %v", err)
		}
		if err := pol.SetManual(ctx, true); err != nil {
			t.Fatalf("SetManual on: %v", err)
		}
		if !pol.Auto() {
			t.Fatal("expected Auto true after manual on")
		}

		idle() // auto-blank
		if ctrl.IsOn() {
			t.Fatal("expected idle-blank after manual on + idle")
		}
		bus.Publish(motionEvent(1))
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("expected motion to wake display after manual on")
		}
	})
}

func TestPolicyWakeFailureStaysBlanked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle() // blank
		ctrl.OnErr = errors.New("wake boom")
		bus.Publish(motionEvent(1))
		synctest.Wait()

		if ctrl.IsOn() {
			t.Fatal("display must stay off when wake fails")
		}
		// Error cleared: the next motion still wakes it (idleBlank was reverted).
		ctrl.OnErr = nil
		bus.Publish(motionEvent(1))
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("expected wake to succeed after error cleared")
		}
	})
}

func TestPolicyBlankFailureStaysOn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		ctrl.OffErr = errors.New("blank boom")
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		idle()

		if !ctrl.IsOn() {
			t.Fatal("display must stay on when blank fails")
		}
	})
}

func TestPolicyCancelStopsRun(t *testing.T) {
	synctest.Test(t, func(*testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		pol := newPolicy(ctrl, state.NewBus(), nil, testBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			pol.Run(ctx)
			close(done)
		}()
		synctest.Wait()

		cancel()
		<-done // synctest fails the test if Run never returns
	})
}

func TestStartRestoresManualOff(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true) // compositor came up on; persisted intent says off
	bus := state.NewBus()
	ch, unsub := bus.Subscribe()
	defer unsub()

	display.NewPolicy(display.PolicyConfig{
		Log: testutil.NopLogger(), Display: ctrl, Bus: bus,
		Store: &mockStore{manualOff: true}, BlankAfter: time.Minute,
	}).Start(context.Background())

	if ctrl.IsOn() {
		t.Fatal("startup must honor persisted manual-off and turn the panel off")
	}
	// Manual-off at startup: live power off and intent (auto) off.
	assertScreenEvent(t, ch, false, false)
}

func TestStartAdoptsActualOffInAuto(t *testing.T) {
	// Auto + compositor holds it off: adopt off (don't force on), still wake on motion.
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(false)
		bus := state.NewBus()
		pol := newPolicy(ctrl, bus, nil, testBlankAfter)

		if on, off := ctrl.Calls(); on != 0 || off != 0 {
			t.Fatalf("startup must not toggle an already-off auto panel; on=%d off=%d", on, off)
		}
		if !pol.Auto() {
			t.Error("expected Auto true")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)
		synctest.Wait() // ensure Run has subscribed before publishing

		bus.Publish(motionEvent(1))
		synctest.Wait()

		if !ctrl.IsOn() {
			t.Fatal("expected motion to wake the adopted-off panel")
		}
	})
}

func TestStartAdoptsActualOnInAuto(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, state.NewBus(), nil, time.Minute)
	if !pol.Auto() || !ctrl.IsOn() {
		t.Fatal("auto + actual-on should stay on with auto-wake enabled")
	}
}

func TestStartStateErrorDegrades(t *testing.T) {
	ctrl := &display.Mock{StateErr: errors.New("compositor not up")}
	pol := newPolicy(ctrl, state.NewBus(), nil, time.Minute)
	// Must not panic; auto by default.
	if !pol.Auto() {
		t.Error("expected Auto true after state-read failure")
	}
}

func TestStartStoreLoadErrorDefaultsAuto(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, state.NewBus(), &mockStore{loadErr: errors.New("read boom")}, time.Minute)
	if !pol.Auto() {
		t.Error("a store load error must default to auto, not crash")
	}
}

func TestStartApplyErrorIsNonFatal(t *testing.T) {
	// manual-off persisted, panel came up on, Off fails: Start must not crash.
	ctrl := &display.Mock{OffErr: errors.New("off boom")}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, state.NewBus(), &mockStore{manualOff: true}, time.Minute)
	if pol.Auto() {
		t.Error("intent should remain manual-off after a failed startup apply")
	}
	if !ctrl.IsOn() {
		t.Error("panel stays on when the startup Off fails")
	}
}

func TestSetManualPersistsAndPublishes(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	bus := state.NewBus()
	store := &mockStore{}
	pol := newPolicy(ctrl, bus, store, time.Minute)

	ch, unsub := bus.Subscribe()
	defer unsub()

	if err := pol.SetManual(context.Background(), false); err != nil {
		t.Fatalf("SetManual off: %v", err)
	}
	if ctrl.IsOn() {
		t.Error("expected panel off")
	}
	if pol.Auto() {
		t.Error("expected Auto false after manual off")
	}
	assertScreenEvent(t, ch, false, false)
	if len(store.saved) != 1 || store.saved[0] != true {
		t.Errorf("expected persisted manual-off=true, got %v", store.saved)
	}

	if err := pol.SetManual(context.Background(), true); err != nil {
		t.Fatalf("SetManual on: %v", err)
	}
	assertScreenEvent(t, ch, true, true)
	if len(store.saved) != 2 || store.saved[1] != false {
		t.Errorf("expected persisted manual-off=false, got %v", store.saved)
	}
}

func TestSetManualNoStore(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute) // no store

	if err := pol.SetManual(context.Background(), false); err != nil {
		t.Fatalf("SetManual off: %v", err)
	}
	if ctrl.IsOn() {
		t.Error("expected panel off even without a store")
	}
}

func TestSetManualOnErrorLeavesIntent(t *testing.T) {
	ctrl := &display.Mock{OnErr: errors.New("on boom")}
	ctrl.SetOn(false)
	store := &mockStore{manualOff: true}
	pol := newPolicy(ctrl, nil, store, time.Minute) // starts off (manual)

	if pol.Auto() {
		t.Fatal("precondition: Auto should be false (manual off)")
	}
	if err := pol.SetManual(context.Background(), true); err == nil {
		t.Fatal("expected error from On")
	}
	if pol.Auto() {
		t.Error("intent must remain manual-off when On fails")
	}
	if ctrl.IsOn() {
		t.Error("panel must remain off when On fails")
	}
	if len(store.saved) != 0 {
		t.Errorf("nothing should be persisted on failure, got %v", store.saved)
	}
}

func TestSetManualOffErrorLeavesIntent(t *testing.T) {
	ctrl := &display.Mock{OffErr: errors.New("off boom")}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute) // starts on (auto)

	if err := pol.SetManual(context.Background(), false); err == nil {
		t.Fatal("expected error from Off")
	}
	if !pol.Auto() {
		t.Error("intent must remain auto when Off fails")
	}
	if !ctrl.IsOn() {
		t.Error("panel must remain on when Off fails")
	}
}

func TestSetManualPersistErrorStillApplies(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	bus := state.NewBus()
	pol := newPolicy(ctrl, bus, &mockStore{saveErr: errors.New("disk full")}, time.Minute)

	ch, unsub := bus.Subscribe()
	defer unsub()

	if err := pol.SetManual(context.Background(), false); err != nil {
		t.Fatalf("a persist error must not fail the toggle: %v", err)
	}
	if ctrl.IsOn() {
		t.Error("panel should still be off despite persist failure")
	}
	assertScreenEvent(t, ch, false, false)
}

func TestReconcileCorrectsDrift(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)

	ctrl.SetOn(false) // simulate drift: hardware went off, policy thinks on
	pol.Reconcile(context.Background())

	if !ctrl.IsOn() {
		t.Error("reconcile should have turned display back on (desired=on)")
	}
}

// When reconcile can't fix the drift (the apply fails), it still surfaces the
// real live power so the binary_sensor reflects reality, without moving intent.
func TestReconcileSurfacesUnfixablePower(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	bus := state.NewBus()
	pol := newPolicy(ctrl, bus, nil, time.Minute)

	ch, unsub := bus.Subscribe()
	defer unsub()

	ctrl.SetOn(false)                    // drift off
	ctrl.OnErr = errors.New("wake boom") // and we can't turn it back on
	pol.Reconcile(context.Background())

	if ctrl.IsOn() {
		t.Fatal("precondition: On failed, panel stays off")
	}
	// Live power off (reality), but intent stays auto: the switch must not move.
	assertScreenEvent(t, ch, false, true)
}

func TestReconcileStateErrorIsNonFatal(t *testing.T) {
	ctrl := &display.Mock{StateErr: errors.New("compositor not up")}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)

	onBefore, offBefore := ctrl.Calls()
	pol.Reconcile(context.Background()) // must not panic; can't read, so does nothing
	onAfter, offAfter := ctrl.Calls()
	if onAfter != onBefore || offAfter != offBefore {
		t.Errorf("reconcile must not actuate when the panel read fails; on Δ=%d off Δ=%d",
			onAfter-onBefore, offAfter-offBefore)
	}
}

func TestReconcileNoopWhenAligned(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)
	onBefore, offBefore := ctrl.Calls()
	pol.Reconcile(context.Background())
	onAfter, offAfter := ctrl.Calls()
	if onAfter != onBefore || offAfter != offBefore {
		t.Errorf("reconcile should be no-op when aligned; on delta=%d off delta=%d",
			onAfter-onBefore, offAfter-offBefore)
	}
}

func TestAuto(t *testing.T) {
	ctrl := &display.Mock{}
	ctrl.SetOn(true)
	pol := newPolicy(ctrl, nil, nil, time.Minute)
	if !pol.Auto() {
		t.Error("expected Auto true initially")
	}
	if err := pol.SetManual(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if pol.Auto() {
		t.Error("expected Auto false after manual off")
	}
}

func TestSetBlankAfterTakesEffect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := &display.Mock{}
		ctrl.SetOn(true)
		bus := state.NewBus()
		// Start with a very long blank period so idle() doesn't trigger it.
		longBlankAfter := 10 * testBlankAfter
		pol := newPolicy(ctrl, bus, nil, longBlankAfter)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go pol.Run(ctx)

		// Sleep past testBlankAfter but not longBlankAfter; display should stay on.
		time.Sleep(2 * testBlankAfter)
		synctest.Wait()
		if !ctrl.IsOn() {
			t.Fatal("display should still be on with longBlankAfter")
		}

		// Shorten blankAfter to testBlankAfter; time already elapsed exceeds it.
		pol.SetBlankAfter(testBlankAfter)

		// Next idle check should now blank the display (idle check fires every 10s).
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if ctrl.IsOn() {
			t.Fatal("expected display off after SetBlankAfter reduced threshold")
		}
	})
}

// assertScreenEvent waits for the next KindScreen event (skipping sensor/other
// events a co-subscribed test may also receive) and asserts both facets:
// On is live panel power, Auto is the manual intent.
func assertScreenEvent(t *testing.T, ch <-chan state.Event, wantOn, wantAuto bool) {
	t.Helper()
	timeout := time.After(time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Kind != state.KindScreen {
				continue
			}
			sp, ok := ev.Payload.(state.ScreenPayload)
			if !ok || sp.On != wantOn || sp.Auto != wantAuto {
				t.Errorf("want KindScreen{On:%v, Auto:%v}, got %v %#v", wantOn, wantAuto, ev.Kind, ev.Payload)
			}
			return
		case <-timeout:
			t.Fatalf("expected a KindScreen{On:%v, Auto:%v} event", wantOn, wantAuto)
		}
	}
}
