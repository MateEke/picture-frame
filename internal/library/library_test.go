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

func TestSetRandomizeEnableShufflesOnNextReshuffle(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	rng := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
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

func TestSetRandomizeDisablesSorts(t *testing.T) {
	original := imgs("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg")
	rng := rand.New(rand.NewPCG(42, 42)) //nolint:gosec
	l := library.New(original, true, library.WithTestRNG(rng))

	l.SetRandomize(false)
	list := l.List()
	for i := 1; i < len(list); i++ {
		if list[i].Name < list[i-1].Name {
			t.Errorf("expected sorted order after SetRandomize(false), got %v", list)
			break
		}
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
		rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec
		l := library.New(images, true, library.WithTestRNG(rng))
		prevLast := l.List()[len(images)-1].Name

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
