package brew

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes external commands. Production uses ExecRunner;
// tests inject fakes that record invocations and script outputs.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			// err.Error() alone is just "exit status 1"; surface brew's
			// stderr so callers get a readable failure message.
			return out, fmt.Errorf("%s: %w", stderrSummary(exitErr.Stderr), err)
		}
	}
	return out, err
}

// stderrSummary extracts a short, human-readable message from stderr:
// the last non-empty line, truncated to 200 characters.
func stderrSummary(stderr []byte) string {
	lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if len(last) > 200 {
		last = last[:200]
	}
	return last
}
