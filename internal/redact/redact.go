// Package redact scrubs sensitive details from strings shown in the admin UI.
package redact

import "regexp"

var pathRe = regexp.MustCompile(`/(?:home|var|etc|tmp|opt|usr|root|mnt|run|srv)\S*`)

// Path replaces absolute filesystem paths with "<path>" and caps the length, so
// server-side error strings don't leak paths (or grow unbounded) through the UI.
func Path(s string) string {
	s = pathRe.ReplaceAllString(s, "<path>")
	const maxLen = 160
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}
