package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDirs are the locations brewmaster audits. ~ is expanded by the caller.
func DefaultDirs(home string) []string {
	return []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}
}

// ScanDirs enumerates .app bundles one level deep in each dir.
// Missing dirs and unreadable bundles are skipped, not errors.
func ScanDirs(dirs []string) ([]App, error) {
	var apps []App
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") || !isDirEntry(dir, e) {
				continue
			}
			app, err := ReadApp(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			apps = append(apps, app)
		}
	}
	return apps, nil
}

// isDirEntry reports whether e is a directory, following symlinks: os.ReadDir
// uses lstat semantics, so a symlinked .app bundle has IsDir() == false and
// must be resolved with os.Stat.
func isDirEntry(dir string, e fs.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && info.IsDir()
}
