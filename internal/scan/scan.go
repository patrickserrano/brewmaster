package scan

import (
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
			if !e.IsDir() || !strings.HasSuffix(e.Name(), ".app") {
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
