// Package updater fetches, verifies, and applies signed GitHub release tarballs from
// the admin UI, with pre-flight and systemd-native rollback.
package updater

import (
	"log/slog"
	"sync"
	"time"

	"aead.dev/minisign"
)

// Phase is the updater's current activity, surfaced to the UI.
type Phase int

const (
	PhaseIdle Phase = iota
	PhaseChecking
	PhaseDownloading
	PhaseVerifying
	PhaseApplying
)

// String is the lowercase phase name the UI displays.
func (p Phase) String() string {
	switch p {
	case PhaseChecking:
		return "checking"
	case PhaseDownloading:
		return "downloading"
	case PhaseVerifying:
		return "verifying"
	case PhaseApplying:
		return "applying"
	default:
		return "idle"
	}
}

// Status is the snapshot the admin UI polls. LastCheckOK distinguishes "checked, no
// update" from "couldn't reach the update server" (offline is normal, not an error).
type Status struct {
	Current       string
	Platform      string
	Latest        string
	NotesURL      string // release page for Latest (GitHub html_url); "" if none
	Available     bool
	LastCheck     time.Time
	LastCheckOK   bool
	Phase         Phase
	LastResult    string // "", "ok", "failed: …", "rolled back from vX"
	LastResultSeq int    // bumps each time LastResult is set, so a repeated outcome still registers
}

// Options configures a new Updater.
type Options struct {
	Log        *slog.Logger
	Source     ReleaseSource
	Downloader Downloader
	Preflight  PreflightRunner
	PubKey     minisign.PublicKey
	Current    string // this binary's version (version.Version)
	Platform   string // this binary's build target (version.Platform)
	AutoUpdate bool
	UpdateHour int
	InstallDir string
	Restart    func() error
}

// Updater checks for and applies releases. Checking always runs; AutoUpdate gates only
// the nightly auto-apply. Mirrors library.Syncer's Run/Trigger/Status shape.
type Updater struct {
	log        *slog.Logger
	source     ReleaseSource
	current    string
	platform   string
	autoUpdate bool
	updateHour int
	installDir string
	restart    func() error
	downloader Downloader
	preflight  PreflightRunner
	pubKey     minisign.PublicKey
	trigger    chan struct{}
	checkNow   chan struct{}

	mu        sync.Mutex
	status    Status
	latestTgt *target // newest of any major (manual apply); nil if none
	autoTgt   *target // newest same-major (auto apply); nil if none

	retryDelay time.Duration // backoff after a failed check; Run goroutine only
	skipAuto   []string      // versions a rollback recorded; excluded from auto-apply; Run goroutine only
}

func New(o Options) *Updater {
	return &Updater{
		log:        o.Log,
		source:     o.Source,
		current:    o.Current,
		platform:   o.Platform,
		autoUpdate: o.AutoUpdate,
		updateHour: o.UpdateHour,
		installDir: o.InstallDir,
		restart:    o.Restart,
		downloader: o.Downloader,
		preflight:  o.Preflight,
		pubKey:     o.PubKey,
		trigger:    make(chan struct{}, 1), // buffered so Trigger never blocks
		checkNow:   make(chan struct{}, 1), // buffered so Check never blocks
		status:     Status{Current: o.Current, Platform: o.Platform, Phase: PhaseIdle},
	}
}

// Status returns the latest snapshot. Safe from any goroutine.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// Trigger requests an out-of-band apply. Non-blocking; coalesces if one is queued.
func (u *Updater) Trigger() {
	select {
	case u.trigger <- struct{}{}:
	default:
	}
}

// Check requests an out-of-band release check (no apply). Non-blocking; coalesces.
func (u *Updater) Check() {
	select {
	case u.checkNow <- struct{}{}:
	default:
	}
}
