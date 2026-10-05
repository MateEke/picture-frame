// Package renderer defines the seam between the slideshow core and the
// display pipeline: the core decides WHAT to show (a Slide), a Renderer
// decides HOW to show it on a specific panel.
//
// Current implementation (web): slideshow.publish puts a state.ImagePayload
// on the bus, httpapi fans it out over SSE (/events) and serves the files
// over /img/, and the kiosk (web/src/routes/kiosk) renders them with a
// crossfading Fader. That pipeline satisfies this contract with Slide.Names
// = payload Names and Slide.Next = payload Next.
//
// Future implementations (framebuffer/DRM, DSI/MIPI panels) implement
// Renderer without touching slideshow, cache, or providers.
package renderer

// Slide is one displayable unit: a solo image (len(Names) == 1) or a
// side-by-side split pair (len == 2). Next is a best-effort preload hint
// for the following slide and may be nil.
type Slide struct {
	Names []string
	Next  []string
}

// Renderer presents slides on a display. Show must return once the slide is
// queued or visible; implementations handle their own transitions and
// preloading. It must be safe for concurrent use.
type Renderer interface {
	Show(slide Slide) error
}
