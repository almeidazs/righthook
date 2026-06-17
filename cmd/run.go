package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var runOptions cli.RunOptions

var runCmd = &cobra.Command{
	Use:   "run <hook> [hook args...]",
	Short: "Run configured jobs for a Git hook",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		runOptions.Hook = args[0]
		runOptions.Args = append([]string(nil), args[1:]...)
		return commands.Run(runOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	runCmd.Flags().StringVar(&runOptions.Path, "path", ".", "Git repo root or .git path")
	runCmd.Flags().StringVar(&runOptions.ConfigPath, "config", "", "Config path to inspect")
	runCmd.Flags().BoolVar(&runOptions.NoCache, "no-cache", false, "Disable job cache for this run")
	runCmd.Flags().BoolVar(&runOptions.Fix, "fix", false, "Enable fix mode for jobs that honor RIGHTHOOK_FIX")
	runCmd.Flags().BoolVar(&runOptions.DryRun, "dry-run", false, "Show what would run without executing jobs")
	runCmd.Flags().StringSliceVar(&runOptions.Except, "except", nil, "Skip specific jobs by name")
	runCmd.Flags().StringSliceVar(&runOptions.Only, "only", nil, "Run only specific jobs by name")
	runCmd.Flags().BoolVar(&runOptions.Changed, "changed", false, "Resolve file placeholders using changed files")
	runCmd.Flags().BoolVar(&runOptions.Staged, "staged", false, "Resolve file placeholders using staged files")

	rootCmd.AddCommand(runCmd)
}
