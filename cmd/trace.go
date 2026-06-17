package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var traceOptions cli.TraceOptions

var traceCmd = &cobra.Command{
	Use:   "trace <hook> [hook args...]",
	Short: "Run a hook and write a detailed execution trace to JSON",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		traceOptions.Hook = args[0]
		traceOptions.Args = append([]string(nil), args[1:]...)
		return commands.Trace(traceOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	traceCmd.Flags().StringVar(&traceOptions.Path, "path", ".", "Git repo root or .git path")
	traceCmd.Flags().StringVar(&traceOptions.ConfigPath, "config", "", "Config path to inspect")
	traceCmd.Flags().BoolVar(&traceOptions.NoCache, "no-cache", false, "Disable job cache for this run")
	traceCmd.Flags().BoolVar(&traceOptions.Fix, "fix", false, "Enable fix mode for jobs that honor RIGHTHOOK_FIX")
	traceCmd.Flags().BoolVar(&traceOptions.DryRun, "dry-run", false, "Show what would run without executing jobs")
	traceCmd.Flags().StringSliceVar(&traceOptions.Except, "except", nil, "Skip specific jobs by name")
	traceCmd.Flags().StringSliceVar(&traceOptions.Only, "only", nil, "Run only specific jobs by name")
	traceCmd.Flags().BoolVar(&traceOptions.Changed, "changed", false, "Resolve file placeholders using changed files")
	traceCmd.Flags().BoolVar(&traceOptions.Staged, "staged", false, "Resolve file placeholders using staged files")
	traceCmd.Flags().StringVar(&traceOptions.OutputPath, "output", "", "Required .json file path to write the trace")

	rootCmd.AddCommand(traceCmd)
}
