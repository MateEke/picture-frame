package library

import (
	"testing"
	"time"
)

// backoffDelay is jittered, so assert ranges: [d/2, d] per level with d
// doubling from base and capped at maxDelay.
func TestBackoffDelayRanges(t *testing.T) {
	const base, maxDelay = time.Second, time.Minute
	for i := 0; i < 50; i++ {
		if d := backoffDelay(1, base, maxDelay); d < 500*time.Millisecond || d > base {
			t.Fatalf("level 1 delay = %v, want [500ms, 1s]", d)
		}
		if d := backoffDelay(2, base, maxDelay); d < time.Second || d > 2*time.Second {
			t.Fatalf("level 2 delay = %v, want [1s, 2s]", d)
		}
	}
	// Deep failures pin to the cap.
	for i := 0; i < 20; i++ {
		if d := backoffDelay(100, base, maxDelay); d < 30*time.Second || d > maxDelay {
			t.Fatalf("capped delay = %v, want [30s, 1m]", d)
		}
	}
}

func TestBackoffDelayDefaults(t *testing.T) {
	if d := backoffDelay(1, 0, 0); d < 15*time.Second || d > 30*time.Second {
		t.Fatalf("default delay = %v, want [15s, 30s]", d)
	}
	// A sub-base interval caps the backoff below the default base.
	if d := backoffDelay(10, 0, time.Second); d < 500*time.Millisecond || d > time.Second {
		t.Fatalf("small-cap delay = %v, want [500ms, 1s]", d)
	}
}
