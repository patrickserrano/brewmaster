package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sample() Report {
	return Report{
		ManagedCount: 12,
		Adoptable:    []Entry{{App: "Slack.app", Token: "slack", Version: "4.39.0"}},
		Ambiguous:    []Entry{{App: "Thing.app", Candidates: []string{"thing", "thing@beta"}}},
		AppStore:     []Entry{{App: "Things3.app", Token: "things"}},
		Unmatched:    []Entry{{App: "Custom.app"}},
	}
}

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	sample().RenderTable(&buf, false)
	out := buf.String()
	for _, want := range []string{"Slack.app", "slack", "thing@beta", "Things3.app", "Custom.app", "12 app(s) already managed"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

// The APP STORE section shows the matched cask token so users can see
// what --include-mas would convert.
func TestRenderTableAppStoreShowsToken(t *testing.T) {
	var buf bytes.Buffer
	sample().RenderTable(&buf, false)
	out := buf.String()
	masSection := out[strings.Index(out, "APP STORE"):]
	if i := strings.Index(masSection, "\nUNMATCHED"); i >= 0 {
		masSection = masSection[:i]
	}
	if !strings.Contains(masSection, "things") {
		t.Errorf("APP STORE section missing matched token %q:\n%s", "things", out)
	}
}

func TestRenderTableVerbose(t *testing.T) {
	r := sample()
	r.Managed = []Entry{{App: "Raycast.app", Token: "raycast", Version: "1.80.0"}}
	var buf bytes.Buffer
	r.RenderTable(&buf, true)
	out := buf.String()
	for _, want := range []string{"MANAGED", "Raycast.app", "raycast"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose table missing %q:\n%s", want, out)
		}
	}
}

func TestRenderJSONEmptyBucketsAreArrays(t *testing.T) {
	var buf bytes.Buffer
	if err := (Report{}).RenderJSON(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"adoptable": []`, `"ambiguous": []`, `"app_store": []`, `"unmatched": []`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "null") {
		t.Errorf("JSON contains null bucket:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := sample().RenderJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Adoptable) != 1 || decoded.Adoptable[0].Token != "slack" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
}

func TestHasAdoptable(t *testing.T) {
	if !sample().HasAdoptable() {
		t.Error("want true")
	}
	if (Report{}).HasAdoptable() {
		t.Error("want false for empty report")
	}
}
