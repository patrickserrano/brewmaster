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
	Force  bool                    // --force: escalate failed adopts to a forced reinstall
	Yes    bool                    // --yes: don't defer upgrades of running apps
	Trash  func(path string) error // moves a bundle to the Trash (never rm -rf)
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

// MASApp identifies a Mac App Store app slated for cask replacement.
type MASApp struct {
	App        string
	Path       string
	Token      string
	Executable string
}

// ReplaceMAS converts a Mac App Store app to its cask equivalent:
// refuse if running, move the MAS bundle to the Trash (recoverable),
// then install the cask. Caller is responsible for user confirmation.
func (e Engine) ReplaceMAS(ctx context.Context, m MASApp) Result {
	res := Result{App: m.App, Token: m.Token}
	fail := func(err error) Result {
		res.Outcome = Failed
		res.OutcomeName = res.Outcome.String()
		res.Err = err
		res.ErrText = err.Error()
		return res
	}
	if e.Trash == nil {
		return fail(fmt.Errorf("no Trash implementation configured; refusing to replace %s", m.App))
	}
	if err := ctx.Err(); err != nil {
		// Don't mislabel a Ctrl-C/timeout as "app is running".
		return fail(fmt.Errorf("cancelled before replacing %s: %w", m.App, err))
	}
	if m.Executable == "" {
		// Without an executable name we cannot pgrep, so we cannot prove
		// the app isn't running. Trashing a live bundle is destructive:
		// fail closed.
		return fail(fmt.Errorf("cannot verify %s is not running (no executable name); refusing to replace", m.App))
	}
	if e.isRunning(ctx, m.Executable) {
		return fail(fmt.Errorf("%s is running; quit it first", m.App))
	}
	if err := e.Trash(m.Path); err != nil {
		return fail(fmt.Errorf("trash: %w", err))
	}
	if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", m.Token); err != nil {
		return fail(fmt.Errorf("install after trash (restore %s from Trash): %w", m.App, err))
	}
	res.Outcome = Reinstalled
	res.OutcomeName = res.Outcome.String()
	return res
}

// isRunning reports whether a process named executable exists. On a
// cancelled context it reports true: callers defer or refuse rather
// than acting (upgrading, trashing) on a dead context.
func (e Engine) isRunning(ctx context.Context, executable string) bool {
	if ctx.Err() != nil {
		return true
	}
	if executable == "" {
		return false
	}
	_, err := e.Runner.Run(ctx, "pgrep", "-xq", executable)
	return err == nil // pgrep exits 0 iff a process matched
}
