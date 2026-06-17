package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var uninstallOptions cli.UninstallOptions

var uninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove"},
	Short:   "Remove Righthook Git hook scripts",
	RunE: func(_ *cobra.Command, _ []string) error {
		return commands.Uninstall(uninstallOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	uninstallCmd.Flags().StringVar(&uninstallOptions.Hook, "hook", "", "Remove only one hook: pre-commit|commit-msg|pre-push")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.RemoveConfig, "remove-config", false, "Remove the default config and the path from --config when present")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.All, "all", false, "Remove all installed Righthook hooks")
	uninstallCmd.Flags().StringVar(&uninstallOptions.Path, "path", ".", "Git repo root or .git path")
	uninstallCmd.Flags().StringVar(&uninstallOptions.ConfigPath, "config", "", "Also remove this config path when used with --remove-config")

	rootCmd.AddCommand(uninstallCmd)
}
