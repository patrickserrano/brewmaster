package brew

import (
	"encoding/json"
	"os"
	"reflect"
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
	if len(casks) != 3 {
		t.Fatalf("want 3 casks, got %d", len(casks))
	}
	if casks[0].Token != "visual-studio-code" || casks[0].Version != "1.100.0" {
		t.Errorf("cask[0] = %+v", casks[0])
	}
	if len(casks[0].Apps) != 1 || casks[0].Apps[0] != "Visual Studio Code.app" {
		t.Errorf("cask[0].Apps = %v", casks[0].Apps)
	}
	// A renamed cask carries an entry-level "target": the cask owns the
	// app under both its original and renamed bundle names.
	if got, want := casks[2].Apps, []string{"Foo.app", "Foo Renamed.app"}; !reflect.DeepEqual(got, want) {
		t.Errorf("renamed cask Apps = %v, want %v", got, want)
	}
}

// appArtifacts must collect entry-level "target" renames (case-insensitive
// .app match) and dedupe names within a cask.
func TestAppArtifactsTargetRenames(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	tests := []struct {
		name      string
		artifacts []json.RawMessage
		want      []string
	}{
		{
			name:      "target rename adds renamed app",
			artifacts: []json.RawMessage{raw(`{"app":["Foo.app"],"target":"/Applications/Foo Renamed.app"}`)},
			want:      []string{"Foo.app", "Foo Renamed.app"},
		},
		{
			name:      "uppercase .APP target still matches",
			artifacts: []json.RawMessage{raw(`{"app":["Foo.app"],"target":"/Applications/Bar.APP"}`)},
			want:      []string{"Foo.app", "Bar.APP"},
		},
		{
			name:      "non-app target ignored",
			artifacts: []json.RawMessage{raw(`{"binary":["foo"],"target":"/usr/local/bin/foo"}`)},
			want:      nil,
		},
		{
			name: "duplicate names deduped within cask",
			artifacts: []json.RawMessage{
				raw(`{"app":["Foo.app"],"target":"/Applications/Foo.app"}`),
				raw(`{"app":["foo.APP"]}`),
			},
			want: []string{"Foo.app"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appArtifacts(tt.artifacts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("appArtifacts() = %v, want %v", got, tt.want)
			}
		})
	}
}
