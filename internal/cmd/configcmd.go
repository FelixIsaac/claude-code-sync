package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/crypto"
	"github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

// toUnixPath converts Windows backslashes to forward slashes for display
func toUnixPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

var (
	configLocalOnly    bool
	configInteractive  bool
)

// configCmd is the parent command for config subcommands
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration and initialization",
	Long: `Configuration and initialization commands.

Subcommands:
  init    Initialize sync (generate keys, clone/create repo)
  repo    Show or update repository URL`,
}

// configInitCmd initializes sync
var configInitCmd = &cobra.Command{
	Use:   "init [repo-url]",
	Short: "Initialize sync (generate keys, clone/create repo)",
	Long: `Initialize claude-code-sync for this machine.

If no repo URL is provided, creates a local repo that you can later
connect to a remote with: git -C ~/.claude-sync/repo remote add origin <url>

Use --interactive for a guided setup wizard (coming soon).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigInit,
}

// configRepoCmd shows or updates repo URL
var configRepoCmd = &cobra.Command{
	Use:   "repo [new-url]",
	Short: "Show or update repository URL",
	Long: `Show the current repository URL, or set a new one.

Examples:
  claude-code-sync config repo                           # Show current URL
  claude-code-sync config repo git@github.com:u/r.git   # Set new URL`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigRepo,
}

func init() {
	configInitCmd.Flags().BoolVar(&configLocalOnly, "local", false, "Create local repo only (no remote)")
	configInitCmd.Flags().BoolVar(&configInteractive, "interactive", false, "Use interactive setup wizard")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configRepoCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()
	repoURL := ""
	if len(args) > 0 {
		repoURL = args[0]
	}

	// TODO: Add interactive wizard support
	if configInteractive {
		logWarn("Interactive wizard coming soon. Using standard init flow.")
	}

	logInfo("Initializing claude-code-sync...")

	// Check dependencies
	if !git.IsInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Create directories
	if err := sync.EnsureDir(paths.SyncDir); err != nil {
		return fmt.Errorf("failed to create sync dir: %w", err)
	}
	if err := sync.EnsureDir(paths.BackupDir); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Generate or show existing key
	if sync.FileExists(paths.KeyFile) {
		logWarn(fmt.Sprintf("Key already exists at %s", paths.KeyFile))
		pubKey, err := crypto.GetPublicKey(paths.KeyFile)
		if err != nil {
			return err
		}
		logInfo(fmt.Sprintf("Public key: %s", pubKey))
	} else {
		logInfo("Generating age keypair...")

		identity, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}

		// Write key file
		keyContent := fmt.Sprintf("# public key: %s\n%s\n",
			identity.Recipient().String(),
			identity.String(),
		)
		if err := os.WriteFile(paths.KeyFile, []byte(keyContent), 0600); err != nil {
			return fmt.Errorf("failed to write key: %w", err)
		}

		// Display key prominently
		fmt.Println()
		color.Red("========================================")
		color.Red("   IMPORTANT: SAVE YOUR PRIVATE KEY!   ")
		color.Red("========================================")
		fmt.Println()
		fmt.Print("Your PRIVATE KEY is the line starting with ")
		color.Cyan("AGE-SECRET-KEY-")
		fmt.Println(":")
		fmt.Println()
		fmt.Printf("# public key: %s\n", identity.Recipient().String())
		color.Green(identity.String())
		fmt.Print("   ")
		color.Yellow("<-- COPY THIS!")
		fmt.Println()
		fmt.Println()
		color.Cyan("Copy the ENTIRE block above (including comments) to import on other machines.")
		color.Yellow("This key will NOT be shown again!")
		fmt.Println()
	}

	// Setup repo
	g := git.New(paths.RepoDir)

	if repoURL != "" {
		// Validate URL format
		if !git.IsValidRepoURL(repoURL) {
			return fmt.Errorf("invalid repo URL: %s\nExpected format: https://github.com/user/repo or git@github.com:user/repo.git", repoURL)
		}

		// Check if URL is reachable
		logInfo("Verifying repo URL...")
		if err := git.CheckRemote(repoURL); err != nil {
			return fmt.Errorf("cannot access repo: %w\nCheck the URL and your permissions", err)
		}

		if g.IsRepo() {
			logWarn(fmt.Sprintf("Repo already exists at %s", toUnixPath(paths.RepoDir)))
		} else {
			logInfo("Cloning repo...")
			if err := git.Clone(repoURL, paths.RepoDir); err != nil {
				return fmt.Errorf("failed to clone: %w", err)
			}
		}
	} else {
		if !g.IsRepo() {
			logInfo("Creating local repo (you'll need to add a remote later)...")
			if err := g.Init(); err != nil {
				return fmt.Errorf("failed to init repo: %w", err)
			}
			if err := g.CreateInitialCommit(); err != nil {
				return fmt.Errorf("failed to create initial commit: %w", err)
			}
		}
		fmt.Println()
		logInfo("No repo URL provided. To add a remote later:")
		fmt.Printf("  git -C \"%s\" remote add origin <your-repo-url>\n", toUnixPath(paths.RepoDir))
		fmt.Println("  claude-code-sync push")
	}

	logSuccess("Initialization complete!")
	return nil
}

func runConfigRepo(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()
	g := git.New(paths.RepoDir)

	if !g.IsRepo() {
		return fmt.Errorf("no repository found. Run 'claude-code-sync config init' first")
	}

	if len(args) == 0 {
		// Show current URL
		url := g.GetRemoteURL()
		if url == "" {
			logWarn("No remote URL configured")
			fmt.Println()
			fmt.Println("To add a remote:")
			fmt.Printf("  claude-code-sync config repo git@github.com:user/repo.git\n")
		} else {
			fmt.Printf("Repository: %s\n", url)
		}
		return nil
	}

	// Set new URL
	newURL := args[0]
	if !git.IsValidRepoURL(newURL) {
		return fmt.Errorf("invalid repo URL: %s", newURL)
	}

	logInfo("Verifying new URL...")
	if err := git.CheckRemote(newURL); err != nil {
		return fmt.Errorf("cannot access repo: %w", err)
	}

	if err := g.SetRemoteURL(newURL); err != nil {
		return fmt.Errorf("failed to set remote URL: %w", err)
	}

	logSuccess(fmt.Sprintf("Repository URL updated to: %s", newURL))
	return nil
}
