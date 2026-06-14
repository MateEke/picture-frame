package adapter

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecPreflight runs a candidate binary's `--health-check` to prove it boots here.
type ExecPreflight struct{}

// Check runs `<binPath> --health-check` and returns its combined output on failure.
func (ExecPreflight) Check(binPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binPath, "--health-check").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
