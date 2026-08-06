package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Configure GitHub authentication for sync repo",
	Long: `Configure GitHub credentials for the sync repository.

This sets up repo-specific authentication so you can use a fine-grained
Personal Access Token (PAT) scoped to just your sync repo, without
affecting other git/gh credentials on the machine.

Usage:
  claude-code-sync auth         # Show current auth status
  claude-code-sync auth setup   # Configure authentication
  claude-code-sync auth test    # Test if authentication works`,
	RunE: runAuthStatus,
}

var authSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up GitHub authentication",
	RunE:  runAuthSetup,
}

var authTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test GitHub authentication",
	RunE:  runAuthTest,
}

func init() {
	authCmd.AddCommand(authSetupCmd)
	authCmd.AddCommand(authTestCmd)
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	color.Cyan("=== GitHub Authentication Status ===")
	fmt.Println()

	// Check if repo exists
	if !sync.FileExists(paths.RepoDir) {
		logWarn("No sync repo found. Run 'claude-code-sync init' first.")
		return nil
	}

	g := gitpkg.New(paths.RepoDir)

	// Show remote URL
	remoteURL := g.GetRemoteURL()
	fmt.Printf("Remote URL: %s\n", remoteURL)

	// Determine auth type
	if strings.HasPrefix(remoteURL, "git@") {
		fmt.Printf("Auth type: SSH\n")
	} else if strings.HasPrefix(remoteURL, "https://") {
		fmt.Printf("Auth type: HTTPS\n")
	}

	// Check if credentials are configured
	fmt.Print("Credentials: ")
	if canAccessRepo(paths.RepoDir) {
		color.Green("OK (can access remote)")
	} else {
		color.Red("FAILED (cannot access remote)")
		fmt.Println()
		logInfo("Run 'claude-code-sync auth setup' to configure authentication")
	}

	return nil
}

func runAuthSetup(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.RepoDir) {
		return fmt.Errorf("no sync repo found. Run 'claude-code-sync init' first")
	}

	g := gitpkg.New(paths.RepoDir)
	remoteURL := g.GetRemoteURL()

	color.Cyan("=== GitHub Authentication Setup ===")
	fmt.Println()
	fmt.Println("This will configure authentication for your sync repo only.")
	fmt.Println("Other repos on this machine won't be affected.")
	fmt.Println()

	// Extract repo info from URL
	repoPath := extractRepoPath(remoteURL)
	if repoPath == "" {
		return fmt.Errorf("could not parse remote URL: %s", remoteURL)
	}

	fmt.Printf("Sync repo: %s\n", repoPath)
	fmt.Println()

	// Guide user to create PAT
	color.Yellow("Step 1: Create a Fine-grained Personal Access Token")
	fmt.Println()
	fmt.Println("1. Go to: https://github.com/settings/tokens?type=beta")
	fmt.Println("2. Click 'Generate new token'")
	fmt.Println("3. Name: 'claude-code-sync' (or any name)")
	fmt.Println("4. Repository access: 'Only select repositories'")
	fmt.Printf("   → Select: %s\n", repoPath)
	fmt.Println("5. Permissions: Contents (Read and write)")
	fmt.Println("6. Generate and copy the token")
	fmt.Println()

	// Prompt for token
	color.Yellow("Step 2: Enter your token")
	fmt.Print("Paste your Personal Access Token: ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		logWarn("No token provided. Setup cancelled.")
		return nil
	}

	// Extract username from repo path
	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid repo path: %s", repoPath)
	}
	username := parts[0]

	// Configure git credential for this repo
	color.Yellow("Step 3: Configuring credentials...")

	// Set credential helper to store for this repo
	if err := runGitConfig(paths.RepoDir, "credential.helper", "store"); err != nil {
		return fmt.Errorf("failed to configure credential helper: %w", err)
	}

	// Write credentials file
	credFile := filepath.Join(paths.SyncDir, ".git-credentials")
	credLine := fmt.Sprintf("https://%s:%s@github.com\n", username, token)
	if err := os.WriteFile(credFile, []byte(credLine), 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	// Point git to use this credentials file
	if err := runGitConfig(paths.RepoDir, "credential.helper", fmt.Sprintf("store --file=%s", credFile)); err != nil {
		return fmt.Errorf("failed to configure credential file: %w", err)
	}

	// Ensure HTTPS URL
	if strings.HasPrefix(remoteURL, "git@") {
		httpsURL := convertToHTTPS(remoteURL)
		if err := g.SetRemoteURL(httpsURL); err != nil {
			logWarn(fmt.Sprintf("Could not switch to HTTPS URL: %v", err))
		} else {
			logInfo(fmt.Sprintf("Switched remote to HTTPS: %s", httpsURL))
		}
	}

	fmt.Println()

	// Test connection
	color.Yellow("Step 4: Testing connection...")
	if canAccessRepo(paths.RepoDir) {
		logSuccess("Authentication configured successfully!")
		fmt.Println()
		fmt.Println("You can now use 'claude-code-sync push' and 'claude-code-sync pull'")
	} else {
		logWarn("Authentication test failed. Please check your token.")
		logInfo("Make sure the token has 'Contents' permission for the repo.")
	}

	return nil
}

func runAuthTest(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.RepoDir) {
		return fmt.Errorf("no sync repo found. Run 'claude-code-sync init' first")
	}

	fmt.Print("Testing GitHub access... ")

	if canAccessRepo(paths.RepoDir) {
		color.Green("OK")
		logSuccess("Can access remote repository")
	} else {
		color.Red("FAILED")
		logWarn("Cannot access remote repository")
		logInfo("Run 'claude-code-sync auth setup' to configure authentication")
	}

	return nil
}

// canAccessRepo tests if we can fetch from the remote
func canAccessRepo(repoDir string) bool {
	cmd := exec.Command("git", "-C", repoDir, "ls-remote", "--exit-code", "origin", "HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.Run() == nil
}

// extractRepoPath extracts "owner/repo" from a git URL
func extractRepoPath(url string) string {
	// Handle SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@github.com:") {
		path := strings.TrimPrefix(url, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		return path
	}

	// Handle HTTPS: https://github.com/owner/repo.git
	if strings.Contains(url, "github.com/") {
		idx := strings.Index(url, "github.com/")
		path := url[idx+len("github.com/"):]
		path = strings.TrimSuffix(path, ".git")
		return path
	}

	return ""
}

// convertToHTTPS converts SSH URL to HTTPS
func convertToHTTPS(sshURL string) string {
	// git@github.com:owner/repo.git -> https://github.com/owner/repo.git
	if strings.HasPrefix(sshURL, "git@github.com:") {
		path := strings.TrimPrefix(sshURL, "git@github.com:")
		return "https://github.com/" + path
	}
	return sshURL
}

// runGitConfig runs git config for the repo
func runGitConfig(repoDir, key, value string) error {
	cmd := exec.Command("git", "-C", repoDir, "config", key, value)
	return cmd.Run()
}
