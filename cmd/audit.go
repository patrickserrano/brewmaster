package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/pipeline"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// ErrAdoptableFound makes audit exit non-zero when drift exists (exit 1;
// real errors exit 2 via main).
var ErrAdoptableFound = fmt.Errorf("adoptable apps found")

func newAuditCmd() *cobra.Command {
	var jsonOut, verbose bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report which apps are Homebrew-managed, adoptable, or App Store-installed",
		RunE: func(c *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			apps, err := scan.ScanDirs(scan.DefaultDirs(home))
			if err != nil {
				return err
			}
			installed, err := brew.InstalledCasks(c.Context(), brew.ExecRunner{})
			if err != nil {
				return fmt.Errorf("is Homebrew installed? %w", err)
			}
			data, stale, err := caskindex.Fetch(caskindex.DefaultCatalogURL,
				caskindex.DefaultCachePath(home), 24*time.Hour)
			if err != nil {
				return err
			}
			if stale {
				fmt.Fprintln(c.ErrOrStderr(), "warning: using stale cask catalog (network unavailable)")
			}
			casks, err := caskindex.ParseCatalog(data)
			if err != nil {
				return err
			}
			r := pipeline.BuildReport(apps, installed, caskindex.BuildIndex(casks))
			if !verbose {
				r.Managed = nil
			}
			if jsonOut {
				if err := r.RenderJSON(c.OutOrStdout()); err != nil {
					return err
				}
			} else {
				r.RenderTable(c.OutOrStdout(), verbose)
				if len(r.Adoptable) == 0 && len(r.Ambiguous) == 0 && len(r.Unmatched) == 0 {
					fmt.Fprintln(c.OutOrStdout(),
						"\nNothing to adopt — everything Homebrew can manage is already managed.")
				}
			}
			if r.HasAdoptable() {
				return ErrAdoptableFound
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "list managed apps too")
	return cmd
}
