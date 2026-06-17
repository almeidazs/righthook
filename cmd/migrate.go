package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var migrateOptions cli.MigrateOptions

var migrateCmd = &cobra.Command{
	Use:   "migrate <lefthook|husky>",
	Short: "Migrate hook configuration from Lefthook or Husky",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		migrateOptions.Target = args[0]

		return commands.Migrate(migrateOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	migrateCmd.Flags().StringVar(&migrateOptions.Path, "path", ".", "Git repo root or .git path")
	migrateCmd.Flags().StringVar(&migrateOptions.ConfigPath, "config", "", "Righthook config path to write or merge into")
	migrateCmd.Flags().BoolVar(&migrateOptions.DryRun, "dry-run", false, "Show what would be migrated without writing files")
	migrateCmd.Flags().BoolVar(&migrateOptions.KeepTargetConfig, "keep-target-config", true, "Keep the target manager config after a successful migration")
	migrateCmd.Flags().BoolVar(&migrateOptions.Write, "write", false, "Write the merged Righthook config without prompting")

	rootCmd.AddCommand(migrateCmd)
}
