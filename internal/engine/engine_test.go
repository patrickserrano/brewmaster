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
