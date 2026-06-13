package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// Scan errors propagate out of the adopt command.
func TestAdoptScanError(t *testing.T) {
	deps := slackDeps(&recordingRunner{})
	deps.Scan = func() ([]scan.App, error) { return nil, errors.New("scan boom") }
	if _, _, err := runAdopt(t, deps, ""); err == nil || !strings.Contains(err.Error(), "scan boom") {
		t.Fatalf("scan error should propagate, got %v", err)
	}
}

// Catalog errors propagate out of the adopt command.
func TestAdoptCatalogError(t *testing.T) {
	deps := slackDeps(&recordingRunner{})
	deps.Catalog = func() ([]caskindex.Cask, bool, error) { return nil, false, errors.New("catalog boom") }
	if _, _, err := runAdopt(t, deps, ""); err == nil || !strings.Contains(err.Error(), "catalog boom") {
		t.Fatalf("catalog error should propagate, got %v", err)
	}
}

// A brew failure is wrapped with the "is Homebrew installed?" hint.
func TestAdoptBrewError(t *testing.T) {
	deps := slackDeps(&recordingRunner{})
	deps.Runner = errRunner{}
	if _, _, err := runAdopt(t, deps, ""); err == nil || !strings.Contains(err.Error(), "is Homebrew installed?") {
		t.Fatalf("brew error should be wrapped, got %v", err)
	}
}

// A stale catalog warns on stderr even in dry-run.
func TestAdoptStaleCacheWarning(t *testing.T) {
	deps := slackDeps(&recordingRunner{})
	deps.Catalog = func() ([]caskindex.Cask, bool, error) {
		return []caskindex.Cask{{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}}}, true, nil
	}
	_, stderr, err := runAdopt(t, deps, "", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "stale cask catalog") {
		t.Errorf("expected stale-cache warning:\n%s", stderr)
	}
}

// --cask requires exactly one positional app argument.
func TestCaskOverrideRequiresOneArg(t *testing.T) {
	deps := slackDeps(&recordingRunner{})
	_, _, err := runAdopt(t, deps, "", "--cask", "slack")
	if err == nil || !strings.Contains(err.Error(), "exactly one app argument") {
		t.Fatalf("expected one-arg error, got %v", err)
	}
}

// With nothing adoptable and no args, adopt prints the no-op message.
func TestAdoptNothingToAdopt(t *testing.T) {
	r := &recordingRunner{}
	deps := AdoptDeps{
		Runner: r,
		Scan: func() ([]scan.App, error) {
			return []scan.App{{Name: "Firefox.app", Path: "/Applications/Firefox.app",
				BundleID: "org.mozilla.firefox", Version: "127.0", Executable: "firefox"}}, nil
		},
		Catalog: func() ([]caskindex.Cask, bool, error) {
			return []caskindex.Cask{{Token: "firefox", Version: "127.0", Apps: []string{"Firefox.app"}}}, false, nil
		},
		Trash: func(string) error { return nil },
	}
	r.installedJSON = `{"formulae":[],"casks":[{"token":"firefox","version":"127.0",` +
		`"artifacts":[{"app":["Firefox.app"]}]}]}`
	out, _, err := runAdopt(t, deps, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Nothing to adopt") {
		t.Errorf("expected no-op message:\n%s", out)
	}
	if len(r.mutatingCalls()) > 0 {
		t.Errorf("nothing should run: %v", r.calls)
	}
}

// runningRunner reports the adopted app as running (pgrep exits 0) so the
// post-adopt upgrade is deferred rather than executed.
type runningRunner struct{ recordingRunner }

func (r *runningRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "pgrep" {
		r.calls = append(r.calls, name+" "+strings.Join(args, " "))
		return []byte("123\n"), nil // a process matched
	}
	return r.recordingRunner.Run(ctx, name, args...)
}

// A running adopted app defers its upgrade and prints the deferred line.
func TestAdoptDefersUpgradeForRunningApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &runningRunner{}
	deps := slackDeps(&recordingRunner{})
	deps.Runner = r
	out, _, err := runAdopt(t, deps, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "deferred upgrades") || !strings.Contains(out, "slack") {
		t.Errorf("expected deferred-upgrade line for running app:\n%s", out)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "upgrade") {
			t.Errorf("a running app's upgrade must be deferred, not run: %v", r.calls)
		}
	}
}

// upgradeFailRunner adopts fine but fails the upgrade step.
type upgradeFailRunner struct{ recordingRunner }

func (r *upgradeFailRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	if strings.Contains(call, "upgrade") {
		r.calls = append(r.calls, call)
		return nil, errors.New("upgrade exploded")
	}
	return r.recordingRunner.Run(ctx, name, args...)
}

// A failed post-adopt upgrade is surfaced as a warning on stderr.
func TestAdoptUpgradeFailureWarns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &upgradeFailRunner{}
	deps := slackDeps(&recordingRunner{})
	deps.Runner = r
	_, stderr, err := runAdopt(t, deps, "", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "post-adopt upgrade failed") {
		t.Errorf("expected upgrade-failure warning:\n%s", stderr)
	}
}

// With --yes and --include-mas, confirmed MAS apps are replaced (trashed
// then reinstalled) without prompting.
func TestAdoptMASConfirmedReplaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &recordingRunner{}
	var trashed []string
	out, _, err := runAdopt(t, mixedDeps(r, &trashed), "", "--include-mas", "--yes", "Things3")
	if err != nil {
		t.Fatal(err)
	}
	if len(trashed) == 0 {
		t.Errorf("confirmed MAS replacement should trash the bundle; out:\n%s", out)
	}
	joined := strings.Join(r.calls, "\n")
	if !strings.Contains(joined, "brew install --cask things") {
		t.Errorf("confirmed MAS replacement should install the cask: %v", r.calls)
	}
}

// MAS confirmation via an interactive "yes" on stdin proceeds.
func TestAdoptMASConfirmedViaStdin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &recordingRunner{}
	var trashed []string
	out, _, err := runAdopt(t, mixedDeps(r, &trashed), "yes\n", "--include-mas", "Things3")
	if err != nil {
		t.Fatal(err)
	}
	if len(trashed) == 0 {
		t.Errorf("typing 'yes' should confirm MAS replacement; out:\n%s", out)
	}
}

// A MAS app named without --include-mas reports it needs the flag.
func TestAdoptMASArgWithoutFlag(t *testing.T) {
	r := &recordingRunner{}
	_, stderr, err := runAdopt(t, mixedDeps(r, nil), "", "--dry-run", "Things3")
	if !errors.Is(err, ErrUnmatchedArgs) {
		t.Fatalf("MAS arg without --include-mas should error, got %v", err)
	}
	if !strings.Contains(stderr, "--include-mas") {
		t.Errorf("stderr should point at --include-mas:\n%s", stderr)
	}
}
