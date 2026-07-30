package power

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeLogind struct {
	verdict map[Action]string
	canErr  error
	doErr   error
	done    []Action
	asked   []Action
	hang    bool // block until the caller's context expires
}

func (f *fakeLogind) Can(ctx context.Context, a Action) (string, error) {
	f.asked = append(f.asked, a)
	if f.hang {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if f.canErr != nil {
		return "", f.canErr
	}
	return f.verdict[a], nil
}

func (f *fakeLogind) Do(ctx context.Context, a Action) error {
	if f.hang {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.doErr != nil {
		return f.doErr
	}
	f.done = append(f.done, a)
	return nil
}

func newManager(t *testing.T, f *fakeLogind) *Manager {
	t.Helper()
	return New(testutil.NopLogger(), f)
}

func TestProbeAllowsActionsLogindReportsYes(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes", PowerOff: "yes"}}
	caps := newManager(t, f).Probe(context.Background())

	if !caps.Reboot || !caps.PowerOff {
		t.Fatalf("want both permitted, got %+v", caps)
	}
	if len(f.asked) != 2 {
		t.Fatalf("want both actions probed, got %v", f.asked)
	}
}

// "challenge" needs an auth agent the service does not have.
func TestProbeRejectsNonYesVerdicts(t *testing.T) {
	for _, verdict := range []string{"challenge", "no", "na", ""} {
		t.Run(verdict, func(t *testing.T) {
			f := &fakeLogind{verdict: map[Action]string{Reboot: verdict, PowerOff: verdict}}
			caps := newManager(t, f).Probe(context.Background())

			if caps.Reboot || caps.PowerOff {
				t.Fatalf("verdict %q: want none permitted, got %+v", verdict, caps)
			}
		})
	}
}

func TestProbeReportsUnsupportedWhenLogindErrors(t *testing.T) {
	f := &fakeLogind{canErr: errors.New("no system bus")}
	caps := newManager(t, f).Probe(context.Background())

	if caps.Reboot || caps.PowerOff {
		t.Fatalf("want none permitted on error, got %+v", caps)
	}
}

func TestProbeAllowsRebootOnlyWhenPowerOffDenied(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes", PowerOff: "challenge"}}
	caps := newManager(t, f).Probe(context.Background())

	if !caps.Reboot || caps.PowerOff {
		t.Fatalf("want reboot only, got %+v", caps)
	}
}

func TestRebootInvokesLogindWhenPermitted(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes"}}
	m := newManager(t, f)
	m.Probe(context.Background())

	if err := m.Reboot(context.Background()); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	if len(f.done) != 1 || f.done[0] != Reboot {
		t.Fatalf("want one Reboot, got %v", f.done)
	}
}

func TestPowerOffInvokesLogindWhenPermitted(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{PowerOff: "yes"}}
	m := newManager(t, f)
	m.Probe(context.Background())

	if err := m.PowerOff(context.Background()); err != nil {
		t.Fatalf("PowerOff: %v", err)
	}
	if len(f.done) != 1 || f.done[0] != PowerOff {
		t.Fatalf("want one PowerOff, got %v", f.done)
	}
}

// A denied action must not reach logind at all.
func TestRebootRefusesWithoutCapability(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "challenge"}}
	m := newManager(t, f)
	m.Probe(context.Background())

	if err := m.Reboot(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
	if len(f.done) != 0 {
		t.Fatalf("want no logind call, got %v", f.done)
	}
}

func TestPowerOffRefusesWithoutCapability(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{PowerOff: "no"}}
	m := newManager(t, f)
	m.Probe(context.Background())

	if err := m.PowerOff(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
	if len(f.done) != 0 {
		t.Fatalf("want no logind call, got %v", f.done)
	}
}

// An unprobed Manager must stay closed rather than assume permission.
func TestRebootRefusesBeforeProbe(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes"}}

	if err := newManager(t, f).Reboot(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}

func TestRebootPropagatesLogindError(t *testing.T) {
	wantErr := errors.New("dbus refused")
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes"}, doErr: wantErr}
	m := newManager(t, f)
	m.Probe(context.Background())

	if err := m.Reboot(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestCapabilitiesReturnsProbedResult(t *testing.T) {
	f := &fakeLogind{verdict: map[Action]string{Reboot: "yes", PowerOff: "no"}}
	m := newManager(t, f)
	m.Probe(context.Background())

	if got := m.Capabilities(); !got.Reboot || got.PowerOff {
		t.Fatalf("want reboot only, got %+v", got)
	}
}

func TestMockLogindPermitsWhenConfigured(t *testing.T) {
	m := New(testutil.NopLogger(), NewMockLogind(testutil.NopLogger(), true))
	caps := m.Probe(context.Background())

	if !caps.Reboot || !caps.PowerOff {
		t.Fatalf("want both permitted, got %+v", caps)
	}
	// The whole point is that a dev box survives a press.
	if err := m.Reboot(context.Background()); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	if err := m.PowerOff(context.Background()); err != nil {
		t.Fatalf("PowerOff: %v", err)
	}
}

func TestMockLogindDeniesWhenConfigured(t *testing.T) {
	caps := New(testutil.NopLogger(), NewMockLogind(testutil.NopLogger(), false)).Probe(context.Background())

	if caps.Reboot || caps.PowerOff {
		t.Fatalf("want none permitted, got %+v", caps)
	}
}

// A wedged bus must not hang boot; the probe runs before the HTTP server starts.
func TestProbeGivesUpOnAHungBus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		caps := New(testutil.NopLogger(), &fakeLogind{hang: true}).Probe(context.Background())

		if caps.Reboot || caps.PowerOff {
			t.Fatalf("a hung bus must report nothing permitted, got %+v", caps)
		}
		// Both probes time out in turn, so the boot cost is bounded.
		if elapsed := time.Since(start); elapsed != 2*callTimeout {
			t.Fatalf("probe took %v, want %v", elapsed, 2*callTimeout)
		}
	})
}

// The action runs on the MQTT callback goroutine; an unbounded wait stalls it.
func TestActionGivesUpOnAHungBus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeLogind{verdict: map[Action]string{Reboot: "yes"}}
		m := New(testutil.NopLogger(), f)
		m.Probe(context.Background())
		f.hang = true

		start := time.Now()
		if err := m.Reboot(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
		if elapsed := time.Since(start); elapsed != callTimeout {
			t.Fatalf("action took %v, want %v", elapsed, callTimeout)
		}
	})
}
