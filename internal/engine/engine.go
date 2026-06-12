// Package engine runs the adoption state machine that brings unmanaged
// apps under Homebrew management via brew install --cask --adopt.
package engine

import (
	"context"
	"fmt"
	"strings"

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
	Yes    bool // --yes: don't defer upgrades of running apps
}

// AdoptedApp identifies a successfully adopted app for the post-adopt
// upgrade pass.
type AdoptedApp struct {
	Token      string
	Executable string // CFBundleExecutable, for pgrep
}

// AdoptOne runs the two-step adopt state machine for a single app:
// try --adopt; on failure escalate to --force only when opted in.
func (e Engine) AdoptOne(ctx context.Context, app, token string) Result {
	res := Result{App: app, Token: token}
	if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", "--adopt", token); err == nil {
		res.Outcome = Adopted
	} else if ctx.Err() != nil {
		// Cancelled/timed out: don't run --force on a dead context, and
		// don't mislabel a Ctrl-C as needs-reinstall.
		res.Outcome = Failed
		res.Err = err
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

// UpgradeAdopted converges adopted apps to their cask versions in one
// brew upgrade call. Running apps are deferred (returned) unless Yes.
// A failed upgrade is surfaced as upgradeErr so the caller can report it.
func (e Engine) UpgradeAdopted(ctx context.Context, apps []AdoptedApp) (deferred []string, upgradeErr error) {
	var upgrade []string
	for _, a := range apps {
		if !e.Yes && e.isRunning(ctx, a.Executable) {
			deferred = append(deferred, a.Token)
			continue
		}
		upgrade = append(upgrade, a.Token)
	}
	if len(upgrade) > 0 {
		args := append([]string{"upgrade", "--cask"}, upgrade...)
		if _, err := e.Runner.Run(ctx, "brew", args...); err != nil {
			upgradeErr = fmt.Errorf("brew upgrade --cask %s: %w", strings.Join(upgrade, " "), err)
		}
	}
	return deferred, upgradeErr
}

// isRunning reports whether a process named executable exists.
func (e Engine) isRunning(ctx context.Context, executable string) bool {
	if executable == "" {
		return false
	}
	_, err := e.Runner.Run(ctx, "pgrep", "-xq", executable)
	return err == nil // pgrep exits 0 iff a process matched
}
