package brew

import (
	"context"
	"os/exec"
)

// Runner executes external commands. Production uses ExecRunner;
// tests inject fakes that record invocations and script outputs.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
