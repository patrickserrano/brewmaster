package brew

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunnerWrapsStderrInError(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), "sh", "-c", "echo boom >&2; exit 1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q should contain stderr message %q", err.Error(), "boom")
	}
}

func TestExecRunnerSuccess(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), "sh", "-c", "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("out = %q, want hello", out)
	}
}
