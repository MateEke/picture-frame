package slideshow

import (
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/slideplan"
)

type librarySource struct {
	lib *library.Library
}

// NewLibrarySource returns a slideplan.Source backed by lib.
func NewLibrarySource(lib *library.Library) slideplan.Source {
	return librarySource{lib: lib}
}

func (s librarySource) Order() []string {
	return imageNames(s.lib.List())
}

func (s librarySource) NextCycle() []string {
	return imageNames(s.lib.Reshuffle())
}

func imageNames(images []library.Image) []string {
	names := make([]string, len(images))
	for i, img := range images {
		names[i] = img.Name
	}
	return names
}
