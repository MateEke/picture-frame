package library

import (
	"context"
	"io"
)

// Asset is one image in a RemoteAlbum.
type Asset struct {
	ID      string // stable identity; used as the filename stem
	Version string // opaque change token; a new value means re-download
}

// RemoteAlbum is a read-only view of an album in a remote photo service.
type RemoteAlbum interface {
	List(ctx context.Context) ([]Asset, error)
	Fetch(ctx context.Context, id string) (io.ReadCloser, error)
}
