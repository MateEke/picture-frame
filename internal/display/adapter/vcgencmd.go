package adapter

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Vcgencmd implements display.Controller via the vcgencmd tool.
// Requires dtoverlay=vc4-fkms-v3d in /boot/firmware/config.txt.
type Vcgencmd struct{}

func NewVcgencmd() *Vcgencmd { return &Vcgencmd{} }

func (v *Vcgencmd) On(ctx context.Context) error  { return v.set(ctx, "1") }
func (v *Vcgencmd) Off(ctx context.Context) error { return v.set(ctx, "0") }

func (v *Vcgencmd) State(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "vcgencmd", "display_power").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("vcgencmd display_power: %w (output: %s)", err, out)
	}
	val, err := parseDisplayPower(out)
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func (v *Vcgencmd) set(ctx context.Context, want string) error {
	out, err := exec.CommandContext(ctx, "vcgencmd", "display_power", want).CombinedOutput()
	if err != nil {
		return fmt.Errorf("vcgencmd display_power %s: %w (output: %s)", want, err, out)
	}
	// vcgencmd exits 0 even when the change is rejected (e.g. fkms not active).
	// The output always reflects the actual current state, so we verify it.
	got, err := parseDisplayPower(out)
	if err != nil {
		return fmt.Errorf("vcgencmd display_power %s: %w", want, err)
	}
	if got != want {
		return fmt.Errorf("vcgencmd display_power %s: display remained %s", want, got)
	}
	return nil
}

// parseDisplayPower extracts the numeric state from vcgencmd output ("display_power=N").
func parseDisplayPower(out []byte) (string, error) {
	s := strings.TrimSpace(string(out))
	val, ok := strings.CutPrefix(s, "display_power=")
	if !ok {
		return "", fmt.Errorf("unexpected vcgencmd output: %q", s)
	}
	return val, nil
}
