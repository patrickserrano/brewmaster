package brew

import (
	"strings"
	"testing"
)

func TestParseInstalledCasksRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseInstalledCasks([]byte("not json {{{")); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

// An empty casks array parses to a non-nil, zero-length slice.
func TestParseInstalledCasksEmpty(t *testing.T) {
	casks, err := ParseInstalledCasks([]byte(`{"formulae":[],"casks":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(casks) != 0 {
		t.Errorf("want 0 casks, got %d", len(casks))
	}
}

// ParseInstalledCasks tolerates a non-object artifact entry (e.g. a bare
// array), skipping it instead of failing the whole parse.
func TestParseInstalledCasksSkipsNonObjectArtifact(t *testing.T) {
	data := `{"casks":[{"token":"edge","version":"1","artifacts":[["binary","x"],{"app":["Edge.app"]}]}]}`
	casks, err := ParseInstalledCasks([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(casks) != 1 || len(casks[0].Apps) != 1 || casks[0].Apps[0] != "Edge.app" {
		t.Errorf("casks = %+v, want one Edge.app", casks)
	}
}

func TestStderrSummary(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{"single line", "boom", "boom"},
		{"multi line takes last non-empty", "warning: x\nError: real failure\n", "Error: real failure"},
		{"trailing blank lines trimmed", "Error: deep\n\n\n", "Error: deep"},
		{"empty input", "", ""},
		{"whitespace only", "   \n  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stderrSummary([]byte(tt.stderr)); got != tt.want {
				t.Errorf("stderrSummary(%q) = %q, want %q", tt.stderr, got, tt.want)
			}
		})
	}
}

// A line longer than 200 characters is truncated to 200.
func TestStderrSummaryTruncatesLongLine(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := stderrSummary([]byte(long))
	if len(got) != 200 {
		t.Errorf("len = %d, want 200", len(got))
	}
}
