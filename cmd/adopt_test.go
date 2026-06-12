package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// recordingRunner records every invocation. brew info returns an empty
// installed set; pgrep reports not-running (non-zero exit).
type recordingRunner struct{ calls []string }

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, "--json=v2") {
		return []byte(`{"formulae":[],"casks":[]}`), nil
	}
	if name == "pgrep" {
		return nil, errors.New("exit status 1") // no process matched
	}
	return []byte("ok"), nil
}

// slackDeps returns AdoptDeps with one adoptable app (Slack) injected.
func slackDeps(r *recordingRunner) AdoptDeps {
	return AdoptDeps{
		Runner: r,
		Scan: func() ([]scan.App, error) {
			return []scan.App{{
				Name:       "Slack.app",
				Path:       "/Applications/Slack.app",
				BundleID:   "com.tinyspeck.slackmacgap",
				Version:    "4.39.0",
				Executable: "Slack",
			}}, nil
		},
		Catalog: func() ([]caskindex.Cask, bool, error) {
			return []caskindex.Cask{
				{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
			}, false, nil
		},
		Trash: func(string) error { return nil },
	}
}

func TestAdoptDryRunMakesNoMutatingCalls(t *testing.T) {
	r := &recordingRunner{}
	out := &bytes.Buffer{}
	cmd := newAdoptCmd(slackDeps(r))
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.calls {
		// "brew info --json=v2 --installed" is read-only; mutating calls
		// are "install --cask ..." and "upgrade ...".
		if strings.Contains(c, "install --cask") || strings.Contains(c, "upgrade") {
			t.Errorf("dry-run must not mutate: %v", r.calls)
		}
	}
	if !strings.Contains(out.String(), "brew install --cask --adopt slack") {
		t.Errorf("dry-run should print the planned command:\n%s", out.String())
	}
}

func TestAdoptRunsAdoptAndUpgrade(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // state log writes under $HOME

	r := &recordingRunner{}
	out := &bytes.Buffer{}
	cmd := newAdoptCmd(slackDeps(r))
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(r.calls, "\n")
	adoptIdx := strings.Index(joined, "brew install --cask --adopt slack")
	upgradeIdx := strings.Index(joined, "brew upgrade --cask slack")
	if adoptIdx < 0 {
		t.Errorf("missing adopt call:\n%s", joined)
	}
	if upgradeIdx < 0 {
		t.Errorf("missing upgrade call:\n%s", joined)
	}
	if adoptIdx >= 0 && upgradeIdx >= 0 && upgradeIdx < adoptIdx {
		t.Errorf("upgrade ran before adopt:\n%s", joined)
	}
	if !strings.Contains(out.String(), "summary:") {
		t.Errorf("missing summary line:\n%s", out.String())
	}

	logs, err := os.ReadDir(filepath.Join(home, ".local", "state", "brewmaster"))
	if err != nil || len(logs) == 0 {
		t.Errorf("expected a state log under ~/.local/state/brewmaster: %v", err)
	}
}
