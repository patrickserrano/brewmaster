# FAQ

## Is my data safe?

Yes. For hand-installed apps, `adopt` uses `brew install --cask --adopt`, which
takes ownership of the *existing* bundle in place — it doesn't reinstall or
delete it. Either way, your app data lives in `~/Library` (preferences,
databases, documents) and is never touched by adoption.

For Mac App Store apps converted with `--include-mas`, the old bundle is moved to
the **Trash** (recoverable, never `rm -rf`) and the cask is installed fresh. App
data in `~/Library` still survives — but you do lose App Store receipts and
possibly in-app purchases. See [Mac App Store](mac-app-store.md).

## What if a match is wrong?

brewmaster only auto-adopts **high-confidence** matches: an exact artifact-name
match with no conflicting signals. Anything with more than one candidate, or only
a fuzzy match, is flagged **ambiguous** and never adopted automatically — you
pick the token yourself with `--cask`.

Want to see everything before acting? Run `brewmaster audit` (read-only) and
`brewmaster adopt --dry-run` (prints the exact commands, changes nothing). If an
adopt does the wrong thing, the bundle isn't deleted — `--adopt` takes ownership
in place — and the run is recorded in `~/.local/state/brewmaster/`.

## What about Mac App Store apps?

They're detected and reported, but never touched by default. The
`Contents/_MASReceipt/receipt` file is a canonical, offline signal that an app
came from the App Store. Converting one to a cask is destructive, so it requires
the `--include-mas` flag plus a typed `yes` confirmation. Full details on the
[Mac App Store page](mac-app-store.md).

## Does it need Go?

No. brewmaster ships as a single self-contained binary that requires no Go at
runtime — just macOS and Homebrew. Go is only needed if you choose to install
via `go install` or build from source. See [Installation](installation.md).

## How does it behave offline?

brewmaster caches the Homebrew cask catalog for 24 hours at
`~/.cache/brewmaster/cask.json`. If the cache is fresh, it never touches the
network. If the cache is stale and the network is unavailable, it falls back to
the stale cache and warns on stderr rather than failing. The first run (or the
first run after 24 hours) does need network to download the catalog. See
[How it works](how-it-works.md#offline-behavior).

## Why doesn't an app I installed by hand show up as adoptable?

A few common reasons:

- **No matching cask exists.** It'll appear under **UNMATCHED** in the audit.
- **It's an Apple system app** (`com.apple.*` bundle ID) — those are excluded
  entirely.
- **It's ambiguous.** If several casks claim the same name (variants like
  `@beta`/`@nightly`), it's listed under **AMBIGUOUS** with candidates; adopt it
  with `brewmaster adopt --cask <token> "<App>"`.
- **It's already managed.** Run `brewmaster audit --verbose` to see managed apps.

## Can I adopt just one app?

Yes — pass its name as an argument: `brewmaster adopt Figma`. The `.app` suffix
is optional. For an ambiguous app, use `brewmaster adopt --cask <token> "<App>"`.
