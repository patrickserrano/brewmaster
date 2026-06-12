package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/patrickserrano/brewmaster/internal/brew"
	"github.com/patrickserrano/brewmaster/internal/caskindex"
	"github.com/patrickserrano/brewmaster/internal/engine"
	"github.com/patrickserrano/brewmaster/internal/pipeline"
	"github.com/patrickserrano/brewmaster/internal/report"
	"github.com/patrickserrano/brewmaster/internal/scan"
)

// AdoptDeps holds the adopt command's external dependencies so tests can
// inject fakes. Nil fields are filled with production defaults.
type AdoptDeps struct {
	Runner  brew.Runner
	Scan    func() ([]scan.App, error)
	Catalog func() (casks []caskindex.Cask, stale bool, err error)
	Trash   func(path string) error
}

func (d *AdoptDeps) fillDefaults() {
	if d.Runner == nil {
		d.Runner = brew.ExecRunner{}
	}
	if d.Scan == nil {
		d.Scan = func() ([]scan.App, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			return scan.ScanDirs(scan.DefaultDirs(home))
		}
	}
	if d.Catalog == nil {
		d.Catalog = func() ([]caskindex.Cask, bool, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, false, err
			}
			data, stale, err := caskindex.Fetch(caskindex.DefaultCatalogURL,
				caskindex.DefaultCachePath(home), 24*time.Hour)
			if err != nil {
				return nil, false, err
			}
			casks, err := caskindex.ParseCatalog(data)
			return casks, stale, err
		}
	}
	if d.Trash == nil {
		d.Trash = engine.TrashPath
	}
}

// job is one planned adoption: an on-disk app and the cask that owns it.
type job struct{ app, token, executable, path string }

func newAdoptCmd(deps AdoptDeps) *cobra.Command {
	var (
		dryRun, includeMAS, force, yes bool
		caskOverride                   string
	)
	cmd := &cobra.Command{
		Use:   "adopt [apps...]",
		Short: "Convert unmanaged apps to Homebrew-managed via --adopt",
		RunE: func(c *cobra.Command, args []string) error {
			deps.fillDefaults()
			out := c.OutOrStdout()
			errOut := c.ErrOrStderr()

			apps, err := deps.Scan()
			if err != nil {
				return err
			}
			casks, stale, err := deps.Catalog()
			if err != nil {
				return err
			}
			if stale {
				fmt.Fprintln(errOut, "warning: using stale cask catalog (network unavailable)")
			}
			installed, err := brew.InstalledCasks(c.Context(), deps.Runner)
			if err != nil {
				return fmt.Errorf("is Homebrew installed? %w", err)
			}
			r := pipeline.BuildReport(apps, installed, caskindex.BuildIndex(casks))

			byName := map[string]scan.App{}
			for _, a := range apps {
				byName[a.Name] = a
			}

			// Worklist: adoptable apps, filtered to positional args if given.
			var jobs []job
			for _, e := range r.Adoptable {
				if len(args) > 0 && !matchesArgs(e.App, args) {
					continue
				}
				a := byName[e.App]
				jobs = append(jobs, job{e.App, e.Token, a.Executable, a.Path})
			}
			// Explicit override: adopt --cask <token> "<App>" handles one
			// ambiguous app.
			if caskOverride != "" {
				if len(args) != 1 {
					return fmt.Errorf("--cask requires exactly one app argument")
				}
				a, ok := byName[normalizeAppArg(args[0])]
				if !ok {
					return fmt.Errorf("app %q not found in scan", args[0])
				}
				jobs = []job{{a.Name, caskOverride, a.Executable, a.Path}}
			}

			masJobs := masWorklist(r.AppStore, byName)
			if len(jobs) == 0 && (!includeMAS || len(masJobs) == 0) {
				fmt.Fprintln(out, "Nothing to adopt — run `brewmaster audit` to see why.")
				return nil
			}

			if dryRun {
				for _, j := range jobs {
					fmt.Fprintf(out, "would run: brew install --cask --adopt %s  # %s\n", j.token, j.app)
				}
				if includeMAS {
					for _, j := range masJobs {
						fmt.Fprintf(out, "would replace MAS app: %s -> brew install --cask %s\n", j.app, j.token)
					}
				}
				return nil
			}

			eng := engine.Engine{Runner: deps.Runner, Force: force, Yes: yes, Trash: deps.Trash}
			var results []engine.Result
			var adopted []engine.AdoptedApp
			for _, j := range jobs {
				res := eng.AdoptOne(c.Context(), j.app, j.token)
				results = append(results, res)
				printResult(out, res)
				if res.Outcome == engine.Adopted {
					adopted = append(adopted, engine.AdoptedApp{Token: j.token, Executable: j.executable})
				}
			}

			deferred, upgradeErr := eng.UpgradeAdopted(c.Context(), adopted)
			if len(deferred) > 0 {
				fmt.Fprintf(out, "deferred upgrades (apps appear to be running; re-run with --yes or quit them): %s\n",
					strings.Join(deferred, ", "))
			}
			if upgradeErr != nil {
				fmt.Fprintf(errOut, "warning: post-adopt upgrade failed: %v\n", upgradeErr)
			}

			if includeMAS && len(masJobs) > 0 {
				if !yes && !confirmMAS(c, len(masJobs)) {
					fmt.Fprintln(out, "MAS conversion skipped.")
				} else {
					for _, j := range masJobs {
						res := eng.ReplaceMAS(c.Context(), engine.MASApp{
							App: j.app, Path: j.path, Token: j.token, Executable: j.executable,
						})
						results = append(results, res)
						printResult(out, res)
					}
				}
			}

			writeStateLog(results)
			summarize(out, results)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print planned actions without executing")
	cmd.Flags().BoolVar(&includeMAS, "include-mas", false,
		"also replace Mac App Store apps (loses MAS receipts/IAP; app data survives)")
	cmd.Flags().BoolVar(&force, "force", false,
		"reinstall when adopt fails on artifact mismatch (overwrites the app bundle)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmations; upgrade running apps too")
	cmd.Flags().StringVar(&caskOverride, "cask", "",
		"explicit cask token for a single (ambiguous) app")
	return cmd
}

// masWorklist collects App Store apps with a confident cask match.
func masWorklist(appStore []report.Entry, byName map[string]scan.App) []job {
	var jobs []job
	for _, e := range appStore {
		if e.Token == "" {
			continue
		}
		a := byName[e.App]
		jobs = append(jobs, job{e.App, e.Token, a.Executable, a.Path})
	}
	return jobs
}

func printResult(out io.Writer, res engine.Result) {
	line := fmt.Sprintf("%-16s %s (%s)", res.OutcomeName+":", res.App, res.Token)
	if res.ErrText != "" {
		line += " — " + res.ErrText
	}
	fmt.Fprintln(out, line)
}

func matchesArgs(appName string, args []string) bool {
	for _, a := range args {
		if strings.EqualFold(normalizeAppArg(a), appName) {
			return true
		}
	}
	return false
}

func normalizeAppArg(s string) string {
	if !strings.HasSuffix(s, ".app") {
		return s + ".app"
	}
	return s
}

func confirmMAS(c *cobra.Command, n int) bool {
	fmt.Fprintf(c.OutOrStdout(),
		"\nWARNING: replacing %d App Store app(s) with cask versions.\n"+
			"You will LOSE: App Store receipts, App Store auto-updates, and possibly in-app purchases.\n"+
			"App data in ~/Library is preserved. Old bundles go to the Trash.\n"+
			"Type 'yes' to continue: ", n)
	scanner := bufio.NewScanner(c.InOrStdin())
	return scanner.Scan() && strings.TrimSpace(scanner.Text()) == "yes"
}

// writeStateLog records the run under ~/.local/state/brewmaster. Logging
// is best-effort: failures never block the adoption itself.
func writeStateLog(results []engine.Result) {
	if len(results) == 0 {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".local", "state", "brewmaster")
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return
	}
	name := fmt.Sprintf("adopt-%s.json", time.Now().Format("2006-01-02T15-04-05"))
	os.WriteFile(filepath.Join(dir, name), data, 0o644) //nolint:errcheck // best-effort log
}

func summarize(out io.Writer, results []engine.Result) {
	counts := map[string]int{}
	for _, r := range results {
		counts[r.OutcomeName]++
	}
	fmt.Fprintf(out, "\nsummary: %d adopted, %d reinstalled, %d needs-reinstall, %d failed\n",
		counts["adopted"], counts["reinstalled"], counts["needs-reinstall"], counts["failed"])
}
