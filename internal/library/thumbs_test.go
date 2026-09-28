package library_test

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/testutil"
)

func writeJPEG(t *testing.T, root *os.Root, name string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	f, err := root.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func newThumbStore(t *testing.T) (*os.Root, *library.ThumbStore) {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	ts, err := library.NewThumbStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, ts
}

func TestThumbOpenGeneratesScaledJPEG(t *testing.T) {
	root, ts := newThumbStore(t)
	writeJPEG(t, root, "big.jpg", 1200, 800)
	f, err := ts.Open("big.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != library.ThumbEdge || cfg.Height != library.ThumbEdge*800/1200 {
		t.Fatalf("thumb %dx%d", cfg.Width, cfg.Height)
	}
}

func TestThumbSmallImageKeepsSize(t *testing.T) {
	root, ts := newThumbStore(t)
	writeJPEG(t, root, "small.jpg", 100, 200)
	f, err := ts.Open("small.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _ := jpeg.DecodeConfig(f)
	if cfg.Width != 100 || cfg.Height != 200 {
		t.Fatalf("thumb %dx%d", cfg.Width, cfg.Height)
	}
}

func TestThumbMissingSourceErrors(t *testing.T) {
	_, ts := newThumbStore(t)
	if _, err := ts.Open("nope.jpg"); err == nil {
		t.Fatal("expected error")
	}
}

func TestThumbBackfillAndDelete(t *testing.T) {
	root, ts := newThumbStore(t)
	writeJPEG(t, root, "a.jpg", 500, 500)
	writeJPEG(t, root, "b.jpg", 500, 400)
	lib := library.New(imgs("a.jpg", "b.jpg"), false)
	ts.BackfillMissing(context.Background(), lib)
	for _, n := range []string{"a.jpg", "b.jpg"} {
		if _, err := root.Stat(".thumbs/" + n); err != nil {
			t.Fatalf("missing thumb %s: %v", n, err)
		}
	}
	ts.Delete("a.jpg")
	if _, err := root.Stat(".thumbs/a.jpg"); err == nil {
		t.Fatal("thumb not deleted")
	}
	ts.Delete("a.jpg") // idempotent
}
