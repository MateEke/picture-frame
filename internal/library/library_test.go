package library_test

import (
	"math/rand/v2"
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
	if l.Current() != nil {
		t.Fatal("Current on empty library should be nil")
	}
	if l.Next() != nil {
		t.Fatal("Next on empty library should be nil")
	}
}

func TestNewWithImages(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	if l.Len() != 2 {
		t.Fatalf("expected 2, got %d", l.Len())
	}
	if l.Current().Name != "a.jpg" {
		t.Errorf("expected a.jpg, got %s", l.Current().Name)
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

func TestNextWraps(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	if l.Next().Name != "b.jpg" {
		t.Error("expected b.jpg after first Next")
	}
	if l.Next().Name != "c.jpg" {
		t.Error("expected c.jpg")
	}
	if l.Next().Name != "a.jpg" {
		t.Error("expected wrap back to a.jpg")
	}
}

func TestAdd(t *testing.T) {
	l := library.New(nil, false)
	l.Add("a.jpg")
	if l.Len() != 1 {
		t.Fatalf("expected 1, got %d", l.Len())
	}
	if l.Current().Name != "a.jpg" {
		t.Errorf("expected a.jpg, got %s", l.Current().Name)
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

func TestRemoveClampsIndex(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	l.Next() // idx = 1
	l.Next() // idx = 2 (c.jpg)
	l.Remove("c.jpg")
	// idx was 2, now len=2, so idx should clamp to 1
	if l.Current().Name != "b.jpg" {
		t.Errorf("expected index clamped to b.jpg, got %s", l.Current().Name)
	}
}

func TestRemoveBeforeIndexPreservesCurrent(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg"), false)
	l.Next() // idx = 1
	l.Next() // idx = 2 (c.jpg)
	if !l.Remove("a.jpg") {
		t.Fatal("expected Remove to return true")
	}
	// idx should shift from 2 to 1 so Current() is still c.jpg.
	if l.Current().Name != "c.jpg" {
		t.Errorf("expected Current to still be c.jpg, got %s", l.Current().Name)
	}
}

func TestRemoveCurrentAdvancesToNext(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	l.Next() // idx = 1 (b.jpg)
	if !l.Remove("b.jpg") {
		t.Fatal("expected Remove to return true")
	}
	// Removed current image, slice shifted: c.jpg now at idx 1, so Current() = c.jpg.
	if l.Current().Name != "c.jpg" {
		t.Errorf("expected Current to be c.jpg (next), got %s", l.Current().Name)
	}
}

func TestListReturnsCopy(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	list := l.List()
	list[0].Name = "mutated"
	if l.Current().Name != "a.jpg" {
		t.Error("List should return a copy, not a reference to internal slice")
	}
}

func TestNewRandomizedDeterministic(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")

	rng1 := rand.New(rand.NewPCG(99, 99)) //nolint:gosec
	rng2 := rand.New(rand.NewPCG(99, 99)) //nolint:gosec

	lib1 := library.New(original, true, library.WithTestRNG(rng1))
	lib2 := library.New(original, true, library.WithTestRNG(rng2))

	list1 := lib1.List()
	list2 := lib2.List()

	if hasExactOrder(original, list1) {
		t.Fatal("expected randomize=true to shuffle images, but order remained original")
	}

	if !hasExactOrder(list1, list2) {
		t.Fatalf("expected deterministic shuffles to match perfectly, got \n%v \nand \n%v", list1, list2)
	}
}

func TestNextWrapsRandomizedDeterministic(t *testing.T) {
	original := imgs("1.jpg", "2.jpg", "3.jpg", "4.jpg", "5.jpg")

	rng := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
	l := library.New(original, true, library.WithTestRNG(rng))

	firstCycle := l.List()

	for i := 0; i < len(original)-1; i++ {
		l.Next()
	}

	l.Next()

	secondCycle := l.List()

	if !hasAllImages(firstCycle, secondCycle) {
		t.Fatalf("Randomized library dropped or duplicated images during wrap-around")
	}

	if hasExactOrder(firstCycle, secondCycle) {
		t.Fatal("expected wrap-around to reshuffle images, but order remained identical")
	}
}

func TestSetRandomizeEnablesRestartsFromBeginning(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	rng := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
	l := library.New(original, false, library.WithTestRNG(rng))

	l.Next() // advance mid-cycle
	l.Next()

	l.SetRandomize(true)

	// idx is reset to last position; Next() wraps to 0 and shuffles.
	first := l.Next()
	if first == nil {
		t.Fatal("Next() after SetRandomize(true) returned nil")
	}
	// All images must still be present.
	if !hasAllImages(original, l.List()) {
		t.Errorf("images lost after SetRandomize(true): %v", l.List())
	}
}

func TestSetRandomizeDisablesSortsAndRestartsFromBeginning(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	rng := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
	l := library.New(original, true, library.WithTestRNG(rng))

	l.Next()
	l.Next()

	l.SetRandomize(false)

	// Images should be sorted alphabetically.
	list := l.List()
	for i := 1; i < len(list); i++ {
		if list[i].Name < list[i-1].Name {
			t.Errorf("expected sorted order after SetRandomize(false), got %v", list)
			break
		}
	}
	// Next() wraps to 0 and returns the alphabetically first image.
	if got := l.Next().Name; got != "a.jpg" {
		t.Errorf("expected restart at a.jpg, got %q", got)
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

func TestNextWrapsRandomizedNoAdjacentDuplicate(t *testing.T) {
	// The last image of cycle N must not equal the first image of cycle N+1.
	// Use many seeds to exercise the swap-if-equal path.
	images := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	for seed := range uint64(50) {
		rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec
		l := library.New(images, true, library.WithTestRNG(rng))

		var prev string
		for range 3 {
			for range images {
				img := l.Next()
				if img.Name == prev && prev != "" {
					t.Fatalf("seed=%d: adjacent duplicate %q at cycle boundary", seed, prev)
				}
				prev = img.Name
			}
		}
	}
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
