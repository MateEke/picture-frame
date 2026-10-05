package immich

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/providers"
)

// apiKeyHeader carries the full-access Immich API key on every request.
const apiKeyHeader = "x-api-key"

// APIConfig configures the API-key client: a server base URL, a full-access
// API key (created in the Immich UI), and the albums to display.
type APIConfig struct {
	BaseURL  string // e.g. "https://immich.example.com" (no path)
	APIKey   string
	AlbumIDs []string
	HTTP     *http.Client // optional override (defaults to a 30s-timeout client)
	Logger   *slog.Logger // optional; defaults to a discard logger
}

// albumState is one album's cached enumeration with its ETag gate.
type albumState struct {
	id     string
	assets []library.Asset
	etag   string // album response ETag; gates re-enumeration
	loaded bool   // an enumeration has succeeded at least once
}

// APIClient fetches Immich albums using an API key. Unlike Client
// (shared-link), the key is static, so there is no token exchange and a 401
// is returned as-is instead of retried.
type APIClient struct {
	base   string
	apiKey string
	http   *http.Client
	log    *slog.Logger

	// One entry per configured album, in config order; merged on List.
	albums []albumState
}

var _ providers.Provider = (*APIClient)(nil)

// NewAPIClient validates cfg and returns a ready-to-use APIClient.
func NewAPIClient(cfg APIConfig) (*APIClient, error) {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("immich: invalid base url %q: must be http(s)://host", cfg.BaseURL)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("immich: api key required")
	}
	if len(cfg.AlbumIDs) == 0 {
		return nil, fmt.Errorf("immich: at least one album id required")
	}
	for _, id := range cfg.AlbumIDs {
		if id == "" {
			return nil, fmt.Errorf("immich: album id must not be empty")
		}
	}
	c := &APIClient{base: base, apiKey: cfg.APIKey, http: cfg.HTTP, log: cfg.Logger}
	for _, id := range cfg.AlbumIDs {
		c.albums = append(c.albums, albumState{id: id})
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: defaultTimeout}
	}
	if c.log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}
	return c, nil
}

// List returns the albums' image assets merged in config order (album order,
// then timeline order within each album), deduplicated by asset ID: a photo in
// several albums downloads once. Each album re-enumerates only when its ETag
// moved (same conditional-GET gate as the shared-link client); a failed album
// fails the whole List and the syncer keeps its cache for a backoff retry.
func (c *APIClient) List(ctx context.Context) ([]library.Asset, error) {
	buckets, reenumerated := 0, 0
	for i := range c.albums {
		a := &c.albums[i]
		changed, etag, err := c.albumChanged(ctx, a)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		assets, n, err := c.enumerate(ctx, a.id)
		if err != nil {
			return nil, err
		}
		a.assets, a.etag, a.loaded = assets, etag, true
		buckets += n
		reenumerated++
	}
	out := mergeAssets(c.albums)
	c.log.Debug("immich: listed albums", "albums", len(c.albums), "assets", len(out), "re-enumerated", reenumerated, "buckets", buckets)
	return out, nil
}

// mergeAssets concatenates per-album assets in order, dropping duplicate IDs
// (first album wins). An asset shared by several albums has the same Version
// everywhere, so any copy is equivalent.
func mergeAssets(albums []albumState) []library.Asset {
	seen := make(map[string]bool)
	var out []library.Asset
	for _, a := range albums {
		for _, asset := range a.assets {
			if seen[asset.ID] {
				continue
			}
			seen[asset.ID] = true
			out = append(out, asset)
		}
	}
	return out
}

// albumChanged conditional-GETs one album: a 304 reuses the cache, a 200 means
// re-enumerate. The new ETag is committed only with a successful enumeration.
func (c *APIClient) albumChanged(ctx context.Context, a *albumState) (changed bool, etag string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/albums/"+a.id, nil)
	if err != nil {
		return false, "", err
	}
	c.auth(req)
	if a.loaded && a.etag != "" {
		req.Header.Set("If-None-Match", a.etag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("immich: album gate: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return false, "", nil
	case http.StatusOK:
		return true, resp.Header.Get("ETag"), nil
	default:
		return false, "", &httpError{Status: resp.StatusCode}
	}
}

// enumerate lists one album's image assets via the timeline API, the same
// listing the shared-link client uses, authenticated by API key.
func (c *APIClient) enumerate(ctx context.Context, albumID string) ([]library.Asset, int, error) {
	var buckets []timelineBucketMeta
	if err := c.getJSON(ctx, "/api/timeline/buckets", url.Values{"albumId": {albumID}}, &buckets); err != nil {
		return nil, 0, fmt.Errorf("immich: list buckets: %w", err)
	}
	var out []library.Asset
	for _, b := range buckets {
		var bucket timelineBucket
		q := url.Values{"albumId": {albumID}, "timeBucket": {b.TimeBucket}}
		if err := c.getJSON(ctx, "/api/timeline/bucket", q, &bucket); err != nil {
			return nil, 0, fmt.Errorf("immich: bucket %s: %w", b.TimeBucket, err)
		}
		out = appendImageAssets(out, bucket)
	}
	return out, len(buckets), nil
}

// Fetch returns the preview-sized thumbnail stream for assetID.
func (c *APIClient) Fetch(ctx context.Context, assetID string) (io.ReadCloser, error) {
	u := c.base + "/api/assets/" + assetID + "/thumbnail?" + url.Values{"size": {thumbnailSize}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("immich: fetch %s: %w", assetID, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &httpError{Status: resp.StatusCode}
	}
	return resp.Body, nil
}

func (c *APIClient) getJSON(ctx context.Context, path string, q url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &httpError{Status: resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// auth attaches the API key. The key never goes into URLs or logs.
func (c *APIClient) auth(req *http.Request) {
	req.Header.Set(apiKeyHeader, c.apiKey)
}
