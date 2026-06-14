package updater

import "testing"

func TestStatusSnapshotInitial(t *testing.T) {
	u := New(Options{Current: "v1.2.0", Platform: "linux_armv6"})
	s := u.Status()
	if s.Current != "v1.2.0" || s.Platform != "linux_armv6" {
		t.Fatalf("current/platform: %+v", s)
	}
	if s.Available || s.Phase != PhaseIdle || !s.LastCheck.IsZero() {
		t.Errorf("initial status should be idle/empty: %+v", s)
	}
}

func TestTriggerCoalesces(t *testing.T) {
	u := New(Options{})
	u.Trigger()
	u.Trigger() // buffered + coalesced: must not block
	select {
	case <-u.trigger:
	default:
		t.Fatal("expected one queued trigger")
	}
}

func TestCheckCoalesces(t *testing.T) {
	u := New(Options{})
	u.Check()
	u.Check() // buffered + coalesced: must not block
	select {
	case <-u.checkNow:
	default:
		t.Fatal("expected one queued check")
	}
}

func TestPhaseString(t *testing.T) {
	cases := map[Phase]string{
		PhaseIdle: "idle", PhaseChecking: "checking", PhaseDownloading: "downloading",
		PhaseVerifying: "verifying", PhaseApplying: "applying", Phase(99): "idle",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Phase(%d).String() = %q, want %q", p, got, want)
		}
	}
}
