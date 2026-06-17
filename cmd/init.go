package cmd

import (
	"os"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var initOptions cli.InitOptions

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Righthook in the current repository",
	RunE: func(cmd *cobra.Command, _ []string) error {
		initOptions.InstallSpecified = cmd.Flags().Changed("install") || cmd.Flags().Changed("no-install")
		initOptions.CacheSpecified = cmd.Flags().Changed("cache") || cmd.Flags().Changed("no-cache")
		initOptions.SafetySpecified = cmd.Flags().Changed("safety")
		initOptions.PMSpecified = cmd.Flags().Changed("pm")
		initOptions.ModeSpecified = cmd.Flags().Changed("mode")
		initOptions.MonorepoSpecified = cmd.Flags().Changed("monorepo")

		return commands.Init(initOptions, cli.Runtime{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initOptions.Yes, "yes", "y", false, "Use safe recommended defaults without prompts")
	initCmd.Flags().BoolVar(&initOptions.Install, "install", false, "Install generated Git hooks")
	initCmd.Flags().BoolVar(&initOptions.NoInstall, "no-install", false, "Only write config and skip hook installation")
	initCmd.Flags().BoolVar(&initOptions.Force, "force", false, "Overwrite existing config and hooks")
	initCmd.Flags().BoolVar(&initOptions.DryRun, "dry-run", false, "Show what would be created without writing files")
	initCmd.Flags().BoolVar(&initOptions.PrintOnly, "print", false, "Print generated config to stdout only")
	initCmd.Flags().StringVar(&initOptions.Mode, "mode", string(cli.ModeRecommended), "Init mode: recommended|minimal|strict|custom")
	initCmd.Flags().StringVar(&initOptions.Preset, "preset", "", "Preset: node|next|nestjs|monorepo|go|rust|python")
	initCmd.Flags().StringVar(&initOptions.PM, "pm", "", "Package manager: pnpm|npm|yarn|bun")
	initCmd.Flags().StringVar(&initOptions.Hooks, "hooks", "", "Comma-separated hooks: pre-commit,commit-msg,pre-push")
	initCmd.Flags().BoolVar(&initOptions.Cache, "cache", false, "Enable cache")
	initCmd.Flags().BoolVar(&initOptions.NoCache, "no-cache", false, "Disable cache")
	initCmd.Flags().StringVar(&initOptions.Safety, "safety", "", "Safety mode: smart|fast|strict|off")
	initCmd.Flags().StringVar(&initOptions.Monorepo, "monorepo", "auto", "Monorepo mode: auto|on|off")
	initCmd.Flags().StringVar(&initOptions.Base, "base", "origin/main", "Base ref for affected/monorepo runs")
	initCmd.Flags().StringVar(&initOptions.Migrate, "migrate", "", "Migration source: lefthook|husky|lint-staged")
	initCmd.Flags().StringVar(&initOptions.ConfigPath, "config", "righthook.yml", "Path to write config")
	initCmd.Flags().StringVar(&initOptions.CWD, "cwd", ".", "Working directory")
	initCmd.Flags().BoolVar(&initOptions.NoColor, "no-color", false, "Disable colored output")
	initCmd.Flags().BoolVar(&initOptions.NoEmoji, "no-emoji", false, "Disable unicode symbols")
	initCmd.Flags().BoolVar(&initOptions.JSON, "json", false, "Emit machine-readable JSON output")

	rootCmd.AddCommand(initCmd)
}
