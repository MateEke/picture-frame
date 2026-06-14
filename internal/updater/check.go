package updater

import (
	"context"
	"time"
)

// Run checks on startup, then daily at UpdateHour, applying same-major updates when
// AutoUpdate is on. A manual Trigger applies the newest release of any major.
func (u *Updater) Run(ctx context.Context) {
	// A rollback the systemd unit performed on the previous boot surfaces once, here; the
	// version is also remembered so we don't auto-apply that broken release again.
	if v, ok := readRolledBackVersion(u.installDir); ok {
		if v == "" {
			u.setLastResult("rolled back from a failed update")
		} else {
			u.setLastResult("rolled back from " + v)
			if err := recordSkipAuto(u.installDir, v); err != nil {
				u.log.Warn("updater: could not record rolled-back version", "err", err)
			}
		}
	}
	u.skipAuto = skipAutoVersions(u.installDir)
	u.check(ctx)
	for {
		delay, scheduled := u.nextCheck()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			u.check(ctx)
			// Only the scheduled daily run auto-applies; a backoff retry just re-checks, so a
			// failed startup check can't become an unscheduled apply.
			if scheduled {
				u.mu.Lock()
				auto, checkOK := u.autoTgt, u.status.LastCheckOK
				u.mu.Unlock()
				// checkOK gates a stale target: an offline nightly check would
				// otherwise start a doomed download of yesterday's target.
				if u.autoUpdate && checkOK && auto != nil {
					u.applyTarget(ctx, auto)
				}
			}
		case <-u.checkNow:
			timer.Stop()
			u.check(ctx)
		case <-u.trigger:
			timer.Stop()
			u.mu.Lock()
			latest := u.latestTgt
			u.mu.Unlock()
			if latest != nil {
				u.applyTarget(ctx, latest)
			}
		}
	}
}

// nextCheck returns the delay until the next check and whether it's the scheduled daily run.
// A failed check (e.g. network not ready after boot) retries with backoff instead, so a
// transient miss doesn't strand the frame on stale info until tomorrow.
func (u *Updater) nextCheck() (time.Duration, bool) {
	u.mu.Lock()
	ok := u.status.LastCheckOK
	u.mu.Unlock()
	if ok {
		u.retryDelay = 0
		return time.Until(nextRunAfter(time.Now(), u.updateHour)), true
	}
	u.retryDelay = nextBackoff(u.retryDelay)
	return u.retryDelay, false
}

// nextBackoff doubles the retry delay from 30s up to a 30m ceiling.
func nextBackoff(prev time.Duration) time.Duration {
	const floor, ceil = 30 * time.Second, 30 * time.Minute
	switch {
	case prev <= 0:
		return floor
	case prev*2 > ceil:
		return ceil
	default:
		return prev * 2
	}
}

// check queries the release source and records availability. A list failure means
// offline/unreachable, recorded (LastCheckOK=false), logged at info, never an error.
func (u *Updater) check(ctx context.Context) {
	u.setPhase(PhaseChecking)
	defer u.setPhase(PhaseIdle)

	releases, err := u.source.List(ctx)
	if err != nil {
		u.log.Info("updater: could not reach the release source (offline?)", "err", err)
		u.mu.Lock()
		u.status.LastCheck = time.Now()
		u.status.LastCheckOK = false
		u.mu.Unlock()
		return
	}

	// Manual updates (latest) offer every release so a rolled-back version can still be retried;
	// auto-apply (auto) excludes rolled-back versions so a broken release can't loop daily.
	latest, err := resolveLatest(releases, u.current, u.platform, AnyMajor, nil)
	if err != nil {
		u.log.Debug("updater: newest release has no asset for this platform", "err", err)
	}
	auto, autoErr := resolveLatest(releases, u.current, u.platform, SameMajor, u.skipAuto)
	if autoErr != nil {
		u.log.Debug("updater: newest same-major release has no asset for this platform", "err", autoErr)
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.LastCheck = time.Now()
	u.status.LastCheckOK = true
	u.latestTgt, u.autoTgt = latest, auto
	if latest != nil {
		u.status.Available, u.status.Latest, u.status.NotesURL = true, latest.version, latest.notesURL
	} else {
		u.status.Available, u.status.Latest, u.status.NotesURL = false, "", ""
	}
}

func (u *Updater) setPhase(p Phase) {
	u.mu.Lock()
	u.status.Phase = p
	u.mu.Unlock()
}

// nextRunAfter returns the next occurrence of hour:00 strictly after now (today if
// now is before hour, otherwise tomorrow), in now's location.
func nextRunAfter(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
