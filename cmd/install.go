package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var installOptions cli.InstallOptions

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Righthook Git hook scripts",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.Install(installOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	installCmd.Flags().BoolVar(&installOptions.Force, "force", false, "Overwrite existing Git hooks")
	installCmd.Flags().StringVar(&installOptions.Hook, "hook", "", "Install only one hook: pre-commit|commit-msg|pre-push")
	installCmd.Flags().StringVar(&installOptions.Path, "path", ".", "Git repo root or .git path")
	installCmd.Flags().StringVar(&installOptions.ConfigPath, "config", "", "Read hook selection from this config path")

	rootCmd.AddCommand(installCmd)
}
