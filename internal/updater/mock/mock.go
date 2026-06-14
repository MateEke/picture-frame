// Package mock simulates the self-updater for development and e2e. It walks the apply
// phases and reaches a terminal result without ever downloading or swapping a binary,
// so the UI can be exercised on a dev machine.
package mock

import (
	"context"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/updater"
)

// Options configures the simulated updater.
type Options struct {
	Current  string        // running version
	Platform string        // build target, e.g. linux_armv6
	Latest   string        // the fake newer version; "" means no update available
	Outcome  string        // terminal LastResult after an apply; "" defaults to "ok"
	Offline  bool          // simulate an unreachable release source (LastCheckOK=false)
	Delay    time.Duration // per-phase delay; keep small in dev, 0 in tests
}

// Updater is a drop-in updater.Updater for non-production builds.
type Updater struct {
	opts   Options
	mu     sync.Mutex
	status updater.Status
	apply  chan struct{}
	check  chan struct{}
}

func New(o Options) *Updater {
	if o.Outcome == "" {
		o.Outcome = "ok"
	}
	// Seed availability up front so callers see a stable status before Run's first check
	// (which would otherwise race a freshly-booted server answering /healthz).
	u := &Updater{
		opts:   o,
		status: updater.Status{Current: o.Current, Platform: o.Platform, Phase: updater.PhaseIdle},
		apply:  make(chan struct{}, 1),
		check:  make(chan struct{}, 1),
	}
	u.runCheck()
	return u
}

func (u *Updater) Status() updater.Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

func (u *Updater) Trigger() { trySend(u.apply) }
func (u *Updater) Check()   { trySend(u.check) }

// Run performs an initial check, then reacts to Trigger/Check until ctx is done.
func (u *Updater) Run(ctx context.Context) {
	u.runCheck()
	for {
		select {
		case <-ctx.Done():
			return
		case <-u.check:
			u.runCheck()
		case <-u.apply:
			u.runApply(ctx)
		}
	}
}

func (u *Updater) runCheck() {
	u.setPhase(updater.PhaseChecking)
	u.mu.Lock()
	u.status.LastCheck = time.Now()
	u.status.LastCheckOK = !u.opts.Offline
	if u.opts.Offline {
		u.status.Available, u.status.Latest, u.status.NotesURL = false, "", ""
	} else if u.opts.Latest != "" {
		u.status.Available, u.status.Latest = true, u.opts.Latest
		u.status.NotesURL = "https://example.test/releases/" + u.opts.Latest
	} else {
		u.status.Available, u.status.Latest, u.status.NotesURL = false, "", ""
	}
	u.status.Phase = updater.PhaseIdle
	u.mu.Unlock()
}

func (u *Updater) runApply(ctx context.Context) {
	for _, p := range []updater.Phase{updater.PhaseDownloading, updater.PhaseVerifying, updater.PhaseApplying} {
		u.setPhase(p)
		select {
		case <-ctx.Done():
			return
		case <-time.After(u.opts.Delay):
		}
	}
	u.mu.Lock()
	u.status.Phase = updater.PhaseIdle
	u.status.LastResult = u.opts.Outcome
	u.status.LastResultSeq++
	if u.opts.Outcome == "ok" { // now on the latest version
		u.status.Current = u.opts.Latest
		u.status.Available = false
		u.status.Latest = ""
	}
	u.mu.Unlock()
}

func (u *Updater) setPhase(p updater.Phase) {
	u.mu.Lock()
	u.status.Phase = p
	u.mu.Unlock()
}

func trySend(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
