# Brewmaster Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** A Go CLI that audits installed macOS apps, matches unmanaged apps to Homebrew casks with confidence tiers, and converts them via `brew install --cask --adopt`, with MAS apps behind a flag.

**Architecture:** Pure logic (matching, classification, reporting) lives in `internal/` packages behind small interfaces; all outside-world interaction (brew subprocesses, filesystem, HTTP cask catalog) is injected so unit tests never touch the real system. Two cobra commands (`audit`, `adopt`) compose the same scan→classify→match pipeline; `adopt` adds the adoption engine state machine.

**Tech Stack:** Go 1.22+, spf13/cobra (CLI), howett.net/plist (Info.plist parsing), stdlib `text/tabwriter` + `encoding/json` (output). Design doc: `docs/plans/2026-06-11-brewmaster-design.md` — read it first.

**Conventions for every task:** Run tests with `go test ./... -run <TestName> -v` from repo root. Commit messages use conventional commits (`feat:`, `test:`, `chore:`). TDD is mandatory: write the failing test, see it fail, implement, see it pass, commit.

---

## Task 1: Project scaffold + cobra root command

**Files:**
- Create: `go.mod`, `main.go`, `cmd/root.go`
- Test: `cmd/root_test.go`

**Step 1: Initialize module and dependencies**

```bash
cd /Users/patrickserrano/Developer/brewmaster
go mod init github.com/patrickserrano/brewmaster
go get github.com/spf13/cobra@latest
go get howett.net/plist@latest
```

**Step 2: Write the failing test**

`cmd/root_test.go`:
```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootShowsHelp(t *testing.T) {
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "brewmaster") {
		t.Errorf("help output missing program name: %q", out.String())
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./cmd/ -v`
Expected: FAIL — `undefined: NewRootCmd`

**Step 4: Write minimal implementation**

`cmd/root.go`:
```go
package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "brewmaster",
		Short: "Audit installed macOS apps and adopt them into Homebrew",
		SilenceUsage: true,
	}
	return root
}
```

`main.go`:
```go
package main

import (
	"os"

	"github.com/patrickserrano/brewmaster/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(2)
	}
}
```

**Step 5: Verify pass, tidy, commit**

```bash
go test ./... && go vet ./... && go mod tidy
git add -A && git commit -m "feat: scaffold brewmaster CLI with cobra root command"
```

---

## Task 2: Brew runner interface + installed-cask parsing

The seam for all subprocess work. `Runner` is the interface everything else fakes in tests.

**Files:**
- Create: `internal/brew/runner.go`, `internal/brew/installed.go`
- Test: `internal/brew/installed_test.go`, fixture `internal/brew/testdata/info_installed.json`

**Step 1: Create the fixture**

`internal/brew/testdata/info_installed.json` (trimmed real shape of `brew info --json=v2 --installed`):
```json
{
  "formulae": [],
  "casks": [
    {
      "token": "visual-studio-code",
      "version": "1.100.0",
      "artifacts": [
        { "app": ["Visual Studio Code.app"] },
        { "binary": ["something"] }
      ]
    },
    {
      "token": "raycast",
      "version": "1.99.0",
      "artifacts": [
        { "app": ["Raycast.app"] },
        { "zap": [{ "trash": ["~/Library/Caches/com.raycast.macos"] }] }
      ]
    }
  ]
}
```

**Step 2: Write the failing test**

`internal/brew/installed_test.go`:
```go
package brew

import (
	"os"
	"testing"
)

func TestParseInstalledCasks(t *testing.T) {
	data, err := os.ReadFile("testdata/info_installed.json")
	if err != nil {
		t.Fatal(err)
	}
	casks, err := ParseInstalledCasks(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(casks) != 2 {
		t.Fatalf("want 2 casks, got %d", len(casks))
	}
	if casks[0].Token != "visual-studio-code" || casks[0].Version != "1.100.0" {
		t.Errorf("cask[0] = %+v", casks[0])
	}
	if len(casks[0].Apps) != 1 || casks[0].Apps[0] != "Visual Studio Code.app" {
		t.Errorf("cask[0].Apps = %v", casks[0].Apps)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/brew/ -v`
Expected: FAIL — `undefined: ParseInstalledCasks`

**Step 4: Write minimal implementation**

`internal/brew/runner.go`:
```go
package brew

import (
	"context"
	"os/exec"
)

// Runner executes external commands. Production uses ExecRunner;
// tests inject fakes that record invocations and script outputs.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
```

`internal/brew/installed.go`:
```go
package brew

import (
	"context"
	"encoding/json"
	"strings"
)

type InstalledCask struct {
	Token   string
	Version string
	Apps    []string // .app artifact names this cask owns
}

type infoV2 struct {
	Casks []struct {
		Token     string            `json:"token"`
		Version   string            `json:"version"`
		Artifacts []json.RawMessage `json:"artifacts"`
	} `json:"casks"`
}

func ParseInstalledCasks(data []byte) ([]InstalledCask, error) {
	var info infoV2
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	out := make([]InstalledCask, 0, len(info.Casks))
	for _, c := range info.Casks {
		out = append(out, InstalledCask{
			Token:   c.Token,
			Version: c.Version,
			Apps:    appArtifacts(c.Artifacts),
		})
	}
	return out, nil
}

// appArtifacts extracts .app names from a cask artifacts array, whose
// entries are heterogeneous objects like {"app": ["Foo.app"]}.
func appArtifacts(artifacts []json.RawMessage) []string {
	var apps []string
	for _, raw := range artifacts {
		var entry map[string]json.RawMessage
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		appRaw, ok := entry["app"]
		if !ok {
			continue
		}
		var vals []any
		if json.Unmarshal(appRaw, &vals) != nil {
			continue
		}
		for _, v := range vals {
			if s, ok := v.(string); ok && strings.HasSuffix(s, ".app") {
				apps = append(apps, s)
			}
		}
	}
	return apps
}

func InstalledCasks(ctx context.Context, r Runner) ([]InstalledCask, error) {
	data, err := r.Run(ctx, "brew", "info", "--json=v2", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseInstalledCasks(data)
}
```

**Step 5: Verify pass, commit**

```bash
go test ./internal/brew/ -v
git add -A && git commit -m "feat: brew runner seam and installed-cask JSON parsing"
```

---

## Task 3: App bundle reading (Info.plist + MAS receipt)

**Files:**
- Create: `internal/scan/app.go`
- Test: `internal/scan/app_test.go`

**Step 1: Write the failing test**

Test fixtures are built on the fly in `t.TempDir()` — an `.app` is just a directory tree. Helper builds one with an XML Info.plist (howett.net/plist reads XML and binary identically).

`internal/scan/app_test.go`:
```go
package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeApp(t *testing.T, dir, name, bundleID, version string, mas bool) string {
	t.Helper()
	app := filepath.Join(dir, name)
	contents := filepath.Join(app, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleIdentifier</key><string>` + bundleID + `</string>
  <key>CFBundleShortVersionString</key><string>` + version + `</string>
  <key>CFBundleName</key><string>Fixture</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if mas {
		receiptDir := filepath.Join(contents, "_MASReceipt")
		if err := os.MkdirAll(receiptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(receiptDir, "receipt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestReadApp(t *testing.T) {
	dir := t.TempDir()
	path := writeApp(t, dir, "Slack.app", "com.tinyspeck.slackmacgap", "4.39.0", false)

	app, err := ReadApp(path)
	if err != nil {
		t.Fatalf("ReadApp: %v", err)
	}
	if app.Name != "Slack.app" || app.BundleID != "com.tinyspeck.slackmacgap" ||
		app.Version != "4.39.0" || app.MASReceipt {
		t.Errorf("got %+v", app)
	}
}

func TestReadAppDetectsMASReceipt(t *testing.T) {
	dir := t.TempDir()
	path := writeApp(t, dir, "Things3.app", "com.culturedcode.ThingsMac", "3.20", true)

	app, err := ReadApp(path)
	if err != nil {
		t.Fatal(err)
	}
	if !app.MASReceipt {
		t.Error("expected MASReceipt=true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/scan/ -v`
Expected: FAIL — `undefined: ReadApp`

**Step 3: Write minimal implementation**

`internal/scan/app.go`:
```go
package scan

import (
	"os"
	"path/filepath"

	"howett.net/plist"
)

type App struct {
	Path        string
	Name        string // bundle dir name, e.g. "Slack.app"
	BundleID    string
	Version     string
	DisplayName string
	MASReceipt  bool
}

type infoPlist struct {
	BundleID    string `plist:"CFBundleIdentifier"`
	Version     string `plist:"CFBundleShortVersionString"`
	DisplayName string `plist:"CFBundleName"`
}

func ReadApp(path string) (App, error) {
	data, err := os.ReadFile(filepath.Join(path, "Contents", "Info.plist"))
	if err != nil {
		return App{}, err
	}
	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return App{}, err
	}
	_, receiptErr := os.Stat(filepath.Join(path, "Contents", "_MASReceipt", "receipt"))
	return App{
		Path:        path,
		Name:        filepath.Base(path),
		BundleID:    info.BundleID,
		Version:     info.Version,
		DisplayName: info.DisplayName,
		MASReceipt:  receiptErr == nil,
	}, nil
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/scan/ -v
git add -A && git commit -m "feat: read app bundle metadata and MAS receipt marker"
```

---

## Task 4: Directory scanning

**Files:**
- Modify: `internal/scan/app.go` (or create `internal/scan/scan.go`)
- Test: append to `internal/scan/app_test.go`

**Step 1: Write the failing test**

```go
func TestScanDirsFindsAppsOneLevelDeep(t *testing.T) {
	dir := t.TempDir()
	writeApp(t, dir, "Slack.app", "com.tinyspeck.slackmacgap", "4.39.0", false)
	writeApp(t, dir, "Things3.app", "com.culturedcode.ThingsMac", "3.20", true)
	// Non-app dir and nested app must be ignored.
	os.MkdirAll(filepath.Join(dir, "NotAnApp"), 0o755)
	nested := filepath.Join(dir, "SomeFolder")
	os.MkdirAll(nested, 0o755)
	writeApp(t, nested, "Hidden.app", "com.x.hidden", "1.0", false)

	apps, err := ScanDirs([]string{dir, filepath.Join(dir, "does-not-exist")})
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("want 2 apps, got %d: %+v", len(apps), apps)
	}
}
```

**Step 2: Run to verify it fails** — `undefined: ScanDirs`

**Step 3: Implement**

`internal/scan/scan.go`:
```go
package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultDirs are the locations brewmaster audits. ~ is expanded by the caller.
func DefaultDirs(home string) []string {
	return []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}
}

// ScanDirs enumerates .app bundles one level deep in each dir.
// Missing dirs and unreadable bundles are skipped, not errors.
func ScanDirs(dirs []string) ([]App, error) {
	var apps []App
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			app, err := ReadApp(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			apps = append(apps, app)
		}
	}
	return apps, nil
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/scan/ -v
git add -A && git commit -m "feat: scan application directories for app bundles"
```

---

## Task 5: Provenance classification

**Files:**
- Create: `internal/scan/classify.go`
- Test: `internal/scan/classify_test.go`

**Step 1: Write the failing test**

```go
package scan

import "testing"

func TestClassify(t *testing.T) {
	brewOwned := map[string]string{"visual studio code.app": "visual-studio-code"}
	cases := []struct {
		name string
		app  App
		want Provenance
	}{
		{"brew-managed", App{Name: "Visual Studio Code.app", BundleID: "com.microsoft.VSCode"}, Managed},
		{"mas", App{Name: "Things3.app", BundleID: "com.culturedcode.ThingsMac", MASReceipt: true}, AppStore},
		{"apple system", App{Name: "Safari.app", BundleID: "com.apple.Safari"}, System},
		{"unmanaged", App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap"}, Unmanaged},
		// MAS receipt wins over brew ownership (shouldn't co-occur, but be deterministic).
		{"mas wins", App{Name: "Visual Studio Code.app", MASReceipt: true}, AppStore},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.app, brewOwned); got != c.want {
				t.Errorf("Classify(%s) = %v, want %v", c.app.Name, got, c.want)
			}
		})
	}
}
```

**Step 2: Run to verify it fails** — `undefined: Classify`

**Step 3: Implement**

`internal/scan/classify.go`:
```go
package scan

import "strings"

type Provenance int

const (
	Unmanaged Provenance = iota
	Managed
	AppStore
	System
)

func (p Provenance) String() string {
	return [...]string{"unmanaged", "managed", "app-store", "system"}[p]
}

// Classify determines an app's provenance. brewOwned maps lowercased
// .app artifact names to the owning cask token.
func Classify(app App, brewOwned map[string]string) Provenance {
	switch {
	case strings.HasPrefix(app.BundleID, "com.apple."):
		return System
	case app.MASReceipt:
		return AppStore
	default:
		if _, ok := brewOwned[strings.ToLower(app.Name)]; ok {
			return Managed
		}
		return Unmanaged
	}
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/scan/ -v
git add -A && git commit -m "feat: classify app provenance (managed/mas/system/unmanaged)"
```

---

## Task 6: Cask catalog parsing + index

**Files:**
- Create: `internal/caskindex/catalog.go`
- Test: `internal/caskindex/catalog_test.go`, fixture `internal/caskindex/testdata/cask.json`

**Step 1: Create the fixture**

`internal/caskindex/testdata/cask.json` — trimmed real shape of `https://formulae.brew.sh/api/cask.json` (an array). Include a variant collision (`slack` / `slack@beta`) and a quit-stanza bundle ID:
```json
[
  {
    "token": "visual-studio-code",
    "name": ["Microsoft Visual Studio Code", "VS Code"],
    "version": "1.100.0",
    "artifacts": [
      { "app": ["Visual Studio Code.app"] },
      { "uninstall": [{ "quit": "com.microsoft.VSCode" }] }
    ]
  },
  {
    "token": "slack",
    "name": ["Slack"],
    "version": "4.39.0",
    "artifacts": [{ "app": ["Slack.app"] }]
  },
  {
    "token": "slack@beta",
    "name": ["Slack Beta"],
    "version": "4.40.0-beta",
    "artifacts": [{ "app": ["Slack.app"] }]
  },
  {
    "token": "firefox",
    "name": ["Mozilla Firefox"],
    "version": "127.0",
    "artifacts": [
      { "app": ["Firefox.app"] },
      { "uninstall": [{ "quit": ["org.mozilla.firefox"] }] }
    ]
  }
]
```

**Step 2: Write the failing test**

`internal/caskindex/catalog_test.go`:
```go
package caskindex

import (
	"os"
	"testing"
)

func loadIndex(t *testing.T) Index {
	t.Helper()
	data, err := os.ReadFile("testdata/cask.json")
	if err != nil {
		t.Fatal(err)
	}
	casks, err := ParseCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	return BuildIndex(casks)
}

func TestParseCatalogExtractsQuitIDs(t *testing.T) {
	data, _ := os.ReadFile("testdata/cask.json")
	casks, err := ParseCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	byToken := map[string]Cask{}
	for _, c := range casks {
		byToken[c.Token] = c
	}
	if got := byToken["visual-studio-code"].QuitIDs; len(got) != 1 || got[0] != "com.microsoft.VSCode" {
		t.Errorf("vscode QuitIDs = %v", got)
	}
	// quit can be a string OR an array of strings
	if got := byToken["firefox"].QuitIDs; len(got) != 1 || got[0] != "org.mozilla.firefox" {
		t.Errorf("firefox QuitIDs = %v", got)
	}
}

func TestIndexLookups(t *testing.T) {
	idx := loadIndex(t)
	if got := idx.ByArtifact("Visual Studio Code.app"); len(got) != 1 || got[0].Token != "visual-studio-code" {
		t.Errorf("ByArtifact(vscode) = %+v", got)
	}
	if got := idx.ByArtifact("Slack.app"); len(got) != 2 {
		t.Errorf("ByArtifact(Slack.app) should collide with 2 casks, got %+v", got)
	}
	if got := idx.ByBundleID("com.microsoft.VSCode"); len(got) != 1 || got[0].Token != "visual-studio-code" {
		t.Errorf("ByBundleID = %+v", got)
	}
}
```

**Step 3: Run to verify it fails** — `undefined: ParseCatalog`

**Step 4: Implement**

`internal/caskindex/catalog.go`:
```go
package caskindex

import (
	"encoding/json"
	"strings"
)

type Cask struct {
	Token   string
	Names   []string
	Version string
	Apps    []string // declared .app artifacts
	QuitIDs []string // bundle IDs from uninstall/zap quit: stanzas
}

type rawCask struct {
	Token     string            `json:"token"`
	Name      []string          `json:"name"`
	Version   string            `json:"version"`
	Artifacts []json.RawMessage `json:"artifacts"`
}

func ParseCatalog(data []byte) ([]Cask, error) {
	var raws []rawCask
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	casks := make([]Cask, 0, len(raws))
	for _, r := range raws {
		c := Cask{Token: r.Token, Names: r.Name, Version: r.Version}
		for _, art := range r.Artifacts {
			var entry map[string]json.RawMessage
			if json.Unmarshal(art, &entry) != nil {
				continue
			}
			if appRaw, ok := entry["app"]; ok {
				c.Apps = append(c.Apps, stringsFromArray(appRaw, ".app")...)
			}
			for _, key := range []string{"uninstall", "zap"} {
				if raw, ok := entry[key]; ok {
					c.QuitIDs = append(c.QuitIDs, quitIDs(raw)...)
				}
			}
		}
		casks = append(casks, c)
	}
	return casks, nil
}

func stringsFromArray(raw json.RawMessage, suffix string) []string {
	var vals []any
	if json.Unmarshal(raw, &vals) != nil {
		return nil
	}
	var out []string
	for _, v := range vals {
		if s, ok := v.(string); ok && strings.HasSuffix(s, suffix) {
			out = append(out, s)
		}
	}
	return out
}

// quitIDs extracts quit: bundle IDs; the value may be a string or array.
func quitIDs(raw json.RawMessage) []string {
	var stanzas []map[string]json.RawMessage
	if json.Unmarshal(raw, &stanzas) != nil {
		return nil
	}
	var out []string
	for _, s := range stanzas {
		q, ok := s["quit"]
		if !ok {
			continue
		}
		var one string
		if json.Unmarshal(q, &one) == nil {
			out = append(out, one)
			continue
		}
		var many []string
		if json.Unmarshal(q, &many) == nil {
			out = append(out, many...)
		}
	}
	return out
}

type Index struct {
	byArtifact map[string][]Cask // key: lowercased .app name
	byBundleID map[string][]Cask // key: lowercased bundle ID
}

func BuildIndex(casks []Cask) Index {
	idx := Index{
		byArtifact: map[string][]Cask{},
		byBundleID: map[string][]Cask{},
	}
	for _, c := range casks {
		for _, app := range c.Apps {
			k := strings.ToLower(app)
			idx.byArtifact[k] = append(idx.byArtifact[k], c)
		}
		for _, id := range c.QuitIDs {
			k := strings.ToLower(id)
			idx.byBundleID[k] = append(idx.byBundleID[k], c)
		}
	}
	return idx
}

func (i Index) ByArtifact(app string) []Cask  { return i.byArtifact[strings.ToLower(app)] }
func (i Index) ByBundleID(id string) []Cask   { return i.byBundleID[strings.ToLower(id)] }
```

**Step 5: Verify pass, commit**

```bash
go test ./internal/caskindex/ -v
git add -A && git commit -m "feat: parse cask catalog and build artifact/bundle-id index"
```

---

## Task 7: Cached catalog fetch

**Files:**
- Create: `internal/caskindex/fetch.go`
- Test: `internal/caskindex/fetch_test.go`

**Step 1: Write the failing test**

Use `net/http/httptest` and a temp cache dir. Verify: first call hits HTTP and writes cache; second call within TTL serves from cache (server hit count stays 1); expired cache refetches.

```go
package caskindex

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchCachesCatalog(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`[{"token":"slack","name":["Slack"],"version":"1","artifacts":[{"app":["Slack.app"]}]}]`))
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cask.json")

	for i := 0; i < 2; i++ {
		data, err := Fetch(srv.URL, cache, 24*time.Hour)
		if err != nil {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if len(data) == 0 {
			t.Fatalf("fetch %d returned empty", i)
		}
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (second call should be cached)", hits)
	}

	// Expire the cache; next call must refetch.
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(cache, old, old)
	if _, err := Fetch(srv.URL, cache, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("server hits = %d, want 2 after cache expiry", hits)
	}
}
```

**Step 2: Run to verify it fails** — `undefined: Fetch`

**Step 3: Implement**

`internal/caskindex/fetch.go`:
```go
package caskindex

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultCatalogURL = "https://formulae.brew.sh/api/cask.json"

// DefaultCachePath returns ~/.cache/brewmaster/cask.json.
func DefaultCachePath(home string) string {
	return filepath.Join(home, ".cache", "brewmaster", "cask.json")
}

// Fetch returns the cask catalog, serving from cachePath when fresher than ttl.
func Fetch(url, cachePath string, ttl time.Duration) ([]byte, error) {
	if st, err := os.Stat(cachePath); err == nil && time.Since(st.ModTime()) < ttl {
		return os.ReadFile(cachePath)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog fetch: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		return nil, err
	}
	return data, nil
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/caskindex/ -v
git add -A && git commit -m "feat: fetch cask catalog with 24h file cache"
```

---

## Task 8: The matcher

The heart of the tool. Pure function, table-driven tests. Re-read the "Matching" section of the design doc before implementing.

**Files:**
- Create: `internal/match/match.go`
- Test: `internal/match/match_test.go`

**Step 1: Write the failing test**

```go
package match

import (
	"testing"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

func idx() caskindex.Index {
	return caskindex.BuildIndex([]caskindex.Cask{
		{Token: "visual-studio-code", Version: "1.100.0",
			Apps: []string{"Visual Studio Code.app"}, QuitIDs: []string{"com.microsoft.VSCode"}},
		{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
		{Token: "slack@beta", Version: "4.40.0-beta", Apps: []string{"Slack.app"}},
		{Token: "iterm2", Version: "3.5.0", Apps: []string{"iTerm.app"},
			QuitIDs: []string{"com.googlecode.iterm2"}},
	})
}

func TestMatchApp(t *testing.T) {
	cases := []struct {
		name      string
		app       scan.App
		wantTier  Tier
		wantToken string
	}{
		{"unique artifact match is High",
			scan.App{Name: "Visual Studio Code.app", BundleID: "com.microsoft.VSCode"},
			High, "visual-studio-code"},
		{"unique artifact, no bundle id in index, still High",
			scan.App{Name: "iTerm.app", BundleID: "com.googlecode.iterm2"},
			High, "iterm2"},
		{"variant collision resolved by version tie-breaker",
			scan.App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0"},
			High, "slack"},
		{"variant collision with unknown version is Ambiguous",
			scan.App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "9.9.9"},
			Ambiguous, ""},
		{"bundle-id-only match (renamed bundle) is Ambiguous",
			scan.App{Name: "VSCode Renamed.app", BundleID: "com.microsoft.VSCode"},
			Ambiguous, ""},
		{"no match is None",
			scan.App{Name: "TotallyCustom.app", BundleID: "com.example.custom"},
			None, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MatchApp(c.app, idx())
			if got.Tier != c.wantTier {
				t.Fatalf("tier = %v, want %v (match=%+v)", got.Tier, c.wantTier, got)
			}
			if got.Tier == High && got.Token != c.wantToken {
				t.Errorf("token = %q, want %q", got.Token, c.wantToken)
			}
			if got.Tier == Ambiguous && len(got.Candidates) == 0 {
				t.Error("ambiguous match must carry candidates")
			}
		})
	}
}
```

**Step 2: Run to verify it fails** — `undefined: MatchApp`

**Step 3: Implement**

`internal/match/match.go`:
```go
package match

import (
	"strings"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

type Tier int

const (
	None Tier = iota
	Ambiguous
	High
)

func (t Tier) String() string { return [...]string{"none", "ambiguous", "high"}[t] }

type Match struct {
	Tier       Tier
	Token      string   // set when Tier == High
	Candidates []string // set when Tier == Ambiguous
}

// MatchApp matches an unmanaged app to a cask token.
//
// High confidence requires the on-disk bundle name to exactly match a
// cask's declared app artifact AND either the bundle ID agrees or no
// other cask claims that artifact. Collisions between variants are
// tie-broken by bundle ID, then by exact version. Bundle-ID-only
// matches (artifact name differs, e.g. user renamed the bundle) are
// never auto-adopted.
func MatchApp(app scan.App, idx caskindex.Index) Match {
	candidates := idx.ByArtifact(app.Name)

	switch len(candidates) {
	case 0:
		// Fall back to bundle ID — informative, never auto-adoptable.
		byID := idx.ByBundleID(app.BundleID)
		if len(byID) > 0 {
			return Match{Tier: Ambiguous, Candidates: tokens(byID)}
		}
		return Match{Tier: None}
	case 1:
		c := candidates[0]
		if disagrees(app.BundleID, c.QuitIDs) {
			return Match{Tier: Ambiguous, Candidates: tokens(candidates)}
		}
		return Match{Tier: High, Token: c.Token}
	}

	// Multiple casks claim this artifact name. Tie-break: bundle ID, then version.
	if byID := filter(candidates, func(c caskindex.Cask) bool {
		return contains(c.QuitIDs, app.BundleID)
	}); len(byID) == 1 {
		return Match{Tier: High, Token: byID[0].Token}
	}
	if byVer := filter(candidates, func(c caskindex.Cask) bool {
		return app.Version != "" && c.Version == app.Version
	}); len(byVer) == 1 {
		return Match{Tier: High, Token: byVer[0].Token}
	}
	return Match{Tier: Ambiguous, Candidates: tokens(candidates)}
}

// disagrees reports whether the cask declares bundle IDs and none of
// them match the app's. An empty QuitIDs list is not a disagreement.
func disagrees(bundleID string, quitIDs []string) bool {
	if bundleID == "" || len(quitIDs) == 0 {
		return false
	}
	return !contains(quitIDs, bundleID)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}

func filter(casks []caskindex.Cask, keep func(caskindex.Cask) bool) []caskindex.Cask {
	var out []caskindex.Cask
	for _, c := range casks {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func tokens(casks []caskindex.Cask) []string {
	out := make([]string, len(casks))
	for i, c := range casks {
		out[i] = c.Token
	}
	return out
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/match/ -v
git add -A && git commit -m "feat: confidence-tiered cask matcher with variant tie-breakers"
```

---

## Task 9: Report model + rendering

**Files:**
- Create: `internal/report/report.go`
- Test: `internal/report/report_test.go`

**Step 1: Write the failing test**

```go
package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sample() Report {
	return Report{
		ManagedCount: 12,
		Adoptable: []Entry{{App: "Slack.app", Token: "slack", Version: "4.39.0"}},
		Ambiguous: []Entry{{App: "Thing.app", Candidates: []string{"thing", "thing@beta"}}},
		AppStore:  []Entry{{App: "Things3.app"}},
		Unmatched: []Entry{{App: "Custom.app"}},
	}
}

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	sample().RenderTable(&buf, false)
	out := buf.String()
	for _, want := range []string{"Slack.app", "slack", "thing@beta", "Things3.app", "Custom.app", "12 app(s) already managed"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := sample().RenderJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Adoptable) != 1 || decoded.Adoptable[0].Token != "slack" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

func TestHasAdoptable(t *testing.T) {
	if !sample().HasAdoptable() {
		t.Error("want true")
	}
	if (Report{}).HasAdoptable() {
		t.Error("want false for empty report")
	}
}
```

**Step 2: Run to verify it fails**

**Step 3: Implement**

`internal/report/report.go`:
```go
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Entry struct {
	App        string   `json:"app"`
	Token      string   `json:"token,omitempty"`
	Version    string   `json:"version,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}

type Report struct {
	ManagedCount int     `json:"managed_count"`
	Managed      []Entry `json:"managed,omitempty"` // populated only with --verbose
	Adoptable    []Entry `json:"adoptable"`
	Ambiguous    []Entry `json:"ambiguous"`
	AppStore     []Entry `json:"app_store"`
	Unmatched    []Entry `json:"unmatched"`
}

func (r Report) HasAdoptable() bool { return len(r.Adoptable) > 0 }

func (r Report) RenderJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func (r Report) RenderTable(w io.Writer, verbose bool) {
	fmt.Fprintf(w, "%d app(s) already managed by Homebrew\n", r.ManagedCount)
	if verbose {
		section(w, "MANAGED", r.Managed, func(e Entry) string { return e.Token })
	}
	section(w, "ADOPTABLE", r.Adoptable, func(e Entry) string { return e.Token })
	section(w, "AMBIGUOUS (use: brewmaster adopt --cask <token> \"<App>\")", r.Ambiguous,
		func(e Entry) string { return strings.Join(e.Candidates, ", ") })
	section(w, "APP STORE (untouched; use --include-mas to convert)", r.AppStore,
		func(Entry) string { return "" })
	section(w, "UNMATCHED (no cask available)", r.Unmatched, func(Entry) string { return "" })
}

func section(w io.Writer, title string, entries []Entry, detail func(Entry) string) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", e.App, e.Version, detail(e))
	}
	tw.Flush()
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/report/ -v
git add -A && git commit -m "feat: audit report model with table and JSON rendering"
```

---

## Task 10: Pipeline + `audit` command

Compose scan → classify → match → report, then wire the cobra command. The pipeline takes all dependencies as parameters so the test drives it with fakes end-to-end.

**Files:**
- Create: `internal/pipeline/pipeline.go`, `cmd/audit.go`
- Test: `internal/pipeline/pipeline_test.go`

**Step 1: Write the failing test**

```go
package pipeline

import (
	"testing"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

func TestBuildReport(t *testing.T) {
	apps := []scan.App{
		{Name: "Raycast.app", BundleID: "com.raycast.macos"},                       // managed
		{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0"}, // adoptable
		{Name: "Things3.app", BundleID: "com.culturedcode.ThingsMac", MASReceipt: true},
		{Name: "Safari.app", BundleID: "com.apple.Safari"}, // excluded
		{Name: "Custom.app", BundleID: "com.example.custom"}, // unmatched
	}
	installed := []brew.InstalledCask{{Token: "raycast", Apps: []string{"Raycast.app"}}}
	idx := caskindex.BuildIndex([]caskindex.Cask{
		{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
		{Token: "things", Version: "3.20", Apps: []string{"Things3.app"}},
	})

	r := BuildReport(apps, installed, idx)

	if r.ManagedCount != 1 {
		t.Errorf("ManagedCount = %d, want 1", r.ManagedCount)
	}
	if len(r.Adoptable) != 1 || r.Adoptable[0].Token != "slack" {
		t.Errorf("Adoptable = %+v", r.Adoptable)
	}
	// MAS apps are matched too (so --include-mas knows the token) but stay in their bucket.
	if len(r.AppStore) != 1 || r.AppStore[0].Token != "things" {
		t.Errorf("AppStore = %+v", r.AppStore)
	}
	if len(r.Unmatched) != 1 || r.Unmatched[0].App != "Custom.app" {
		t.Errorf("Unmatched = %+v", r.Unmatched)
	}
	// System apps appear nowhere.
	total := r.ManagedCount + len(r.Adoptable) + len(r.Ambiguous) + len(r.AppStore) + len(r.Unmatched)
	if total != 4 {
		t.Errorf("system app leaked into report; total entries = %d, want 4", total)
	}
}
```

**Step 2: Run to verify it fails** — `undefined: BuildReport`

**Step 3: Implement**

`internal/pipeline/pipeline.go`:
```go
package pipeline

import (
	"strings"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/match"
	"github.com/patrickserrano/brewmaster/internal/report"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

func BuildReport(apps []scan.App, installed []brew.InstalledCask, idx caskindex.Index) report.Report {
	brewOwned := map[string]string{}
	for _, c := range installed {
		for _, app := range c.Apps {
			brewOwned[strings.ToLower(app)] = c.Token
		}
	}

	var r report.Report
	for _, app := range apps {
		switch scan.Classify(app, brewOwned) {
		case scan.System:
			continue
		case scan.Managed:
			r.ManagedCount++
			r.Managed = append(r.Managed, report.Entry{
				App: app.Name, Token: brewOwned[strings.ToLower(app.Name)], Version: app.Version,
			})
		case scan.AppStore:
			e := report.Entry{App: app.Name, Version: app.Version}
			if m := match.MatchApp(app, idx); m.Tier == match.High {
				e.Token = m.Token
			}
			r.AppStore = append(r.AppStore, e)
		case scan.Unmanaged:
			e := report.Entry{App: app.Name, Version: app.Version}
			switch m := match.MatchApp(app, idx); m.Tier {
			case match.High:
				e.Token = m.Token
				r.Adoptable = append(r.Adoptable, e)
			case match.Ambiguous:
				e.Candidates = m.Candidates
				r.Ambiguous = append(r.Ambiguous, e)
			default:
				r.Unmatched = append(r.Unmatched, e)
			}
		}
	}
	return r
}
```

**Step 4: Verify pass, then wire the audit command**

`cmd/audit.go` (wiring code — covered by the pipeline test plus a manual smoke run, not unit-tested line by line):
```go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/pipeline"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// ErrAdoptableFound makes audit exit non-zero when drift exists (exit 1;
// real errors exit 2 via main).
var ErrAdoptableFound = fmt.Errorf("adoptable apps found")

func newAuditCmd() *cobra.Command {
	var jsonOut, verbose bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report which apps are Homebrew-managed, adoptable, or App Store-installed",
		RunE: func(c *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			apps, err := scan.ScanDirs(scan.DefaultDirs(home))
			if err != nil {
				return err
			}
			installed, err := brew.InstalledCasks(c.Context(), brew.ExecRunner{})
			if err != nil {
				return fmt.Errorf("is Homebrew installed? %w", err)
			}
			data, err := caskindex.Fetch(caskindex.DefaultCatalogURL,
				caskindex.DefaultCachePath(home), 24*time.Hour)
			if err != nil {
				return err
			}
			casks, err := caskindex.ParseCatalog(data)
			if err != nil {
				return err
			}
			r := pipeline.BuildReport(apps, installed, caskindex.BuildIndex(casks))
			if !verbose {
				r.Managed = nil
			}
			if jsonOut {
				if err := r.RenderJSON(c.OutOrStdout()); err != nil {
					return err
				}
			} else {
				r.RenderTable(c.OutOrStdout(), verbose)
			}
			if r.HasAdoptable() {
				return ErrAdoptableFound
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "list managed apps too")
	return cmd
}
```

Register in `cmd/root.go` (add inside `NewRootCmd` before `return`):
```go
	root.AddCommand(newAuditCmd())
```

Update `main.go` to distinguish exit codes:
```go
func main() {
	err := cmd.NewRootCmd().Execute()
	switch {
	case err == nil:
	case errors.Is(err, cmd.ErrAdoptableFound):
		os.Exit(1)
	default:
		os.Exit(2)
	}
}
```
(add `"errors"` to imports)

**Step 5: Smoke test on the real machine, commit**

```bash
go test ./... && go build -o /tmp/brewmaster . && /tmp/brewmaster audit | head -40
```
Expected: a real report of this Mac's apps. Sanity-check a few rows by hand (e.g. a brew-installed app shows as managed). Then:
```bash
git add -A && git commit -m "feat: audit command with drift-detecting exit codes"
```

---

## Task 11: Adoption engine — adopt state machine

Re-read the "Adoption engine" section of the design doc. All brew calls go through `brew.Runner`; the test fake records invocations and scripts failures.

**Files:**
- Create: `internal/engine/engine.go`
- Test: `internal/engine/engine_test.go`

**Step 1: Write the failing test**

```go
package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner scripts responses per command-line prefix and records calls.
type fakeRunner struct {
	calls []string
	fail  map[string]error // command prefix -> error to return
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, cmd)
	for prefix, err := range f.fail {
		if strings.HasPrefix(cmd, prefix) {
			return nil, err
		}
	}
	return []byte("ok"), nil
}

func TestAdoptSuccess(t *testing.T) {
	r := &fakeRunner{}
	e := Engine{Runner: r}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Adopted {
		t.Fatalf("outcome = %v, want Adopted", res.Outcome)
	}
	if len(r.calls) != 1 || r.calls[0] != "brew install --cask --adopt slack" {
		t.Errorf("calls = %v", r.calls)
	}
}

func TestAdoptFailureWithoutForceReportsNeedsReinstall(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install --cask --adopt": errors.New("artifact differs")}}
	e := Engine{Runner: r}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != NeedsReinstall {
		t.Fatalf("outcome = %v, want NeedsReinstall", res.Outcome)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "--force") {
			t.Errorf("must not escalate to --force without opt-in: %v", r.calls)
		}
	}
}

func TestAdoptFailureWithForceEscalates(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install --cask --adopt": errors.New("artifact differs")}}
	e := Engine{Runner: r, Force: true}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Reinstalled {
		t.Fatalf("outcome = %v, want Reinstalled", res.Outcome)
	}
	want := "brew install --cask --force slack"
	if len(r.calls) != 2 || r.calls[1] != want {
		t.Errorf("calls = %v, want second call %q", r.calls, want)
	}
}

func TestForceEscalationCanAlsoFail(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"brew install": errors.New("boom")}}
	e := Engine{Runner: r, Force: true}
	res := e.AdoptOne(context.Background(), "Slack.app", "slack")
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed with error", res.Outcome, res.Err)
	}
}
```

**Step 2: Run to verify it fails** — `undefined: Engine`

**Step 3: Implement**

`internal/engine/engine.go`:
```go
package engine

import (
	"context"

	"github.com/patrickserrano/brewmaster/internal/brew"
)

type Outcome int

const (
	Adopted Outcome = iota
	Reinstalled
	NeedsReinstall // adopt failed; user did not pass --force
	Failed
)

func (o Outcome) String() string {
	return [...]string{"adopted", "reinstalled", "needs-reinstall", "failed"}[o]
}

type Result struct {
	App     string `json:"app"`
	Token   string `json:"token"`
	Outcome Outcome `json:"-"`
	OutcomeName string `json:"outcome"`
	Err     error  `json:"-"`
	ErrText string `json:"error,omitempty"`
}

type Engine struct {
	Runner brew.Runner
	Force  bool
}

// AdoptOne runs the two-step adopt state machine for a single app:
// try --adopt; on failure escalate to --force only when opted in.
func (e Engine) AdoptOne(ctx context.Context, app, token string) Result {
	res := Result{App: app, Token: token}
	if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", "--adopt", token); err == nil {
		res.Outcome = Adopted
	} else if !e.Force {
		res.Outcome = NeedsReinstall
		res.Err = err
	} else if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", "--force", token); err == nil {
		res.Outcome = Reinstalled
	} else {
		res.Outcome = Failed
		res.Err = err
	}
	res.OutcomeName = res.Outcome.String()
	if res.Err != nil {
		res.ErrText = res.Err.Error()
	}
	return res
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/engine/ -v
git add -A && git commit -m "feat: adoption state machine with opt-in force escalation"
```

---

## Task 12: Engine — post-adopt upgrade with running-app deferral

**Files:**
- Modify: `internal/engine/engine.go`
- Test: append to `internal/engine/engine_test.go`

**Step 1: Write the failing test**

Running detection uses `pgrep -xq <CFBundleExecutable>`; the fake scripts pgrep success/failure. (`pgrep` exits non-zero when no process matches, which the Runner surfaces as an error.)

```go
func TestUpgradeAdoptedSkipsRunningApps(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"pgrep -xq Slack": nil}} // see note below
	// fakeRunner returns error for matched prefixes; pgrep SUCCESS means running.
	// Script it the other way: fail pgrep for the NOT-running app.
	r = &fakeRunner{fail: map[string]error{"pgrep -xq Code": errors.New("exit 1")}}
	e := Engine{Runner: r}

	deferred := e.UpgradeAdopted(context.Background(), []AdoptedApp{
		{Token: "slack", Executable: "Slack"},              // running -> deferred
		{Token: "visual-studio-code", Executable: "Code"},  // not running -> upgraded
	})

	if len(deferred) != 1 || deferred[0] != "slack" {
		t.Errorf("deferred = %v, want [slack]", deferred)
	}
	want := "brew upgrade --cask visual-studio-code"
	found := false
	for _, c := range r.calls {
		if c == want {
			found = true
		}
		if strings.Contains(c, "upgrade") && strings.Contains(c, "slack") {
			t.Errorf("running app must not be upgraded: %v", r.calls)
		}
	}
	if !found {
		t.Errorf("missing %q in calls %v", want, r.calls)
	}
}

func TestUpgradeAllWhenYes(t *testing.T) {
	r := &fakeRunner{} // every pgrep "succeeds" -> everything looks running
	e := Engine{Runner: r, Yes: true}
	deferred := e.UpgradeAdopted(context.Background(), []AdoptedApp{{Token: "slack", Executable: "Slack"}})
	if len(deferred) != 0 {
		t.Errorf("with Yes, nothing defers: %v", deferred)
	}
}

func TestUpgradeNoAppsNoCalls(t *testing.T) {
	r := &fakeRunner{}
	Engine{Runner: r}.UpgradeAdopted(context.Background(), nil)
	if len(r.calls) != 0 {
		t.Errorf("no apps should mean no brew calls: %v", r.calls)
	}
}
```

**Step 2: Run to verify it fails** — `undefined: AdoptedApp` / `UpgradeAdopted`

**Step 3: Implement** (add to `internal/engine/engine.go`)

```go
type AdoptedApp struct {
	Token      string
	Executable string // CFBundleExecutable, for pgrep
}

// Add field to Engine:
//   Yes bool  // --yes: don't defer upgrades of running apps

// UpgradeAdopted converges adopted apps to their cask versions in one
// brew upgrade call. Running apps are deferred (returned) unless Yes.
func (e Engine) UpgradeAdopted(ctx context.Context, apps []AdoptedApp) (deferred []string) {
	var upgrade []string
	for _, a := range apps {
		if !e.Yes && e.isRunning(ctx, a.Executable) {
			deferred = append(deferred, a.Token)
			continue
		}
		upgrade = append(upgrade, a.Token)
	}
	if len(upgrade) > 0 {
		args := append([]string{"upgrade", "--cask"}, upgrade...)
		e.Runner.Run(ctx, "brew", args...) //nolint:errcheck // best-effort; report-level concern
	}
	return deferred
}

func (e Engine) isRunning(ctx context.Context, executable string) bool {
	if executable == "" {
		return false
	}
	_, err := e.Runner.Run(ctx, "pgrep", "-xq", executable)
	return err == nil // pgrep exits 0 iff a process matched
}
```

Also add `Executable string` to `scan.App` and populate from `CFBundleExecutable` in `ReadApp` (one-line addition to the `infoPlist` struct: `Executable string \`plist:"CFBundleExecutable"\`` — update Task 3's fixture helper to include the key and extend `TestReadApp` to assert it).

**Step 4: Verify pass (whole repo), commit**

```bash
go test ./... -v
git add -A && git commit -m "feat: post-adopt upgrade with running-app deferral"
```

---

## Task 13: Engine — MAS replacement

**Files:**
- Modify: `internal/engine/engine.go`
- Test: append to `internal/engine/engine_test.go`

**Step 1: Write the failing test**

Trash-ing is filesystem work, so it's injected as a function; production wiring supplies the real implementation.

```go
func TestReplaceMASRefusesRunningApp(t *testing.T) {
	r := &fakeRunner{} // pgrep succeeds -> app is running
	e := Engine{Runner: r, Trash: func(string) error { t.Fatal("must not trash a running app"); return nil }}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "Things3.app", Path: "/Applications/Things3.app", Token: "things", Executable: "Things"})
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want Failed (running)", res.Outcome)
	}
}

func TestReplaceMASTrashesThenInstalls(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{"pgrep": errors.New("not running")}}
	var trashed string
	e := Engine{Runner: r, Trash: func(p string) error { trashed = p; return nil }}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "Things3.app", Path: "/Applications/Things3.app", Token: "things", Executable: "Things"})
	if res.Outcome != Reinstalled {
		t.Fatalf("outcome = %v (err=%v)", res.Outcome, res.Err)
	}
	if trashed != "/Applications/Things3.app" {
		t.Errorf("trashed = %q", trashed)
	}
	wantInstall := "brew install --cask things"
	found := false
	for _, c := range r.calls {
		if c == wantInstall {
			found = true
		}
	}
	if !found {
		t.Errorf("missing %q in %v", wantInstall, r.calls)
	}
}

func TestReplaceMASInstallFailureSurfaces(t *testing.T) {
	r := &fakeRunner{fail: map[string]error{
		"pgrep":        errors.New("not running"),
		"brew install": errors.New("no such cask"),
	}}
	e := Engine{Runner: r, Trash: func(string) error { return nil }}
	res := e.ReplaceMAS(context.Background(), MASApp{App: "X.app", Path: "/Applications/X.app", Token: "x", Executable: "X"})
	if res.Outcome != Failed || res.Err == nil {
		t.Fatalf("outcome = %v err=%v, want Failed", res.Outcome, res.Err)
	}
}
```

**Step 2: Run to verify it fails**

**Step 3: Implement** (add to `internal/engine/engine.go`)

```go
type MASApp struct {
	App        string
	Path       string
	Token      string
	Executable string
}

// Add field to Engine:
//   Trash func(path string) error  // moves a bundle to the Trash (never rm -rf)

// ReplaceMAS converts a Mac App Store app to its cask equivalent:
// refuse if running, move the MAS bundle to the Trash (recoverable),
// then install the cask. Caller is responsible for user confirmation.
func (e Engine) ReplaceMAS(ctx context.Context, m MASApp) Result {
	res := Result{App: m.App, Token: m.Token}
	fail := func(err error) Result {
		res.Outcome = Failed
		res.OutcomeName = res.Outcome.String()
		res.Err = err
		res.ErrText = err.Error()
		return res
	}
	if e.isRunning(ctx, m.Executable) {
		return fail(fmt.Errorf("%s is running; quit it first", m.App))
	}
	if err := e.Trash(m.Path); err != nil {
		return fail(fmt.Errorf("trash: %w", err))
	}
	if _, err := e.Runner.Run(ctx, "brew", "install", "--cask", m.Token); err != nil {
		return fail(fmt.Errorf("install after trash (restore %s from Trash): %w", m.App, err))
	}
	res.Outcome = Reinstalled
	res.OutcomeName = res.Outcome.String()
	return res
}
```

Production `Trash` implementation goes in `internal/engine/trash_darwin.go`:
```go
package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TrashPath moves a path to the user's Trash. Prefers the macOS 14+
// /usr/bin/trash binary (handles permissions and Finder put-back);
// falls back to mv into ~/.Trash with a timestamp suffix on collision.
func TrashPath(path string) error {
	if _, err := os.Stat("/usr/bin/trash"); err == nil {
		return exec.Command("/usr/bin/trash", path).Run()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, ".Trash", filepath.Base(path))
	if _, err := os.Stat(dest); err == nil {
		dest = fmt.Sprintf("%s.%d", dest, time.Now().Unix())
	}
	return os.Rename(path, dest)
}
```

**Step 4: Verify pass, commit**

```bash
go test ./internal/engine/ -v
git add -A && git commit -m "feat: MAS replacement via trash-then-install with running-app guard"
```

---

## Task 14: `adopt` command wiring

Brings everything together: flags, dry-run, ambiguous overrides, MAS confirmation, summary table, JSON state log.

**Files:**
- Create: `cmd/adopt.go`
- Modify: `cmd/root.go` (register), `internal/report/report.go` if summary helpers needed
- Test: `cmd/adopt_test.go` (dry-run behavior — the one piece of cmd logic worth unit-testing)

**Step 1: Write the failing test**

Make the command's dependencies injectable for tests via a small constructor parameter struct:

```go
package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type recordingRunner struct{ calls []string }

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if strings.Contains(strings.Join(args, " "), "--json=v2") {
		return []byte(`{"formulae":[],"casks":[]}`), nil
	}
	return []byte("ok"), nil
}

func TestAdoptDryRunMakesNoMutatingCalls(t *testing.T) {
	r := &recordingRunner{}
	deps := AdoptDeps{
		Runner:  r,
		Scan:    func() ([]scan.App, error) {
			return []scan.App{{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0"}}, nil
		},
		Catalog: func() ([]caskindex.Cask, error) {
			return []caskindex.Cask{{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}}}, nil
		},
	}
	out := &bytes.Buffer{}
	cmd := newAdoptCmd(deps)
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "install") || strings.Contains(c, "upgrade") {
			t.Errorf("dry-run must not mutate: %v", r.calls)
		}
	}
	if !strings.Contains(out.String(), "brew install --cask --adopt slack") {
		t.Errorf("dry-run should print the planned command:\n%s", out.String())
	}
}
```

(Add the imports for `scan` and `caskindex`. `AdoptDeps` defaults — real scanner, real catalog fetch, `brew.ExecRunner{}` — are filled in by `newAdoptCmd` when fields are nil, so `cmd/root.go` registers `newAdoptCmd(AdoptDeps{})`.)

**Step 2: Run to verify it fails**

**Step 3: Implement**

`cmd/adopt.go`:
```go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/engine"
	"github.com/patrickserrano/brewmaster/internal/pipeline"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

type AdoptDeps struct {
	Runner  brew.Runner
	Scan    func() ([]scan.App, error)
	Catalog func() ([]caskindex.Cask, error)
	Trash   func(string) error
}

func (d *AdoptDeps) fillDefaults() {
	if d.Runner == nil {
		d.Runner = brew.ExecRunner{}
	}
	if d.Scan == nil {
		d.Scan = func() ([]scan.App, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			return scan.ScanDirs(scan.DefaultDirs(home))
		}
	}
	if d.Catalog == nil {
		d.Catalog = func() ([]caskindex.Cask, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			data, err := caskindex.Fetch(caskindex.DefaultCatalogURL,
				caskindex.DefaultCachePath(home), 24*time.Hour)
			if err != nil {
				return nil, err
			}
			return caskindex.ParseCatalog(data)
		}
	}
	if d.Trash == nil {
		d.Trash = engine.TrashPath
	}
}

func newAdoptCmd(deps AdoptDeps) *cobra.Command {
	var (
		dryRun, includeMAS, force, yes bool
		caskOverride                   string
	)
	cmd := &cobra.Command{
		Use:   "adopt [apps...]",
		Short: "Convert unmanaged apps to Homebrew-managed via --adopt",
		RunE: func(c *cobra.Command, args []string) error {
			deps.fillDefaults()
			out := c.OutOrStdout()

			apps, err := deps.Scan()
			if err != nil {
				return err
			}
			casks, err := deps.Catalog()
			if err != nil {
				return err
			}
			idx := caskindex.BuildIndex(casks)
			installed, err := brew.InstalledCasks(c.Context(), deps.Runner)
			if err != nil {
				return err
			}
			r := pipeline.BuildReport(apps, installed, idx)

			// Build worklist: adoptable apps, filtered to positional args if given.
			type job struct{ app, token, executable, path string }
			byName := map[string]scan.App{}
			for _, a := range apps {
				byName[a.Name] = a
			}
			var jobs []job
			for _, e := range r.Adoptable {
				if len(args) > 0 && !matchesArgs(e.App, args) {
					continue
				}
				a := byName[e.App]
				jobs = append(jobs, job{e.App, e.Token, a.Executable, a.Path})
			}
			// Explicit override: adopt --cask <token> "<App>" handles one ambiguous app.
			if caskOverride != "" {
				if len(args) != 1 {
					return fmt.Errorf("--cask requires exactly one app argument")
				}
				a, ok := byName[normalizeAppArg(args[0])]
				if !ok {
					return fmt.Errorf("app %q not found in scan", args[0])
				}
				jobs = []job{{a.Name, caskOverride, a.Executable, a.Path}}
			}

			if dryRun {
				for _, j := range jobs {
					fmt.Fprintf(out, "would run: brew install --cask --adopt %s  # %s\n", j.token, j.app)
				}
				if includeMAS {
					for _, e := range r.AppStore {
						if e.Token != "" {
							fmt.Fprintf(out, "would replace MAS app: %s -> brew install --cask %s\n", e.App, e.Token)
						}
					}
				}
				return nil
			}

			eng := engine.Engine{Runner: deps.Runner, Force: force, Yes: yes, Trash: deps.Trash}
			var results []engine.Result
			var adopted []engine.AdoptedApp
			for _, j := range jobs {
				res := eng.AdoptOne(c.Context(), j.app, j.token)
				results = append(results, res)
				fmt.Fprintf(out, "%-14s %s (%s)\n", res.OutcomeName+":", j.app, j.token)
				if res.Outcome == engine.Adopted {
					adopted = append(adopted, engine.AdoptedApp{Token: j.token, Executable: j.executable})
				}
			}

			if deferred := eng.UpgradeAdopted(c.Context(), adopted); len(deferred) > 0 {
				fmt.Fprintf(out, "deferred upgrades (apps running): %s\n", strings.Join(deferred, ", "))
			}

			if includeMAS {
				if !yes && !confirmMAS(c, len(r.AppStore)) {
					fmt.Fprintln(out, "MAS conversion skipped.")
				} else {
					for _, e := range r.AppStore {
						if e.Token == "" {
							continue
						}
						a := byName[e.App]
						res := eng.ReplaceMAS(c.Context(), engine.MASApp{
							App: e.App, Path: a.Path, Token: e.Token, Executable: a.Executable,
						})
						results = append(results, res)
						fmt.Fprintf(out, "%-14s %s (%s)\n", res.OutcomeName+":", e.App, e.Token)
					}
				}
			}

			writeStateLog(results)
			summarize(out, results)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print planned actions without executing")
	cmd.Flags().BoolVar(&includeMAS, "include-mas", false, "also replace Mac App Store apps (loses MAS receipts/IAP; app data survives)")
	cmd.Flags().BoolVar(&force, "force", false, "reinstall when adopt fails on artifact mismatch (overwrites the app bundle)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmations; upgrade running apps too")
	cmd.Flags().StringVar(&caskOverride, "cask", "", "explicit cask token for a single (ambiguous) app")
	return cmd
}

func matchesArgs(appName string, args []string) bool {
	for _, a := range args {
		if strings.EqualFold(normalizeAppArg(a), appName) {
			return true
		}
	}
	return false
}

func normalizeAppArg(s string) string {
	if !strings.HasSuffix(s, ".app") {
		return s + ".app"
	}
	return s
}

func confirmMAS(c *cobra.Command, n int) bool {
	fmt.Fprintf(c.OutOrStdout(),
		"\nWARNING: replacing %d App Store app(s) with cask versions.\n"+
			"You will LOSE: App Store receipts, App Store auto-updates, and possibly in-app purchases.\n"+
			"App data in ~/Library is preserved. Old bundles go to the Trash.\n"+
			"Type 'yes' to continue: ", n)
	scanner := bufio.NewScanner(c.InOrStdin())
	return scanner.Scan() && strings.TrimSpace(scanner.Text()) == "yes"
}

func writeStateLog(results []engine.Result) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".local", "state", "brewmaster")
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	data, _ := json.MarshalIndent(results, "", "  ")
	name := fmt.Sprintf("adopt-%s.json", time.Now().Format("2006-01-02T15-04-05"))
	os.WriteFile(filepath.Join(dir, name), data, 0o644) //nolint:errcheck
}

func summarize(out io.Writer, results []engine.Result) {
	counts := map[string]int{}
	for _, r := range results {
		counts[r.OutcomeName]++
	}
	fmt.Fprintf(out, "\nsummary: %d adopted, %d reinstalled, %d needs-reinstall, %d failed\n",
		counts["adopted"], counts["reinstalled"], counts["needs-reinstall"], counts["failed"])
}
```
(add `"io"` import; register `root.AddCommand(newAdoptCmd(AdoptDeps{}))` in `cmd/root.go`)

**Step 4: Verify pass + real dry-run smoke test, commit**

```bash
go test ./... && go build -o /tmp/brewmaster . && /tmp/brewmaster adopt --dry-run
```
Expected: planned `brew install --cask --adopt ...` lines for this Mac's actual adoptable apps, zero mutations. Then:
```bash
git add -A && git commit -m "feat: adopt command with dry-run, MAS flag, overrides, and state log"
```

---

## Task 15: Integration tests (opt-in)

**Files:**
- Create: `internal/brew/integration_test.go`

**Step 1: Write the test** (no TDD failure step — this validates real-world assumptions)

```go
//go:build integration

package brew

import (
	"context"
	"testing"
)

// Validates our parsing against the real brew on this machine —
// brew's JSON shape changing underneath us is the likeliest breakage.
func TestRealBrewInfoParses(t *testing.T) {
	casks, err := InstalledCasks(context.Background(), ExecRunner{})
	if err != nil {
		t.Fatalf("brew info failed (is brew installed?): %v", err)
	}
	t.Logf("parsed %d installed casks", len(casks))
	for _, c := range casks {
		if c.Token == "" {
			t.Errorf("cask with empty token: %+v", c)
		}
	}
}
```

**Step 2: Run and verify**

Run: `go test -tags integration ./internal/brew/ -v`
Expected: PASS on a machine with brew; logs the real cask count. Verify the count matches `brew list --cask | wc -l`.

**Step 3: Commit**

```bash
git add -A && git commit -m "test: opt-in integration test against real brew"
```

---

## Task 16: CI, release config, README

**Files:**
- Create: `.github/workflows/ci.yml`, `.goreleaser.yaml`, `README.md`, `.gitignore`

**Step 1: CI workflow**

`.github/workflows/ci.yml`:
```yaml
name: ci
on:
  push: { branches: [main] }
  pull_request:
jobs:
  test:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.22" }
      - run: go vet ./...
      - run: go test ./...
      - run: go test -tags integration ./internal/brew/
```

**Step 2: GoReleaser**

`.goreleaser.yaml`:
```yaml
version: 2
builds:
  - main: .
    binary: brewmaster
    goos: [darwin]
    goarch: [arm64, amd64]
    env: [CGO_ENABLED=0]
brews:
  - repository:
      owner: patrickserrano
      name: homebrew-tap
    homepage: https://github.com/patrickserrano/brewmaster
    description: "Audit installed macOS apps and adopt them into Homebrew"
release:
  github:
    owner: patrickserrano
    name: brewmaster
```

**Step 3: README** — short: what it does, install (`brew install patrickserrano/tap/brewmaster`), the two commands with example output, the MAS warning, exit codes (0 clean / 1 adoptable found / 2 error), link to design doc.

`.gitignore`: `/brewmaster`, `dist/`.

**Step 4: Verify and commit**

```bash
go test ./... && go vet ./...
git add -A && git commit -m "chore: CI workflow, goreleaser config, README"
```

**Step 5: Final verification sweep**

Per @superpowers:verification-before-completion: run the full suite (`go test ./... && go test -tags integration ./internal/brew/`), build, run `/tmp/brewmaster audit` and `adopt --dry-run` on the real machine, and hand-verify three rows of audit output (one managed, one adoptable, one MAS) against reality (`brew list --cask`, presence of `_MASReceipt`).

---

## Deferred (do NOT build in v1)

Undo command, config file, non-app artifacts (binaries/prefpanes), MacPorts/nix detection, watch/cron mode, fuzzy display-name matching beyond what's specified.
