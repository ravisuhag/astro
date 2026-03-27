package cli

import (
	"github.com/raystack/salt/cli/commander"
	"github.com/spf13/cobra"
)

// New creates the root command for the astro CLI.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "astro <command> <subcommand> [flags]",
		Short: "CCSDS and ECSS space communication toolkit",
		Long:  "Astro is a CLI toolkit for working with CCSDS and ECSS space communication protocols.",
		Annotations: map[string]string{
			"help:learn":    "Use 'astro <command> --help' for more information about a command.",
			"help:feedback": "Open an issue at https://github.com/ravisuhag/astro/issues",
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(sppCmd())

	mgr := commander.New(cmd)
	mgr.Init()

	return cmd
}
