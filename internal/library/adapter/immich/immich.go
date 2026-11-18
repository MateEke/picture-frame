// Package immich implements providers.Provider against an Immich shared link.
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
	"github.com/MateEke/picture-frame/providers"
	"strconv"
)

// StatusError is the surface callers use to branch on the HTTP status Immich
// returned without parsing an error string.
type StatusError interface {
	error
	HTTPStatus() int
}

// httpError carries the Immich status plus a short slice of the response body.
// The body is what turns an opaque "status 400" into an actionable admin error.
type httpError struct {
	Status int
	Detail string
}

func (e *httpError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("status %d: %s", e.Status, e.Detail)
	}
	return fmt.Sprintf("status %d", e.Status)
}

func (e *httpError) HTTPStatus() int { return e.Status }

// maxErrorDetail caps how much of an Immich error body reaches logs and the UI.
const maxErrorDetail = 240

// statusError reads a bounded, single-line excerpt of an error response. It is
// called before the body is closed; the caller remains responsible for closing.
func statusError(resp *http.Response) *httpError {
	detail := ""
	if resp.Body != nil {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorDetail))
		detail = strings.Join(strings.Fields(string(b)), " ")
	}
	return &httpError{Status: resp.StatusCode, Detail: detail}
}

// bucketParam returns the timeBucket value Immich expects. Immich 2 accepted a
// bare YYYY-MM-DD; Immich 3 requires a full ISO-8601 timestamp. getTimelineBucket
// tries the other form on a 400, so one client works with both server versions.
func bucketParam(bucket string) string {
	if len(bucket) == 10 && bucket[4] == '-' && bucket[7] == '-' {
		return bucket + "T00:00:00.000Z"
	}
	return bucket
}

// legacyBucketParam is the inverse of bucketParam: it recovers the bare date
// even when Immich reported the bucket in ISO form.
func legacyBucketParam(bucket string) string {
	return strings.TrimSuffix(bucket, "T00:00:00.000Z")
}

type jsonGetter func(ctx context.Context, path string, q url.Values, dst any) error

// getTimelineBucket fetches one bucket. Immich versions disagree on the
// timeBucket format: /api/timeline/buckets reports bare dates on v3, while
// /api/timeline/bucket wants a full ISO timestamp (and older servers the bare
// date). Try both forms on a 400 instead of assuming which one Immich returned.
func getTimelineBucket(ctx context.Context, get jsonGetter, albumID, bucket string, dst any) error {
	iso := bucketParam(bucket)
	legacy := legacyBucketParam(bucket)
	first := url.Values{"albumId": {albumID}, "timeBucket": {iso}}
	err := get(ctx, "/api/timeline/bucket", first, dst)
	var httpErr *httpError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || legacy == iso {
		return err
	}
	return get(ctx, "/api/timeline/bucket", url.Values{"albumId": {albumID}, "timeBucket": {legacy}}, dst)
}

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
	slugPathPrefix  = "/s/"
	// sharedLinkCookieName carries the password-exchange token on Immich >= 2.6.0.
	sharedLinkCookieName = "immich_shared_link_token"
)

// shareRef addresses a shared link, by generated key (/share/<key>) or by the
// custom slug Immich >= 1.137 offers (/s/<slug>). Every shared-link endpoint
// takes either as a query parameter, but they are separate namespaces: a slug
// sent as key, or the reverse, is rejected with 401.
type shareRef struct {
	param string // "key" or "slug"
	value string
}

func (r shareRef) values() url.Values { return url.Values{r.param: {r.value}} }

// Client fetches an Immich album over a shared link: a share reference plus an
// optional password. Immich >= 2.6.0 (GHSA-78x4-6x83-jx75) offers a login/token
// cookie exchange, preferred here because it keeps the password out of URLs and
// access logs; servers without the login endpoint get it as a query parameter.
// Fields are mutated only from the syncer goroutine.
var _ providers.Provider = (*Client)(nil)

type Client struct {
	base     string // e.g. "https://immich.example.com"
	ref      shareRef
	password string
	http     *http.Client
	log      *slog.Logger

	albumID        string // cached after first List
	albumName      string // display name, resolved alongside albumID
	token          string // immich_shared_link_token cookie value; set by login
	legacyPassword bool   // pre-2.6 server: send the password as a query parameter

	cached    []library.Asset // last enumeration; returned when the album ETag is unchanged
	albumETag string          // album response ETag; gates re-enumeration
	loaded    bool            // an enumeration has succeeded at least once
}

// Config configures the client.
type Config struct {
	ShareURL string // full share URL: https://host/share/<key> or https://host/s/<slug>
	Password string
	HTTP     *http.Client // optional override (defaults to a 30s-timeout client)
	Logger   *slog.Logger // optional; defaults to a discard logger
}

// New parses cfg.ShareURL and returns a ready-to-use Client.
func New(cfg Config) (*Client, error) {
	base, ref, err := parseShareURL(cfg.ShareURL)
	if err != nil {
		return nil, err
	}
	c := &Client{base: base, ref: ref, password: cfg.Password, http: cfg.HTTP, log: cfg.Logger}
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
	u := c.base + "/api/shared-links/login?" + c.ref.values().Encode()
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
		return fmt.Errorf("immich: shared-link login: %w", statusError(resp))
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
	changed, year, etag, err := c.albumChanged(ctx, albumID)
	if err != nil {
		return nil, err
	}
	if !changed {
		c.log.Debug("immich: album unchanged, served from cache", "assets", len(c.cached))
		return c.cached, nil
	}
	assets, buckets, err := c.enumerate(ctx, albumID, year)
	if err != nil {
		return nil, err
	}
	c.cached, c.albumETag, c.loaded = assets, etag, true
	c.log.Debug("immich: enumerated album", "assets", len(assets), "buckets", buckets)
	return assets, nil
}

// albumChanged conditional-GETs the album: a 304 reuses the cache, a 200 means
// re-enumerate. The 200 body also carries startDate (the earliest asset's
// local taken date), so the gate is also where the album year is learned for
// free. It returns the new ETag rather than storing it, so listOnce can
// commit it only with a successful enumeration (else a failed enumeration would
// leave a stale cache behind an advanced ETag).
func (c *Client) albumChanged(ctx context.Context, albumID string) (changed bool, year int, etag string, err error) {
	forceFull := !c.loaded || c.albumETag == ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/api/albums/"+albumID, nil), nil)
	if err != nil {
		return false, 0, "", err
	}
	c.addAuthCookie(req)
	if !forceFull {
		req.Header.Set("If-None-Match", c.albumETag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, 0, "", fmt.Errorf("immich: album gate: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return false, 0, "", nil
	case http.StatusOK:
		var album Album
		if err := json.NewDecoder(resp.Body).Decode(&album); err != nil {
			return false, 0, "", fmt.Errorf("immich: decode album %s: %w", albumID, err)
		}
		return true, yearFromStartDate(album.StartDate), resp.Header.Get("ETag"), nil
	default:
		return false, 0, "", statusError(resp)
	}
}

// enumerate lists the album's image assets via the timeline API (the only
// key-authenticated listing after Immich v3 dropped AlbumResponseDto.assets) and
// returns the number of buckets it fetched. Every asset carries the album name
// and the taken year shared by the album: preferYear (the album's startDate,
// already learned by the ETag gate) wins, so no photo date is parsed in the
// common case; a zero preferYear falls back to the earliest photo's taken
// date, then to the bucket name.
func (c *Client) enumerate(ctx context.Context, albumID string, preferYear int) ([]library.Asset, int, error) {
	var metas []timelineBucketMeta
	if err := c.getJSON(ctx, "/api/timeline/buckets", url.Values{"albumId": {albumID}}, &metas); err != nil {
		return nil, 0, fmt.Errorf("immich: list buckets: %w", err)
	}
	bodies := make([]timelineBucket, 0, len(metas))
	for _, m := range metas {
		var body timelineBucket
		if err := getTimelineBucket(ctx, c.getJSON, albumID, m.TimeBucket, &body); err != nil {
			return nil, 0, fmt.Errorf("immich: bucket %s: %w", m.TimeBucket, err)
		}
		bodies = append(bodies, body)
	}
	// One label per album: the taken year is shared, so compute it once.
	year := preferYear
	if year == 0 {
		year = albumYear(bodies, metas)
	}
	var out []library.Asset
	for _, body := range bodies {
		out = appendImageAssets(out, body, c.albumName, year)
	}
	return out, len(metas), nil
}

type timelineBucketMeta struct {
	TimeBucket string `json:"timeBucket"`
	Count      int    `json:"count"`
}

// timelineBucket is the columnar (struct-of-arrays) shape Immich returns per bucket.
type timelineBucket struct {
	ID               []string  `json:"id"`
	IsImage          []bool    `json:"isImage"`
	Thumbhash        []string  `json:"thumbhash"`
	FileCreatedAt    []string  `json:"fileCreatedAt"`
	LocalOffsetHours []float64 `json:"localOffsetHours"`
}

// yearFromStartDate extracts the year from an album's startDate (the earliest
// asset's local taken date, "YYYY-MM-DD" or RFC3339). Returns 0 when absent.
func yearFromStartDate(s string) int {
	if len(s) >= 4 && s[0] >= '0' && s[0] <= '9' {
		if y, err := strconv.Atoi(s[:4]); err == nil && y >= 1000 && y <= 9999 {
			return y
		}
	}
	return 0
}

// albumYear returns the year of the album's earliest taken photo, computed
// once from the bucket contents (oldest-first): fileCreatedAt is UTC, so the
// photo's local offset is added back to recover the photographer's local
// time, whose year is the kiosk label. When no photo carries a parseable
// taken date (older servers omit those columns), it falls back to the
// oldest date-like bucket name. Returns 0 when neither yields a year.
func albumYear(bodies []timelineBucket, metas []timelineBucketMeta) int {
	for _, b := range bodies {
		for i, ts := range b.FileCreatedAt {
			if ts == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			if i < len(b.LocalOffsetHours) {
				t = t.Add(time.Duration(b.LocalOffsetHours[i] * float64(time.Hour)))
			}
			if y := t.Year(); y >= 1000 && y <= 9999 {
				return y
			}
		}
	}
	for _, m := range metas {
		if len(m.TimeBucket) >= 4 && m.TimeBucket[0] >= '0' && m.TimeBucket[0] <= '9' {
			if y, err := strconv.Atoi(m.TimeBucket[:4]); err == nil && y >= 1000 && y <= 9999 {
				return y
			}
		}
	}
	return 0
}

// appendImageAssets maps the columnar payload to image assets, tolerating short
// or absent optional arrays. Every asset carries the album name and the year of
// the album's earliest photo so the kiosk can label a slide without a second lookup.
func appendImageAssets(out []library.Asset, b timelineBucket, album string, year int) []library.Asset {
	album = displayAlbumName(album)
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
		out = append(out, library.Asset{ID: id, Version: thumbhashToken(th), Album: album, Year: year})
	}
	return out
}

// displayAlbumName keeps Immich's year-in-parentheses convention compact on
// the frame. Other album names are preserved verbatim.
func displayAlbumName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) < 7 || !strings.HasSuffix(name, ")") {
		return name
	}

	open := strings.LastIndex(name, " (")
	if open < 1 || len(name)-open != 7 {
		return name
	}
	year, err := strconv.Atoi(name[open+2 : len(name)-1])
	if err != nil || year < 1000 || year > 9999 {
		return name
	}

	return fmt.Sprintf("%s %02d'", strings.TrimSpace(name[:open]), year%100)
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
			ID   string `json:"id"`
			Name string `json:"albumName"`
		} `json:"album"`
	}
	if err := c.getJSON(ctx, "/api/shared-links/me", nil, &share); err != nil {
		return "", fmt.Errorf("immich: resolve share: %w", err)
	}
	if share.Album.ID == "" {
		return "", fmt.Errorf("immich: share is not an album")
	}
	c.albumID = share.Album.ID
	c.albumName = share.Album.Name
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
		err := statusError(resp)
		_ = resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func (c *Client) url(path string, extra url.Values) string {
	q := c.ref.values()
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

// parseShareURL splits the URL into base ("https://host") and the share
// reference carried by its first path segment.
func parseShareURL(shareURL string) (base string, ref shareRef, err error) {
	u, err := url.Parse(shareURL)
	if err != nil {
		return "", shareRef{}, fmt.Errorf("immich: parse share url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", shareRef{}, fmt.Errorf("immich: share url must be http(s), got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", shareRef{}, fmt.Errorf("immich: share url missing host")
	}
	switch {
	case strings.HasPrefix(u.Path, sharePathPrefix):
		ref = shareRef{param: "key", value: firstSegment(u.Path, sharePathPrefix)}
	case strings.HasPrefix(u.Path, slugPathPrefix):
		ref = shareRef{param: "slug", value: firstSegment(u.Path, slugPathPrefix)}
	default:
		return "", shareRef{}, fmt.Errorf("immich: share url path must start with %s or %s", sharePathPrefix, slugPathPrefix)
	}
	if ref.value == "" {
		return "", shareRef{}, fmt.Errorf("immich: share url missing %s", ref.param)
	}
	return u.Scheme + "://" + u.Host, ref, nil
}

// firstSegment drops any deeper path: a share opened on one photo reads
// /share/<key>/photos/<id>.
func firstSegment(path, prefix string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(path, prefix), "/")
	return seg
}
