package power_test

import (
	"testing"

	hardwarepower "github.com/MateEke/picture-frame/internal/hardware/power"
)

func TestNoopPowerCycle(t *testing.T) {
	p := &hardwarepower.Noop{}
	ctx := t.Context()

	if on, _ := p.State(ctx); on {
		t.Error("zero Noop should be off")
	}
	if err := p.On(ctx); err != nil {
		t.Fatal(err)
	}
	if on, _ := p.State(ctx); !on {
		t.Error("expected on after On")
	}
	if err := p.Off(ctx); err != nil {
		t.Fatal(err)
	}
	if on, _ := p.State(ctx); on {
		t.Error("expected off after Off")
	}
}

func TestNoopBrightness(t *testing.T) {
	p := &hardwarepower.Noop{}
	ctx := t.Context()

	if err := p.SetBrightness(ctx, 70); err != nil {
		t.Fatal(err)
	}
	if got := p.Brightness(); got != 70 {
		t.Errorf("Brightness = %d, want 70", got)
	}
	for _, level := range []int{-1, 101} {
		if err := p.SetBrightness(ctx, level); err == nil {
			t.Errorf("SetBrightness(%d): expected range error", level)
		}
	}
	// A rejected value must not clobber the stored one.
	if got := p.Brightness(); got != 70 {
		t.Errorf("Brightness = %d after rejected set, want 70", got)
	}
}
