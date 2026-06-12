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
	"github.com/patrickserrano/brewmaster/internal/engine"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// recordingRunner records every invocation. brew info returns
// installedJSON (default: empty installed set); pgrep reports
// not-running (non-zero exit).
type recordingRunner struct {
	calls         []string
	installedJSON string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, "--json=v2") {
		if r.installedJSON != "" {
			return []byte(r.installedJSON), nil
		}
		return []byte(`{"formulae":[],"casks":[]}`), nil
	}
	if name == "pgrep" {
		return nil, errors.New("exit status 1") // no process matched
	}
	return []byte("ok"), nil
}

// mutatingCalls returns recorded calls that would change system state.
func (r *recordingRunner) mutatingCalls() []string {
	var out []string
	for _, c := range r.calls {
		if strings.Contains(c, "install --cask") || strings.Contains(c, "upgrade") {
			out = append(out, c)
		}
	}
	return out
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

// mixedDeps returns AdoptDeps covering every classification bucket:
// Slack (adoptable), Things3 + Cinch (MAS with confident cask matches),
// Tunnelblick (ambiguous: two casks claim the artifact), and Firefox
// (managed by Homebrew). Trash calls are recorded into trashed.
func mixedDeps(r *recordingRunner, trashed *[]string) AdoptDeps {
	r.installedJSON = `{"formulae":[],"casks":[{"token":"firefox","version":"127.0",` +
		`"artifacts":[{"app":["Firefox.app"]}]}]}`
	return AdoptDeps{
		Runner: r,
		Scan: func() ([]scan.App, error) {
			return []scan.App{
				{Name: "Slack.app", Path: "/Applications/Slack.app",
					BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0", Executable: "Slack"},
				{Name: "Things3.app", Path: "/Applications/Things3.app",
					BundleID: "com.culturedcode.ThingsMac", Version: "3.20.0",
					Executable: "Things3", MASReceipt: true},
				{Name: "Cinch.app", Path: "/Applications/Cinch.app",
					BundleID: "com.irradiatedsoftware.Cinch", Version: "1.2.0",
					Executable: "Cinch", MASReceipt: true},
				{Name: "Tunnelblick.app", Path: "/Applications/Tunnelblick.app",
					BundleID: "net.tunnelblick.tunnelblick", Version: "5.0", Executable: "Tunnelblick"},
				{Name: "Firefox.app", Path: "/Applications/Firefox.app",
					BundleID: "org.mozilla.firefox", Version: "127.0", Executable: "firefox"},
			}, nil
		},
		Catalog: func() ([]caskindex.Cask, bool, error) {
			return []caskindex.Cask{
				{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
				{Token: "things", Version: "3.20.0", Apps: []string{"Things3.app"}},
				{Token: "cinch", Version: "1.2.0", Apps: []string{"Cinch.app"}},
				{Token: "tunnelblick", Version: "6.0", Apps: []string{"Tunnelblick.app"}},
				{Token: "tunnelblick-beta", Version: "7.0beta1", Apps: []string{"Tunnelblick.app"}},
				{Token: "firefox", Version: "127.0", Apps: []string{"Firefox.app"}},
			}, false, nil
		},
		Trash: func(p string) error {
			if trashed != nil {
				*trashed = append(*trashed, p)
			}
			return nil
		},
	}
}

func runAdopt(t *testing.T, deps AdoptDeps, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := newAdoptCmd(deps)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
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

// Fix 1: positional args must scope MAS jobs too — naming one MAS app
// must not plan replacement of every matched MAS app.
func TestPositionalArgsFilterMASJobs(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAdopt(t, mixedDeps(r, nil), "", "--dry-run", "--include-mas", "--yes", "Things3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "would replace MAS app: Things3.app") {
		t.Errorf("Things3 should be planned for replacement:\n%s", out)
	}
	if strings.Contains(out, "Cinch.app") {
		t.Errorf("Cinch.app was not named and must not be planned:\n%s", out)
	}
	if strings.Contains(out, "would run: brew install --cask --adopt slack") {
		t.Errorf("Slack was not named and must not be planned:\n%s", out)
	}
}

// Fix 1: an active --cask override targets exactly one app; MAS jobs
// must be excluded entirely even with --include-mas.
func TestCaskOverrideExcludesMASJobs(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAdopt(t, mixedDeps(r, nil), "",
		"--dry-run", "--include-mas", "--cask", "tunnelblick", "Tunnelblick")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "would replace MAS app") {
		t.Errorf("--cask must not plan MAS replacements:\n%s", out)
	}
	if !strings.Contains(out, "would run: brew install --cask --adopt tunnelblick") {
		t.Errorf("expected planned adopt for the override target:\n%s", out)
	}
}

// Fix 2: --cask must only target adoptable or ambiguous apps.
func TestCaskOverrideRejectsMASApp(t *testing.T) {
	r := &recordingRunner{}
	var trashed []string
	_, _, err := runAdopt(t, mixedDeps(r, &trashed), "", "--cask", "cinch", "Cinch")
	if err == nil {
		t.Fatal("expected error targeting a MAS app with --cask")
	}
	if !strings.Contains(err.Error(), "Mac App Store") || !strings.Contains(err.Error(), "--include-mas") {
		t.Errorf("error should explain MAS apps need --include-mas: %v", err)
	}
	if len(r.mutatingCalls()) > 0 || len(trashed) > 0 {
		t.Errorf("rejected override must not mutate: %v / trashed %v", r.calls, trashed)
	}
}

func TestCaskOverrideRejectsManagedApp(t *testing.T) {
	r := &recordingRunner{}
	_, _, err := runAdopt(t, mixedDeps(r, nil), "", "--cask", "firefox", "Firefox")
	if err == nil {
		t.Fatal("expected error targeting a managed app with --cask")
	}
	if !strings.Contains(err.Error(), "already managed by Homebrew") {
		t.Errorf("error should say the app is already managed: %v", err)
	}
	if len(r.mutatingCalls()) > 0 {
		t.Errorf("rejected override must not mutate: %v", r.calls)
	}
}

func TestCaskOverrideRejectsUnknownApp(t *testing.T) {
	r := &recordingRunner{}
	_, _, err := runAdopt(t, mixedDeps(r, nil), "", "--cask", "nope", "NotARealApp")
	if err == nil {
		t.Fatal("expected error targeting an unknown app with --cask")
	}
	if !strings.Contains(err.Error(), "not found among adoptable or ambiguous apps") {
		t.Errorf("error should say the app was not found: %v", err)
	}
}

func TestCaskOverrideAmbiguousAppWorks(t *testing.T) {
	r := &recordingRunner{}
	out, _, err := runAdopt(t, mixedDeps(r, nil), "", "--dry-run", "--cask", "tunnelblick", "Tunnelblick")
	if err != nil {
		t.Fatalf("--cask on an ambiguous app should work: %v", err)
	}
	if !strings.Contains(out, "would run: brew install --cask --adopt tunnelblick  # Tunnelblick.app") {
		t.Errorf("dry-run should plan the override adopt:\n%s", out)
	}
}

// Fix 3: positional args that match nothing must be reported and make
// the command exit non-zero rather than silently succeeding.
func TestUnmatchedPositionalArgErrors(t *testing.T) {
	r := &recordingRunner{}
	out, stderr, err := runAdopt(t, slackDeps(r), "", "Slak")
	if err == nil {
		t.Fatalf("unmatched arg must produce an error; stdout:\n%s", out)
	}
	if !errors.Is(err, ErrUnmatchedArgs) {
		t.Errorf("error must wrap ErrUnmatchedArgs (exit 1, not 2): %v", err)
	}
	if !strings.Contains(stderr, "no adoptable app matched: Slak") {
		t.Errorf("stderr should name the unmatched arg:\n%s", stderr)
	}
	if len(r.mutatingCalls()) > 0 {
		t.Errorf("nothing matched, nothing should run: %v", r.calls)
	}
}

func TestAmbiguousPositionalArgSuggestsCask(t *testing.T) {
	r := &recordingRunner{}
	_, stderr, err := runAdopt(t, mixedDeps(r, nil), "", "--dry-run", "Tunnelblick")
	if err == nil {
		t.Fatal("ambiguous-only arg must produce an error")
	}
	if !strings.Contains(stderr, "--cask") {
		t.Errorf("stderr should point at --cask for ambiguous apps:\n%s", stderr)
	}
}

// Non-fatal: matched args still run even when another arg is unmatched.
func TestUnmatchedArgIsNonFatalForMatchedWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &recordingRunner{}
	out, stderr, err := runAdopt(t, mixedDeps(r, nil), "", "Slack", "Slak")
	if err == nil {
		t.Fatal("expected non-zero exit when any arg is unmatched")
	}
	if !errors.Is(err, ErrUnmatchedArgs) {
		t.Errorf("error must wrap ErrUnmatchedArgs (exit 1, not 2): %v", err)
	}
	if !strings.Contains(stderr, "no adoptable app matched: Slak") {
		t.Errorf("stderr should name the unmatched arg:\n%s", stderr)
	}
	if !strings.Contains(strings.Join(r.calls, "\n"), "brew install --cask --adopt slack") {
		t.Errorf("matched arg should still be adopted: %v", r.calls)
	}
	_ = out
}

// Fix 4: confirmMAS must fail closed on EOF (closed stdin) — MAS
// conversion is skipped and nothing is trashed.
func TestConfirmMASFailsClosedOnEOF(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := &recordingRunner{}
	var trashed []string
	out, _, err := runAdopt(t, mixedDeps(r, &trashed), "" /* stdin EOF */, "--include-mas")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "MAS conversion skipped.") {
		t.Errorf("EOF on stdin must skip MAS conversion:\n%s", out)
	}
	if len(trashed) > 0 {
		t.Errorf("nothing may be trashed without confirmation: %v", trashed)
	}
	for _, c := range r.calls {
		if c == "brew install --cask things" || c == "brew install --cask cinch" {
			t.Errorf("MAS cask install must not run without confirmation: %v", r.calls)
		}
	}
}

// Fix 4: the full flag matrix in dry-run mode makes zero mutating calls.
func TestDryRunFullFlagMatrixMakesNoMutatingCalls(t *testing.T) {
	r := &recordingRunner{}
	var trashed []string
	out, _, err := runAdopt(t, mixedDeps(r, &trashed), "", "--dry-run", "--include-mas", "--yes", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if calls := r.mutatingCalls(); len(calls) > 0 {
		t.Errorf("dry-run must not mutate: %v", calls)
	}
	if len(trashed) > 0 {
		t.Errorf("dry-run must not trash: %v", trashed)
	}
	if !strings.Contains(out, "would run: brew install --cask --adopt slack") ||
		!strings.Contains(out, "would replace MAS app: Things3.app") {
		t.Errorf("dry-run should print the full plan:\n%s", out)
	}
}

// Fix 5: two state logs written within the same second must not collide.
func TestStateLogFilenamesDoNotCollide(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	results := []engine.Result{{App: "Slack.app", Token: "slack", OutcomeName: "adopted"}}
	writeStateLog(results)
	writeStateLog(results)
	logs, err := os.ReadDir(filepath.Join(home, ".local", "state", "brewmaster"))
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 distinct state logs, got %d", len(logs))
	}
}

// dedupeByName keeps the first scanned app per bundle name and warns
// about every later duplicate.
func TestDedupeByName(t *testing.T) {
	apps := []scan.App{
		{Name: "Slack.app", Path: "/Applications/Slack.app"},
		{Name: "Slack.app", Path: "/Users/me/Applications/Slack.app"},
		{Name: "Other.app", Path: "/Applications/Other.app"},
	}
	var stderr bytes.Buffer
	deduped, byName := dedupeByName(apps, &stderr)
	if len(deduped) != 2 || deduped[0].Name != "Slack.app" || deduped[1].Name != "Other.app" {
		t.Errorf("deduped = %+v", deduped)
	}
	if byName["Slack.app"].Path != "/Applications/Slack.app" {
		t.Errorf("first scanned path must win: %+v", byName["Slack.app"])
	}
	if !strings.Contains(stderr.String(), "duplicate app name Slack.app") {
		t.Errorf("expected duplicate warning:\n%s", stderr.String())
	}
}

// Fix 5: duplicate bundle names keep the first scanned app and warn.
func TestDuplicateBundleNamesWarnAndKeepFirst(t *testing.T) {
	r := &recordingRunner{}
	deps := slackDeps(r)
	deps.Scan = func() ([]scan.App, error) {
		return []scan.App{
			{Name: "Slack.app", Path: "/Applications/Slack.app",
				BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0", Executable: "Slack"},
			{Name: "Slack.app", Path: "/Users/me/Applications/Slack.app",
				BundleID: "com.tinyspeck.slackmacgap", Version: "4.38.0", Executable: "Slack"},
		}, nil
	}
	out, stderr, err := runAdopt(t, deps, "", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "duplicate app name Slack.app") {
		t.Errorf("expected a duplicate-name warning on stderr:\n%s", stderr)
	}
	if n := strings.Count(out, "would run: brew install --cask --adopt slack"); n != 1 {
		t.Errorf("expected exactly one planned adopt for Slack, got %d:\n%s", n, out)
	}
}
