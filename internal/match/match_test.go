package match

import (
	"sort"
	"testing"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

func idx() caskindex.Index {
	return caskindex.BuildIndex([]caskindex.Cask{
		{Token: "visual-studio-code", Version: "1.100.0",
			Apps: []string{"Visual Studio Code.app"}, QuitIDs: []string{"com.microsoft.VSCode"}},
		{Token: "slack", Version: "4.39.0,123", Apps: []string{"Slack.app"}},
		{Token: "slack@beta", Version: "4.40.0-beta", Apps: []string{"Slack.app"}},
		{Token: "iterm2", Version: "3.5.0", Apps: []string{"iTerm.app"},
			QuitIDs: []string{"com.googlecode.iterm2"}},
		{Token: "noquit", Version: "1.0.0", Apps: []string{"NoQuit.app"}},
		{Token: "bar", Version: "2.0.0", Apps: []string{"Bar.app"},
			QuitIDs: []string{"com.real.bar"}},
		{Token: "bar@beta", Version: "2.1.0-beta", Apps: []string{"Bar.app"},
			QuitIDs: []string{"com.real.bar"}},
	})
}

func TestMatchApp(t *testing.T) {
	cases := []struct {
		name           string
		app            scan.App
		wantTier       Tier
		wantToken      string
		wantCandidates []string // checked when Tier == Ambiguous
	}{
		{"unique artifact match is High",
			scan.App{Name: "Visual Studio Code.app", BundleID: "com.microsoft.VSCode"},
			High, "visual-studio-code", nil},
		{"unique artifact, bundle id agrees with quit id, High",
			scan.App{Name: "iTerm.app", BundleID: "com.googlecode.iterm2"},
			High, "iterm2", nil},
		{"unique artifact, cask declares no quit ids, High",
			scan.App{Name: "NoQuit.app", BundleID: "com.example.noquit"},
			High, "noquit", nil},
		{"unique artifact but bundle id disagrees with quit ids is Ambiguous",
			scan.App{Name: "Visual Studio Code.app", BundleID: "com.evil.imposter"},
			Ambiguous, "", []string{"visual-studio-code"}},
		{"variant collision resolved by version tie-breaker ignoring build metadata",
			scan.App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "4.39.0"},
			High, "slack", nil},
		{"variant collision with unknown version is Ambiguous",
			scan.App{Name: "Slack.app", BundleID: "com.tinyspeck.slackmacgap", Version: "9.9.9"},
			Ambiguous, "", []string{"slack", "slack@beta"}},
		{"variant collision where all candidates' quit ids disagree is Ambiguous despite version match",
			scan.App{Name: "Bar.app", BundleID: "com.evil.imposter", Version: "2.0.0"},
			Ambiguous, "", []string{"bar", "bar@beta"}},
		{"bundle-id-only match (renamed bundle) is Ambiguous",
			scan.App{Name: "VSCode Renamed.app", BundleID: "com.microsoft.VSCode"},
			Ambiguous, "", []string{"visual-studio-code"}},
		{"no artifact match and empty bundle id is None",
			scan.App{Name: "Nameless.app", BundleID: ""},
			None, "", nil},
		{"no match is None",
			scan.App{Name: "TotallyCustom.app", BundleID: "com.example.custom"},
			None, "", nil},
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
			if got.Tier == Ambiguous {
				if len(got.Candidates) == 0 {
					t.Fatal("ambiguous match must carry candidates")
				}
				if c.wantCandidates != nil && !equalSets(got.Candidates, c.wantCandidates) {
					t.Errorf("candidates = %v, want %v", got.Candidates, c.wantCandidates)
				}
			}
		})
	}
}

func TestTierString(t *testing.T) {
	cases := []struct {
		tier Tier
		want string
	}{
		{None, "none"},
		{Ambiguous, "ambiguous"},
		{High, "high"},
		{Tier(-1), "unknown"},
		{Tier(42), "unknown"},
	}
	for _, c := range cases {
		if got := c.tier.String(); got != c.want {
			t.Errorf("Tier(%d).String() = %q, want %q", int(c.tier), got, c.want)
		}
	}
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}
