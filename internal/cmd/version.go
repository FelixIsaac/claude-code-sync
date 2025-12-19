package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"-v", "--version"},
	Short:   "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("claude-code-sync v%s\n", version)
	},
}
