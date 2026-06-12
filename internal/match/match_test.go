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
