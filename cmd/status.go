package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var statusOptions cli.StatusOptions

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether Righthook is installed correctly",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.Status(statusOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusOptions.Path, "path", ".", "Git repo root or .git path")
	statusCmd.Flags().StringVar(&statusOptions.ConfigPath, "config", "", "Config path to inspect")

	rootCmd.AddCommand(statusCmd)
}
