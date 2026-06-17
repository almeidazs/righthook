package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var statsOptions cli.StatsOptions

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregated hook execution stats",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.Stats(statsOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	statsCmd.Flags().StringVar(&statsOptions.Path, "path", ".", "Git repo root or .git path")
	statsCmd.Flags().StringVar(&statsOptions.ConfigPath, "config", "", "Config path to inspect")

	rootCmd.AddCommand(statsCmd)
}
