# brewmaster

[![CI](https://github.com/patrickserrano/brewmaster/actions/workflows/ci.yml/badge.svg)](https://github.com/patrickserrano/brewmaster/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/patrickserrano/brewmaster)](https://goreportcard.com/report/github.com/patrickserrano/brewmaster)
[![Latest release](https://img.shields.io/github/v/release/patrickserrano/brewmaster)](https://github.com/patrickserrano/brewmaster/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/patrickserrano/brewmaster)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform: macOS](https://img.shields.io/badge/platform-macOS-lightgrey.svg)](#install)

Audit the apps installed on your Mac and adopt the unmanaged ones into Homebrew.

**📖 Full documentation: [patrickserrano.github.io/brewmaster](https://patrickserrano.github.io/brewmaster/)**

Most Macs accumulate apps installed by hand — downloaded DMGs, vendor installers,
auto-updaters. Homebrew can manage those apps (updates, uninstalls, `brew bundle`
manifests), but only if it knows about them. `brew install --cask --adopt` can take
ownership of an existing app without reinstalling it; brewmaster finds every app
that qualifies and runs the adoption for you.

## Install

```sh
brew install patrickserrano/tap/brewmaster
```

## Usage

```sh
brewmaster audit            # read-only report; changes nothing
brewmaster adopt --dry-run  # preview the adoption plan
brewmaster adopt            # adopt every high-confidence match
```

`audit` classifies every app into five buckets (managed / adoptable / ambiguous
/ app-store / unmatched) and exits non-zero when drift exists, so it doubles as a
drift detector for cron and CI. `adopt` runs `brew install --cask --adopt` on
each adoptable app and upgrades them to converge with Homebrew's records.

Mac App Store apps are never touched without the explicit `--include-mas` flag
and a typed confirmation, since converting them loses App Store receipts and
possibly in-app purchases (app data in `~/Library` survives).

See the [documentation site](https://patrickserrano.github.io/brewmaster/) for
the full command reference, flags, exit codes, JSON output, the state log, and
how matching works:

- [audit](https://patrickserrano.github.io/brewmaster/audit/) — buckets, JSON, drift detection
- [adopt](https://patrickserrano.github.io/brewmaster/adopt/) — workflow, overrides, state log
- [Mac App Store](https://patrickserrano.github.io/brewmaster/mac-app-store/) — what `--include-mas` does
- [How it works](https://patrickserrano.github.io/brewmaster/how-it-works/) — detection and matching

## Contributing & design

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, and
[docs/plans/2026-06-11-brewmaster-design.md](docs/plans/2026-06-11-brewmaster-design.md)
for the full design doc and
[docs/plans/2026-06-11-brewmaster-implementation.md](docs/plans/2026-06-11-brewmaster-implementation.md)
for the implementation plan.
