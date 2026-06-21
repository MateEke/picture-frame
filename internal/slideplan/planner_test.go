package slideplan_test

import (
	"sync"
	"testing"

	"github.com/MateEke/picture-frame/internal/slideplan"
)

// orderCalls counts snapshot reads so tests can assert lazy rebuilds.
type fakeSource struct {
	mu         sync.Mutex
	order      []string
	cycles     [][]string
	orderCalls int
}

func (f *fakeSource) Order() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orderCalls++
	return append([]string(nil), f.order...)
}

func (f *fakeSource) NextCycle() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.cycles) > 0 {
		f.order = f.cycles[0]
		f.cycles = f.cycles[1:]
	}
	return append([]string(nil), f.order...)
}

func (f *fakeSource) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.orderCalls
}

func portraitRatios() func(string) (float64, bool) {
	return ratios(map[string]float64{
		"P1": 0.66, "P2": 0.66, "P3": 0.66, "L1": landscape,
	})
}

func newPlanner(src slideplan.Source) *slideplan.Planner {
	return slideplan.NewPlanner(src, portraitRatios(), slideplan.Threshold{Factor: 1.5}, true)
}

func TestPlannerCurrentBuildsPlan(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2", "P3"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)

	got := p.Current()
	if got == nil || len(got.Names) != 2 || got.Names[0] != "P1" || got.Names[1] != "P2" {
		t.Fatalf("Current = %v, want [P1 P2]", got)
	}
}

func TestPlannerEmpty(t *testing.T) {
	p := newPlanner(&fakeSource{})
	if p.Current() != nil {
		t.Error("Current on empty source should be nil")
	}
	if p.Next() != nil {
		t.Error("Next on empty source should be nil")
	}
}

func TestPlannerNextAdvances(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2", "P3"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)

	p.Current() // [P1 P2]
	got := p.Next()
	if got == nil || got.Names[0] != "P2" || got.Names[1] != "P3" {
		t.Fatalf("Next = %v, want [P2 P3]", got)
	}
}

func TestPlannerRestartCycleStartsFreshFromTop(t *testing.T) {
	src := &fakeSource{order: []string{"L1", "L2"}, cycles: [][]string{{"P1", "P2"}}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)

	p.Current()
	p.Next()
	p.RestartCycle()

	got := p.Current()
	if got == nil || len(got.Names) != 2 || got.Names[0] != "P1" || got.Names[1] != "P2" {
		t.Fatalf("after RestartCycle Current = %v, want paired [P1 P2] from the top", got)
	}
}

func TestPlannerNextWrapsToNewCycle(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}, cycles: [][]string{{"L1"}}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)

	p.Current()     // [P1 P2] (only slide this cycle)
	got := p.Next() // wrap: NextCycle -> ["L1"]
	if got == nil || len(got.Names) != 1 || got.Names[0] != "L1" {
		t.Fatalf("Next after wrap = %v, want [L1]", got)
	}
}

func TestPlannerNextWrapsToEmpty(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}, cycles: [][]string{{}}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)

	p.Current() // [P1 P2] (only slide)
	if p.Next() != nil {
		t.Fatal("Next wrapping to an empty cycle should be nil")
	}
}

func TestPlannerSetConfigUnchangedIsNoop(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)
	p.Current()
	calls := src.calls()

	p.SetConfig(true, slideplan.Threshold{Factor: 1.5}) // identical to constructor
	if got := p.Current(); len(got.Names) != 2 {
		t.Fatalf("want pair retained, got %v", got)
	}
	if src.calls() != calls {
		t.Fatalf("unchanged config should not rebuild, calls %d -> %d", calls, src.calls())
	}
}

func TestPlannerSetScreenAspectRebuildsOnlyOnChange(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}}
	p := newPlanner(src)

	if got := p.Current(); got == nil || len(got.Names) != 1 {
		t.Fatalf("Current with unknown screen = %v, want solo", got)
	}
	afterFirst := src.calls()

	p.SetScreenAspect(landscape)
	if got := p.Current(); got == nil || len(got.Names) != 2 {
		t.Fatalf("Current after aspect set = %v, want pair", got)
	}
	afterChange := src.calls()
	if afterChange != afterFirst+1 {
		t.Fatalf("expected one rebuild on change, calls %d -> %d", afterFirst, afterChange)
	}

	p.SetScreenAspect(landscape)
	p.Current()
	if src.calls() != afterChange {
		t.Fatalf("unchanged aspect should not rebuild, calls now %d", src.calls())
	}
}

func TestPlannerSetConfigDisables(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)
	if got := p.Current(); len(got.Names) != 2 {
		t.Fatalf("want pair before disable, got %v", got)
	}

	p.SetConfig(false, slideplan.Threshold{Factor: 1.5})
	if got := p.Current(); len(got.Names) != 1 {
		t.Fatalf("want solo after disable, got %v", got)
	}
}

func TestPlannerInvalidateRereadsOrder(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)
	p.Current()

	src.mu.Lock()
	src.order = []string{"L1"}
	src.mu.Unlock()
	p.Invalidate()

	if got := p.Current(); got == nil || got.Names[0] != "L1" {
		t.Fatalf("Current after Invalidate = %v, want [L1]", got)
	}
}

func TestPlannerNextAfterInvalidateShowsFirstSlide(t *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2", "P3"}}
	p := newPlanner(src)
	p.SetScreenAspect(landscape)
	p.Current() // [P1 P2], idx 0

	p.Invalidate()
	got := p.Next()
	if got == nil || got.Names[0] != "P1" || got.Names[1] != "P2" {
		t.Fatalf("Next after Invalidate = %v, want [P1 P2]", got)
	}
}

func TestPlannerDirtyRebuildKeepsCursor(t *testing.T) {
	// A config/aspect change mid-cycle must not jump back to the start and re-show.
	src := &fakeSource{order: []string{"L1", "L2", "L3", "L4"}}
	fit := func(string) (float64, bool) { return landscape, true } // all fit on a landscape screen
	p := slideplan.NewPlanner(src, fit, slideplan.Threshold{Factor: 1.5}, true)
	p.SetScreenAspect(landscape)
	p.Current()
	p.Next()
	p.Next() // at L3

	p.SetConfig(true, slideplan.Threshold{Factor: 2.0}) // dirty, plan stays all-solo
	if got := p.Next(); got == nil || got.Names[0] != "L3" {
		t.Fatalf("dirty rebuild should keep the cursor at L3, got %v", got)
	}
}

func TestPlannerDirtyRebuildClampsShrunkCursor(t *testing.T) {
	// Enabling split pairs images, shrinking the plan under the cursor; a preserved
	// idx past the new end must clamp, not read out of range.
	src := &fakeSource{order: []string{"P1", "P2", "P3", "P4"}}
	portrait := func(string) (float64, bool) { return 0.66, true }
	p := slideplan.NewPlanner(src, portrait, slideplan.Threshold{Factor: 1.5}, false)
	p.SetScreenAspect(landscape)
	p.Current()
	p.Next()
	p.Next() // idx 2, plan has 4 solo slides

	p.SetConfig(true, slideplan.Threshold{Factor: 1.5}) // enable split -> 2 paired slides
	if got := p.Next(); got == nil || len(got.Names) != 2 {
		t.Fatalf("cursor past the shrunk plan should clamp, got %v", got)
	}
}

func TestPlannerConcurrent(_ *testing.T) {
	src := &fakeSource{order: []string{"P1", "P2", "P3"}}
	p := newPlanner(src)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				p.Next()
				p.SetScreenAspect(landscape)
				p.Current()
				p.Invalidate()
			}
		})
	}
	wg.Wait()
}
