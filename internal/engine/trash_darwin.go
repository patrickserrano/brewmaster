package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TrashPath moves a path to the user's Trash. Prefers the macOS 14+
// /usr/bin/trash binary (handles permissions and Finder put-back);
// falls back to mv into ~/.Trash with a timestamp suffix on collision.
func TrashPath(path string) error {
	if _, err := os.Stat("/usr/bin/trash"); err == nil {
		return exec.Command("/usr/bin/trash", path).Run()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, ".Trash", filepath.Base(path))
	if _, err := os.Stat(dest); err == nil {
		dest = fmt.Sprintf("%s.%d", dest, time.Now().Unix())
	}
	return os.Rename(path, dest)
}
