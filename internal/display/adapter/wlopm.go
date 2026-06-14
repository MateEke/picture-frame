package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Wlopm implements display.Controller via wlopm (wlr-output-power-management-v1
// through labwc): it DPMS-toggles the panel without forcing an output relayout.
// Needs full KMS, labwc running, and WAYLAND_DISPLAY/XDG_RUNTIME_DIR set (see
// deploy/systemd/kiosk-backend.service). Writes are serialized, concurrent wlopm
// processes can return empty responses on constrained hardware. Reads use sysfs.
type Wlopm struct {
	mu        sync.Mutex
	log       *slog.Logger
	output    string // connector name, e.g. "HDMI-A-1"
	sysfsBase string // override for tests; defaults to /sys/class/drm

	pathMu   sync.Mutex
	dpmsPath string // resolved dpms path, cached after the first successful glob
}

func NewWlopm(output string, log *slog.Logger) *Wlopm {
	return &Wlopm{output: output, log: log}
}

func (w *Wlopm) On(ctx context.Context) error  { return w.set(ctx, "--on") }
func (w *Wlopm) Off(ctx context.Context) error { return w.set(ctx, "--off") }

func (w *Wlopm) State(_ context.Context) (bool, error) {
	path, err := w.drmDPMSPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("display state: %w", err)
	}
	state := strings.TrimSpace(string(data))
	w.log.Debug("display: state read from sysfs", "path", path, "state", state)
	return state == "On", nil
}

// drmDPMSPath finds <sysfsBase>/card*-<output>/dpms. The connector path is
// stable, so the first successful resolution is cached; a boot-race miss returns
// an error (uncached) and retries on the next call.
func (w *Wlopm) drmDPMSPath() (string, error) {
	w.pathMu.Lock()
	cached := w.dpmsPath
	w.pathMu.Unlock()
	if cached != "" {
		return cached, nil
	}
	base := w.sysfsBase
	if base == "" {
		base = "/sys/class/drm"
	}
	pattern := fmt.Sprintf("%s/card*-%s/dpms", base, w.output)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("display dpms glob: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no DRM dpms file for output %q (pattern: %s)", w.output, pattern)
	}
	w.pathMu.Lock()
	w.dpmsPath = matches[0]
	w.pathMu.Unlock()
	return matches[0], nil
}

// set toggles the panel; wlopm exits non-zero on failure, so the exit code is
// authoritative (no read-back needed).
func (w *Wlopm) set(ctx context.Context, flag string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log.Debug("wlopm: setting power", "flag", flag, "output", w.output)
	// gosec G204: a constant flag + an operator-set connector name, not user input.
	out, err := exec.CommandContext(ctx, "wlopm", flag, w.output).CombinedOutput() //nolint:gosec
	if err != nil {
		w.log.Debug("wlopm: set failed", "flag", flag, "err", err, "raw", string(out))
		return fmt.Errorf("wlopm %s %s: %w (output: %s)", flag, w.output, err, out)
	}
	w.log.Debug("wlopm: set succeeded", "flag", flag, "output", w.output)
	return nil
}
