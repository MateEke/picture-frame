package immich_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/library/adapter/immich"
)

const (
	testKey      = "test-key-123"
	testPassword = "Almafa123"
	testAlbumID  = "1e4bb746-6072-4aec-9a20-a37b130cde4b"
	assetA       = "9b7b87ad-f032-442d-a7c3-046fed72e7bc"
	assetB       = "ab7b87ad-f032-442d-a7c3-046fed72e7bd"
)

type recordedRequest struct {
	method string
	path   string
	key    string
	pw     string
	size   string
	cookie string // immich_shared_link_token value, if sent
}

type fakeImmich struct {
	mu       sync.Mutex
	requests []recordedRequest
	album    string
	assets   string
	preview  []byte
	status   int
	// loginStatus drives POST /api/shared-links/login: 0 mimics a pre-2.6 server
	// (404, no such endpoint); 201 issues loginToken as the token cookie.
	loginStatus int
	loginToken  string
}

func newFakeImmich(t *testing.T) *fakeImmich {
	t.Helper()
	return &fakeImmich{
		album:   `{"album":{"id":"` + testAlbumID + `"}}`,
		assets:  `{"assets":[]}`,
		preview: []byte("PREVIEW-BYTES"),
		status:  http.StatusOK,
	}
}

func (f *fakeImmich) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cookie := ""
	if ck, err := r.Cookie("immich_shared_link_token"); err == nil {
		cookie = ck.Value
	}
	f.requests = append(f.requests, recordedRequest{
		method: r.Method,
		path:   r.URL.Path,
		key:    r.URL.Query().Get("key"),
		pw:     r.URL.Query().Get("password"),
		size:   r.URL.Query().Get("size"),
		cookie: cookie,
	})
}

func (f *fakeImmich) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		f.mu.Lock()
		status, loginStatus, loginToken := f.status, f.loginStatus, f.loginToken
		f.mu.Unlock()
		if r.URL.Path == "/api/shared-links/login" {
			switch loginStatus {
			case 0:
				http.NotFound(w, r)
			case http.StatusOK, http.StatusCreated:
				http.SetCookie(w, &http.Cookie{Name: "immich_shared_link_token", Value: loginToken, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
				w.WriteHeader(loginStatus)
			default:
				http.Error(w, "denied", loginStatus)
			}
			return
		}
		if status != http.StatusOK {
			http.Error(w, "boom", status)
			return
		}
		switch {
		case r.URL.Path == "/api/shared-links/me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, f.album)
		case r.URL.Path == "/api/albums/"+testAlbumID:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, f.assets)
		case strings.HasPrefix(r.URL.Path, "/api/assets/") && strings.HasSuffix(r.URL.Path, "/thumbnail"):
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(f.preview)
		default:
			http.NotFound(w, r)
		}
	})
}

func newClient(t *testing.T, srv *httptest.Server, password string) *immich.Client {
	t.Helper()
	c, err := immich.New(immich.Config{
		ShareURL: srv.URL + "/share/" + testKey,
		Password: password,
		HTTP:     srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseShareURLErrors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"empty key", "https://host/share/"},
		{"bad path", "https://host/album/x"},
		{"missing host", "https:///share/x"},
		{"wrong scheme", "ftp://host/share/x"},
		{"unparseable", "://nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := immich.New(immich.Config{ShareURL: tc.in}); err == nil {
				t.Errorf("expected error for %q", tc.in)
			}
		})
	}
}

// Trailing query/fragment shouldn't corrupt the parsed key.
func TestParseShareURLIgnoresQueryAndFragment(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	cases := []string{
		srv.URL + "/share/" + testKey + "?utm_source=email",
		srv.URL + "/share/" + testKey + "#preview",
		srv.URL + "/share/" + testKey + "?x=1#y",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			fake.requests = nil
			c, err := immich.New(immich.Config{ShareURL: in, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.List(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, r := range fake.requests {
				if r.key != testKey {
					t.Errorf("key = %q, want %q (input %q)", r.key, testKey, in)
				}
			}
		})
	}
}

func TestListReturnsImageAssetsOnly(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = `{"assets":[
		{"id":"` + assetA + `","type":"IMAGE","updatedAt":"2026-04-20T08:00:11Z"},
		{"id":"video","type":"VIDEO","updatedAt":"2026-04-20T08:00:11Z"},
		{"id":"` + assetB + `","type":"IMAGE","updatedAt":"2026-04-21T08:00:11Z"}
	]}`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d assets, want 2", len(got))
	}
	if got[0].ID != assetA || got[1].ID != assetB {
		t.Errorf("ids: %v", got)
	}
	wantTime := time.Date(2026, 4, 20, 8, 0, 11, 0, time.UTC)
	if !got[0].UpdatedAt.Equal(wantTime) {
		t.Errorf("updatedAt: got %v, want %v", got[0].UpdatedAt, wantTime)
	}
}

// A pre-2.6 server has no login endpoint (404): the client falls back to the
// legacy password query parameter on every request.
func TestLegacyServerFallsBackToPasswordQuery(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	logins, others := 0, 0
	for _, r := range fake.requests {
		if r.key != testKey {
			t.Errorf("%s: key=%q, want %q", r.path, r.key, testKey)
		}
		if r.path == "/api/shared-links/login" {
			logins++
			continue
		}
		others++
		if r.pw != testPassword {
			t.Errorf("%s: password=%q, want %q", r.path, r.pw, testPassword)
		}
	}
	if logins != 1 {
		t.Errorf("login probed %d times, want 1", logins)
	}
	if others < 2 {
		t.Fatalf("expected at least 2 data requests (share-link + album), got %d", others)
	}
}

// Immich >= 2.6.0: the password is exchanged once for the token cookie; data
// requests carry the cookie and never the password query parameter.
func TestLoginTokenFlow(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusCreated
	fake.loginToken = "tok-abc123"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	for range 2 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	logins := 0
	for _, r := range fake.requests {
		if r.path == "/api/shared-links/login" {
			logins++
			if r.method != http.MethodPost {
				t.Errorf("login method=%s, want POST", r.method)
			}
			continue
		}
		if r.pw != "" {
			t.Errorf("%s: password leaked into query: %q", r.path, r.pw)
		}
		if r.cookie != "tok-abc123" {
			t.Errorf("%s: token cookie=%q, want tok-abc123", r.path, r.cookie)
		}
	}
	if logins != 1 {
		t.Errorf("login called %d times across two Lists, want 1 (token cached)", logins)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusUnauthorized
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "wrong")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error when the share password is rejected")
	}
}

// A 401 mid-run (share password rotated upstream) clears the token; the retry
// re-runs the login exchange and succeeds with the fresh token.
func TestListRetriesLoginOn401(t *testing.T) {
	fake := newFakeImmich(t)
	fake.loginStatus = http.StatusCreated
	fake.loginToken = "tok-1"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, testPassword)

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Invalidate the held token: data requests 401 until a re-login happens.
	fake.mu.Lock()
	fake.status = http.StatusUnauthorized
	fake.loginToken = "tok-2"
	fake.mu.Unlock()
	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("expected 401 while the server rejects the stale token")
	}
	fake.mu.Lock()
	fake.status = http.StatusOK
	fake.mu.Unlock()
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("expected recovery after re-login, got %v", err)
	}
	last := fake.requests[len(fake.requests)-1]
	if last.cookie != "tok-2" {
		t.Errorf("final request used token %q, want the re-issued tok-2", last.cookie)
	}
}

func TestListOmitsPasswordWhenNotConfigured(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, r := range fake.requests {
		if r.pw != "" {
			t.Errorf("%s: password sent unexpectedly: %q", r.path, r.pw)
		}
	}
}

func TestListCachesAlbumID(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	for range 3 {
		if _, err := c.List(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	shareLinkCalls := 0
	for _, r := range fake.requests {
		if r.path == "/api/shared-links/me" {
			shareLinkCalls++
		}
	}
	if shareLinkCalls != 1 {
		t.Errorf("shared-links/me called %d times, want 1 (cached)", shareLinkCalls)
	}
}

func TestListPropagatesHTTPError(t *testing.T) {
	fake := newFakeImmich(t)
	fake.status = http.StatusUnauthorized
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error on 401")
	}
}

func TestFetchReturnsPreviewBody(t *testing.T) {
	fake := newFakeImmich(t)
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	body, err := c.Fetch(context.Background(), assetA)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "PREVIEW-BYTES" {
		t.Errorf("body: %q", got)
	}
	var thumbReq *recordedRequest
	for i := range fake.requests {
		if strings.HasSuffix(fake.requests[i].path, "/thumbnail") {
			thumbReq = &fake.requests[i]
		}
	}
	if thumbReq == nil {
		t.Fatal("no thumbnail request recorded")
		return
	}
	if thumbReq.size != "preview" {
		t.Errorf("size=%q, want preview", thumbReq.size)
	}
}

func TestFetchPropagatesHTTPError(t *testing.T) {
	fake := newFakeImmich(t)
	fake.status = http.StatusForbidden
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.Fetch(context.Background(), assetA); err == nil {
		t.Error("expected error on 403")
	}
}

func TestListSkipsAssetsWithZeroUpdatedAt(t *testing.T) {
	fake := newFakeImmich(t)
	fake.assets = `{"assets":[
		{"id":"` + assetA + `","type":"IMAGE","updatedAt":"2026-04-20T08:00:11Z"},
		{"id":"zero","type":"IMAGE","updatedAt":"0001-01-01T00:00:00Z"}
	]}`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != assetA {
		t.Errorf("zero-updatedAt asset should be skipped: %+v", got)
	}
}

func TestListInvalidatesAlbumIDOn404(t *testing.T) {
	fake := newFakeImmich(t)
	var albumCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/shared-links/me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, fake.album)
		case strings.HasPrefix(r.URL.Path, "/api/albums/"):
			albumCalls++
			if albumCalls == 1 {
				http.Error(w, "gone", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, fake.assets)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newClient(t, srv, "")

	// First album call 404s; the retry succeeds after re-resolving via /me.
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("expected retry success, got %v", err)
	}
	if albumCalls != 2 {
		t.Errorf("expected 2 album calls (first 404 → re-resolve → retry), got %d", albumCalls)
	}
}

func TestListErrorsOnMissingAlbumID(t *testing.T) {
	fake := newFakeImmich(t)
	fake.album = `{"album":{"id":""}}`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newClient(t, srv, "")

	if _, err := c.List(context.Background()); err == nil {
		t.Error("expected error for share with no album")
	}
}
