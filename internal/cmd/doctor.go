package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health",
	Long:  `Verify that all dependencies and configurations are correct.`,
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()
	allOk := true

	color.Cyan("=== claude-code-sync doctor ===")
	fmt.Println()

	// Check git
	fmt.Print("Git installed: ")
	if gitpkg.IsInstalled() {
		color.Green("OK")
	} else {
		color.Red("NOT FOUND")
		allOk = false
	}

	// Check age library (it's built-in, so always OK)
	fmt.Print("Age encryption: ")
	color.Green("OK (built-in)")

	// Check sync directory
	fmt.Print("Sync directory: ")
	if sync.FileExists(paths.SyncDir) {
		color.Green("OK (%s)", paths.SyncDir)
	} else {
		color.Yellow("NOT INITIALIZED")
	}

	// Check key file
	fmt.Print("Private key: ")
	if sync.FileExists(paths.KeyFile) {
		color.Green("OK (%s)", paths.KeyFile)
	} else {
		color.Yellow("NOT FOUND - run 'init' or 'import-key'")
	}

	// Check repo
	fmt.Print("Local repo: ")
	if sync.FileExists(paths.RepoDir) {
		g := gitpkg.New(paths.RepoDir)
		if g.IsRepo() {
			color.Green("OK (%s)", paths.RepoDir)
		} else {
			color.Yellow("EXISTS but not a git repo")
		}
	} else {
		color.Yellow("NOT FOUND - run 'init'")
	}

	// Check remote
	fmt.Print("Remote origin: ")
	if sync.FileExists(paths.RepoDir) {
		g := gitpkg.New(paths.RepoDir)
		if g.HasRemote() {
			color.Green("CONFIGURED")
		} else {
			color.Yellow("NOT CONFIGURED")
		}
	} else {
		color.Yellow("N/A")
	}

	// Check claude directory
	fmt.Print("Claude directory: ")
	if sync.FileExists(paths.ClaudeDir) {
		color.Green("OK (%s)", paths.ClaudeDir)
	} else {
		color.Yellow("NOT FOUND")
	}

	// Check claude.json
	fmt.Print("Claude config: ")
	if sync.FileExists(paths.ClaudeJSON) {
		color.Green("OK (%s)", paths.ClaudeJSON)
	} else {
		color.Yellow("NOT FOUND (optional)")
	}

	fmt.Println()
	if allOk {
		logSuccess("All checks passed!")
	} else {
		logWarn("Some issues found. Please install missing dependencies.")
	}

	return nil
}
