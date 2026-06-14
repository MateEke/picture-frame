package adapter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HTTPDownloader fetches release assets over HTTPS to a local file.
type HTTPDownloader struct {
	token  string // optional; needed to pull assets from a private release
	client *http.Client
}

// NewHTTPDownloader returns a downloader with a generous timeout for the ~15MB tarball.
// An empty token downloads public assets; a token authenticates private ones.
func NewHTTPDownloader(token string) *HTTPDownloader {
	return &HTTPDownloader{token: token, client: newHTTPClient(5 * time.Minute)}
}

// Download GETs url and writes the body to dest.
func (d *HTTPDownloader) Download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// octet-stream makes the GitHub asset API serve the binary, not its JSON. net/http drops
	// Authorization on the cross-host redirect to the asset store, so the token never reaches S3.
	req.Header.Set("Accept", "application/octet-stream")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
