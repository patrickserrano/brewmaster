package caskindex

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultCatalogURL = "https://formulae.brew.sh/api/cask.json"

// DefaultCachePath returns ~/.cache/brewmaster/cask.json.
func DefaultCachePath(home string) string {
	return filepath.Join(home, ".cache", "brewmaster", "cask.json")
}

// Fetch returns the cask catalog, serving from cachePath when fresher than ttl.
func Fetch(url, cachePath string, ttl time.Duration) ([]byte, error) {
	if st, err := os.Stat(cachePath); err == nil && time.Since(st.ModTime()) < ttl {
		return os.ReadFile(cachePath)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog fetch: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		return nil, err
	}
	return data, nil
}
