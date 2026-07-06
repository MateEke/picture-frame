package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WlrRandr implements display.Rotator by shelling wlr-randr. One mutex spans
// each whole operation so Set can't interleave with Reconcile's query-then-apply.
type WlrRandr struct {
	mu         sync.Mutex
	log        *slog.Logger
	output     string // connector name, e.g. "HDMI-A-1"
	runtimeDir string // XDG_RUNTIME_DIR; "" skips intent-file writes (dev)
	desired    int    // counter-clockwise degrees
	applied    bool   // last exec for desired succeeded
}

// NewWlrRandr writes the intent file immediately: cog's launch script must see
// it even before labwc is up.
func NewWlrRandr(output string, desired int, runtimeDir string, log *slog.Logger) *WlrRandr {
	w := &WlrRandr{log: log, output: output, runtimeDir: runtimeDir, desired: desired}
	w.writeIntent()
	return w
}

// A wedged compositor must fail the exec, not hang the save/SSE/startup paths
// that hold w.mu.
const execTimeout = 5 * time.Second

// transformArg maps degrees to wlr-randr vocabulary ("0" is not accepted).
func transformArg(degrees int) (string, error) {
	switch degrees {
	case 0:
		return "normal", nil
	case 90, 180, 270:
		return strconv.Itoa(degrees), nil
	default:
		return "", fmt.Errorf("rotation must be 0, 90, 180 or 270, got %d", degrees)
	}
}

func (w *WlrRandr) Supported() bool {
	_, err := exec.LookPath("wlr-randr")
	return err == nil
}

func (w *WlrRandr) Set(ctx context.Context, degrees int) error {
	if _, err := transformArg(degrees); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if degrees == w.desired && w.applied {
		return nil // saves re-exec on unrelated config saves
	}
	w.desired = degrees
	w.writeIntent()
	return w.applyLocked(ctx)
}

func (w *WlrRandr) Reconcile(ctx context.Context) {
	// No wlr-randr (pre-rotation install): skip silently, this runs on every SSE connect.
	if !w.Supported() {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	want, _ := transformArg(w.desired) // desired is always valid here
	current, err := w.queryLocked(ctx)
	if err != nil {
		w.log.Warn("wlr-randr: transform query failed", "err", err)
		w.applied = false
		return
	}
	if current == want {
		w.applied = true
		return
	}
	w.log.Info("wlr-randr: transform drifted, re-applying", "current", current, "want", want)
	if err := w.applyLocked(ctx); err != nil {
		w.log.Warn("wlr-randr: re-apply failed", "err", err)
	}
}

func (w *WlrRandr) applyLocked(ctx context.Context) error {
	arg, err := transformArg(w.desired)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "wlr-randr", "--output", w.output, "--transform", arg).CombinedOutput() //nolint:gosec
	if err != nil {
		w.applied = false
		return fmt.Errorf("wlr-randr --transform %s on %s: %w (output: %s)", arg, w.output, err, out)
	}
	w.applied = true
	w.log.Info("wlr-randr: transform applied", "output", w.output, "transform", arg)
	return nil
}

func (w *WlrRandr) queryLocked(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "wlr-randr").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wlr-randr query: %w (output: %s)", err, out)
	}
	return parseTransform(out, w.output)
}

// parseTransform reads the connector block's indented "Transform:" line.
func parseTransform(out []byte, connector string) (string, error) {
	inBlock := false
	for line := range strings.SplitSeq(string(out), "\n") {
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if !indented {
			first, _, _ := strings.Cut(strings.TrimSpace(line), " ")
			inBlock = first == connector
			continue
		}
		if !inBlock {
			continue
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Transform:"); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("output %q not in wlr-randr listing", connector)
}

// writeIntent records desired rotation for kiosk-browser-launch.sh (intent, not state).
func (w *WlrRandr) writeIntent() {
	if w.runtimeDir == "" {
		return
	}
	path := filepath.Join(w.runtimeDir, "rotation")
	if err := os.WriteFile(path, []byte(strconv.Itoa(w.desired)), 0600); err != nil {
		w.log.Warn("wlr-randr: intent file write failed", "path", path, "err", err)
	}
}
