package brew

import (
	"context"
	"encoding/json"
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
// entries are heterogeneous objects like {"app": ["Foo.app"]}.
func appArtifacts(artifacts []json.RawMessage) []string {
	var apps []string
	for _, raw := range artifacts {
		var entry map[string]json.RawMessage
		if json.Unmarshal(raw, &entry) != nil {
			continue
		}
		appRaw, ok := entry["app"]
		if !ok {
			continue
		}
		var vals []any
		if json.Unmarshal(appRaw, &vals) != nil {
			continue
		}
		for _, v := range vals {
			if s, ok := v.(string); ok && strings.HasSuffix(s, ".app") {
				apps = append(apps, s)
			}
		}
	}
	return apps
}

func InstalledCasks(ctx context.Context, r Runner) ([]InstalledCask, error) {
	data, err := r.Run(ctx, "brew", "info", "--json=v2", "--installed")
	if err != nil {
		return nil, err
	}
	return ParseInstalledCasks(data)
}
