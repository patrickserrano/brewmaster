// Package engine runs the adoption state machine that brings unmanaged
// apps under Homebrew management via brew install --cask --adopt.
package engine

import (
	"context"

	"github.com/patrickserrano/brewmaster/internal/brew"
)

// Outcome classifies the result of adopting a single app.
type Outcome int

const (
	Adopted Outcome = iota
	Reinstalled
	NeedsReinstall // adopt failed; user did not pass --force
	Failed
)

func (o Outcome) String() string {
	return [...]string{"adopted", "reinstalled", "needs-reinstall", "failed"}[o]
}

// Result records what happened to one app during adoption.
type Result struct {
	App         string  `json:"app"`
	Token       string  `json:"token"`
	Outcome     Outcome `json:"-"`
	OutcomeName string  `json:"outcome"`
	Err         error   `json:"-"`
	ErrText     string  `json:"error,omitempty"`
}

// Engine drives adoption. All external commands go through Runner so
// tests can inject fakes.
type Engine struct {
	Runner brew.Runner
	Force  bool // --force: escalate failed adopts to a forced reinstall
}

// AdoptOne runs the two-step adopt state machine for a single app:
// try --adopt; on failure escalate to --force only when opted in.
func (e Engine) AdoptOne(ctx context.Context, app, token string) Result {
	res := Result{App: app, Token: token}
	if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", "--adopt", token); err == nil {
		res.Outcome = Adopted
	} else if !e.Force {
		res.Outcome = NeedsReinstall
		res.Err = err
	} else if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", "--force", token); err == nil {
		res.Outcome = Reinstalled
	} else {
		res.Outcome = Failed
		res.Err = err
	}
	res.OutcomeName = res.Outcome.String()
	if res.Err != nil {
		res.ErrText = res.Err.Error()
	}
	return res
}
