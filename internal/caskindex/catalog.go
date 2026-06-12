// Package caskindex parses the Homebrew cask catalog and indexes casks
// by declared .app artifact and by quit-stanza bundle ID.
package caskindex

import (
	"encoding/json"
	"strings"
)

// Cask is a single entry from the Homebrew cask catalog, reduced to the
// fields needed for matching installed apps to casks.
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

// ParseCatalog decodes the cask.json API payload (a JSON array of casks).
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

// stringsFromArray decodes a JSON array and keeps string elements with the
// given suffix. Artifact arrays can mix strings with option objects.
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

// Index supports case-insensitive lookups of casks by .app artifact name
// or by bundle ID. Lookups may return multiple casks (variant collisions,
// e.g. slack and slack@beta both install Slack.app).
type Index struct {
	byArtifact map[string][]Cask // key: lowercased .app name
	byBundleID map[string][]Cask // key: lowercased bundle ID
}

// BuildIndex builds lookup maps over the parsed catalog.
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

// ByArtifact returns the casks declaring the given .app artifact name.
func (i Index) ByArtifact(app string) []Cask { return i.byArtifact[strings.ToLower(app)] }

// ByBundleID returns the casks whose quit stanzas reference the bundle ID.
func (i Index) ByBundleID(id string) []Cask { return i.byBundleID[strings.ToLower(id)] }
