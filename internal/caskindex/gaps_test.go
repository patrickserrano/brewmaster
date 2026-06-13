package caskindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCachePath(t *testing.T) {
	got := DefaultCachePath("/home/me")
	want := filepath.Join("/home/me", ".cache", "brewmaster", "cask.json")
	if got != want {
		t.Errorf("DefaultCachePath = %q, want %q", got, want)
	}
}

// writeCache fails when the cache directory cannot be created because a
// parent path component is a regular file, not a directory.
func TestWriteCacheMkdirError(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "notadir")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// cachePath's parent (fileAsParent) is a file, so MkdirAll must fail.
	cachePath := filepath.Join(fileAsParent, "sub", "cask.json")
	if err := writeCache(cachePath, []byte(validCatalog)); err == nil {
		t.Fatal("expected writeCache to fail when a parent path is a file")
	}
}

// writeCache fails to create the temp file when the target directory is
// not writable (0o000), exercising the CreateTemp error branch.
func TestWriteCacheCreateTempError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	roDir := filepath.Join(dir, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The directory exists (MkdirAll is a no-op) but is unwritable, so the
	// temp file cannot be created.
	if err := os.Chmod(roDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	cachePath := filepath.Join(roDir, "cask.json")
	if err := writeCache(cachePath, []byte(validCatalog)); err == nil {
		t.Fatal("expected writeCache to fail in an unwritable directory")
	}
}

// writeCache succeeds and writes the exact payload with 0o644 perms.
func TestWriteCacheSuccess(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "nested", "cask.json")
	if err := writeCache(cachePath, []byte(validCatalog)); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != validCatalog {
		t.Errorf("cached data = %q, want %q", data, validCatalog)
	}
}

func TestParseCatalogRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseCatalog([]byte("not json {{{")); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

// ParseCatalog tolerates malformed individual artifacts and unusual
// shapes without failing the whole parse.
func TestParseCatalogArtifactEdgeCases(t *testing.T) {
	// First artifact is a bare array (not an object) -> skipped.
	// "app" entry mixes a string, an option object with target, and a
	// non-string/non-object element. Empty target is ignored.
	data := `[{
		"token":"edge","name":["Edge"],"version":"1",
		"artifacts":[
			["binary","x"],
			{"app":["Edge.app",{"target":"Renamed.app"},42],"target":""},
			{"app":["NotAnApp"]}
		]
	}]`
	casks, err := ParseCatalog([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(casks) != 1 {
		t.Fatalf("want 1 cask, got %d", len(casks))
	}
	apps := casks[0].Apps
	if len(apps) != 2 || apps[0] != "Edge.app" || apps[1] != "Renamed.app" {
		t.Errorf("Apps = %v, want [Edge.app Renamed.app]", apps)
	}
}

// appNames returns nil for a non-array payload.
func TestAppNamesNonArray(t *testing.T) {
	if got := appNames([]byte(`{"not":"an array"}`)); got != nil {
		t.Errorf("appNames(object) = %v, want nil", got)
	}
}

// quitIDs handles malformed input, a missing quit key, a string quit, and
// an array quit.
func TestQuitIDsBranches(t *testing.T) {
	if got := quitIDs([]byte(`{"not":"stanzas"}`)); got != nil {
		t.Errorf("malformed quitIDs = %v, want nil", got)
	}
	// no quit key -> skipped; string quit; array quit
	raw := `[{"signal":["TERM","x"]},{"quit":"com.a.one"},{"quit":["com.b.two","com.b.three"]}]`
	got := quitIDs([]byte(raw))
	want := []string{"com.a.one", "com.b.two", "com.b.three"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("quitIDs = %v, want %v", got, want)
	}
}
