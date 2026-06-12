package brew

import (
	"context"
	"encoding/json"
	"path/filepath"
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
// entries are heterogeneous objects like {"app": ["Foo.app"]}. Renamed
// apps also surface the install destination as an entry-level "target"
// full path ({"app": ["Foo.app"], "target": "/Applications/Bar.app"});
// both names are collected, deduplicated within the cask.
func appArtifacts(artifacts []json.RawMessage) []string {
	var apps []string
	seen := map[string]bool{}
	add := func(name string) {
		if !hasSuffixFold(name, ".app") {
			return
		}
		k := strings.ToLower(name)
		if seen[k] {
			return
		}
		seen[k] = true
		apps = append(apps, name)
	}
	for _, raw := range artifacts {
		var entry map[string]json.RawMessage
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		if appRaw, ok := entry["app"]; ok {
			var vals []any
			if json.Unmarshal(appRaw, &vals) == nil {
				for _, v := range vals {
					if s, ok := v.(string); ok {
						add(s)
					}
				}
			}
		}
		if tgtRaw, ok := entry["target"]; ok {
			var tgt string
			if json.Unmarshal(tgtRaw, &tgt) == nil && tgt != "" {
				add(filepath.Base(tgt))
			}
		}
	}
	return apps
}

// hasSuffixFold reports whether s ends with suffix, case-insensitively.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func InstalledCasks(ctx context.Context, r Runner) ([]InstalledCask, error) {
	data, err := r.Run(ctx, "brew", "info", "--json=v2", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseInstalledCasks(data)
}
