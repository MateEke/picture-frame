package adapter

import (
	"net/http"
	"time"
)

// newHTTPClient returns a client tolerant of a slow TLS handshake: a Pi Zero W right after boot
// can exceed the default 10s, failing the updater's first check with "TLS handshake timeout".
func newHTTPClient(timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = 30 * time.Second
	return &http.Client{Timeout: timeout, Transport: tr}
}
