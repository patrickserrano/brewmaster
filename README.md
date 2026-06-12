# brewmaster

Audit the apps installed on your Mac and adopt the unmanaged ones into Homebrew.

Most Macs accumulate apps installed by hand — downloaded DMGs, vendor installers,
auto-updaters. Homebrew can manage those apps (updates, uninstalls, `brew bundle`
manifests), but only if it knows about them. `brew install --cask --adopt` can take
ownership of an existing app without reinstalling it; brewmaster finds every app
that qualifies and runs the adoption for you.

## Install

```sh
brew install patrickserrano/tap/brewmaster
```

## Commands

### `brewmaster audit`

Scans `/Applications` (and `~/Applications`), classifies every app, and matches
unmanaged apps against the Homebrew cask catalog:

```
26 app(s) already managed by Homebrew

ADOPTABLE
  Alcove.app                1.7.2                  alcove
  ChatGPT.app               1.2026.104             chatgpt
  Claude.app                1.9255.0               claude
  Docker.app                4.76.0                 docker-desktop
  Figma.app                 116.5.18               figma
  HandBrake.app             1.10.2                 handbrake-app
  ...

AMBIGUOUS (use: brewmaster adopt --cask <token> "<App>")
  Kaleidoscope.app     6.3          kaleidoscope, kaleidoscope@2, kaleidoscope@3
  iTerm.app            3.6.1        iterm2, iterm2@beta, iterm2@nightly

APP STORE (untouched; use --include-mas to convert)
  Amphetamine.app            5.3.2
  Ivory.app                  2.5.2
  ...
```

Use `--verbose` to also include the Homebrew-managed apps, and `--json` for
machine-readable output (the same report as JSON; combine with `--verbose`
to include managed apps).

### `brewmaster adopt [apps...]`

Adopts every adoptable app (or just the ones named as arguments) via
`brew install --cask --adopt`, then upgrades them so they're current:

```
$ brewmaster adopt --dry-run
would run: brew install --cask --adopt alcove  # Alcove.app
would run: brew install --cask --adopt chatgpt  # ChatGPT.app
would run: brew install --cask --adopt docker-desktop  # Docker.app
would run: brew install --cask --adopt figma  # Figma.app
...
```

| Flag | Effect |
| --- | --- |
| `--dry-run` | Print planned actions without executing anything |
| `--include-mas` | Also replace Mac App Store apps with their cask versions (see warning below) |
| `--force` | Reinstall when adopt fails on artifact mismatch (overwrites the app bundle) |
| `--yes` | Skip confirmations; upgrade apps even if they appear to be running |
| `--cask <token>` | Explicit cask token for a single (ambiguous) app, e.g. `brewmaster adopt --cask iterm2 iTerm` |

## Mac App Store apps

Apps installed from the Mac App Store are never touched by default. With
`--include-mas`, brewmaster replaces a MAS app with the cask version:
the old bundle goes to the Trash and the cask is installed fresh. You will
**lose App Store receipts, App Store auto-updates, and possibly in-app
purchases**; app data in `~/Library` is preserved. brewmaster asks for explicit
confirmation before doing this (unless `--yes` is passed) and refuses to
replace an app that is currently running.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Clean — nothing adoptable (audit), or all requested work done (adopt) |
| 1 | Adoptable apps found (audit), or positional args matched no adoptable app (adopt) |
| 2 | Error (brew missing, catalog fetch failed, etc.) |

This makes `brewmaster audit` usable as a drift check in scripts and cron.

## State log

Every non-dry-run `adopt` writes a JSON record of its results to
`~/.local/state/brewmaster/adopt-<timestamp>.json` (best-effort; logging
failures never block adoption).

## Design

See [docs/plans/2026-06-11-brewmaster-design.md](docs/plans/2026-06-11-brewmaster-design.md)
for the full design doc, and
[docs/plans/2026-06-11-brewmaster-implementation.md](docs/plans/2026-06-11-brewmaster-implementation.md)
for the implementation plan.
