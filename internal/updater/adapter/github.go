// Package adapter holds the updater's external integrations (GitHub release source,
// HTTP downloader, exec pre-flight). They talk to the network and the OS, so they are
// integration-tested on hardware and excluded from unit coverage.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MateEke/picture-frame/internal/updater"
)

const githubAPI = "https://api.github.com"

// GitHub lists releases for a repo ("owner/name") via the GitHub Releases API.
type GitHub struct {
	repo   string
	token  string // optional; authenticates for private repos + the higher rate limit
	api    string // base URL; overridable in tests
	client *http.Client
}

// NewGitHub returns a release source for repo "owner/name". An empty token means
// unauthenticated (public repos); a personal-access token unlocks private sources.
func NewGitHub(repo, token string) *GitHub {
	return &GitHub{repo: repo, token: token, api: githubAPI, client: newHTTPClient(45 * time.Second)}
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	HTMLURL    string `json:"html_url"`
	Assets     []struct {
		Name string `json:"name"`
		// API asset URL, not browser_download_url, the latter 404s for private-repo assets.
		URL string `json:"url"`
	} `json:"assets"`
}

// List fetches up to 100 releases (per_page max) so same-major resolution sees history.
func (g *GitHub) List(ctx context.Context) ([]updater.Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=100", g.api, g.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: status %d", resp.StatusCode)
	}

	var raw []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	releases := make([]updater.Release, 0, len(raw))
	for _, r := range raw {
		if r.Draft {
			continue
		}
		assets := make(map[string]string, len(r.Assets))
		for _, a := range r.Assets {
			assets[a.Name] = a.URL
		}
		releases = append(releases, updater.Release{
			Version: r.TagName, Prerelease: r.Prerelease, NotesURL: r.HTMLURL, Assets: assets,
		})
	}
	return releases, nil
}
