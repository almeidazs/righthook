package cmd

import (
	"fmt"
	"os"

	"github.com/almeidazs/righthook/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "righthook",
	Short:         "Lefthook, but it hits from the right.",
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var (
	Verbose bool
)

func Exec() {
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Enable verbose logging")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
