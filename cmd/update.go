package cmd

import (
	"github.com/almeidazs/righthook/internal/commands"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Update the CLI to new version if it is available",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Update()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
