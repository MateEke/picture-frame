package immich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Album is one album as listed by the Immich API. The field names mirror
// Immich's own camelCase JSON so the response decodes without a translation
// layer.
type Album struct {
	ID         string `json:"id"`
	AlbumName  string `json:"albumName"`
	AssetCount int    `json:"assetCount"`
}

// ListAlbums returns every album visible to the API key, so the admin UI can
// offer a picker instead of asking the user to paste UUIDs.
//
// A package function rather than an APIClient method because discovery has to
// work before any album is configured — NewAPIClient rejects empty album_ids.
// The key travels in the x-api-key header only, never in the URL or a log,
// matching APIClient.auth.
func ListAlbums(ctx context.Context, baseURL, apiKey string, httpc *http.Client) ([]Album, error) {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("immich: invalid base url: must not be empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("immich: api key required")
	}
	if httpc == nil {
		httpc = &http.Client{Timeout: defaultTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/albums", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(apiKeyHeader, apiKey)
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("immich: list albums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{Status: resp.StatusCode}
	}
	var albums []Album
	if err := json.NewDecoder(resp.Body).Decode(&albums); err != nil {
		return nil, fmt.Errorf("immich: decode albums: %w", err)
	}
	return albums, nil
}
