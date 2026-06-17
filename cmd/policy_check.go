package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var policyCheckOptions cli.PolicyCheckOptions

var policyCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate the configured Righthook policy",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.PolicyCheck(policyCheckOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	policyCheckCmd.Flags().StringVar(&policyCheckOptions.Path, "path", ".", "Git repo root or .git path")
	policyCheckCmd.Flags().StringVar(&policyCheckOptions.ConfigPath, "config", "", "Config path to inspect")

	policyCmd.AddCommand(policyCheckCmd)
}
