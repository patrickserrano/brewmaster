package report

import (
	"bytes"
	"strings"
	"testing"
)

// An empty report renders only the managed-count line: every section is
// skipped because it has no entries.
func TestRenderTableEmptySkipsSections(t *testing.T) {
	var buf bytes.Buffer
	(Report{}).RenderTable(&buf, true)
	out := buf.String()
	if !strings.Contains(out, "0 app(s) already managed") {
		t.Errorf("expected managed-count line:\n%s", out)
	}
	for _, section := range []string{"ADOPTABLE", "AMBIGUOUS", "APP STORE", "UNMATCHED", "MANAGED"} {
		if strings.Contains(out, section) {
			t.Errorf("empty report must not render the %s section:\n%s", section, out)
		}
	}
}
