package library

import (
	"cmp"
	"math/rand/v2"
	"regexp"
	"slices"
	"sync"
)

// ImageNamePattern is the canonical image filename rule; the HTTP routes embed
// it as a huma `pattern` tag (a test keeps them in sync).
const ImageNamePattern = `^[a-zA-Z0-9_.~-]+\.(jpe?g|png)$`

var imageNameRe = regexp.MustCompile(ImageNamePattern)

// ValidImageName reports whether name is servable by the /img/{name} route.
func ValidImageName(name string) bool {
	return imageNameRe.MatchString(name)
}

// Image represents a stored image file.
type Image struct {
	Name string // filename only, e.g. "1716038400000.jpg"
}

// Library maintains an ordered list of images and a current playback position.
// It is safe for concurrent use.
type Library struct {
	mu        sync.Mutex
	images    []Image
	idx       int
	randomize bool
	rng       *rand.Rand // nil shuffles via the global PRNG; tests inject a seeded one
}

// Option defines a functional configuration for the Library.
type Option func(*Library)

// WithTestRNG injects a deterministic random number generator for unit tests.
func WithTestRNG(rng *rand.Rand) Option {
	return func(l *Library) {
		l.rng = rng
	}
}

// New creates a Library pre-populated with the given images.
func New(images []Image, randomize bool, opts ...Option) *Library {
	l := &Library{
		randomize: randomize,
	}
	for _, opt := range opts {
		opt(l)
	}
	if len(images) > 0 {
		l.images = make([]Image, len(images))
		copy(l.images, images)
		if l.randomize {
			l.shuffle()
		}
	}
	return l
}

// Len returns the number of images.
func (l *Library) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.images)
}

// List returns a copy of all images in order.
func (l *Library) List() []Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Image, len(l.images))
	copy(out, l.images)
	return out
}

// Current returns the image at the current position, or nil if empty.
func (l *Library) Current() *Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.images) == 0 {
		return nil
	}
	img := l.images[l.idx]
	return &img
}

// Next advances to the next image (wrapping) and returns it, or nil if empty.
func (l *Library) Next() *Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.images) == 0 {
		return nil
	}

	prev := l.images[l.idx]
	l.idx++

	if l.idx >= len(l.images) {
		l.idx = 0
		if l.randomize {
			l.shuffle()
			// Avoid repeating the last image of the previous cycle.
			if len(l.images) > 1 && l.images[0].Name == prev.Name {
				l.images[0], l.images[1] = l.images[1], l.images[0]
			}
		}
	}
	img := l.images[l.idx]
	return &img
}

// Has reports whether an image with the given name is present.
func (l *Library) Has(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, img := range l.images {
		if img.Name == name {
			return true
		}
	}
	return false
}

// Add appends a new image to the end of the library.
func (l *Library) Add(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.images = append(l.images, Image{Name: name})
}

// Remove deletes the first image with the given name (false if absent), adjusting
// the current index so Current keeps pointing at the same image.
func (l *Library) Remove(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, img := range l.images {
		if img.Name != name {
			continue
		}
		l.images = slices.Delete(l.images, i, i+1)
		// Removed an image strictly before the current one: shift idx down so
		// Current() still returns the same image.
		if i < l.idx {
			l.idx--
		}
		// Clamp in case we removed the current (or only) image at the tail.
		if len(l.images) > 0 && l.idx >= len(l.images) {
			l.idx = len(l.images) - 1
		}
		return true
	}
	return false
}

// SetRandomize enables or disables random ordering. The slideshow restarts
// from the beginning on the next advance: disabling sorts images alphabetically
// first, enabling leaves the shuffle to the natural wrap in Next(). idx is
// reset to the last position so the next Next() call wraps cleanly to 0.
func (l *Library) SetRandomize(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.randomize == enabled {
		return
	}
	l.randomize = enabled
	if len(l.images) <= 1 {
		return
	}
	if !enabled {
		slices.SortFunc(l.images, func(a, b Image) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}
	l.idx = len(l.images) - 1
}

// Assumes the caller already holds the mutex.
func (l *Library) shuffle() {
	doShuffle := rand.Shuffle

	if l.rng != nil {
		doShuffle = l.rng.Shuffle
	}

	doShuffle(len(l.images), func(i, j int) {
		l.images[i], l.images[j] = l.images[j], l.images[i]
	})
}
