package slideplan_test

import (
	"testing"

	"github.com/MateEke/picture-frame/internal/slideplan"
)

const landscape = 16.0 / 9.0

func ratios(m map[string]float64) func(string) (float64, bool) {
	return func(n string) (float64, bool) {
		r, ok := m[n]
		return r, ok
	}
}

func names(slides []slideplan.Slide) [][]string {
	out := make([][]string, len(slides))
	for i, s := range slides {
		out[i] = s.Names
	}
	return out
}

func equal(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func TestPlan(t *testing.T) {
	// On a 16:9 screen with Factor 1.5: tall outlier if ratio <= 1.185,
	// wide outlier if ratio >= 2.667, else Fit.
	thr := slideplan.Threshold{Factor: 1.5}
	r := ratios(map[string]float64{
		"L": landscape, "L1": landscape, "L2": landscape, // fit
		"P": 0.66, "P1": 0.66, "P2": 0.66, "P3": 0.66, "P4": 0.66, "P5": 0.66, // tall
		"W1": 3.0, "W2": 3.0, "W3": 3.0, // wide
	})

	cases := []struct {
		name    string
		order   []string
		screen  float64
		enabled bool
		want    [][]string
	}{
		{"empty", nil, landscape, true, nil},
		{"single fit", []string{"L"}, landscape, true, [][]string{{"L"}}},
		{"single portrait shows solo", []string{"P"}, landscape, true, [][]string{{"P"}}},
		{"two portraits pair", []string{"P1", "P2"}, landscape, true, [][]string{{"P1", "P2"}}},
		{"three portraits reuse earlier", []string{"P1", "P2", "P3"}, landscape, true, [][]string{{"P1", "P2"}, {"P2", "P3"}}},
		{"five portraits reuse last paired", []string{"P1", "P2", "P3", "P4", "P5"}, landscape, true, [][]string{{"P1", "P2"}, {"P3", "P4"}, {"P4", "P5"}}},
		{"all fit stay solo", []string{"L1", "L2"}, landscape, true, [][]string{{"L1"}, {"L2"}}},
		{"fit interleaved with pair", []string{"P1", "L1", "P2"}, landscape, true, [][]string{{"L1"}, {"P1", "P2"}}},
		{"portrait and wide use independent queues", []string{"P1", "W1", "P2", "W2"}, landscape, true, [][]string{{"P1", "P2"}, {"W1", "W2"}}},
		{"three wide outliers reuse earlier", []string{"W1", "W2", "W3"}, landscape, true, [][]string{{"W1", "W2"}, {"W2", "W3"}}},
		{"unknown ratio treated as fit", []string{"U"}, landscape, true, [][]string{{"U"}}},
		{"zero screen aspect disables pairing", []string{"P1", "P2"}, 0, true, [][]string{{"P1"}, {"P2"}}},
		{"disabled forces solo", []string{"P1", "P2"}, landscape, false, [][]string{{"P1"}, {"P2"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := names(slideplan.Plan(tc.order, tc.screen, r, thr, tc.enabled))
			if !equal(got, tc.want) {
				t.Errorf("Plan(%v) = %v, want %v", tc.order, got, tc.want)
			}
		})
	}
}
