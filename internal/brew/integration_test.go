//go:build integration

package brew

import (
	"context"
	"testing"
)

// Validates our parsing against the real brew on this machine —
// brew's JSON shape changing underneath us is the likeliest breakage.
func TestRealBrewInfoParses(t *testing.T) {
	casks, err := InstalledCasks(context.Background(), ExecRunner{})
	if err != nil {
		t.Fatalf("brew info failed (is brew installed?): %v", err)
	}
	t.Logf("parsed %d installed casks", len(casks))
	for _, c := range casks {
		if c.Token == "" {
			t.Errorf("cask with empty token: %+v", c)
		}
	}
}
