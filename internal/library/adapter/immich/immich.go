// Package immich implements library.RemoteAlbum against an Immich shared link.
package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MateEke/picture-frame/internal/library"
)

// httpError lets callers branch on HTTP status without parsing strings.
type httpError struct{ Status int }

func (e *httpError) Error() string { return fmt.Sprintf("status %d", e.Status) }

const noThumbhashToken = "0000000000000000"

// thumbhashToken derives a fixed-length, filesystem-safe change token. thumbhash
// tracks rendered content, so the token moves on edits and is stable otherwise.
func thumbhashToken(thumbhash string) string {
	if thumbhash == "" {
		return noThumbhashToken
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(thumbhash))
	return fmt.Sprintf("%016x", h.Sum64())
}

const (
	defaultTimeout  = 30 * time.Second
	thumbnailSize   = "preview"
	sharePathPrefix = "/share/"
	// sharedLinkCookieName carries the password-exchange token on Immich >= 2.6.0.
	sharedLinkCookieName = "immich_shared_link_token"
)

// Client fetches an Immich album over a shared link: share key plus an optional
// password. Immich >= 2.6.0 (GHSA-78x4-6x83-jx75) offers a login/token-cookie
// exchange, preferred here because it keeps the password out of URLs and access
// logs; servers without the login endpoint get it as a query parameter instead.
// Fields are mutated only from the syncer goroutine.
type Client struct {
	base     string // e.g. "https://immich.example.com"
	key      string
	password string
	http     *http.Client
	log      *slog.Logger

	albumID        string // cached after first List
	token          string // immich_shared_link_token cookie value; set by login
	legacyPassword bool   // pre-2.6 server: send the password as a query parameter

	cached    []library.Asset // last enumeration; returned when the album ETag is unchanged
	albumETag string          // album response ETag; gates re-enumeration
	loaded    bool            // an enumeration has succeeded at least once
}

// Config configures the client.
type Config struct {
	ShareURL string // full share URL, e.g. https://host/share/<key>
	Password string
	HTTP     *http.Client // optional override (defaults to a 30s-timeout client)
	Logger   *slog.Logger // optional; defaults to a discard logger
}

// New parses cfg.ShareURL and returns a ready-to-use Client.
func New(cfg Config) (*Client, error) {
	base, key, err := parseShareURL(cfg.ShareURL)
	if err != nil {
		return nil, err
	}
	c := &Client{base: base, key: key, password: cfg.Password, http: cfg.HTTP, log: cfg.Logger}
	if c.http == nil {
		c.http = &http.Client{Timeout: defaultTimeout}
	}
	if c.log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}
	return c, nil
}

// List returns the album's image assets. Retries once on 404 (stale album ID)
// or 401 (stale token, e.g. the share password changed).
func (c *Client) List(ctx context.Context) ([]library.Asset, error) {
	out, err := c.listOnce(ctx)
	if err == nil {
		return out, nil
	}
	var hErr *httpError
	if !errors.As(err, &hErr) {
		return nil, err
	}
	switch hErr.Status {
	case http.StatusNotFound:
		c.albumID = ""
		c.albumETag = ""
	case http.StatusUnauthorized:
		c.token = ""
	default:
		return nil, err
	}
	return c.listOnce(ctx)
}

// ensureAuth exchanges the share password for the token cookie (Immich >= 2.6.0);
// a 404 marks the server legacy, falling back to the password query parameter.
func (c *Client) ensureAuth(ctx context.Context) error {
	if c.password == "" || c.token != "" || c.legacyPassword {
		return nil
	}
	body, err := json.Marshal(map[string]string{"password": c.password})
	if err != nil {
		return err
	}
	u := c.base + "/api/shared-links/login?" + url.Values{"key": {c.key}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("immich: shared-link login: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		for _, ck := range resp.Cookies() {
			if ck.Name == sharedLinkCookieName {
				c.token = ck.Value
				return nil
			}
		}
		return fmt.Errorf("immich: shared-link login: no %s cookie in response", sharedLinkCookieName)
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		c.legacyPassword = true
		return nil
	default:
		return fmt.Errorf("immich: shared-link login: %w", &httpError{Status: resp.StatusCode})
	}
}

func (c *Client) listOnce(ctx context.Context) ([]library.Asset, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	albumID, err := c.resolveAlbumID(ctx)
	if err != nil {
		return nil, err
	}
	changed, etag, err := c.albumChanged(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if !changed {
		c.log.Debug("immich: album unchanged, served from cache", "assets", len(c.cached))
		return c.cached, nil
	}
	assets, buckets, err := c.enumerate(ctx, albumID)
	if err != nil {
		return nil, err
	}
	c.cached, c.albumETag, c.loaded = assets, etag, true
	c.log.Debug("immich: enumerated album", "assets", len(assets), "buckets", buckets)
	return assets, nil
}

// albumChanged conditional-GETs the album: a 304 reuses the cache, a 200 means
// re-enumerate. It returns the new ETag rather than storing it, so listOnce can
// commit it only with a successful enumeration (else a failed enumeration would
// leave a stale cache behind an advanced ETag).
func (c *Client) albumChanged(ctx context.Context, albumID string) (changed bool, etag string, err error) {
	forceFull := !c.loaded || c.albumETag == ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/api/albums/"+albumID, nil), nil)
	if err != nil {
		return false, "", err
	}
	c.addAuthCookie(req)
	if !forceFull {
		req.Header.Set("If-None-Match", c.albumETag)
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

// enumerate lists the album's image assets via the timeline API (the only
// key-authenticated listing after Immich v3 dropped AlbumResponseDto.assets) and
// returns the number of buckets it fetched.
func (c *Client) enumerate(ctx context.Context, albumID string) ([]library.Asset, int, error) {
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

type timelineBucketMeta struct {
	TimeBucket string `json:"timeBucket"`
}

// timelineBucket is the columnar (struct-of-arrays) shape Immich returns per bucket.
type timelineBucket struct {
	ID        []string `json:"id"`
	IsImage   []bool   `json:"isImage"`
	Thumbhash []string `json:"thumbhash"`
}

// appendImageAssets maps the columnar payload to image assets, tolerating short
// or absent optional arrays.
func appendImageAssets(out []library.Asset, b timelineBucket) []library.Asset {
	for i, id := range b.ID {
		if id == "" {
			continue
		}
		if i < len(b.IsImage) && !b.IsImage[i] {
			continue
		}
		th := ""
		if i < len(b.Thumbhash) {
			th = b.Thumbhash[i]
		}
		out = append(out, library.Asset{ID: id, Version: thumbhashToken(th)})
	}
	return out
}

// Fetch returns the preview-sized thumbnail stream for assetID.
func (c *Client) Fetch(ctx context.Context, assetID string) (io.ReadCloser, error) {
	u := c.url("/api/assets/"+assetID+"/thumbnail", url.Values{"size": {thumbnailSize}})
	resp, err := c.do(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("immich: fetch %s: %w", assetID, err)
	}
	return resp.Body, nil
}

func (c *Client) resolveAlbumID(ctx context.Context) (string, error) {
	if c.albumID != "" {
		return c.albumID, nil
	}
	var share struct {
		Album struct {
			ID string `json:"id"`
		} `json:"album"`
	}
	if err := c.getJSON(ctx, "/api/shared-links/me", nil, &share); err != nil {
		return "", fmt.Errorf("immich: resolve share: %w", err)
	}
	if share.Album.ID == "" {
		return "", fmt.Errorf("immich: share is not an album")
	}
	c.albumID = share.Album.ID
	return c.albumID, nil
}

func (c *Client) getJSON(ctx context.Context, path string, extra url.Values, dst any) error {
	resp, err := c.do(ctx, c.url(path, extra))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(dst)
}

// addAuthCookie attaches the shared-link token via the Cookie header. Response
// attributes (Secure/HttpOnly) don't apply to an outgoing cookie.
func (c *Client) addAuthCookie(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Cookie", sharedLinkCookieName+"="+c.token)
	}
}

func (c *Client) do(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.addAuthCookie(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &httpError{Status: resp.StatusCode}
	}
	return resp, nil
}

func (c *Client) url(path string, extra url.Values) string {
	q := url.Values{}
	q.Set("key", c.key)
	// Only when the server has no login endpoint; the query form lands in access logs.
	if c.legacyPassword && c.password != "" {
		q.Set("password", c.password)
	}
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	return c.base + path + "?" + q.Encode()
}

// parseShareURL splits the URL into base ("https://host") and key (path tail).
func parseShareURL(shareURL string) (base, key string, err error) {
	u, err := url.Parse(shareURL)
	if err != nil {
		return "", "", fmt.Errorf("immich: parse share url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", "", fmt.Errorf("immich: share url must be http(s), got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("immich: share url missing host")
	}
	if !strings.HasPrefix(u.Path, sharePathPrefix) {
		return "", "", fmt.Errorf("immich: share url path must start with %s", sharePathPrefix)
	}
	key = strings.TrimPrefix(u.Path, sharePathPrefix)
	if key == "" {
		return "", "", fmt.Errorf("immich: share url missing key")
	}
	base = u.Scheme + "://" + u.Host
	return base, key, nil
}
