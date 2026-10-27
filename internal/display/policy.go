package display

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MateEke/picture-frame/internal/state"
)

// idleCheckInterval is how often Run checks for idle-blank.
const idleCheckInterval = 10 * time.Second

// displayOpTimeout bounds one Controller call so a wedged compositor can't hang the policy.
const displayOpTimeout = 5 * time.Second

// scheduleRelockDefault is how long a touch keeps the panel lit inside the
// schedule window when blank_after is 0; otherwise blank_after is used.
const scheduleRelockDefault = 5 * time.Minute

// Schedule is a daily window during which the panel is held off, e.g. overnight.
// Off and On are "HH:MM" in Location (nil = time.Local); the window may cross
// midnight. Validated by config; an unparseable or empty window never matches.
type Schedule struct {
	Enabled  bool
	Off, On  string
	Location *time.Location
}

func (s Schedule) contains(t time.Time) bool {
	if !s.Enabled {
		return false
	}
	off, err := time.Parse("15:04", s.Off)
	if err != nil {
		return false
	}
	on, err := time.Parse("15:04", s.On)
	if err != nil {
		return false
	}
	if s.Location != nil {
		t = t.In(s.Location)
	}
	m := t.Hour()*60 + t.Minute()
	offM, onM := off.Hour()*60+off.Minute(), on.Hour()*60+on.Minute()
	if offM < onM {
		return m >= offM && m < onM
	}
	return offM > onM && (m >= offM || m < onM)
}

// Policy is the single owner of panel power: it unifies manual on/off (persisted),
// motion-wake, idle-blank and the daily schedule, and is the only caller of the Controller. Hardware
// state is read once at startup; after that, successful writes are authoritative.
type Policy struct {
	log        *slog.Logger
	display    Controller
	bus        *state.Bus
	store      IntentStore // nil disables intent persistence
	blankAfter time.Duration
	// motionAvailable gates idle-blank: without a motion sensor to wake the screen,
	// blanking would be a one-way trip, so it's disabled. Fixed at startup since
	// sensors only change across a restart.
	motionAvailable bool

	mu          sync.Mutex
	schedule    Schedule
	manualOff   bool      // user turned the screen off; suppresses motion auto-wake
	idleBlank   bool      // auto-blanked after blankAfter of no motion
	scheduleOff bool      // held off by the schedule window; only a touch or manual on lifts it
	inSchedule  bool      // window state at the last check, to act on its edges only
	lastMotion  time.Time // idle baseline
	lastTouch   time.Time // re-blank baseline inside the window; presence "motion" must not hold it
	on          bool      // last power state we applied to the Controller
}

// Bus is required (Run subscribes to it, and it carries published intent);
// Store may be nil (no persistence).
type PolicyConfig struct {
	Log        *slog.Logger
	Display    Controller
	Bus        *state.Bus
	Store      IntentStore
	BlankAfter time.Duration
	// MotionAvailable must be true for idle-blank to engage (a motion sensor is
	// needed to wake the screen again).
	MotionAvailable bool
	Schedule        Schedule
}

func NewPolicy(cfg PolicyConfig) *Policy {
	return &Policy{
		log:             cfg.Log,
		display:         cfg.Display,
		bus:             cfg.Bus,
		store:           cfg.Store,
		blankAfter:      cfg.BlankAfter,
		motionAvailable: cfg.MotionAvailable,
		schedule:        cfg.Schedule,
	}
}

// SetBlankAfter updates the idle-blank duration. Takes effect on the next idle check.
func (p *Policy) SetBlankAfter(d time.Duration) {
	p.mu.Lock()
	p.blankAfter = d
	p.mu.Unlock()
}

// SetSchedule replaces the daily off window. Takes effect on the next idle check.
func (p *Policy) SetSchedule(s Schedule) {
	p.mu.Lock()
	p.schedule = s
	p.mu.Unlock()
}

// Start restores persisted intent and reconciles to the compositor's actual
// power. Call once before Run; unlike a blind On() it won't wake a manually-off
// or idle-blanked screen across a restart.
func (p *Policy) Start(ctx context.Context) {
	p.mu.Lock()
	if p.store != nil {
		off, err := p.store.LoadManualOff()
		if err != nil {
			p.log.Warn("policy: failed to load screen intent; defaulting to auto", "err", err)
		} else {
			p.manualOff = off
		}
	}
	p.lastMotion = time.Now()
	p.inSchedule = p.schedule.contains(p.lastMotion)
	p.scheduleOff = p.inSchedule
	sctx, cancel := context.WithTimeout(ctx, displayOpTimeout)
	actual, err := p.display.State(sctx)
	cancel()
	if err != nil {
		// Panel state unknown: assume the opposite of desired so applyLocked
		// actually drives the hardware once, instead of no-opping on a wrong guess.
		p.log.Warn("policy: cannot read panel state at startup; forcing desired", "err", err)
		p.on = p.manualOff
	} else {
		p.on = actual
		if !p.manualOff {
			p.idleBlank = !actual
		}
	}
	if err := p.applyLocked(ctx); err != nil {
		p.log.Warn("policy: failed to apply screen state at startup", "err", err)
	}
	p.log.Info("policy: started", "manualOff", p.manualOff, "on", p.on, "desired", p.desiredLocked())
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	p.publish(on, auto)
}

// State reports the last-known panel power without invoking the controller.
func (p *Policy) State() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.on
}

// Reconcile re-reads actual panel state (via sysfs, not the compositor) and
// corrects drift, e.g. after a labwc restart. Publishes if it corrects.
func (p *Policy) Reconcile(ctx context.Context) {
	sctx, cancel := context.WithTimeout(ctx, displayOpTimeout)
	actual, err := p.display.State(sctx)
	cancel()
	if err != nil {
		p.log.Warn("policy: reconcile could not read panel state", "err", err)
		return
	}
	p.mu.Lock()
	prevOn := p.on
	p.on = actual
	want := p.desiredLocked()
	if actual != want {
		p.log.Info("policy: reconciling display", "want", want, "actual", actual)
		if err := p.applyLocked(ctx); err != nil {
			p.log.Warn("policy: reconcile failed to apply", "err", err)
			// p.on keeps the real (actual) state for the publish below.
		}
	}
	// Publish only when live power actually moved (drift correction or a
	// no-apply read that differs from what we last knew); intent is untouched here.
	changed := p.on != prevOn
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	if changed {
		p.publish(on, auto)
	}
}

// SetManual is the manual on/off entry point. On a Controller error the intent
// is left unchanged.
func (p *Policy) SetManual(ctx context.Context, on bool) error {
	p.mu.Lock()
	prevManual, prevIdle, prevSched, prevMotion, prevTouch := p.manualOff, p.idleBlank, p.scheduleOff, p.lastMotion, p.lastTouch
	p.manualOff = !on
	if on {
		p.idleBlank, p.scheduleOff = false, false
		p.lastMotion = time.Now()
		p.lastTouch = p.lastMotion
	}
	// Detach from the caller's context: a manual power change shouldn't abort
	// half-applied if an HTTP client disconnects. applyLocked still bounds it.
	if err := p.applyLocked(context.WithoutCancel(ctx)); err != nil {
		p.manualOff, p.idleBlank, p.scheduleOff, p.lastMotion, p.lastTouch = prevManual, prevIdle, prevSched, prevMotion, prevTouch
		p.mu.Unlock()
		return err
	}
	if p.store != nil {
		if err := p.store.SaveManualOff(p.manualOff); err != nil {
			p.log.Warn("policy: failed to persist screen intent", "err", err)
		}
	}
	livePower, auto := p.on, !p.manualOff
	p.mu.Unlock()
	p.publish(livePower, auto)
	return nil
}

// Wake turns the panel on for a kiosk touch, overriding a manual off or the
// schedule, and restarts the idle countdown. On a lit panel it is only the timer reset.
func (p *Policy) Wake(ctx context.Context) error {
	p.mu.Lock()
	prevManual, prevIdle, prevSched, prevMotion, prevTouch, prevOn := p.manualOff, p.idleBlank, p.scheduleOff, p.lastMotion, p.lastTouch, p.on
	p.manualOff, p.idleBlank, p.scheduleOff, p.lastMotion = false, false, false, time.Now()
	p.lastTouch = p.lastMotion
	if err := p.applyLocked(context.WithoutCancel(ctx)); err != nil {
		p.manualOff, p.idleBlank, p.scheduleOff, p.lastMotion, p.lastTouch = prevManual, prevIdle, prevSched, prevMotion, prevTouch
		p.mu.Unlock()
		return err
	}
	intentChanged := prevManual != p.manualOff
	if intentChanged && p.store != nil {
		if err := p.store.SaveManualOff(p.manualOff); err != nil {
			p.log.Warn("policy: failed to persist screen intent", "err", err)
		}
	}
	powerMoved := p.on != prevOn
	if powerMoved {
		p.log.Info("policy: display on (touch)")
	}
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	if powerMoved || intentChanged {
		p.publish(on, auto)
	}
	return nil
}

// Auto reports whether motion auto-wake is active (false after a manual Off).
func (p *Policy) Auto() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.manualOff
}

// Run enforces the motion→display policy until ctx is cancelled. Call Start first.
func (p *Policy) Run(ctx context.Context) {
	ch, unsub := p.bus.Subscribe()
	defer unsub()

	idleTicker := time.NewTicker(idleCheckInterval)
	defer idleTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if isMotionEvent(event) {
				p.onMotion(ctx)
			}
		case <-idleTicker.C:
			p.onSchedule(ctx)
			p.onTick(ctx)
		}
	}
}

func (p *Policy) onMotion(ctx context.Context) {
	p.mu.Lock()
	p.lastMotion = time.Now()
	if p.manualOff || !p.idleBlank {
		p.mu.Unlock()
		return
	}
	p.idleBlank = false
	if err := p.applyLocked(ctx); err != nil {
		p.log.Warn("policy: failed to wake display", "err", err)
		p.idleBlank = true
		p.mu.Unlock()
		return
	}
	p.log.Info("policy: display on (motion)")
	// Live power moved; intent (auto) is unchanged, so the switch won't budge.
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	p.publish(on, auto)
}

func (p *Policy) onTick(ctx context.Context) {
	p.mu.Lock()
	if p.blankAfter <= 0 || !p.motionAvailable || p.manualOff || p.idleBlank {
		p.mu.Unlock()
		return
	}
	if time.Since(p.lastMotion) < p.blankAfter {
		p.mu.Unlock()
		return
	}
	p.idleBlank = true
	if err := p.applyLocked(ctx); err != nil {
		p.log.Warn("policy: failed to blank display", "err", err)
		p.idleBlank = false
		p.mu.Unlock()
		return
	}
	p.log.Info("policy: display off (idle)", "idle", time.Since(p.lastMotion).Round(time.Second))
	// Live power moved; intent (auto) is unchanged, so the switch won't budge.
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	p.publish(on, auto)
}

// onSchedule switches the panel off when the window opens and on when it closes,
// and re-blanks a panel a touch lit inside the window relockAfter after that touch.
// Motion never lifts scheduleOff: passing by at night shouldn't light the frame.
func (p *Policy) onSchedule(ctx context.Context) {
	p.mu.Lock()
	in := p.schedule.contains(time.Now())
	prevSched, prevIdle, prevIn, prevMotion := p.scheduleOff, p.idleBlank, p.inSchedule, p.lastMotion
	var reason string
	switch {
	case in && !p.inSchedule:
		p.scheduleOff, reason = true, "display off (schedule)"
	case !in && p.inSchedule:
		// Restart the idle countdown so a motion-gated frame lights for the morning.
		p.scheduleOff, p.idleBlank, p.lastMotion = false, false, time.Now()
		reason = "display on (schedule)"
	case in && !p.scheduleOff && time.Since(p.lastTouch) >= p.relockAfterLocked():
		p.scheduleOff, reason = true, "display off (schedule, idle)"
	}
	p.inSchedule = in
	if reason == "" {
		p.mu.Unlock()
		return
	}
	prevOn := p.on
	if err := p.applyLocked(ctx); err != nil {
		p.log.Warn("policy: failed to apply schedule", "err", err)
		// Roll back fully, inSchedule included, so the next tick retries the edge.
		p.scheduleOff, p.idleBlank, p.inSchedule, p.lastMotion = prevSched, prevIdle, prevIn, prevMotion
		p.mu.Unlock()
		return
	}
	if p.on == prevOn {
		p.mu.Unlock()
		return
	}
	p.log.Info("policy: " + reason)
	on, auto := p.on, !p.manualOff
	p.mu.Unlock()
	p.publish(on, auto)
}

func (p *Policy) relockAfterLocked() time.Duration {
	if p.blankAfter > 0 {
		return p.blankAfter
	}
	return scheduleRelockDefault
}

func (p *Policy) desiredLocked() bool { return !p.manualOff && !p.idleBlank && !p.scheduleOff }

// applyLocked drives the Controller to the desired state. Must hold p.mu, the
// I/O runs under the lock (bounded by displayOpTimeout) on purpose: wlopm already
// serializes its own calls, ops are sub-second, and it keeps each state
// transition atomic with the hardware write.
func (p *Policy) applyLocked(ctx context.Context) error {
	want := p.desiredLocked()
	if want == p.on {
		return nil
	}
	octx, cancel := context.WithTimeout(ctx, displayOpTimeout)
	defer cancel()
	var err error
	if want {
		err = p.display.On(octx)
	} else {
		err = p.display.Off(octx)
	}
	if err != nil {
		return err
	}
	p.on = want
	return nil
}

// publish emits the current screen state: on is live panel power, auto is the
// manual intent. Call after releasing p.mu, never hold the lock across bus I/O.
// Bus is required (see PolicyConfig), so no nil guard.
func (p *Policy) publish(on, auto bool) {
	p.bus.Publish(state.Event{Kind: state.KindScreen, Payload: state.ScreenPayload{On: on, Auto: auto}})
}

func isMotionEvent(e state.Event) bool {
	sp, ok := e.Payload.(state.SensorPayload)
	return ok && sp.Kind == "motion" && sp.Value != 0
}
