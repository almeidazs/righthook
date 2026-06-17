package cmd

import "github.com/spf13/cobra"

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Check repository policy requirements",
}

func init() {
	rootCmd.AddCommand(policyCmd)
}
