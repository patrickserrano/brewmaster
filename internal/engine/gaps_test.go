package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A Trash failure aborts the replacement before any install runs.
func TestReplaceMASTrashFailureSurfaces(t *testing.T) {
	// pgrep fails -> app is not running, so the replacement proceeds to Trash.
	r := &fakeRunner{fail: map[string]error{"pgrep": errors.New("exit status 1")}}
	e := Engine{Runner: r, Trash: func(string) error { return errors.New("trash full") }}
	res := e.ReplaceMAS(context.Background(),
		MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x", Executable: "X"})
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want Failed", res.Outcome)
	}
	if !strings.Contains(res.ErrText, "trash") {
		t.Errorf("error should mention trash: %q", res.ErrText)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "install --cask") {
			t.Errorf("install must not run after a trash failure: %v", r.calls)
		}
	}
}
