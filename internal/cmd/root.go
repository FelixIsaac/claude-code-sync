package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	// Global flags
	verboseFlag bool
	quietFlag   bool
	noColorFlag bool
	debugFlag   bool

	rootCmd = &cobra.Command{
		Use:   "claude-code-sync",
		Short: "Sync Claude Code configs across machines",
		Long: `claude-code-sync - Secure Claude Code config sync across machines

Sync your ~/.claude/ configs via GitHub with age encryption.
Sensitive files (API keys, OAuth tokens) are encrypted before pushing.

QUICK START
  claude-code-sync config init --interactive   # First-time setup wizard
  claude-code-sync push                        # Sync local to remote
  claude-code-sync pull                        # Sync remote to local

COMMANDS
  Core:     push, pull, status
  Setup:    config init, key import/export
  Advanced: sync verify, doctor, reset, upgrade

EXAMPLES
  # First machine: initialize and push
  claude-code-sync config init git@github.com:you/configs.git
  claude-code-sync push

  # Second machine: import key and pull
  claude-code-sync key import
  claude-code-sync config init git@github.com:you/configs.git
  claude-code-sync pull

Learn more: https://github.com/felixisaac/claude-code-sync`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Apply no-color flag
			if noColorFlag {
				color.NoColor = true
			}
		},
	}
)

func SetVersion(v string) {
	version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Only show errors")
	rootCmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Show debug information")

	// ========== CORE COMMANDS (root level) ==========
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(statusCmd)

	// ========== COMMAND GROUPS ==========
	rootCmd.AddCommand(configCmd) // config init, config repo
	rootCmd.AddCommand(keyCmd)    // key import, key export
	rootCmd.AddCommand(syncCmd)   // sync verify, sync conflicts

	// ========== MAINTENANCE (root level) ==========
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(authCmd)

	// ========== OTHER ==========
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(versionCmd)

	// ========== DEPRECATED (keep for backward compat) ==========
	// These will be removed in v1.0
	rootCmd.AddCommand(deprecatedInitCmd)
	rootCmd.AddCommand(deprecatedImportKeyCmd)
	rootCmd.AddCommand(deprecatedExportKeyCmd)
	rootCmd.AddCommand(deprecatedVerifyCmd)
	rootCmd.AddCommand(deprecatedUpdateCmd)
	rootCmd.AddCommand(deprecatedCheckUpdateCmd)
}

// ========== DEPRECATED COMMAND WRAPPERS ==========
// These maintain backward compatibility and show deprecation warnings

var deprecatedInitCmd = &cobra.Command{
	Use:        "init [repo-url]",
	Short:      "Initialize sync (DEPRECATED: use 'config init')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync config init' instead",
	Args:       cobra.MaximumNArgs(1),
	RunE:       runConfigInit,
}

var deprecatedImportKeyCmd = &cobra.Command{
	Use:        "import-key",
	Short:      "Import private key (DEPRECATED: use 'key import')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync key import' instead",
	RunE:       runKeyImport,
}

var deprecatedExportKeyCmd = &cobra.Command{
	Use:        "export-key",
	Short:      "Display private key (DEPRECATED: use 'key export')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync key export' instead",
	RunE:       runKeyExport,
}

var deprecatedVerifyCmd = &cobra.Command{
	Use:        "verify",
	Short:      "Verify file integrity (DEPRECATED: use 'sync verify')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync sync verify' instead",
	RunE:       runSyncVerify,
}

var deprecatedUpdateCmd = &cobra.Command{
	Use:        "update",
	Short:      "Update to latest version (DEPRECATED: use 'upgrade')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync upgrade' instead",
	RunE:       runUpgrade,
}

var deprecatedCheckUpdateCmd = &cobra.Command{
	Use:        "check-update",
	Short:      "Check for updates (DEPRECATED: use 'upgrade --check')",
	Hidden:     true,
	Deprecated: "use 'claude-code-sync upgrade --check' instead",
	RunE: func(cmd *cobra.Command, args []string) error {
		upgradeCheckOnly = true
		return runUpgrade(cmd, args)
	},
}

// ========== UI HELPERS ==========

var (
	infoColor    = color.New(color.FgCyan)
	successColor = color.New(color.FgGreen)
	warnColor    = color.New(color.FgYellow)
	errorColor   = color.New(color.FgRed)
	debugColor   = color.New(color.FgMagenta)
)

// logInfo prints an info message (respects --quiet flag)
func logInfo(msg string) {
	if quietFlag {
		return
	}
	infoColor.Printf("[INFO] %s\n", msg)
}

// logSuccess prints a success message (always shown unless --quiet)
func logSuccess(msg string) {
	if quietFlag {
		return
	}
	successColor.Printf("[OK] %s\n", msg)
}

// logWarn prints a warning message (always shown)
func logWarn(msg string) {
	warnColor.Printf("[WARN] %s\n", msg)
}

// logError prints an error message (always shown)
func logError(msg string) {
	errorColor.Printf("[ERROR] %s\n", msg)
}

// logVerbose prints a verbose message (only with --verbose or --debug)
func logVerbose(msg string) {
	if !verboseFlag && !debugFlag {
		return
	}
	infoColor.Printf("[VERBOSE] %s\n", msg)
}

// logDebug prints a debug message (only with --debug)
func logDebug(msg string) {
	if !debugFlag {
		return
	}
	debugColor.Printf("[DEBUG] %s\n", msg)
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return verboseFlag || debugFlag
}

// IsDebug returns true if debug mode is enabled
func IsDebug() bool {
	return debugFlag
}

// IsQuiet returns true if quiet mode is enabled
func IsQuiet() bool {
	return quietFlag
}

// FatalError prints an error and exits
func FatalError(msg string) {
	logError(msg)
	os.Exit(1)
}

// Fprintf is a helper for formatted output that respects --quiet
func Fprintf(format string, a ...interface{}) {
	if quietFlag {
		return
	}
	fmt.Printf(format, a...)
}
