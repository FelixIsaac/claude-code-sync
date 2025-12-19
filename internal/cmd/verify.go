package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify file integrity",
	Long:  `Verify file integrity using SHA256 checksums from the manifest.`,
	RunE:  runVerify,
}

func runVerify(cmd *cobra.Command, args []string) error {
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
