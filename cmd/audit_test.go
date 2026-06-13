package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// errRunner fails every command, simulating brew being absent.
type errRunner struct{}

func (errRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("exec: \"brew\": executable file not found in $PATH")
}

// auditMixedDeps returns AuditDeps covering every classification bucket:
// Slack (adoptable), Things3 (MAS with a confident cask match),
// Tunnelblick (ambiguous: two casks claim the artifact), Firefox
// (managed by Homebrew), and Mystery (unmatched: no cask available).
func auditMixedDeps(r *recordingRunner) AuditDeps {
	r.installedJSON = `{"formulae":[],"casks":[{"token":"firefox","version":"127.0",` +
		`"artifacts":[{"app":["Firefox.app"]}]}]}`
	return AuditDeps{
		Runner: r,
		Scan: func() ([]scan.App, error) {
			return []scan.App{
				{Name: "Slack.app", Path: "/Applications/Slack.app",
					BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0", Executable: "Slack"},
				{Name: "Things3.app", Path: "/Applications/Things3.app",
					BundleID: "com.culturedcode.ThingsMac", Version: "3.20.0",
					Executable: "Things3", MASReceipt: true},
				{Name: "Tunnelblick.app", Path: "/Applications/Tunnelblick.app",
					BundleID: "net.tunnelblick.tunnelblick", Version: "5.0", Executable: "Tunnelblick"},
				{Name: "Firefox.app", Path: "/Applications/Firefox.app",
					BundleID: "org.mozilla.firefox", Version: "127.0", Executable: "firefox"},
				{Name: "Mystery.app", Path: "/Applications/Mystery.app",
					BundleID: "com.example.mystery", Version: "1.0", Executable: "Mystery"},
			}, nil
		},
		Catalog: func() ([]caskindex.Cask, bool, error) {
			return []caskindex.Cask{
				{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
				{Token: "things", Version: "3.20.0", Apps: []string{"Things3.app"}},
				{Token: "tunnelblick", Version: "6.0", Apps: []string{"Tunnelblick.app"}},
				{Token: "tunnelblick-beta", Version: "7.0beta1", Apps: []string{"Tunnelblick.app"}},
				{Token: "firefox", Version: "127.0", Apps: []string{"Firefox.app"}},
			}, false, nil
		},
	}
}

func runAudit(t *testing.T, deps AuditDeps, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := newAuditCmd(deps)
	cmd.SilenceUsage = true // root sets this in production
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// Drift present: audit renders each bucket and exits with ErrAdoptableFound.
func TestAuditTableRendersAllBuckets(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAudit(t, auditMixedDeps(r))
	if !errors.Is(err, ErrAdoptableFound) {
		t.Fatalf("expected ErrAdoptableFound (drift exit), got %v", err)
	}
	for _, want := range []string{"ADOPTABLE", "Slack.app", "AMBIGUOUS", "Tunnelblick.app",
		"APP STORE", "Things3.app", "UNMATCHED", "Mystery.app"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	// Managed apps are hidden without --verbose.
	if strings.Contains(out, "Firefox.app") {
		t.Errorf("managed app must be hidden without --verbose:\n%s", out)
	}
	if !strings.Contains(out, "1 app(s) already managed") {
		t.Errorf("expected managed count line:\n%s", out)
	}
}

// --verbose lists managed apps too.
func TestAuditVerboseShowsManaged(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAudit(t, auditMixedDeps(r), "--verbose")
	if !errors.Is(err, ErrAdoptableFound) {
		t.Fatalf("expected ErrAdoptableFound, got %v", err)
	}
	if !strings.Contains(out, "MANAGED") || !strings.Contains(out, "Firefox.app") {
		t.Errorf("--verbose should list managed apps:\n%s", out)
	}
}

// --json emits a structured report with every bucket populated.
func TestAuditJSONOutput(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAudit(t, auditMixedDeps(r), "--json")
	if !errors.Is(err, ErrAdoptableFound) {
		t.Fatalf("expected ErrAdoptableFound, got %v", err)
	}
	var got struct {
		ManagedCount int `json:"managed_count"`
		Adoptable    []struct {
			App   string `json:"app"`
			Token string `json:"token"`
		} `json:"adoptable"`
		Ambiguous []struct {
			App        string   `json:"app"`
			Candidates []string `json:"candidates"`
		} `json:"ambiguous"`
		AppStore  []struct{ App string } `json:"app_store"`
		Unmatched []struct{ App string } `json:"unmatched"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.ManagedCount != 1 {
		t.Errorf("managed_count = %d, want 1", got.ManagedCount)
	}
	if len(got.Adoptable) != 1 || got.Adoptable[0].App != "Slack.app" || got.Adoptable[0].Token != "slack" {
		t.Errorf("adoptable = %+v", got.Adoptable)
	}
	if len(got.Ambiguous) != 1 || len(got.Ambiguous[0].Candidates) != 2 {
		t.Errorf("ambiguous = %+v", got.Ambiguous)
	}
	if len(got.AppStore) != 1 || len(got.Unmatched) != 1 {
		t.Errorf("app_store = %+v unmatched = %+v", got.AppStore, got.Unmatched)
	}
}

// --json with --verbose includes managed entries.
func TestAuditJSONVerboseIncludesManaged(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAudit(t, auditMixedDeps(r), "--json", "--verbose")
	if !errors.Is(err, ErrAdoptableFound) {
		t.Fatalf("expected ErrAdoptableFound, got %v", err)
	}
	var got struct {
		Managed []struct{ App string } `json:"managed"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got.Managed) != 1 || got.Managed[0].App != "Firefox.app" {
		t.Errorf("managed = %+v, want Firefox.app", got.Managed)
	}
}

// Clean state: no drift means exit 0 and the "nothing to adopt" line.
func TestAuditCleanExit(t *testing.T) {
	r := &recordingRunner{}
	// Only a managed app; nothing adoptable/ambiguous/unmatched.
	r.installedJSON = `{"formulae":[],"casks":[{"token":"firefox","version":"127.0",` +
		`"artifacts":[{"app":["Firefox.app"]}]}]}`
	deps := AuditDeps{
		Runner: r,
		Scan: func() ([]scan.App, error) {
			return []scan.App{
				{Name: "Firefox.app", Path: "/Applications/Firefox.app",
					BundleID: "org.mozilla.firefox", Version: "127.0", Executable: "firefox"},
			}, nil
		},
		Catalog: func() ([]caskindex.Cask, bool, error) {
			return []caskindex.Cask{
				{Token: "firefox", Version: "127.0", Apps: []string{"Firefox.app"}},
			}, false, nil
		},
	}
	out, _, err := runAudit(t, deps)
	if err != nil {
		t.Fatalf("clean state should exit 0, got %v", err)
	}
	if !strings.Contains(out, "Nothing to adopt") {
		t.Errorf("expected clean-state message:\n%s", out)
	}
}

// A stale catalog prints a warning to stderr.
func TestAuditStaleCacheWarning(t *testing.T) {
	r := &recordingRunner{}
	deps := auditMixedDeps(r)
	deps.Catalog = func() ([]caskindex.Cask, bool, error) {
		return []caskindex.Cask{
			{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
		}, true /* stale */, nil
	}
	_, stderr, err := runAudit(t, deps)
	if !errors.Is(err, ErrAdoptableFound) {
		t.Fatalf("expected ErrAdoptableFound, got %v", err)
	}
	if !strings.Contains(stderr, "stale cask catalog") {
		t.Errorf("expected stale-cache warning on stderr:\n%s", stderr)
	}
}

// Scan errors propagate as real errors (exit 2 via main).
func TestAuditScanError(t *testing.T) {
	r := &recordingRunner{}
	deps := auditMixedDeps(r)
	deps.Scan = func() ([]scan.App, error) { return nil, errors.New("scan boom") }
	if _, _, err := runAudit(t, deps); err == nil || !strings.Contains(err.Error(), "scan boom") {
		t.Fatalf("scan error should propagate, got %v", err)
	}
}

// Catalog errors propagate.
func TestAuditCatalogError(t *testing.T) {
	r := &recordingRunner{}
	deps := auditMixedDeps(r)
	deps.Catalog = func() ([]caskindex.Cask, bool, error) { return nil, false, errors.New("catalog boom") }
	if _, _, err := runAudit(t, deps); err == nil || !strings.Contains(err.Error(), "catalog boom") {
		t.Fatalf("catalog error should propagate, got %v", err)
	}
}

// A brew failure is wrapped with the "is Homebrew installed?" hint.
func TestAuditBrewError(t *testing.T) {
	r := &errRunner{}
	deps := auditMixedDeps(&recordingRunner{})
	deps.Runner = r
	if _, _, err := runAudit(t, deps); err == nil || !strings.Contains(err.Error(), "is Homebrew installed?") {
		t.Fatalf("brew error should be wrapped, got %v", err)
	}
}

// failWriter fails every write, to exercise RenderJSON's error path.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// A write failure during --json output propagates as a real error.
func TestAuditJSONWriteError(t *testing.T) {
	r := &recordingRunner{}
	cmd := newAuditCmd(auditMixedDeps(r))
	cmd.SilenceUsage = true
	cmd.SetOut(failWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected a write error from RenderJSON, got %v", err)
	}
}

// fillDefaults wires non-nil production implementations for every field.
func TestAuditFillDefaults(t *testing.T) {
	var d AuditDeps
	d.fillDefaults()
	if d.Runner == nil || d.Scan == nil || d.Catalog == nil {
		t.Errorf("fillDefaults left a nil field: %+v", d)
	}
}
