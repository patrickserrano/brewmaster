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

func TestRenamedAppTargets(t *testing.T) {
	idx := loadIndex(t)
	// The disk-image name and the renamed install target should both resolve.
	if got := idx.ByArtifact("Thorium Browser.app"); len(got) != 1 || got[0].Token != "thorium" {
		t.Errorf("ByArtifact(Thorium Browser.app) = %+v, want thorium cask", got)
	}
	if got := idx.ByArtifact("Thorium.app"); len(got) != 1 || got[0].Token != "thorium" {
		t.Errorf("ByArtifact(Thorium.app) = %+v, want thorium cask", got)
	}
	// Targets duplicated between the app array object and the entry-level
	// "target" key must be deduped within the cask.
	data, err := os.ReadFile("testdata/cask.json")
	if err != nil {
		t.Fatal(err)
	}
	casks, err := ParseCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range casks {
		if c.Token != "thorium" {
			continue
		}
		want := []string{"Thorium.app", "Thorium Browser.app"}
		if len(c.Apps) != len(want) {
			t.Fatalf("thorium Apps = %v, want %v", c.Apps, want)
		}
		for i, app := range want {
			if c.Apps[i] != app {
				t.Errorf("thorium Apps[%d] = %q, want %q", i, c.Apps[i], app)
			}
		}
	}
}
