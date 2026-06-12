// Package match maps unmanaged apps to Homebrew cask tokens with
// confidence tiers.
package match

import (
	"strings"

	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// Tier is the confidence level of a match.
type Tier int

const (
	None Tier = iota
	Ambiguous
	High
)

func (t Tier) String() string {
	switch t {
	case None:
		return "none"
	case Ambiguous:
		return "ambiguous"
	case High:
		return "high"
	default:
		return "unknown"
	}
}

// Match is the result of matching one app against the cask index.
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
// tie-broken by bundle ID, then by version — the version tie-break
// considers only candidates whose declared bundle IDs don't disagree
// with the app's, and ignores cask build metadata after a comma.
// Bundle-ID-only matches (artifact name differs, e.g. user renamed
// the bundle) are never auto-adopted.
func MatchApp(app scan.App, idx caskindex.Index) Match {
	candidates := idx.ByArtifact(app.Name)

	switch len(candidates) {
	case 0:
		// Fall back to bundle ID — informative, never auto-adoptable.
		if app.BundleID == "" {
			return Match{Tier: None}
		}
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
	// The version tie-break only runs over candidates whose declared
	// bundle IDs don't disagree with the app's — a version coincidence
	// must never promote a cask that names a different bundle ID.
	agreeable := filter(candidates, func(c caskindex.Cask) bool {
		return !disagrees(app.BundleID, c.QuitIDs)
	})
	if byVer := filter(agreeable, func(c caskindex.Cask) bool {
		return app.Version != "" && baseVersion(c.Version) == app.Version
	}); len(byVer) == 1 {
		return Match{Tier: High, Token: byVer[0].Token}
	}
	return Match{Tier: Ambiguous, Candidates: tokens(candidates)}
}

// baseVersion strips Homebrew build metadata: catalog versions are
// often "1.2.3,4567" (version,build), but CFBundleShortVersionString
// only carries the part before the comma.
func baseVersion(v string) string {
	base, _, _ := strings.Cut(v, ",")
	return base
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
