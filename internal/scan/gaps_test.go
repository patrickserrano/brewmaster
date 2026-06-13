package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// ReadApp surfaces an error when Info.plist is present but malformed.
func TestReadAppMalformedPlist(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Bad.app", "Contents")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte("not a plist {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadApp(filepath.Join(dir, "Bad.app")); err == nil {
		t.Fatal("expected an error reading a malformed plist")
	}
}

// ScanDirs skips a plain file whose name ends in .app: isDirEntry returns
// false for a non-directory, non-symlink entry.
func TestScanDirsSkipsNonDirectoryAppEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Fake.app"), []byte("not a bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	apps, err := ScanDirs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 0 {
		t.Errorf("a plain .app file must not be scanned: %+v", apps)
	}
}

func TestDefaultDirs(t *testing.T) {
	dirs := DefaultDirs("/home/me")
	want := []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join("/home/me", "Applications"),
	}
	if len(dirs) != len(want) {
		t.Fatalf("DefaultDirs len = %d, want %d", len(dirs), len(want))
	}
	for i := range want {
		if dirs[i] != want[i] {
			t.Errorf("DefaultDirs[%d] = %q, want %q", i, dirs[i], want[i])
		}
	}
}

func TestProvenanceString(t *testing.T) {
	cases := []struct {
		p    Provenance
		want string
	}{
		{Unmanaged, "unmanaged"},
		{Managed, "managed"},
		{AppStore, "app-store"},
		{System, "system"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", int(c.p), got, c.want)
		}
	}
}
