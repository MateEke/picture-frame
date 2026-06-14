package httpapi

import (
	"net/http"

	"github.com/MateEke/picture-frame/internal/wifi"
)

var captiveProbes = []string{
	"/generate_204",        // Android / Chrome
	"/hotspot-detect.html", // iOS / macOS
	"/ncsi.txt",            // Windows
	"/success.txt",         // Firefox
}

const captiveRedirectHTML = `<!DOCTYPE html>
<html>
<head><meta http-equiv="refresh" content="0;url=/admin/wifi"></head>
<body><a href="/admin/wifi">Open WiFi setup</a></body>
</html>`

func (s *server) handleCaptiveProbe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if s.wifiMgr != nil && s.wifiMgr.Status().Mode == wifi.ModeAP {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(captiveRedirectHTML))
		return
	}
	// Non-AP mode: return expected responses so the OS doesn't flag a portal.
	switch path {
	case "/generate_204":
		w.WriteHeader(http.StatusNoContent)
	case "/hotspot-detect.html":
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"))
	case "/ncsi.txt":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Microsoft NCSI"))
	case "/success.txt":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("success\n"))
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
