package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner scripts responses per command-line prefix and records calls.
type fakeRunner struct {
	calls []string
	fail  map[string]error // command prefix -> error to return
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	for prefix, err := range f.fail {
		if strings.HasPrefix(cmd, prefix) {
			return nil, err
		}
	}
	return []byte("ok"), nil
}

func TestAdoptSuccess(t *testing.T) {
	r := &fakeRunner{}
	e := Engine{Runner: r}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Adopted {
		t.Fatalf("outcome = %v, want Adopted", res.Outcome)
	}
	if len(r.calls) != 1 || r.calls[0] != "brew install --cask --adopt slack" {
		t.Errorf("calls = %v", r.calls)
	}
}

func TestAdoptFailureWithoutForceReportsNeedsReinstall(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install --cask --adopt": errors.New("artifact differs")}}
	e := Engine{Runner: r}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != NeedsReinstall {
		t.Fatalf("outcome = %v, want NeedsReinstall", res.Outcome)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "--force") {
			t.Errorf("must not escalate to --force without opt-in: %v", r.calls)
		}
	}
}

func TestAdoptFailureWithForceEscalates(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install --cask --adopt": errors.New("artifact differs")}}
	e := Engine{Runner: r, Force: true}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Reinstalled {
		t.Fatalf("outcome = %v, want Reinstalled", res.Outcome)
	}
	want := "brew install --cask --force slack"
	if len(r.calls) != 2 || r.calls[1] != want {
		t.Errorf("calls = %v, want second call %q", r.calls, want)
	}
}

func TestForceEscalationCanAlsoFail(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install": errors.New("boom")}}
	e := Engine{Runner: r, Force: true}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed with error", res.Outcome, res.Err)
	}
}

func TestAdoptCancelledContextDoesNotEscalate(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install --cask --adopt": errors.New("context canceled")}}
	e := Engine{Runner: r, Force: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already dead before we start
	res := e.AdoptOne(ctx, "Slack.app", "slack")
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed with error", res.Outcome, res.Err)
	}
	if len(r.calls) != 1 {
		t.Errorf("must not run --force on a dead context: calls = %v", r.calls)
	}
}

func TestUpgradeAdoptedSkipsRunningApps(t *testing.T) {
	// pgrep succeeds by default (app running); script failure for the
	// NOT-running app so it gets upgraded.
	r := &fakeRunner{fail: map[string]error{"pgrep -xq Code": errors.New("exit 1")}}
	e := Engine{Runner: r}

	deferred, err := e.UpgradeAdopted(context.Background(), []AdoptedApp{
		{Token: "slack", Executable: "Slack"},             // running -> deferred
		{Token: "visual-studio-code", Executable: "Code"}, // not running -> upgraded
	})
	if err != nil {
		t.Fatalf("UpgradeAdopted error: %v", err)
	}

	if len(deferred) != 1 || deferred[0] != "slack" {
		t.Errorf("deferred = %v, want [slack]", deferred)
	}
	want := "brew upgrade --cask visual-studio-code"
	found := false
	for _, c := range r.calls {
		if c == want {
			found = true
		}
		if strings.Contains(c, "upgrade") && strings.Contains(c, "slack") {
			t.Errorf("running app must not be upgraded: %v", r.calls)
		}
	}
	if !found {
		t.Errorf("missing %q in calls %v", want, r.calls)
	}
}

func TestUpgradeAllWhenYes(t *testing.T) {
	r := &fakeRunner{} // every pgrep "succeeds" -> everything looks running
	e := Engine{Runner: r, Yes: true}
	deferred, err := e.UpgradeAdopted(context.Background(), []AdoptedApp{{Token: "slack", Executable: "Slack"}})
	if err != nil {
		t.Fatalf("UpgradeAdopted error: %v", err)
	}
	if len(deferred) != 0 {
		t.Errorf("with Yes, nothing defers: %v", deferred)
	}
	want := "brew upgrade --cask slack"
	if len(r.calls) != 1 || r.calls[0] != want {
		t.Errorf("calls = %v, want [%q]", r.calls, want)
	}
}

func TestUpgradeNoAppsNoCalls(t *testing.T) {
	r := &fakeRunner{}
	deferred, err := Engine{Runner: r}.UpgradeAdopted(context.Background(), nil)
	if err != nil {
		t.Fatalf("UpgradeAdopted error: %v", err)
	}
	if len(deferred) != 0 {
		t.Errorf("deferred = %v, want none", deferred)
	}
	if len(r.calls) != 0 {
		t.Errorf("no apps should mean no brew calls: %v", r.calls)
	}
}

func TestUpgradeAdoptedSurfacesUpgradeError(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{
		"pgrep":        errors.New("exit 1"), // nothing running
		"brew upgrade": errors.New("upgrade boom"),
	}}
	e := Engine{Runner: r}
	deferred, err := e.UpgradeAdopted(context.Background(), []AdoptedApp{{Token: "slack", Executable: "Slack"}})
	if err == nil {
		t.Fatal("want upgrade error surfaced, got nil")
	}
	if len(deferred) != 0 {
		t.Errorf("deferred = %v, want none", deferred)
	}
}

func TestUpgradeAdoptedCancelledContextDefersEverything(t *testing.T) {
	// Mirror of TestAdoptCancelledContextDoesNotEscalate: on a dead
	// context, every app is treated as running (deferred) and no brew
	// commands run.
	r := &fakeRunner{fail: map[string]error{"pgrep": errors.New("exit 1")}} // would report "not running"
	e := Engine{Runner: r}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already dead before we start
	deferred, err := e.UpgradeAdopted(ctx, []AdoptedApp{{Token: "slack", Executable: "Slack"}})
	if err != nil {
		t.Fatalf("UpgradeAdopted error: %v", err)
	}
	if len(deferred) != 1 || deferred[0] != "slack" {
		t.Errorf("deferred = %v, want [slack]", deferred)
	}
	if len(r.calls) != 0 {
		t.Errorf("must make no calls on a dead context: %v", r.calls)
	}
}

func TestReplaceMASRefusesRunningApp(t *testing.T) {
	r := &fakeRunner{} // pgrep succeeds -> app is running
	e := Engine{Runner: r, Trash: func(string) error { t.Fatal("must not trash a running app"); return nil }}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "Things3.app", Path: "/Applications/Things3.app", Token: "things", Executable: "Things"})
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want Failed (running)", res.Outcome)
	}
	if !strings.Contains(res.ErrText, "is running; quit it first") {
		t.Errorf("refusal must be distinguishable in the message, got %q", res.ErrText)
	}
}

// funcRunner adapts a function to brew.Runner so tests can record runner
// calls and Trash calls into one shared sequence.
type funcRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func (f funcRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

func TestReplaceMASTrashesThenInstalls(t *testing.T) {
	// Record trash and install in one sequence so we can assert order:
	// the bundle must be trashed BEFORE brew install runs.
	var seq []string
	r := funcRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		seq = append(seq, cmd)
		if strings.HasPrefix(cmd, "pgrep") {
			return nil, errors.New("not running")
		}
		return []byte("ok"), nil
	})
	e := Engine{Runner: r, Trash: func(p string) error {
		seq = append(seq, "trash "+p)
		return nil
	}}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "Things3.app", Path: "/Applications/Things3.app", Token: "things", Executable: "Things"})
	if res.Outcome != Reinstalled {
		t.Fatalf("outcome = %v (err=%v)", res.Outcome, res.Err)
	}
	wantTrash := "trash /Applications/Things3.app"
	wantInstall := "brew install --cask things"
	trashIdx, installIdx := -1, -1
	for i, c := range seq {
		switch c {
		case wantTrash:
			trashIdx = i
		case wantInstall:
			installIdx = i
		}
	}
	if trashIdx == -1 {
		t.Fatalf("missing %q in sequence %v", wantTrash, seq)
	}
	if installIdx == -1 {
		t.Fatalf("missing %q in sequence %v", wantInstall, seq)
	}
	if trashIdx > installIdx {
		t.Errorf("trash must happen before install: sequence %v", seq)
	}
}

func TestReplaceMASInstallFailureSurfaces(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{
		"pgrep":        errors.New("not running"),
		"brew install": errors.New("no such cask"),
	}}
	e := Engine{Runner: r, Trash: func(string) error { return nil }}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x", Executable: "X"})
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed", res.Outcome, res.Err)
	}
	if !strings.Contains(res.ErrText, "restore") {
		t.Errorf("install failure after trashing must hint at restoring from Trash, got %q", res.ErrText)
	}
}

func TestReplaceMASNoExecutableFailsClosed(t *testing.T) {
	// With no executable name we cannot pgrep, so we cannot verify the
	// app isn't running. Refuse to trash anything.
	r := &fakeRunner{}
	e := Engine{Runner: r, Trash: func(string) error {
		t.Fatal("must not trash when we cannot verify the app is not running")
		return nil
	}}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x"})
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want Failed (fail closed)", res.Outcome)
	}
	if !strings.Contains(res.ErrText, "cannot verify") || !strings.Contains(res.ErrText, "refusing to replace") {
		t.Errorf("fail-closed refusal must be explicit, got %q", res.ErrText)
	}
	if len(r.calls) != 0 {
		t.Errorf("must make no calls when failing closed: %v", r.calls)
	}
}

func TestReplaceMASNilTrashFails(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"pgrep": errors.New("not running")}}
	e := Engine{Runner: r} // Trash left nil
	res := e.ReplaceMAS(context.Background(), MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x", Executable: "X"})
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed with error (not a panic)", res.Outcome, res.Err)
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "brew install") {
			t.Errorf("must not install without a Trash implementation: %v", r.calls)
		}
	}
}

func TestReplaceMASCancelledContextReportsCancellation(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"pgrep": errors.New("not running")}}
	e := Engine{Runner: r, Trash: func(string) error {
		t.Fatal("must not trash on a dead context")
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already dead before we start
	res := e.ReplaceMAS(ctx, MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x", Executable: "X"})
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want Failed", res.Outcome)
	}
	if strings.Contains(res.ErrText, "is running; quit it first") {
		t.Errorf("cancellation must not masquerade as a running app, got %q", res.ErrText)
	}
	if !strings.Contains(res.ErrText, "cancelled") {
		t.Errorf("cancellation must be named in the message, got %q", res.ErrText)
	}
	if len(r.calls) != 0 {
		t.Errorf("must make no calls on a dead context: %v", r.calls)
	}
}

func TestUpgradeEmptyExecutableTreatedAsNotRunning(t *testing.T) {
	r := &fakeRunner{}
	deferred, err := Engine{Runner: r}.UpgradeAdopted(context.Background(), []AdoptedApp{{Token: "slack"}})
	if err != nil {
		t.Fatalf("UpgradeAdopted error: %v", err)
	}
	if len(deferred) != 0 {
		t.Errorf("deferred = %v, want none (no executable means no pgrep)", deferred)
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "pgrep") {
			t.Errorf("must not pgrep an empty executable: %v", r.calls)
		}
	}
}
