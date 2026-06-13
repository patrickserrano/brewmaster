# `brewmaster audit`

`audit` is the read-only half of brewmaster. It scans your `/Applications` and
`~/Applications` folders, asks Homebrew which casks it already manages, matches
everything else against the cask catalog, and prints a report. It changes
nothing.

```
Report which apps are Homebrew-managed, adoptable, or App Store-installed

Usage:
  brewmaster audit [flags]

Flags:
  -h, --help      help for audit
      --json      output JSON
  -v, --verbose   list managed apps too
```

## The five buckets

Every app lands in exactly one bucket:

| Bucket | Meaning |
| --- | --- |
| **Managed** | Already owned by a Homebrew cask. Counted by default, listed with `--verbose`. |
| **Adoptable** | Unmanaged, with exactly one high-confidence cask match. These are what `adopt` acts on. |
| **Ambiguous** | Multiple candidate casks (e.g. `@beta`/`@nightly` variants) or a weak match. Listed with candidates; never auto-adopted. |
| **App Store** | Installed from the Mac App Store. Untouched unless you pass `--include-mas` to `adopt`. |
| **Unmatched** | No cask exists for it (Apple system apps are excluded from the report entirely). |

## Human-readable output

```
26 app(s) already managed by Homebrew

ADOPTABLE
  Alcove.app                1.7.2                  alcove
  ChatGPT.app               1.2026.104             chatgpt
  Claude.app                1.12603.1              claude
  Docker.app                4.76.0                 docker-desktop
  Figma.app                 116.5.18               figma
  HandBrake.app             1.10.2                 handbrake-app
  ...

AMBIGUOUS (use: brewmaster adopt --cask <token> "<App>")
  Bartender 5.app      5.5.8        bartender
  Kaleidoscope.app     6.3          kaleidoscope, kaleidoscope@2, kaleidoscope@3
  iTerm.app            3.6.1        iterm2, iterm2@beta, iterm2@nightly
  logioptionsplus.app  1.98.809639  logi-options+

APP STORE (untouched; use --include-mas to convert)
  1Blocker.app               6.5.3
  1Password for Safari.app   8.12.12
  ...
```

Each row shows the on-disk app, its version (`CFBundleShortVersionString`), and
the matched cask token(s).

Pass `--verbose` (`-v`) to also list every Homebrew-managed app instead of just
counting them.

If a network fetch fails and brewmaster falls back to a cached catalog, it
prints `warning: using stale cask catalog (network unavailable)` to stderr.

## JSON output

`--json` emits the same report as a machine-readable object. Combine with
`--verbose` to include the managed apps.

```sh
brewmaster audit --json
```

```json
{
  "managed_count": 26,
  "adoptable": [
    { "app": "Alcove.app", "token": "alcove", "version": "1.7.2" },
    { "app": "ChatGPT.app", "token": "chatgpt", "version": "1.2026.104" }
  ],
  "ambiguous": [
    { "app": "Bartender 5.app", "version": "5.5.8", "candidates": ["bartender"] },
    { "app": "iTerm.app", "version": "3.6.1",
      "candidates": ["iterm2", "iterm2@beta", "iterm2@nightly"] }
  ],
  "app_store": [
    { "app": "1Blocker.app", "version": "6.5.3" }
  ],
  "unmatched": [
    { "app": "Battle.net.app", "version": "2.27.0.14543" }
  ]
}
```

## Exit codes

`audit` is designed to double as a drift detector — its exit code tells a script
whether anything needs attention without parsing output.

| Code | Meaning |
| --- | --- |
| `0` | Clean — nothing adoptable, ambiguous, or unmatched. |
| `1` | Drift — adoptable/ambiguous/unmatched apps exist. |
| `2` | Error (brew missing, catalog fetch failed, etc.). |

## Use it as a drift detector

Because a non-zero exit signals "there are apps Homebrew could be managing but
isn't," `audit` slots straight into CI, cron, or a shell prompt:

```sh
# Cron: nightly nudge if drift appears
brewmaster audit > /dev/null || echo "brewmaster: unmanaged apps detected" | mail -s drift you@example.com
```

```sh
# Makefile / pre-commit style gate
brewmaster audit --json | jq -e '.adoptable | length == 0' > /dev/null \
  || { echo "Adoptable apps found — run 'brewmaster adopt'"; exit 1; }
```
