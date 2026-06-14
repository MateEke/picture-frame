package adapter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MateEke/picture-frame/internal/library"
)

// Load reads all images from dir (sorted by filename) into a Library, creating
// the directory if missing. Unservable names are skipped with a warning, the
// kiosk could never fetch them.
func Load(log *slog.Logger, dir string, randomize bool) (*library.Library, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("library: create images dir %q: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("library: read dir: %w", err)
	}
	var images []library.Image
	for _, e := range entries {
		if e.IsDir() || !looksLikeImage(e.Name()) {
			continue
		}
		if !library.ValidImageName(e.Name()) {
			log.Warn("library: skipping unservable filename (allowed: letters, digits, _.~-, lowercase .jpg/.jpeg/.png)",
				"name", e.Name())
			continue
		}
		images = append(images, library.Image{Name: e.Name()})
	}
	slices.SortFunc(images, func(a, b library.Image) int { return strings.Compare(a.Name, b.Name) })
	return library.New(images, randomize), nil
}

// looksLikeImage gates the skip warning to image-ish files (not e.g. .tmp leftovers).
func looksLikeImage(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}
