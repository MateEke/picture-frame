package slideshow_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/slideshow"
)

func TestLibrarySourceOrderUsesCycle(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	lib := library.New([]library.Image{{Name: "a.jpg"}, {Name: "b.jpg"}, {Name: "c.jpg"}, {Name: "d.jpg"}}, true, library.WithTestRNG(rng))
	src := slideshow.NewLibrarySource(lib)
	first := src.Order()
	second := src.Order()
	if !slices.Equal(first, second) {
		t.Fatalf("Order not stable: %v vs %v", first, second)
	}
	if slices.Equal(first, []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg"}) {
		t.Fatalf("Order returned canonical, expected shuffled cycle")
	}
}
