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
