package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickserrano/brewmaster/internal/engine"
	"github.com/patrickserrano/brewmaster/internal/report"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// fillDefaults must wire non-nil production implementations for every field.
func TestAdoptFillDefaults(t *testing.T) {
	var d AdoptDeps
	d.fillDefaults()
	if d.Runner == nil || d.Scan == nil || d.Catalog == nil || d.Trash == nil {
		t.Errorf("fillDefaults left a nil field: %+v", d)
	}
}

// The default Scan closure resolves $HOME and scans the default dirs
// without error (missing dirs are skipped, so an empty HOME is fine).
func TestAdoptDefaultScanClosureRuns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var d AdoptDeps
	d.fillDefaults()
	if _, err := d.Scan(); err != nil {
		t.Errorf("default Scan should not error on an empty HOME: %v", err)
	}
}

// The default audit Scan closure exercises the same production path.
func TestAuditDefaultScanClosureRuns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var d AuditDeps
	d.fillDefaults()
	if _, err := d.Scan(); err != nil {
		t.Errorf("default Scan should not error on an empty HOME: %v", err)
	}
}

// normalizeAppArg appends .app only when missing.
func TestNormalizeAppArg(t *testing.T) {
	if got := normalizeAppArg("Slack"); got != "Slack.app" {
		t.Errorf("normalizeAppArg(Slack) = %q, want Slack.app", got)
	}
	if got := normalizeAppArg("Slack.app"); got != "Slack.app" {
		t.Errorf("normalizeAppArg(Slack.app) = %q, want unchanged", got)
	}
}

// hasMASJob is true only for App Store entries with a confident token.
func TestHasMASJob(t *testing.T) {
	appStore := []report.Entry{
		{App: "Things3.app", Token: "things"}, // confident match
		{App: "Mystery.app"},                  // no token
	}
	if !hasMASJob(appStore, "Things3.app") {
		t.Error("Things3.app has a token; hasMASJob should be true")
	}
	if hasMASJob(appStore, "Mystery.app") {
		t.Error("Mystery.app has no token; hasMASJob should be false")
	}
	if hasMASJob(appStore, "Absent.app") {
		t.Error("absent app should not match")
	}
}

// printResult appends the error text only when present.
func TestPrintResult(t *testing.T) {
	var buf bytes.Buffer
	printResult(&buf, engine.Result{App: "Slack.app", Token: "slack", OutcomeName: "adopted"})
	if got := buf.String(); !strings.Contains(got, "adopted:") || strings.Contains(got, "—") {
		t.Errorf("clean result must not include an em dash: %q", got)
	}

	buf.Reset()
	printResult(&buf, engine.Result{App: "Slack.app", Token: "slack", OutcomeName: "failed", ErrText: "boom"})
	if got := buf.String(); !strings.Contains(got, "— boom") {
		t.Errorf("failed result must include the error text: %q", got)
	}
}

// writeStateLog is a no-op when there are no results: nothing is written.
func TestWriteStateLogNoResults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeStateLog(nil)
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "brewmaster")); !os.IsNotExist(err) {
		t.Errorf("no results should write no state dir, stat err = %v", err)
	}
}

// reportUnmatchedArgs: a MAS arg with --include-mas and a confident match
// is treated as matched work (no error), exercising the hasMASJob branch.
func TestReportUnmatchedArgsMASMatched(t *testing.T) {
	r := report.Report{
		AppStore: []report.Entry{{App: "Things3.app", Token: "things"}},
	}
	var buf bytes.Buffer
	if err := reportUnmatchedArgs(&buf, []string{"Things3"}, r, true /* includeMAS */); err != nil {
		t.Errorf("a MAS arg with a confident match and --include-mas should match: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("matched MAS arg should print nothing: %q", buf.String())
	}
}

// reportUnmatchedArgs: a MAS app named without a confident match, with
// --include-mas, reports "no confident cask match" and errors.
func TestReportUnmatchedArgsMASNoMatch(t *testing.T) {
	r := report.Report{
		AppStore: []report.Entry{{App: "Mystery.app"}}, // no token
	}
	var buf bytes.Buffer
	err := reportUnmatchedArgs(&buf, []string{"Mystery"}, r, true /* includeMAS */)
	if !errors.Is(err, ErrUnmatchedArgs) {
		t.Errorf("MAS app without a match should wrap ErrUnmatchedArgs: %v", err)
	}
	if !strings.Contains(buf.String(), "no confident cask match") {
		t.Errorf("expected 'no confident cask match' line: %q", buf.String())
	}
}

// masWorklist skips App Store entries without a confident token and
// honors positional-arg filtering.
func TestMASWorklist(t *testing.T) {
	appStore := []report.Entry{
		{App: "Things3.app", Token: "things"}, // confident
		{App: "Mystery.app"},                  // no token: skipped
	}
	byName := map[string]scan.App{
		"Things3.app": {Name: "Things3.app", Path: "/Applications/Things3.app", Executable: "Things3"},
		"Mystery.app": {Name: "Mystery.app", Path: "/Applications/Mystery.app", Executable: "Mystery"},
	}
	// No args: only the confident match is planned.
	jobs := masWorklist(appStore, byName, nil)
	if len(jobs) != 1 || jobs[0].token != "things" {
		t.Fatalf("jobs = %+v, want one things job", jobs)
	}
	// Arg filtering: naming a different app yields no jobs.
	if jobs := masWorklist(appStore, byName, []string{"Other"}); len(jobs) != 0 {
		t.Errorf("unmatched arg should yield no MAS jobs: %+v", jobs)
	}
}

// reportUnmatchedArgs: an adoptable arg matches and produces no error.
func TestReportUnmatchedArgsAdoptableMatched(t *testing.T) {
	r := report.Report{Adoptable: []report.Entry{{App: "Slack.app", Token: "slack"}}}
	var buf bytes.Buffer
	if err := reportUnmatchedArgs(&buf, []string{"Slack"}, r, false); err != nil {
		t.Errorf("adoptable arg should match cleanly: %v", err)
	}
}
