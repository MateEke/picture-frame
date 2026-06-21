package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register decoders for DecodeConfig
	_ "image/jpeg" // register decoders for DecodeConfig
	_ "image/png"  // register decoders for DecodeConfig
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"
)

// The leading dot keeps this out of the fs loader, syncedNameRe, and cleanTmp.
const aspectIndexName = ".aspect-index.json"

type meta struct {
	W int `json:"w"`
	H int `json:"h"`
}

// AspectStore caches per-image aspect ratios as a JSON sidecar in the images
// directory, captured once at upload/sync so reads never decode. Safe for
// concurrent use.
type AspectStore struct {
	log     *slog.Logger
	root    *os.Root
	mu      sync.Mutex
	byName  map[string]meta
	flushMu sync.Mutex // serializes Flush's file I/O against the shared tmp file
}

// LoadAspectStore reads the sidecar index from root (empty when absent).
func LoadAspectStore(log *slog.Logger, root *os.Root) (*AspectStore, error) {
	s := &AspectStore{log: log, root: root, byName: make(map[string]meta)}
	f, err := root.OpenFile(aspectIndexName, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("library: open aspect index: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("library: read aspect index: %w", err)
	}
	if err := json.Unmarshal(data, &s.byName); err != nil {
		return nil, fmt.Errorf("library: parse aspect index: %w", err)
	}
	return s, nil
}

// Ratio returns the cached width/height for name; ok is false when the name is
// absent or has no usable height.
func (s *AspectStore) Ratio(name string) (ratio float64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, present := s.byName[name]
	if !present || m.H <= 0 {
		return 0, false
	}
	return float64(m.W) / float64(m.H), true
}

// Set records dimensions for name in memory; call Flush to persist.
func (s *AspectStore) Set(name string, w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byName[name] = meta{W: w, H: h}
}

// Delete drops name in memory; call Flush to persist.
func (s *AspectStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byName, name)
}

// Missing returns the subset of names with no cached dimensions.
func (s *AspectStore) Missing(names []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, n := range names {
		if _, ok := s.byName[n]; !ok {
			out = append(out, n)
		}
	}
	return out
}

// BackfillMissing decodes and stores dimensions for any library image absent from
// the index, then persists once. Meant to run in the background at startup. It
// reports whether it recorded anything, so the caller can rebuild a plan that was
// built before the ratios were available.
func (s *AspectStore) BackfillMissing(ctx context.Context, lib *Library) bool {
	var names []string
	for _, img := range lib.List() {
		names = append(names, img.Name)
	}
	decoded := 0
	for _, name := range s.Missing(names) {
		if ctx.Err() != nil {
			break
		}
		if w, h := ImageDimensions(s.root, name); w > 0 && h > 0 {
			s.Set(name, w, h)
			decoded++
		}
	}
	if decoded == 0 {
		return false
	}
	if err := s.Flush(); err != nil {
		s.log.Warn("library: flush aspect backfill failed", "err", err)
	}
	s.log.Info("library: aspect backfill complete", "decoded", decoded)
	return true
}

// ImageDimensions reads an image's pixel size from its header only (no full
// decode); returns 0,0 on any error.
func ImageDimensions(root *os.Root, name string) (w, h int) {
	f, err := root.Open(name)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// Flush writes the index atomically (tmp + rename) through the images root.
func (s *AspectStore) Flush() error {
	s.mu.Lock()
	data, err := json.Marshal(s.byName)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("library: marshal aspect index: %w", err)
	}

	return writeFileAtomic(s.root, aspectIndexName, &s.flushMu, data)
}
