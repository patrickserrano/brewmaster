package caskindex

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validCatalog = `[{"token":"slack","name":["Slack"],"version":"1","artifacts":[{"app":["Slack.app"]}]}]`

func TestFetchCachesCatalog(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(validCatalog))
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cask.json")

	for i := 0; i < 2; i++ {
		data, stale, err := Fetch(srv.URL, cache, 24*time.Hour)
		if err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if stale {
			t.Fatalf("fetch %d: stale = true, want false", i)
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
	if _, _, err := Fetch(srv.URL, cache, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("server hits = %d, want 2 after cache expiry", hits)
	}
}

func TestFetchFallsBackToStaleCacheOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cask.json")
	if err := os.WriteFile(cache, []byte(validCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(cache, old, old)

	data, stale, err := Fetch(srv.URL, cache, 24*time.Hour)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if !stale {
		t.Error("stale = false, want true on fallback")
	}
	if string(data) != validCatalog {
		t.Errorf("data = %q, want cached catalog", data)
	}
}

func TestFetchFallsBackToStaleCacheOnTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // connection refused

	cache := filepath.Join(t.TempDir(), "cask.json")
	if err := os.WriteFile(cache, []byte(validCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(cache, old, old)

	data, stale, err := Fetch(srv.URL, cache, 24*time.Hour)
	if err != nil {
		t.Fatalf("expected stale fallback, got error: %v", err)
	}
	if !stale {
		t.Error("stale = false, want true on fallback")
	}
	if string(data) != validCatalog {
		t.Errorf("data = %q, want cached catalog", data)
	}
}

func TestFetchErrorsWithoutCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cask.json")

	t.Run("transport error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close()
		if _, _, err := Fetch(srv.URL, cache, 24*time.Hour); err == nil {
			t.Fatal("expected error with failing server and no cache")
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()
		if _, _, err := Fetch(srv.URL, cache, 24*time.Hour); err == nil {
			t.Fatal("expected error on 500 with no cache")
		}
	})
}

func TestFetchDoesNotCacheInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json {{{`))
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cask.json")

	data, stale, err := Fetch(srv.URL, cache, 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if stale {
		t.Error("stale = true, want false")
	}
	if string(data) != `not json {{{` {
		t.Errorf("data = %q, want raw body returned even when invalid", data)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("cache file should not exist after invalid JSON response, stat err = %v", err)
	}
}

func TestFetchLeavesNoTempFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validCatalog))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cache := filepath.Join(dir, "cask.json")
	if _, _, err := Fetch(srv.URL, cache, 24*time.Hour); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "cask.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cache dir contents = %v, want only cask.json", names)
	}
}
