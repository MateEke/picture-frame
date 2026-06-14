package web

import (
	"embed"
	"encoding/json"
	"io/fs"
)

//go:embed all:build
var BuildFS embed.FS

// BakedVersion reports the build version stamped into the embedded SvelteKit bundle
// (kit.version.name → _app/version.json). Kiosk heartbeats carry this string and the updater
// commits only when it matches version.Version, so a build that bakes a mismatch rolls back
// every update. Returns "" if the file is missing or unparseable.
func BakedVersion() string {
	data, err := fs.ReadFile(BuildFS, "build/_app/version.json")
	if err != nil {
		return ""
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.Version
}
