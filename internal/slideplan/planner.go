package slideplan

import "sync"

// Source supplies the cycle's image names: Order is the current cycle, NextCycle
// advances it (reshuffling when randomized). The Planner calls these without
// holding its own mutex.
type Source interface {
	Order() []string
	NextCycle() []string
}

// Planner caches the current cycle's slide plan and serves it from a cursor,
// rebuilding lazily when dirty or on a wrap. Safe for concurrent use.
type Planner struct {
	src     Source
	ratioOf func(string) (float64, bool)

	mu      sync.Mutex
	screen  float64
	thr     Threshold
	enabled bool
	slides  []Slide
	idx     int
	dirty   bool
}

// NewPlanner starts with an unknown screen aspect (no pairing until SetScreenAspect).
func NewPlanner(src Source, ratioOf func(string) (float64, bool), thr Threshold, enabled bool) *Planner {
	return &Planner{src: src, ratioOf: ratioOf, thr: thr, enabled: enabled, dirty: true}
}

// ensure rebuilds when dirty (reporting whether it did); the snapshot read is
// outside the lock so Source can take its own.
func (p *Planner) ensure() bool {
	p.mu.Lock()
	need := p.slides == nil || p.dirty
	p.mu.Unlock()
	if !need {
		return false
	}
	order := p.src.Order()
	p.rebuild(order, false)
	return true
}

// rebuild re-groups order, keeping the cursor (so a config/aspect change doesn't
// jump back and re-show); only a wrap or out-of-range idx resets to the start.
func (p *Planner) rebuild(order []string, reset bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.slides = Plan(order, p.screen, p.ratioOf, p.thr, p.enabled)
	if reset || p.idx >= len(p.slides) {
		p.idx = 0
	}
	p.dirty = false
}

// Current returns the slide at the cursor, or nil when empty.
func (p *Planner) Current() *Slide {
	p.ensure()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.slides) == 0 {
		return nil
	}
	s := p.slides[p.idx]
	return &s
}

// Next advances the cursor, wrapping at the end (nil when empty). A pending
// rebuild takes precedence: it returns the new plan's first slide, not the next.
func (p *Planner) Next() *Slide {
	if p.ensure() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if len(p.slides) == 0 {
			return nil
		}
		s := p.slides[p.idx]
		return &s
	}

	p.mu.Lock()
	// Advance only while idx stays in range so a concurrent Current never reads an
	// out-of-range cursor; a last/empty cursor falls through to the wrap below.
	if p.idx+1 < len(p.slides) {
		p.idx++
		s := p.slides[p.idx]
		p.mu.Unlock()
		return &s
	}
	p.mu.Unlock()

	order := p.src.NextCycle()
	p.rebuild(order, true)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.slides) == 0 {
		return nil
	}
	s := p.slides[p.idx]
	return &s
}

// SetScreenAspect updates the screen aspect ratio (width/height); it marks the
// plan dirty and reports true only when the value changes.
func (p *Planner) SetScreenAspect(aspect float64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if aspect == p.screen {
		return false
	}
	p.screen = aspect
	p.dirty = true
	return true
}

// SetConfig updates the enable flag and threshold, marking the plan dirty when
// either changes.
func (p *Planner) SetConfig(enabled bool, thr Threshold) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if enabled == p.enabled && thr == p.thr {
		return
	}
	p.enabled = enabled
	p.thr = thr
	p.dirty = true
}

// Invalidate marks the plan stale so the next Current/Next re-reads the order.
func (p *Planner) Invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dirty = true
}

// SlideCount is the number of slides in the current plan. Skipping this many
// guarantees a cycle wrap, after which slides come from the current library.
func (p *Planner) SlideCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.slides)
}
