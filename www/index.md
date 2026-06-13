---
title: brewmaster
summary: Audit the apps installed on your Mac and adopt the unmanaged ones into Homebrew.
description: brewmaster finds the macOS apps you installed by hand and adopts them into Homebrew — without reinstalling or losing data. Mac App Store apps stay untouched unless you opt in.
---

Most Macs accumulate apps installed by hand — downloaded DMGs, vendor installers,
auto-updaters. Homebrew can manage those apps (updates, uninstalls, `brew bundle`
manifests), but only if it knows about them. Apps installed outside of `brew`
are invisible to it: they never show up in `brew list`, never get upgraded by
`brew upgrade`, and never make it into a `Brewfile`.

`brew install --cask --adopt` can take ownership of an existing app *without*
reinstalling it — but it works one app at a time, needs you to already know the
cask token, and has confusing version-drift behavior. brewmaster does the whole
loop for you: it finds every app that qualifies, matches it to the right cask
with confidence tiers, and runs the adoption — keeping your data intact.

## Quickstart

```sh
# Install
brew install patrickserrano/tap/brewmaster

# See what's adoptable (read-only, changes nothing)
brewmaster audit

# Preview the adoption plan (still changes nothing)
brewmaster adopt --dry-run

# Actually adopt every high-confidence match
brewmaster adopt
```

## Safety posture

brewmaster is built to be safe to run by default. Nothing destructive happens
without an explicit opt-in.

- **`audit` is read-only.** It only reads `.app` bundles and the Homebrew
  catalog. It never mutates anything.
- **`adopt` defaults to a real adoption but is undoable.** `brew install
  --cask --adopt` takes ownership of the *existing* bundle in place — it does
  not reinstall or delete it. Use `--dry-run` to preview the exact commands first.
- **App data is never touched.** Adoption operates on the `.app` bundle; your
  preferences, databases, and documents in `~/Library` are left alone.
- **Mac App Store apps are never touched by default.** Converting a MAS app to a
  cask is destructive (you lose receipts and possibly in-app purchases), so it
  is hidden behind the `--include-mas` flag plus a typed confirmation. See
  [Mac App Store](mac-app-store.md).
- **Ambiguous matches are never auto-adopted.** If more than one cask could be
  the right one, brewmaster lists the candidates and waits for you to pick with
  `--cask`.
- **Forced reinstall is opt-in.** When an adopt fails on an artifact mismatch,
  brewmaster reports it but only overwrites the bundle if you pass `--force`.

## Where to go next

- [Installation](installation.md) — the tap, `go install`, and building from source.
- [audit](audit.md) — the five buckets, JSON output, and using it as a drift detector.
- [adopt](adopt.md) — the dry-run-first workflow, overrides, and the state log.
- [How it works](how-it-works.md) — detection signals and the confidence-tiered matcher.
- [FAQ](faq.md) — data safety, wrong matches, offline behavior, and more.
