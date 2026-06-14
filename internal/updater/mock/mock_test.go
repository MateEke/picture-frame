package mock

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/MateEke/picture-frame/internal/updater"
)

func TestNewDefaultsOutcomeOK(t *testing.T) {
	u := New(Options{Current: "v1.2.0", Platform: "linux_armv6"})
	if s := u.Status(); s.Current != "v1.2.0" || s.Platform != "linux_armv6" || s.Phase != updater.PhaseIdle {
		t.Fatalf("initial status: %+v", s)
	}
	if u.opts.Outcome != "ok" {
		t.Errorf("empty outcome should default to ok, got %q", u.opts.Outcome)
	}
}

func TestCheckSetsAvailability(t *testing.T) {
	u := New(Options{Latest: "v1.3.1"})
	u.runCheck()
	if s := u.Status(); !s.Available || s.Latest != "v1.3.1" || !s.LastCheckOK || s.LastCheck.IsZero() {
		t.Errorf("with a latest set: %+v", s)
	}

	none := New(Options{})
	none.runCheck()
	if none.Status().Available {
		t.Error("no latest → not available")
	}
}

func TestCheckOffline(t *testing.T) {
	u := New(Options{Latest: "v1.3.1", Offline: true})
	u.runCheck()
	if s := u.Status(); s.LastCheckOK || s.Available || s.Latest != "" {
		t.Errorf("offline check should be unreachable + no update: %+v", s)
	}
}

func TestApplyOK(t *testing.T) {
	u := New(Options{Current: "v1.2.0", Latest: "v1.3.1", Outcome: "ok"})
	u.runCheck()
	u.runApply(context.Background())
	if s := u.Status(); s.LastResult != "ok" || s.Current != "v1.3.1" || s.Available || s.Phase != updater.PhaseIdle {
		t.Errorf("after ok apply: %+v", s)
	}
}

func TestApplyFailedKeepsAvailable(t *testing.T) {
	u := New(Options{Current: "v1.2.0", Latest: "v1.3.1", Outcome: "rolled back from v1.3.1"})
	u.runCheck()
	u.runApply(context.Background())
	if s := u.Status(); s.LastResult != "rolled back from v1.3.1" || s.Current != "v1.2.0" || !s.Available {
		t.Errorf("a non-ok outcome must keep current/available: %+v", s)
	}
}

func TestRunInitialCheckAndTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u := New(Options{Current: "v1.2.0", Latest: "v1.3.1"})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go u.Run(ctx)
		synctest.Wait()
		if !u.Status().Available {
			t.Fatal("Run should perform an initial check")
		}
		u.Trigger()
		synctest.Wait()
		if s := u.Status(); s.LastResult != "ok" || s.Current != "v1.3.1" {
			t.Errorf("after triggered apply: %+v", s)
		}
	})
}

func TestApplyBumpsResultSeq(t *testing.T) {
	u := New(Options{Current: "v1.2.0", Latest: "v1.3.1", Outcome: "rolled back from v1.3.1"})
	u.runCheck()
	u.runApply(context.Background())
	first := u.Status().LastResultSeq
	u.runCheck()
	u.runApply(context.Background())
	if got := u.Status().LastResultSeq; first < 1 || got != first+1 {
		t.Errorf("LastResultSeq: first %d then %d, want a strict +1 bump", first, got)
	}
}
