package library_test

import (
	"log/slog"
	"os"
	"slices"
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
)

func cycleNames(s []library.Image) []string {
	out := make([]string, len(s))
	for i, img := range s {
		out[i] = img.Name
	}
	return out
}

func TestSetExcludedSkipsInNextCycle(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	if !l.SetExcluded([]string{"b.jpg"}, true) {
		t.Fatal("expected change")
	}
	if got := cycleNames(l.Reshuffle()); !slices.Equal(got, []string{"a.jpg", "c.jpg"}) {
		t.Fatalf("cycle = %v", got)
	}
	if got := cycleNames(l.List()); len(got) != 3 {
		t.Fatalf("List must keep excluded images, got %v", got)
	}
	if !l.Excluded("b.jpg") || l.Excluded("a.jpg") {
		t.Fatal("Excluded mismatch")
	}
}

func TestSetExcludedIgnoresUnknownAndNoOps(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	if l.SetExcluded([]string{"zzz.jpg"}, true) {
		t.Fatal("unknown name must not change anything")
	}
	if l.SetExcluded([]string{"a.jpg"}, false) {
		t.Fatal("including an included image is a no-op")
	}
}

func TestSetExcludedAllFallsBackToEverything(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	l.SetExcluded([]string{"a.jpg", "b.jpg"}, true)
	if got := cycleNames(l.Reshuffle()); !slices.Equal(got, []string{"a.jpg", "b.jpg"}) {
		t.Fatalf("all-excluded cycle = %v", got)
	}
}

func TestIncludeAgainAndExcludedNames(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg", "c.jpg"), false)
	l.SetExcluded([]string{"c.jpg", "a.jpg"}, true)
	if got := l.ExcludedNames(); !slices.Equal(got, []string{"a.jpg", "c.jpg"}) {
		t.Fatalf("ExcludedNames = %v", got)
	}
	l.SetExcluded([]string{"a.jpg"}, false)
	if got := cycleNames(l.Reshuffle()); !slices.Equal(got, []string{"a.jpg", "b.jpg"}) {
		t.Fatalf("cycle = %v", got)
	}
}

func TestRemoveClearsExclusion(t *testing.T) {
	l := library.New(imgs("a.jpg", "b.jpg"), false)
	l.SetExcluded([]string{"b.jpg"}, true)
	l.Remove("b.jpg")
	if len(l.ExcludedNames()) != 0 {
		t.Fatal("removed image must drop its exclusion")
	}
}

func TestExcludeStoreRoundTrip(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	s, got, err := library.LoadExcludeStore(slog.Default(), root)
	if err != nil || got != nil {
		t.Fatalf("fresh load: %v %v", got, err)
	}
	if err := s.Save([]string{"x.jpg"}); err != nil {
		t.Fatal(err)
	}
	_, got, err = library.LoadExcludeStore(slog.Default(), root)
	if err != nil || !slices.Equal(got, []string{"x.jpg"}) {
		t.Fatalf("reload: %v %v", got, err)
	}
	if err := os.WriteFile(root.Name()+"/.slideshow-exclude.json", []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, got, err = library.LoadExcludeStore(slog.Default(), root); err != nil || got != nil {
		t.Fatalf("corrupt: %v %v", got, err)
	}
}
