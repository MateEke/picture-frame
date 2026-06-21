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

// Library maintains the canonical image order (admin source of truth) plus the
// current playback cycle, a shuffled copy when randomized. Safe for concurrent use.
type Library struct {
	mu        sync.Mutex
	images    []Image // canonical order; reordered only by Add/Remove/SetOrder
	cycle     []Image // current playback order; rebuilt by Reshuffle
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

// New creates a Library with the given canonical order and an initial cycle.
func New(images []Image, randomize bool, opts ...Option) *Library {
	l := &Library{randomize: randomize}
	for _, opt := range opts {
		opt(l)
	}
	l.images = make([]Image, len(images))
	copy(l.images, images)
	l.cycle = l.newCycle("")
	return l
}

// Len returns the number of images.
func (l *Library) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.images)
}

// List returns a copy of the canonical order (what the admin UI shows/edits).
func (l *Library) List() []Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	return clone(l.images)
}

// Cycle returns a copy of the current playback order.
func (l *Library) Cycle() []Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	return clone(l.cycle)
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

// Add appends a new image to the canonical order and the current cycle so it
// shows without waiting for a reshuffle.
func (l *Library) Add(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.images = append(l.images, Image{Name: name})
	l.cycle = append(l.cycle, Image{Name: name})
}

// Remove deletes the first image with name from both slices; false if absent
// from the canonical order.
func (l *Library) Remove(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cycle = deleteByName(l.cycle, name)
	for i, img := range l.images {
		if img.Name == name {
			l.images = slices.Delete(l.images, i, i+1)
			return true
		}
	}
	return false
}

// SetOrder reorders the canonical order to match names: unknown names are
// ignored and ones missing from names keep their relative order at the end (so a
// stale payload never drops a file). Returns the resulting names to persist.
func (l *Library) SetOrder(names []string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	pos := make(map[string]int, len(names))
	for i, n := range names {
		if _, ok := pos[n]; !ok {
			pos[n] = i
		}
	}
	slices.SortStableFunc(l.images, func(a, b Image) int {
		ia, oka := pos[a.Name]
		ib, okb := pos[b.Name]
		switch {
		case oka && okb:
			return cmp.Compare(ia, ib)
		case oka:
			return -1
		case okb:
			return 1
		default:
			return 0
		}
	})
	out := make([]string, len(l.images))
	for i, img := range l.images {
		out[i] = img.Name
	}
	return out
}

// SetRandomize toggles random playback and reports whether the value changed.
func (l *Library) SetRandomize(enabled bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.randomize == enabled {
		return false
	}
	l.randomize = enabled
	return true
}

// Randomized reports whether playback is shuffled.
func (l *Library) Randomized() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.randomize
}

// Reshuffle starts a new playback cycle from the canonical order and returns a
// copy. Randomized cycles avoid repeating the previous cycle's last image first.
func (l *Library) Reshuffle() []Image {
	l.mu.Lock()
	defer l.mu.Unlock()
	var prevLast string
	if n := len(l.cycle); n > 0 {
		prevLast = l.cycle[n-1].Name
	}
	l.cycle = l.newCycle(prevLast)
	return clone(l.cycle)
}

// newCycle builds a fresh playback cycle from the canonical order. Caller holds
// the mutex (or is New).
func (l *Library) newCycle(prevLast string) []Image {
	c := clone(l.images)
	if l.randomize && len(c) > 1 {
		l.shuffleSlice(c)
		if prevLast != "" && c[0].Name == prevLast {
			c[0], c[1] = c[1], c[0]
		}
	}
	return c
}

func (l *Library) shuffleSlice(s []Image) {
	doShuffle := rand.Shuffle
	if l.rng != nil {
		doShuffle = l.rng.Shuffle
	}
	doShuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

func clone(s []Image) []Image {
	out := make([]Image, len(s))
	copy(out, s)
	return out
}

func deleteByName(s []Image, name string) []Image {
	for i, img := range s {
		if img.Name == name {
			return slices.Delete(s, i, i+1)
		}
	}
	return s
}
