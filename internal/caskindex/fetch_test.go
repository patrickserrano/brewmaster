package caskindex

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchCachesCatalog(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`[{"token":"slack","name":["Slack"],"version":"1","artifacts":[{"app":["Slack.app"]}]}]`))
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cask.json")

	for i := 0; i < 2; i++ {
		data, err := Fetch(srv.URL, cache, 24*time.Hour)
		if err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if len(data) == 0 {
			t.Fatalf("fetch %d returned empty", i)
		}
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (second call should be cached)", hits)
	}

	// Expire the cache; next call must refetch.
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(cache, old, old)
	if _, err := Fetch(srv.URL, cache, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("server hits = %d, want 2 after cache expiry", hits)
	}
}
