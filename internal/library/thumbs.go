package library

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"sync"

	"golang.org/x/image/draw"
)

// thumbDir holds gallery thumbnails inside the images root. The leading dot
// keeps it out of the fs loader and the syncer.
const thumbDir = ".thumbs"

// ThumbEdge is the long edge of a generated thumbnail in pixels: enough for a
// 3-column grid on a 720px-wide panel at 1x, small enough to decode instantly.
const ThumbEdge = 360

// ThumbStore generates and serves small JPEG previews so the gallery grid never
// decodes full-size photos. Generation is serialized: one decode at a time
// keeps peak memory at a single image. Safe for concurrent use.
type ThumbStore struct {
	log  *slog.Logger
	root *os.Root
	mu   sync.Mutex
}

// NewThumbStore creates the thumbnail directory under root if missing.
func NewThumbStore(log *slog.Logger, root *os.Root) (*ThumbStore, error) {
	if err := root.Mkdir(thumbDir, 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("library: create thumb dir: %w", err)
	}
	return &ThumbStore{log: log, root: root}, nil
}

func thumbPath(name string) string {
	return path.Join(thumbDir, name)
}

// Open returns the thumbnail for name, generating it on first request.
func (t *ThumbStore) Open(name string) (*os.File, error) {
	f, err := t.root.Open(thumbPath(name))
	if err == nil {
		return f, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := t.Generate(name); err != nil {
		return nil, err
	}
	return t.root.Open(thumbPath(name))
}

// Generate writes the thumbnail for name, overwriting any existing one.
func (t *ThumbStore) Generate(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, err := t.root.Stat(thumbPath(name)); err == nil {
		return nil // another caller made it while we waited
	}
	src, err := t.root.Open(name)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(src)
	src.Close()
	if err != nil {
		return fmt.Errorf("library: decode %s: %w", name, err)
	}
	dst := scaleToFit(img, ThumbEdge)
	tmp := thumbPath(name) + ".tmp"
	f, err := t.root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("library: create thumb: %w", err)
	}
	encErr := jpeg.Encode(f, dst, &jpeg.Options{Quality: 80})
	closeErr := f.Close()
	if encErr != nil || closeErr != nil {
		_ = t.root.Remove(tmp)
		return fmt.Errorf("library: write thumb: %w", errors.Join(encErr, closeErr))
	}
	if err := t.root.Rename(tmp, thumbPath(name)); err != nil {
		_ = t.root.Remove(tmp)
		return fmt.Errorf("library: rename thumb: %w", err)
	}
	return nil
}

// Delete removes the thumbnail for name, if any.
func (t *ThumbStore) Delete(name string) {
	if err := t.root.Remove(thumbPath(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.log.Warn("library: remove thumb failed", "name", name, "err", err)
	}
}

// BackfillMissing generates thumbnails for images that lack one, one at a time,
// stopping early when ctx is cancelled. Meant to run in a background goroutine.
func (t *ThumbStore) BackfillMissing(ctx context.Context, lib *Library) {
	for _, img := range lib.List() {
		if ctx.Err() != nil {
			return
		}
		if _, err := t.root.Stat(thumbPath(img.Name)); err == nil {
			continue
		}
		if err := t.Generate(img.Name); err != nil {
			t.log.Warn("library: thumb backfill failed", "name", img.Name, "err", err)
		}
	}
}

// scaleToFit downsizes img so its long edge is at most edge; smaller images are
// returned as-is.
func scaleToFit(img image.Image, edge int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= edge && h <= edge {
		return img
	}
	if w >= h {
		h = max(1, h*edge/w)
		w = edge
	} else {
		w = max(1, w*edge/h)
		h = edge
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
