// Package providers defines the seam between the core slideshow/cache
// and the remote photo services (Immich now, others later).
//
// Core packages must depend only on the Provider interface declared here,
// never on a concrete provider implementation.
package providers

import (
	"context"
	"io"
)

// Asset is one image in a Provider.
type Asset struct {
	ID      string // stable identity; used as the filename stem
	Version string // opaque change token; a new value means re-download
	Album   string // source album's display name; empty when the provider has none
	Year    int    // year of the album's earliest photo; zero when unknown
	AlbumID string // source album's provider ID; empty when the provider has none
}

// Provider is a read-only view of a photo collection in a remote service.
type Provider interface {
	List(ctx context.Context) ([]Asset, error)
	Fetch(ctx context.Context, id string) (io.ReadCloser, error)
}
