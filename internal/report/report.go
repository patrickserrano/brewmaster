package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Entry struct {
	App        string   `json:"app"`
	Token      string   `json:"token,omitempty"`
	Version    string   `json:"version,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}

type Report struct {
	ManagedCount int     `json:"managed_count"`
	Managed      []Entry `json:"managed,omitempty"` // populated only with --verbose
	Adoptable    []Entry `json:"adoptable"`
	Ambiguous    []Entry `json:"ambiguous"`
	AppStore     []Entry `json:"app_store"`
	Unmatched    []Entry `json:"unmatched"`
}

func (r Report) HasAdoptable() bool { return len(r.Adoptable) > 0 }

func (r Report) RenderJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func (r Report) RenderTable(w io.Writer, verbose bool) {
	fmt.Fprintf(w, "%d app(s) already managed by Homebrew\n", r.ManagedCount)
	if verbose {
		section(w, "MANAGED", r.Managed, func(e Entry) string { return e.Token })
	}
	section(w, "ADOPTABLE", r.Adoptable, func(e Entry) string { return e.Token })
	section(w, "AMBIGUOUS (use: brewmaster adopt --cask <token> \"<App>\")", r.Ambiguous,
		func(e Entry) string { return strings.Join(e.Candidates, ", ") })
	section(w, "APP STORE (untouched; use --include-mas to convert)", r.AppStore,
		func(Entry) string { return "" })
	section(w, "UNMATCHED (no cask available)", r.Unmatched, func(Entry) string { return "" })
}

func section(w io.Writer, title string, entries []Entry, detail func(Entry) string) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", title)
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", e.App, e.Version, detail(e))
	}
	tw.Flush()
}
