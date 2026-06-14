package library_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
)

func TestNextSingleRandomizedImageWrapsWithoutPanic(t *testing.T) {
	l := library.New(imgs("only.jpg"), true)
	for i := range 3 {
		if got := l.Next(); got == nil || got.Name != "only.jpg" {
			t.Fatalf("call %d: got %v, want only.jpg", i, got)
		}
	}
}

func TestRemoveLastThenAddKeepsValidIndex(t *testing.T) {
	l := library.New(imgs("a.jpg"), false)
	if !l.Remove("a.jpg") {
		t.Fatal("remove should report found")
	}
	l.Add("b.jpg")
	if got := l.Current(); got == nil || got.Name != "b.jpg" {
		t.Fatalf("got %v, want b.jpg", got)
	}
}
