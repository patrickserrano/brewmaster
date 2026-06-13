# Contributing

Thanks for your interest in brewmaster. This is a small, focused Go CLI; the bar
is high-quality, well-tested changes that respect the safety posture (nothing
destructive without an explicit opt-in).

## Development setup

- **Go 1.22+** (matching `go.mod`). No other toolchain is required to build or
  test.
- **macOS + Homebrew** for running the binary against real apps. The unit tests
  themselves don't require Homebrew.

```sh
git clone https://github.com/patrickserrano/brewmaster.git
cd brewmaster
go build -o brewmaster .
```

## Running tests

```sh
go test ./...                         # unit suite
go vet ./...                          # vet
go test -tags integration ./internal/brew/   # integration tests (real `brew`)
```

The `integration` build tag gates tests that shell out to a real `brew` and
assert its `brew info --json=v2` output still parses — brew's JSON shape drifting
is the likeliest real-world breakage. These run on CI but are opt-in locally.

The external seams (the `brew` subprocess, filesystem scan, and cask API) sit
behind small interfaces so the matcher, classifier, and report formatting can be
tested as pure functions with fixtures and golden files — no network or system
mutation. Keep that seam intact when adding features.

## Quality bar

- **TDD.** Write the failing test first, then the code to make it pass.
- **Coverage ~90%+** on testable logic. New behavior needs tests; bug fixes need
  a regression test.
- **Linting.** `golangci-lint` (v2 config in `.golangci.yml`) must pass. CI runs
  it; run it locally before pushing if you have it installed.
- **Conventional Commits.** Use `feat:`, `fix:`, `docs:`, `test:`, `refactor:`,
  etc. for commit messages.

## Previewing the docs

The documentation site is built with [MkDocs](https://www.mkdocs.org/) and the
`mkdocs-shadcn` theme. Sources live in `www/` (the `docs/plans/` design docs are
intentionally **not** published).

```sh
python3 -m venv .venv
.venv/bin/pip install -r requirements-docs.txt
.venv/bin/mkdocs serve     # live-reload preview at http://127.0.0.1:8000
.venv/bin/mkdocs build     # one-off build into site/
```

The site deploys to GitHub Pages automatically on push to `main` via
`.github/workflows/docs.yml`.
