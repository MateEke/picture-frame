package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubListParsesAndFilters(t *testing.T) {
	const body = `[
		{"tag_name":"v1.3.1","prerelease":false,"draft":false,
		 "assets":[{"name":"picture-frame_1.3.1_linux_armv6.tar.gz","url":"https://api/armv6","browser_download_url":"https://dl/armv6"},
		           {"name":"checksums.txt","url":"https://api/sums","browser_download_url":"https://dl/sums"}]},
		{"tag_name":"v1.4.0-rc.1","prerelease":true,"draft":false,"assets":[]},
		{"tag_name":"v1.5.0","prerelease":false,"draft":true,"assets":[]}
	]`
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	g := NewGitHub("owner/repo", "")
	g.api = srv.URL
	releases, err := g.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if !strings.Contains(gotPath, "/repos/owner/repo/releases") || !strings.Contains(gotPath, "per_page=100") {
		t.Errorf("request path: %q", gotPath)
	}
	if gotAuth != "" {
		t.Errorf("no token → no Authorization header, got %q", gotAuth)
	}
	// The draft is dropped; the prerelease is kept (resolution filters it, not the source).
	if len(releases) != 2 {
		t.Fatalf("want 2 releases (draft dropped), got %d: %+v", len(releases), releases)
	}
	r := releases[0]
	if r.Version != "v1.3.1" || r.Prerelease {
		t.Errorf("first release: %+v", r)
	}
	// Map to the API asset URL (token-authenticatable), not browser_download_url.
	if r.Assets["picture-frame_1.3.1_linux_armv6.tar.gz"] != "https://api/armv6" {
		t.Errorf("asset map: %+v", r.Assets)
	}
	if !releases[1].Prerelease {
		t.Errorf("second release should be the kept prerelease: %+v", releases[1])
	}
}

func TestGitHubListErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // e.g. rate-limited
	}))
	defer srv.Close()

	g := NewGitHub("owner/repo", "")
	g.api = srv.URL
	if _, err := g.List(context.Background()); err == nil {
		t.Error("non-200 status should error")
	}
}

func TestGitHubListSendsToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	g := NewGitHub("owner/repo", "ghp_secret")
	g.api = srv.URL
	if _, err := g.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer ghp_secret" {
		t.Errorf("token → Authorization: got %q", gotAuth)
	}
}
