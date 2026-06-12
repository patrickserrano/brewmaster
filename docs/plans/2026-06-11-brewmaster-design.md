# Brewmaster — Design

**Date:** 2026-06-11
**Status:** Validated via brainstorming, ready for implementation planning

## What it is

A distributable Go CLI that audits the apps installed on a Mac, determines which
are already managed by Homebrew, and safely converts the rest to Homebrew-managed
installs using `brew install --cask --adopt` — without losing data. Mac App Store
apps are detected and reported but never touched unless an explicit flag is given.

## Why it doesn't already exist

The pieces exist; the tool doesn't:

- `brew install --cask --adopt` (Homebrew/brew#14006) is the conversion primitive,
  but it is one-app-at-a-time, requires knowing the cask token, and has confusing
  version-drift behavior (Homebrew/brew#20967).
- `brew file casklist` (rcmdnk/homebrew-file) does the audit half, but its
  migration path predates `--adopt` (delete-and-reinstall), and it has no
  MAS-awareness, confidence-tiered matching, or automated adoption.
- `brew bundle dump` + `mas` export what brew already manages; they don't adopt
  what it doesn't.

Brewmaster's value-add is the full loop: scan → match with confidence tiers →
bulk adopt → post-adopt upgrade, with dry-run-by-default safety.

## Key decisions (made during brainstorming)

| Decision | Choice |
|---|---|
| Audience / form | Distributable CLI, installed via Homebrew tap |
| Default behavior | Read-only audit; conversion only behind explicit `adopt` command |
| Match policy | Strict auto-adopt for high-confidence matches only; ambiguous apps listed with candidates |
| MAS apps | Detected and reported by default; replaced with cask only via `--include-mas` + loud warning (receipts/IAP lost; app data survives) |
| Version drift | Adopt, then `brew upgrade` adopted casks so disk state matches brew records |
| Language | Go (subprocess orchestration + JSON parsing; single static binary) |

## Command surface

```
brewmaster audit              # default: read-only report
brewmaster adopt [apps...]    # convert; no args = all high-confidence matches
brewmaster adopt --include-mas
brewmaster adopt --dry-run    # print exactly what would run, change nothing
brewmaster adopt --cask <token> "<App>"   # explicit override for ambiguous apps
```

Audit buckets every app:

1. **Managed** — already brew-owned (counted; detailed under `--verbose`)
2. **Adoptable** — unmanaged, exactly one high-confidence cask match
3. **Ambiguous** — multiple candidates or weak match; listed with candidates
4. **App Store** — MAS-installed; untouched without `--include-mas`
5. **Unmatched** — no cask exists (Apple system apps excluded entirely)

Human-readable table by default; `--json` for scripting. `audit` exits non-zero
when adoptable apps exist (usable as a drift detector in scripts/cron).
No config file in v1. Optional run log for forensics.

## Detection & provenance

Scan `.app` bundles in `/Applications`, `/Applications/Utilities`,
`~/Applications` (one level deep; skip `/System`). Signals, all local:

1. **Homebrew-managed:** one `brew info --json=v2 --installed` call yields each
   installed cask's declared `app` artifacts; match by path (or Caskroom symlink).
2. **MAS:** presence of `Contents/_MASReceipt/receipt` — canonical, offline.
3. **Apple system apps:** bundle ID `com.apple.*` → excluded from report.
4. **Everything else:** unmanaged → candidate for matching.

For unmanaged apps, read `Contents/Info.plist` for bundle ID, version
(`CFBundleShortVersionString`), and display name.

v1 exclusions (YAGNI): non-app artifacts (CLI tools, prefpanes, plugins);
detection of other package managers (MacPorts, nix).

Performance: one slow brew invocation (~2–5s); everything else is filesystem
reads. Audit completes well under 10s for ~200 apps.

## Matching

**Index:** full cask catalog from `https://formulae.brew.sh/api/cask.json`
(~25MB, cached 24h in `~/.cache/brewmaster/`). Build two maps:

- artifact map: `"Visual Studio Code.app"` → `visual-studio-code`
- bundle ID map: `com.microsoft.VSCode` → `visual-studio-code`
  (from `uninstall quit:`/`pkgutil:` stanzas, where present)

**Confidence tiers:**

- **High (auto-adoptable):** on-disk bundle name exactly matches one cask's app
  artifact, AND (bundle ID agrees OR no other cask claims that artifact name).
- **Ambiguous:** multiple casks declare the same artifact (e.g. `@beta`
  variants) or only fuzzy display-name match. Tie-breaker before demotion: if
  the app's Info.plist version matches exactly one candidate's cask version,
  that candidate wins at High.
- **None:** → Unmatched.

The matcher is a pure function `(app metadata, index) → match result` —
table-driven unit tests with canned cask JSON, no brew or network.

## Adoption engine

Per Adoptable app, a two-step state machine:

1. Try `brew install --cask --adopt <token>`. Success → managed.
2. On artifact-mismatch failure, report "needs reinstall" — escalate to
   `brew install --cask --force` (overwrites the bundle) only when the user
   passed `--force`. App data is safe either way (lives in `~/Library`), but
   replacing a possibly-pinned binary must never happen implicitly.

After all adoptions: one `brew upgrade --cask <adopted tokens...>` to converge
disk state with brew records. Currently-running apps are adopted but their
upgrade is deferred with a note (unless `--yes`).

**MAS replacement** (`--include-mas`): per app — quit check, move bundle to
Trash (never `rm -rf`), then `brew install --cask <token>`. Preceded by loud
receipts/IAP warning and per-run confirmation.

**Failure isolation:** each app converts independently; one failure logs and
continues. Run ends with a summary table (adopted / upgraded / skipped /
failed) and a JSON log in `~/.local/state/brewmaster/`.

## Testing

Seam: outside-world interactions (brew subprocess, filesystem scan, cask API)
behind small interfaces; everything else pure.

- **Unit (bulk):** matcher (table-driven), provenance classifier (fixture
  `.app` trees in `t.TempDir()` with fake Info.plist/_MASReceipt), report
  formatting (golden files for table + JSON).
- **Integration (opt-in, `//go:build integration`):** real `brew` runs,
  exercising `brew info --json=v2` parsing — brew's JSON shape changing is the
  likeliest real-world breakage.
- **Adoption engine:** fake command-runner records invocations and scripts
  failures; verifies adopt → fallback → upgrade state machine with no real
  system mutation.

## Distribution

`patrickserrano/homebrew-tap` with binary bottles via GoReleaser (automates tap
formula, signing, GitHub Releases). Install:
`brew install patrickserrano/tap/brewmaster`. Versioning via git tags; CI on
GitHub Actions (unit suite, `go vet`, `golangci-lint`).

## Naming note

No formula or cask named `brewmaster` exists in Homebrew core (verified against
formulae.brew.sh, 2026-06-11). Two dormant GitHub projects share the name in
the Homebrew-tooling space (tpitale/brewmaster — Ruby gem, config-driven
installs; marhar/brewmaster — Dropbox sync), neither active nor distributed via
brew. Name is usable; rename is trivial pre-release if desired.

## v1 scope cut line

In: audit, adopt, `--include-mas`, `--dry-run`, `--json`, `--force`, `--yes`.
Deferred: undo command, config file, non-app artifacts, MacPorts/nix detection,
watch/cron mode.
