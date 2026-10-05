package library

import (
	"github.com/MateEke/picture-frame/providers"
)

// Asset is one image in a RemoteAlbum.
// Canonical definition lives in providers; this alias keeps existing
// call sites compiling while the core migrates to the Provider seam.
type Asset = providers.Asset

// RemoteAlbum is a read-only view of an album in a remote photo service.
// Canonical interface is providers.Provider; this alias preserves the old
// name until call sites switch over.
type RemoteAlbum = providers.Provider
