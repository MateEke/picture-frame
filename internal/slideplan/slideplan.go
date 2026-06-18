// Package slideplan groups an ordered image list into slides: solo images that
// crop nicely under object-cover, and side-by-side pairs of same-orientation
// outliers whose aspect is too far from the screen to crop well.
package slideplan

type Slide struct {
	Names []string // len 1 (solo) or 2 (paired)
}

// Threshold.Factor is the aspect deviation at which an image becomes an outlier:
// it pairs when its ratio differs from the screen's by >= Factor or <= 1/Factor.
type Threshold struct {
	Factor float64
}

type orientation int

const (
	fit orientation = iota
	tall
	wide
)

func classify(ratio, screen float64, thr Threshold) orientation {
	if screen <= 0 {
		return fit
	}
	dev := ratio / screen
	switch {
	case dev >= thr.Factor:
		return wide
	case dev <= 1/thr.Factor:
		return tall
	default:
		return fit
	}
}

// Plan groups order into slides. Outliers of the same orientation pair in order;
// a leftover re-pairs with the previous same-orientation outlier, except a lone
// outlier of its kind, which shows solo. Disabled or unknown aspect = all solo.
func Plan(order []string, screen float64, ratioOf func(string) (float64, bool), thr Threshold, enabled bool) []Slide {
	var slides []Slide
	if !enabled {
		for _, name := range order {
			slides = append(slides, Slide{Names: []string{name}})
		}
		return slides
	}

	var pending [3]string    // held outlier per orientation, "" if none
	var lastPaired [3]string // most recent outlier consumed into a pair, per orientation

	for _, name := range order {
		ratio, known := ratioOf(name)
		o := fit
		if known {
			o = classify(ratio, screen, thr)
		}
		if o == fit {
			slides = append(slides, Slide{Names: []string{name}})
			continue
		}
		if held := pending[o]; held != "" {
			slides = append(slides, Slide{Names: []string{held, name}})
			pending[o] = ""
			lastPaired[o] = name
			continue
		}
		pending[o] = name
	}

	for o := tall; o <= wide; o++ {
		leftover := pending[o]
		if leftover == "" {
			continue
		}
		if prev := lastPaired[o]; prev != "" {
			slides = append(slides, Slide{Names: []string{prev, leftover}})
		} else {
			slides = append(slides, Slide{Names: []string{leftover}})
		}
	}

	return slides
}
