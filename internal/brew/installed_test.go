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
