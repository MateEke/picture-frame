package library_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
)

func TestReshuffleSingleRandomizedImageNoPanic(t *testing.T) {
	l := library.New(imgs("only.jpg"), true)
	for i := range 3 {
		if got := l.Reshuffle(); len(got) != 1 || got[0].Name != "only.jpg" {
			t.Fatalf("call %d: got %v, want [only.jpg]", i, got)
		}
	}
}

func TestRemoveLastThenAdd(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	if !l.Remove("a.jpg") {
		t.Fatal("remove should report found")
	}
	l.Add("b.jpg")
	if l.Len() != 1 || !l.Has("b.jpg") {
		t.Fatalf("after remove+add: len=%d has(b)=%v", l.Len(), l.Has("b.jpg"))
	}
}
