package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

// syncCmd is the parent command for sync utilities
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Advanced sync operations",
	Long: `Advanced sync operation commands.

Subcommands:
  verify      Verify file integrity using checksums
  conflicts   Resolve merge conflicts (coming soon)`,
}

// syncVerifyCmd verifies file integrity
var syncVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify file integrity using checksums",
	Long:  `Verify file integrity by comparing SHA256 checksums against the manifest.`,
	RunE:  runSyncVerify,
}

// syncConflictsCmd resolves conflicts (placeholder)
var syncConflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "Resolve merge conflicts (coming soon)",
	Long:  `Interactive conflict resolution for merge conflicts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logWarn("Conflict resolution is coming soon.")
		fmt.Println()
		fmt.Println("For now, manually resolve conflicts in:")
		paths := config.GetPaths()
		fmt.Printf("  %s\n", toUnixPath(paths.RepoDir))
		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncVerifyCmd)
	syncCmd.AddCommand(syncConflictsCmd)
}

func runSyncVerify(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()
	manifestPath := filepath.Join(paths.RepoDir, ".sync-manifest")

	if !sync.FileExists(manifestPath) {
		return fmt.Errorf("no manifest found. Run 'claude-code-sync push' first")
	}

	logInfo("Verifying file integrity...")

	entries, err := sync.ReadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	errors := 0
	for _, entry := range entries {
		fullPath := filepath.Join(paths.RepoDir, entry.Path)

		if !sync.FileExists(fullPath) {
			logError(fmt.Sprintf("Missing: %s", entry.Path))
			errors++
			continue
		}

		actualChecksum, err := sync.FileChecksum(fullPath)
		if err != nil {
			logError(fmt.Sprintf("Failed to checksum: %s", entry.Path))
			errors++
			continue
		}

		if actualChecksum != entry.Checksum {
			logError(fmt.Sprintf("Checksum mismatch: %s", entry.Path))
			errors++
		} else {
			logSuccess(fmt.Sprintf("OK: %s", entry.Path))
		}
	}

	fmt.Println()
	if errors == 0 {
		logSuccess("All files verified!")
	} else {
		return fmt.Errorf("%d file(s) failed verification", errors)
	}

	return nil
}
