package immich

import "testing"

func TestNewAPIClientNormalizesAlbumIDs(t *testing.T) {
	c, err := NewAPIClient(APIConfig{
		BaseURL:  "https://photos.example.com",
		APIKey:   "key",
		AlbumIDs: []string{" a1 ", "a2", "a1", "  "},
	})
	if err == nil {
		t.Fatal("expected error for a blank album id")
	}
	_ = c

	c, err = NewAPIClient(APIConfig{
		BaseURL:  "https://photos.example.com",
		APIKey:   "key",
		AlbumIDs: []string{" a1 ", "a2", "a1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.albums) != 2 || c.albums[0].id != "a1" || c.albums[1].id != "a2" {
		t.Errorf("albums = %+v, want trimmed/deduplicated [a1 a2]", c.albums)
	}
}
