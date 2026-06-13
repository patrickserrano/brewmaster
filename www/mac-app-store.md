# Mac App Store apps

Apps installed from the Mac App Store (MAS) are detected and reported by
brewmaster, but **never touched by default**. Converting a MAS app to a Homebrew
cask is genuinely destructive in ways that adopting a hand-installed app is not,
so it lives behind an explicit flag and a typed confirmation.

## What `--include-mas` does

With `--include-mas`, `brewmaster adopt` will replace a MAS app with its cask
version. For each MAS app that has a confident cask match, brewmaster:

1. Refuses if the app is currently running (quit it first).
2. Moves the existing MAS bundle to the **Trash** — never `rm -rf`, so it's
   recoverable.
3. Runs `brew install --cask <token>` to install the cask version fresh.

## The warning, and what you lose

/// admonition | You will lose App Store metadata
    type: warning

Replacing a Mac App Store app with a cask means the new copy is **not** an App
Store install. You will lose:

- **App Store receipts** (`Contents/_MASReceipt/receipt`)
- **App Store auto-updates** — updates now come from Homebrew, not the App Store
- **Possibly in-app purchases** tied to the App Store receipt

Your **app data in `~/Library` is preserved** — preferences, databases, and
documents survive the swap. The old bundle goes to the Trash, not the void, so
you can restore it if something goes wrong.
///

## The typed confirmation

Before doing any of this, brewmaster prints the warning and requires you to type
`yes` (not just `y`):

```
WARNING: replacing 3 App Store app(s) with cask versions.
You will LOSE: App Store receipts, App Store auto-updates, and possibly in-app purchases.
App data in ~/Library is preserved. Old bundles go to the Trash.
Type 'yes' to continue:
```

Anything other than `yes` skips the MAS conversion entirely (the rest of the run
is unaffected). Passing `--yes` skips this prompt — use it only when you already
know exactly what will happen.

## Why it's opt-in

Adopting a hand-installed app via `--adopt` is non-destructive: it takes
ownership of the bundle in place. A MAS conversion is a delete-and-reinstall in
disguise. That asymmetry is why the default `adopt` leaves MAS apps alone and
only the explicit `--include-mas` flag — gated by a typed confirmation and a
running-app check — will replace them.

To preview which MAS apps would be replaced without changing anything, combine
the flags:

```sh
brewmaster adopt --include-mas --dry-run
```
