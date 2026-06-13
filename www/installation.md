# Installation

brewmaster is a single self-contained binary. It runs `brew` and reads your
`/Applications` folders, so it requires Homebrew and macOS — but it does **not**
require Go at runtime.

## Homebrew tap (recommended)

```sh
brew install patrickserrano/tap/brewmaster
```

This installs a notarization-free, prebuilt binary as a Homebrew cask; the
cask strips the macOS quarantine attribute on install, so `brewmaster` runs
immediately. To upgrade later: `brew upgrade --cask brewmaster`.

## go install

If you have a Go toolchain, you can install the latest from source directly:

```sh
go install github.com/patrickserrano/brewmaster@latest
```

This drops the `brewmaster` binary in `$(go env GOPATH)/bin` — make sure that
directory is on your `PATH`.

## Build from source

```sh
git clone https://github.com/patrickserrano/brewmaster.git
cd brewmaster
go build -o brewmaster .
./brewmaster audit
```

The build produces a static binary (CGO is disabled) for Apple Silicon and
Intel Macs. See [CONTRIBUTING](https://github.com/patrickserrano/brewmaster/blob/main/CONTRIBUTING.md)
for the development workflow.

## Requirements

- **macOS** — brewmaster scans `.app` bundles and is macOS-only.
- **Homebrew** — `brew` must be installed and on your `PATH`. brewmaster calls
  `brew info --json=v2 --installed` to discover managed casks; if `brew` is
  missing, the command exits with an error (exit code `2`).
- **Network (first run / once a day)** — brewmaster downloads the Homebrew cask
  catalog and caches it for 24 hours. After that it works offline against the
  cache. See [How it works](how-it-works.md#the-cached-catalog).
