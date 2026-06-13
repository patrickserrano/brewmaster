# How it works

brewmaster's job is to look at the apps on your disk, figure out where each one
came from, and — for the ones Homebrew could manage but doesn't — find the right
cask token with enough confidence to act safely. It does this in three stages:
detection, matching, and (for `adopt`) the adoption engine.

## Detection signals

brewmaster scans `.app` bundles in `/Applications`, `/Applications/Utilities`,
and `~/Applications`, one level deep. It never descends into `/System`. For each
bundle it reads `Contents/Info.plist` for the bundle ID, version
(`CFBundleShortVersionString`), display name, and executable name
(`CFBundleExecutable`, used later to detect running apps).

Every app is then classified by provenance using local, offline signals:

| Signal | Result |
| --- | --- |
| Bundle ID starts with `com.apple.` | **System** — excluded from the report entirely. |
| `Contents/_MASReceipt/receipt` exists | **App Store** — canonical and offline; never touched without `--include-mas`. |
| Path is claimed by an installed cask's `app` artifact | **Managed** — already Homebrew-owned. |
| Everything else | **Unmanaged** — a candidate for matching. |

Managed apps are discovered from a single `brew info --json=v2 --installed`
call, which yields each installed cask's declared `app` artifacts; brewmaster
matches by name.

## The confidence-tiered matcher

For each unmanaged app, the matcher is a pure function — `(app metadata, cask
index) → match` — that assigns one of three tiers:

- **High** (auto-adoptable): the on-disk bundle name exactly matches one cask's
  declared `.app` artifact, **and** either the app's bundle ID agrees with the
  cask's declared bundle ID(s) or no other cask claims that artifact name.
- **Ambiguous**: multiple casks declare the same artifact (e.g. `iterm2`,
  `iterm2@beta`, `iterm2@nightly`), or the only signal is a fuzzy bundle-ID
  match. These are listed with their candidates and never auto-adopted.
- **None**: no cask claims the app → **Unmatched**.

### Tie-breakers for variants

When several casks claim the same artifact name, brewmaster tries to break the
tie before giving up and marking the app ambiguous:

1. **Bundle ID.** If exactly one candidate's declared quit-stanza bundle ID
   matches the app's bundle ID, that candidate wins at **High**.
2. **Version.** Among only the candidates whose bundle IDs don't *disagree* with
   the app's, if exactly one candidate's cask version matches the app's version,
   it wins at **High**. Comparison ignores Homebrew build metadata after a comma,
   so a catalog version of `4.39.0,123` matches an on-disk `4.39.0`.

A bundle-ID-only match (where the artifact *name* differs — say, because you
renamed the bundle) is informative but **never** auto-adopted: brewmaster surfaces
it as ambiguous so you can confirm.

## The cached catalog

Matching runs against the full Homebrew cask catalog from
`https://formulae.brew.sh/api/cask.json` (~25 MB). brewmaster caches it for 24
hours at `~/.cache/brewmaster/cask.json`. From two index maps built off that
catalog:

- **artifact map** — `"Visual Studio Code.app"` → `visual-studio-code`
- **bundle-ID map** — `com.microsoft.VSCode` → `visual-studio-code` (from
  `uninstall`/`zap` `quit:` stanzas, where present)

### Offline behavior

If the cache is fresh (under 24 hours old), brewmaster uses it without touching
the network. If the cache is stale and the network fetch fails, it falls back to
the stale cache and prints `warning: using stale cask catalog (network
unavailable)` to stderr rather than failing outright. Only well-formed JSON is
ever written to the cache, so a garbage HTTP response can't poison it.

## The adoption engine

When you run `adopt`, each app goes through the two-step state machine described
on the [adopt page](adopt.md#the-adoption-state-machine): try `--adopt`, and on
an artifact-mismatch failure either stop at *needs-reinstall* or (with `--force`)
escalate to a forced reinstall. A final `brew upgrade --cask` converges disk
state with Homebrew's records, deferring upgrades for any app that appears to be
running. Running detection uses `pgrep -xq <CFBundleExecutable>`; if the
executable name is unknown, brewmaster fails closed (it won't trash or upgrade an
app it can't prove is quit).
