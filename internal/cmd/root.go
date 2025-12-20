package cmd

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	rootCmd = &cobra.Command{
		Use:   "claude-code-sync",
		Short: "Sync Claude Code configs across machines",
		Long: `claude-code-sync - Secure Claude Code config sync across machines

Sync your ~/.claude/ configs via GitHub with age encryption.
Sensitive files (API keys, OAuth tokens) are encrypted before pushing.`,
	}
)

func SetVersion(v string) {
	version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(importKeyCmd)
	rootCmd.AddCommand(exportKeyCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(checkUpdateCmd)
	rootCmd.AddCommand(updateCmd)
}

// UI helpers
var (
	infoColor    = color.New(color.FgCyan)
	successColor = color.New(color.FgGreen)
	warnColor    = color.New(color.FgYellow)
	errorColor   = color.New(color.FgRed)
)

func logInfo(msg string) {
	infoColor.Printf("[INFO] %s\n", msg)
}

func logSuccess(msg string) {
	successColor.Printf("[OK] %s\n", msg)
}

func logWarn(msg string) {
	warnColor.Printf("[WARN] %s\n", msg)
}

func logError(msg string) {
	errorColor.Printf("[ERROR] %s\n", msg)
}
