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

// Policy is the single owner of panel power: it unifies manual on/off (persisted),
// motion-wake and idle-blank, and is the only caller of the Controller. Hardware
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

	mu         sync.Mutex
	manualOff  bool      // user turned the screen off; suppresses motion auto-wake
	idleBlank  bool      // auto-blanked after blankAfter of no motion
	lastMotion time.Time // idle baseline
	on         bool      // last power state we applied to the Controller
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
}

func NewPolicy(cfg PolicyConfig) *Policy {
	return &Policy{
		log:             cfg.Log,
		display:         cfg.Display,
		bus:             cfg.Bus,
		store:           cfg.Store,
		blankAfter:      cfg.BlankAfter,
		motionAvailable: cfg.MotionAvailable,
	}
}

// SetBlankAfter updates the idle-blank duration. Takes effect on the next idle check.
func (p *Policy) SetBlankAfter(d time.Duration) {
	p.mu.Lock()
	p.blankAfter = d
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
	prevManual, prevIdle, prevMotion := p.manualOff, p.idleBlank, p.lastMotion
	p.manualOff = !on
	if on {
		p.idleBlank = false
		p.lastMotion = time.Now()
	}
	// Detach from the caller's context: a manual power change shouldn't abort
	// half-applied if an HTTP client disconnects. applyLocked still bounds it.
	if err := p.applyLocked(context.WithoutCancel(ctx)); err != nil {
		p.manualOff, p.idleBlank, p.lastMotion = prevManual, prevIdle, prevMotion
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

func (p *Policy) desiredLocked() bool { return !p.manualOff && !p.idleBlank }

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
