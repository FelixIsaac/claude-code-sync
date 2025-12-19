package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long:  `Show the current sync status, including local and remote state.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.RepoDir) {
		return fmt.Errorf("no repo found. Run 'claude-code-sync init' first")
	}

	// Load config
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	color.Cyan("=== claude-code-sync status ===")
	fmt.Println()

	g := gitpkg.New(paths.RepoDir)

	// Check remote status
	if g.HasRemote() {
		g.Fetch()
		localCommit, _ := g.GetLocalCommit()
		remoteCommit, _ := g.GetRemoteCommit()

		if localCommit == remoteCommit && localCommit != "" {
			fmt.Print("Remote: ")
			color.Green("Up to date")
		} else if localCommit != "" && remoteCommit != "" {
			fmt.Print("Remote: ")
			local := localCommit
			remote := remoteCommit
			if len(local) > 7 {
				local = local[:7]
			}
			if len(remote) > 7 {
				remote = remote[:7]
			}
			color.Yellow("Out of sync (local: %s, remote: %s)", local, remote)
		} else {
			fmt.Print("Remote: ")
			color.Yellow("Unknown state")
		}
	} else {
		fmt.Print("Remote: ")
		color.Yellow("Not configured")
	}

	fmt.Println()
	fmt.Println("Local files in ~/.claude:")

	if sync.FileExists(paths.ClaudeDir) {
		files, err := sync.WalkFiles(paths.ClaudeDir)
		if err != nil {
			return err
		}

		for _, file := range files {
			relPath := sync.RelPath(paths.ClaudeDir, file)

			if cfg.ShouldExclude(relPath) {
				color.Yellow("  [excluded] %s", relPath)
			} else if cfg.ShouldEncrypt(relPath) {
				color.Cyan("  [encrypted] %s", relPath)
			} else {
				color.Green("  [plain] %s", relPath)
			}
		}
	} else {
		fmt.Println("  (none)")
	}

	if sync.FileExists(paths.ClaudeJSON) {
		color.Cyan("  [encrypted] ~/.claude.json")
	}

	fmt.Println()
	fmt.Printf("Repo files in %s:\n", paths.RepoDir)

	if sync.FileExists(paths.RepoDir) {
		files, err := sync.WalkFiles(paths.RepoDir)
		if err != nil {
			return err
		}

		for _, file := range files {
			relPath := sync.RelPath(paths.RepoDir, file)

			if strings.HasPrefix(relPath, ".git") {
				continue
			}

			if strings.HasSuffix(relPath, ".age") {
				color.Cyan("  [encrypted] %s", relPath)
			} else {
				color.Green("  [plain] %s", relPath)
			}
		}
	}

	return nil
}
