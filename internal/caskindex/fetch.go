package caskindex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultCatalogURL = "https://formulae.brew.sh/api/cask.json"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// DefaultCachePath returns ~/.cache/brewmaster/cask.json.
func DefaultCachePath(home string) string {
	return filepath.Join(home, ".cache", "brewmaster", "cask.json")
}

// Fetch returns the cask catalog, serving from cachePath when fresher than ttl.
// If the cache is expired and the network fetch fails, it falls back to the
// stale cache when present; stale is true only on that fallback path.
func Fetch(url, cachePath string, ttl time.Duration) (data []byte, stale bool, err error) {
	if st, err := os.Stat(cachePath); err == nil && time.Since(st.ModTime()) < ttl {
		data, err := os.ReadFile(cachePath)
		return data, false, err
	}
	data, err = download(url)
	if err != nil {
		// Fall back to a stale cache rather than failing outright.
		if cached, readErr := os.ReadFile(cachePath); readErr == nil {
			return cached, true, nil
		}
		return nil, false, err
	}
	// Only cache well-formed JSON so a garbage response can't poison the
	// cache; still return the data so the parser surfaces a real error.
	if json.Valid(data) {
		if err := writeCache(cachePath, data); err != nil {
			return nil, false, err
		}
	}
	return data, false, nil
}

func download(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog fetch: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// writeCache writes data to cachePath atomically via a temp file and rename.
func writeCache(cachePath string, data []byte) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "cask-*.json.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() // the write error is the one worth reporting
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close() // the chmod error is the one worth reporting
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), cachePath)
}
