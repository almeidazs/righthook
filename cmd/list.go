package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var listOptions cli.ListOptions

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured hooks and jobs",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.List(listOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	listCmd.Flags().StringVar(&listOptions.Path, "path", ".", "Git repo root or .git path")
	listCmd.Flags().StringVar(&listOptions.ConfigPath, "config", "", "Config path to inspect")
	listCmd.Flags().BoolVar(&listOptions.JSON, "json", false, "Emit machine-readable JSON output")
	listCmd.Flags().BoolVar(&listOptions.OnlyJobs, "only-jobs", false, "Print only jobs without hook headings")

	rootCmd.AddCommand(listCmd)
}
