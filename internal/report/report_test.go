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
		AppStore:     []Entry{{App: "Things3.app"}},
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
