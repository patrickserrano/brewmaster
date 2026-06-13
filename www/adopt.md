# `brewmaster adopt`

`adopt` is the action half of brewmaster. It takes the adoptable apps from the
audit and runs `brew install --cask --adopt <token>` on each, then upgrades them
so the on-disk version matches what Homebrew records.

```
Convert unmanaged apps to Homebrew-managed via --adopt

Usage:
  brewmaster adopt [apps...] [flags]

Flags:
      --cask string   explicit cask token for a single (ambiguous) app
      --dry-run       print planned actions without executing
      --force         reinstall when adopt fails on artifact mismatch (overwrites the app bundle)
  -h, --help          help for adopt
      --include-mas   also replace Mac App Store apps (loses MAS receipts/IAP; app data survives)
      --yes           skip confirmations; upgrade running apps too
```

With no arguments, `adopt` acts on **every** high-confidence (adoptable) match.
Pass app names as positional arguments to narrow it down — `brewmaster adopt
Figma Docker` adopts just those two. The `.app` suffix is optional.

## Preview first with `--dry-run`

`--dry-run` prints the exact commands brewmaster would run and changes nothing.
Start here.

```
$ brewmaster adopt --dry-run
would run: brew install --cask --adopt alcove  # Alcove.app
would run: brew install --cask --adopt chatgpt  # ChatGPT.app
would run: brew install --cask --adopt claude  # Claude.app
would run: brew install --cask --adopt docker-desktop  # Docker.app
would run: brew install --cask --adopt figma  # Figma.app
...
```

With `--include-mas`, dry-run also shows the planned MAS replacements:

```
would replace MAS app: Ivory.app -> brew install --cask ivory
```

## Flags

| Flag | Effect |
| --- | --- |
| `--dry-run` | Print planned actions without executing anything. |
| `--include-mas` | Also replace Mac App Store apps with their cask versions (see [Mac App Store](mac-app-store.md)). |
| `--force` | When an adopt fails on an artifact mismatch, escalate to a forced reinstall that overwrites the app bundle. |
| `--yes` | Skip confirmations; upgrade apps even if they appear to be running. |
| `--cask <token>` | Explicit cask token for a single (ambiguous) app. |

## Resolving ambiguous apps with `--cask`

When the audit lists an app under **AMBIGUOUS**, brewmaster won't guess. Pick
the right token yourself:

```sh
brewmaster adopt --cask iterm2 iTerm
```

`--cask` requires exactly one app argument, and only targets adoptable or
ambiguous apps. It refuses to point at a managed app, an Apple system app, or a
Mac App Store app (for MAS, use `--include-mas` instead).

## The adoption state machine

For each app, `adopt` runs a small two-step state machine:

1. Try `brew install --cask --adopt <token>`. Success → **adopted**.
2. If adopt fails because the on-disk bundle doesn't match the cask's expected
   artifact, brewmaster reports **needs-reinstall** and stops — unless you
   passed `--force`, in which case it escalates to `brew install --cask --force`
   (overwriting the bundle) and reports **reinstalled**.

Each app is converted independently: one failure logs and the run continues.

## Post-adopt upgrade and running-app deferral

After adopting, brewmaster runs a single `brew upgrade --cask <tokens...>` so the
disk state converges with Homebrew's records (adopt itself never re-downloads).

Apps that appear to be **running** at upgrade time have their upgrade *deferred*
rather than forced — upgrading a live app can be disruptive. brewmaster prints:

```
deferred upgrades (apps appear to be running; re-run with --yes or quit them): figma, docker-desktop
```

Quit the apps and re-run, or pass `--yes` to upgrade them anyway.

At the end of a run, `adopt` prints a summary:

```
summary: 24 adopted, 0 reinstalled, 0 needs-reinstall, 0 failed
```

## The state log

Every non-dry-run `adopt` writes a JSON record of its results to:

```
~/.local/state/brewmaster/adopt-<timestamp>.json
```

This is best-effort forensics — if the log can't be written, the adoption still
proceeds. Each record captures the app, the cask token, the outcome, and any
error text, so you can reconstruct exactly what happened in a past run.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | All requested work was done (or there was nothing to do). |
| `1` | Positional arguments matched no adoptable app. |
| `2` | Error (brew missing, catalog fetch failed, etc.). |
