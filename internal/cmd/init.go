package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/crypto"
	"github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [repo-url]",
	Short: "Initialize sync (generate keys, clone/create repo)",
	Long: `Initialize claude-code-sync for this machine.

If no repo URL is provided, creates a local repo that you can later
connect to a remote with: git -C ~/.claude-sync/repo remote add origin <url>`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()
	repoURL := ""
	if len(args) > 0 {
		repoURL = args[0]
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
		if g.IsRepo() {
			logWarn(fmt.Sprintf("Repo already exists at %s", paths.RepoDir))
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
		fmt.Printf("  git -C %s remote add origin <your-repo-url>\n", paths.RepoDir)
		fmt.Println("  claude-code-sync push")
	}

	logSuccess("Initialization complete!")
	return nil
}
