package adapter

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/power"
)

// The calls need a live logind, but a typo here silently disables both entities.
func TestCanMethodNamesMatchLogind(t *testing.T) {
	if got, want := canMethod(power.Reboot), "org.freedesktop.login1.Manager.CanReboot"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := canMethod(power.PowerOff), "org.freedesktop.login1.Manager.CanPowerOff"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDoMethodNamesMatchLogind(t *testing.T) {
	if got, want := doMethod(power.Reboot), "org.freedesktop.login1.Manager.Reboot"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := doMethod(power.PowerOff), "org.freedesktop.login1.Manager.PowerOff"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
