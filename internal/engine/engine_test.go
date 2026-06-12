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
