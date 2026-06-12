// Package pipeline composes scan → classify → match → report.
package pipeline

import (
	"strings"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/match"
	"github.com/patrickserrano/brewmaster/internal/report"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// BuildReport classifies every scanned app, matches the unmanaged ones
// against the cask index, and buckets them into a report. System apps
// are excluded entirely.
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
