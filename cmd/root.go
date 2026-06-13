package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "brewmaster",
		Short:        "Audit installed macOS apps and adopt them into Homebrew",
		Long:         "brewmaster audits installed macOS apps and adopts them into Homebrew.",
		SilenceUsage: true,
	}
	root.AddCommand(newAuditCmd(AuditDeps{}))
	root.AddCommand(newAdoptCmd(AdoptDeps{}))
	return root
}
