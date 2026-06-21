package library_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
)

func imgs(names ...string) []library.Image {
	out := make([]library.Image, len(names))
	for i, n := range names {
		out[i] = library.Image{Name: n}
	}
	return out
}

// hasAllImages reports whether actual contains exactly the expected images, any order.
func hasAllImages(expected, actual []library.Image) bool {
	if len(expected) != len(actual) {
		return false
	}
	counts := make(map[string]int)
	for _, img := range expected {
		counts[img.Name]++
	}
	for _, img := range actual {
		counts[img.Name]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func hasExactOrder(a, b []library.Image) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func TestNewEmpty(t *testing.T) {
	l := library.New(nil, false)
	if l.Len() != 0 {
		t.Fatalf("expected 0, got %d", l.Len())
	}
	if len(l.List()) != 0 {
		t.Fatal("List on empty library should be empty")
	}
}

func TestNewWithImages(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	if l.Len() != 2 {
		t.Fatalf("expected 2, got %d", l.Len())
	}
	if l.List()[0].Name != "a.jpg" {
		t.Errorf("expected a.jpg, got %s", l.List()[0].Name)
	}
}

func TestHas(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	if !l.Has("a.jpg") {
		t.Error("expected Has(a.jpg) true")
	}
	if l.Has("c.jpg") {
		t.Error("expected Has(c.jpg) false")
	}
	l.Remove("a.jpg")
	if l.Has("a.jpg") {
		t.Error("expected Has(a.jpg) false after Remove")
	}
}

func TestList(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	list := l.List()
	if len(list) != 3 || list[0].Name != "a.jpg" || list[2].Name != "c.jpg" {
		t.Errorf("unexpected list: %v", list)
	}
}

func TestAdd(t *testing.T) {
	l := library.New(nil, false)
	l.Add("a.jpg")
	if l.Len() != 1 {
		t.Fatalf("expected 1, got %d", l.Len())
	}
	if l.List()[0].Name != "a.jpg" {
		t.Errorf("expected a.jpg, got %s", l.List()[0].Name)
	}
}

func TestRemoveFound(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	if !l.Remove("b.jpg") {
		t.Fatal("expected Remove to return true")
	}
	if l.Len() != 2 {
		t.Fatalf("expected 2, got %d", l.Len())
	}
	list := l.List()
	if list[0].Name != "a.jpg" || list[1].Name != "c.jpg" {
		t.Errorf("unexpected list after remove: %v", list)
	}
}

func TestRemoveNotFound(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	if l.Remove("missing.jpg") {
		t.Fatal("expected Remove to return false for unknown name")
	}
	if l.Len() != 1 {
		t.Fatal("library length should be unchanged")
	}
}

func TestListReturnsCopy(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	list := l.List()
	list[0].Name = "mutated"
	if l.List()[0].Name != "a.jpg" {
		t.Error("List should return a copy, not a reference to internal slice")
	}
}

func TestNewRandomizedDeterministic(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")

	rng1 := rand.New(rand.NewPCG(99, 99))
	rng2 := rand.New(rand.NewPCG(99, 99))

	lib1 := library.New(original, true, library.WithTestRNG(rng1))
	lib2 := library.New(original, true, library.WithTestRNG(rng2))

	if !hasExactOrder(original, lib1.List()) {
		t.Fatal("List() must be canonical (unshuffled), got shuffled order")
	}

	cycle1 := lib1.Cycle()
	cycle2 := lib2.Cycle()

	if hasExactOrder(original, cycle1) {
		t.Fatal("expected randomize=true to shuffle initial cycle, but order remained original")
	}
	if !hasExactOrder(cycle1, cycle2) {
		t.Fatalf("expected deterministic shuffles to match perfectly, got \n%v \nand \n%v", cycle1, cycle2)
	}
}

func TestSetRandomizeEnableShufflesOnNextReshuffle(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	rng := rand.New(rand.NewPCG(42, 42))
	l := library.New(original, false, library.WithTestRNG(rng))

	l.SetRandomize(true)
	// Enabling does not reorder immediately; the next cycle does.
	if !hasExactOrder(original, l.List()) {
		t.Errorf("SetRandomize(true) should not reorder immediately, got %v", l.List())
	}
	got := l.Reshuffle()
	if !hasAllImages(original, got) {
		t.Errorf("images lost after reshuffle: %v", got)
	}
	if hasExactOrder(original, got) {
		t.Error("expected reshuffle to permute after enabling randomize")
	}
}

func TestSetRandomizeNeverReordersCanonical(t *testing.T) {
	original := imgs("c.jpg", "a.jpg", "b.jpg") // deliberately not alphabetical
	rng := rand.New(rand.NewPCG(42, 42))
	l := library.New(original, true, library.WithTestRNG(rng))

	l.SetRandomize(false)
	if !hasExactOrder(original, l.List()) {
		t.Errorf("SetRandomize must not reorder canonical, got %v", l.List())
	}
}

func TestSetRandomizeNoOpWhenUnchanged(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	before := l.List()
	l.SetRandomize(false)
	if !hasExactOrder(before, l.List()) {
		t.Error("SetRandomize(false) on non-randomized library should not change order")
	}
}

func TestReshuffleAdoptsCanonicalAfterSetOrder(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	l.SetOrder([]string{"c.jpg", "b.jpg", "a.jpg"})
	l.Reshuffle()
	if !hasExactOrder(l.Cycle(), imgs("c.jpg", "b.jpg", "a.jpg")) {
		t.Fatalf("Cycle should adopt canonical order after Reshuffle, got %v", l.Cycle())
	}
}

func TestSetRandomizeReportsChange(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	if l.SetRandomize(false) {
		t.Error("SetRandomize(false) on an already-off library should report no change")
	}
	if !l.SetRandomize(true) {
		t.Error("SetRandomize(true) should report a change")
	}
	if l.SetRandomize(true) {
		t.Error("SetRandomize(true) again should report no change")
	}
}

func TestRandomizedReflectsFlag(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	if l.Randomized() {
		t.Error("Randomized should be false initially")
	}
	l.SetRandomize(true)
	if !l.Randomized() {
		t.Error("Randomized should be true after SetRandomize(true)")
	}
}

func TestReshuffleSequentialKeepsOrder(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	got := l.Reshuffle()
	if !hasExactOrder(got, imgs("a.jpg", "b.jpg", "c.jpg")) {
		t.Fatalf("sequential Reshuffle changed order: %v", got)
	}
}

func TestReshuffleRandomizedPermutesWithoutImmediateRepeat(t *testing.T) {
	images := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	for seed := range uint64(50) {
		rng := rand.New(rand.NewPCG(seed, seed))
		l := library.New(images, true, library.WithTestRNG(rng))
		prevLast := l.Cycle()[len(images)-1].Name

		got := l.Reshuffle()
		if !hasAllImages(got, images) {
			t.Fatalf("seed=%d: Reshuffle dropped or duplicated images", seed)
		}
		if got[0].Name == prevLast {
			t.Fatalf("seed=%d: Reshuffle put the previous last image %q first", seed, prevLast)
		}
	}
}

func TestReshuffleSingleImage(t *testing.T) {
	l := library.New(imgs("a.jpg"), true)
	if got := l.Reshuffle(); len(got) != 1 || got[0].Name != "a.jpg" {
		t.Fatalf("Reshuffle single = %v", got)
	}
}

func TestReshuffleEmpty(t *testing.T) {
	l := library.New(nil, true)
	if got := l.Reshuffle(); len(got) != 0 {
		t.Fatalf("Reshuffle empty = %v", got)
	}
}

func TestListIsCanonicalAndStableAcrossReshuffle(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	imgs := []library.Image{{"a.jpg"}, {"b.jpg"}, {"c.jpg"}, {"d.jpg"}}
	l := library.New(imgs, true, library.WithTestRNG(rng))
	before := l.List()
	l.Reshuffle()
	after := l.List()
	if !slices.Equal(imgNames(before), imgNames(after)) {
		t.Fatalf("List (canonical) changed across Reshuffle: %v -> %v", before, after)
	}
	if !slices.Equal(imgNames(after), []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg"}) {
		t.Fatalf("List not canonical order: %v", after)
	}
}

func TestCycleShuffledWhenRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	imgs := []library.Image{{"a.jpg"}, {"b.jpg"}, {"c.jpg"}, {"d.jpg"}}
	l := library.New(imgs, true, library.WithTestRNG(rng))
	if slices.Equal(imgNames(l.Cycle()), imgNames(l.List())) {
		t.Fatalf("expected shuffled cycle to differ from canonical")
	}
}

func TestSetOrderReconciles(t *testing.T) {
	l := library.New([]library.Image{{"a.jpg"}, {"b.jpg"}, {"c.jpg"}}, false)
	// Unknown "z.jpg" ignored; "c.jpg" omitted so it appends at the end.
	got := l.SetOrder([]string{"b.jpg", "z.jpg", "a.jpg"})
	want := []string{"b.jpg", "a.jpg", "c.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("SetOrder returned %v want %v", got, want)
	}
	if !slices.Equal(imgNames(l.List()), want) {
		t.Fatalf("List after SetOrder %v want %v", l.List(), want)
	}
}

func TestSetOrderMovesKnownAheadOfUnknown(t *testing.T) {
	// Unknown "x.jpg" sits first, so the known images must sort ahead of it.
	l := library.New([]library.Image{{"x.jpg"}, {"a.jpg"}, {"b.jpg"}}, false)
	got := l.SetOrder([]string{"b.jpg", "a.jpg"})
	want := []string{"b.jpg", "a.jpg", "x.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("SetOrder returned %v want %v", got, want)
	}
}

func TestSetOrderNilKeepsOrder(t *testing.T) {
	l := library.New([]library.Image{{"a.jpg"}, {"b.jpg"}}, false)
	got := l.SetOrder(nil)
	if !slices.Equal(got, []string{"a.jpg", "b.jpg"}) {
		t.Fatalf("SetOrder(nil) %v", got)
	}
}

func TestAddAppendsToCanonicalAndCycle(t *testing.T) {
	l := library.New([]library.Image{{"a.jpg"}}, false)
	l.Add("b.jpg")
	if !slices.Equal(imgNames(l.List()), []string{"a.jpg", "b.jpg"}) {
		t.Fatalf("canonical %v", l.List())
	}
	if !slices.Equal(imgNames(l.Cycle()), []string{"a.jpg", "b.jpg"}) {
		t.Fatalf("cycle %v", l.Cycle())
	}
}

func TestRemoveDropsFromBoth(t *testing.T) {
	l := library.New([]library.Image{{"a.jpg"}, {"b.jpg"}}, false)
	if !l.Remove("a.jpg") {
		t.Fatal("Remove returned false")
	}
	if !slices.Equal(imgNames(l.List()), []string{"b.jpg"}) || !slices.Equal(imgNames(l.Cycle()), []string{"b.jpg"}) {
		t.Fatalf("after remove list=%v cycle=%v", l.List(), l.Cycle())
	}
	if l.Remove("missing.jpg") {
		t.Fatal("Remove(missing) should be false")
	}
}

func TestReshuffleAvoidsImmediateRepeat(t *testing.T) {
	// Seed chosen so a naive shuffle would repeat the last element first.
	rng := rand.New(rand.NewPCG(42, 42))
	l := library.New([]library.Image{{"a.jpg"}, {"b.jpg"}, {"c.jpg"}}, true, library.WithTestRNG(rng))
	prev := l.Cycle()
	next := l.Reshuffle()
	if next[0].Name == prev[len(prev)-1].Name {
		t.Fatalf("cycle opened with previous last image %q", next[0].Name)
	}
}

func imgNames(imgs []library.Image) []string {
	out := make([]string, len(imgs))
	for i, img := range imgs {
		out[i] = img.Name
	}
	return out
}

func TestValidImageName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"photo.jpg", true},
		{"photo.jpeg", true},
		{"snap.png", true},
		{"IMG_20221223~2.png", true},
		{"9b7b87ad-f032-442d-a7c3-046fed72e7bc-1745136011.jpg", true}, // immich-synced shape
		{"family photo.jpg", false},                                   // space
		{"nyaralás.jpg", false},                                       // non-ASCII
		{"IMG_1234.JPG", false},                                       // uppercase extension
		{"clip.gif", false},                                           // unservable format
		{"../escape.jpg", false},                                      // path separator
		{".jpg", false},                                               // no stem
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := library.ValidImageName(tc.name); got != tc.want {
				t.Errorf("ValidImageName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
