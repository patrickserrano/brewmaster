package pipeline

import (
	"testing"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

func TestBuildReport(t *testing.T) {
	apps := []scan.App{
		{Name: "Raycast.app", BundleID: "com.raycast.macos"},                          // managed
		{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0"}, // adoptable
		{Name: "Things3.app", BundleID: "com.culturedcode.ThingsMac", MASReceipt: true},
		{Name: "Safari.app", BundleID: "com.apple.Safari"},                 // excluded
		{Name: "Custom.app", BundleID: "com.example.custom"},               // unmatched
		{Name: "Tunnelblick.app", BundleID: "net.tunnelblick.tunnelblick"}, // ambiguous
	}
	installed := []brew.InstalledCask{{Token: "raycast", Apps: []string{"Raycast.app"}}}
	idx := caskindex.BuildIndex([]caskindex.Cask{
		{Token: "slack", Version: "4.39.0", Apps: []string{"Slack.app"}},
		{Token: "things", Version: "3.20", Apps: []string{"Things3.app"}},
		// Two casks claim Tunnelblick.app with no bundle-ID agreement: ambiguous.
		{Token: "tunnelblick", Version: "6.0", Apps: []string{"Tunnelblick.app"}},
		{Token: "tunnelblick-beta", Version: "7.0", Apps: []string{"Tunnelblick.app"}},
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
	if len(r.Ambiguous) != 1 || r.Ambiguous[0].App != "Tunnelblick.app" || len(r.Ambiguous[0].Candidates) != 2 {
		t.Errorf("Ambiguous = %+v", r.Ambiguous)
	}
	// System apps appear nowhere.
	total := r.ManagedCount + len(r.Adoptable) + len(r.Ambiguous) + len(r.AppStore) + len(r.Unmatched)
	if total != 5 {
		t.Errorf("system app leaked into report; total entries = %d, want 5", total)
	}
}
