package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/crypto"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var (
	pullDryRun bool
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull and decrypt configs from GitHub",
	Long:  `Pull configs from your GitHub repo and decrypt them to ~/.claude/`,
	RunE:  runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Show what would be restored without doing it")
}

func runPull(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	// Check prerequisites
	if !sync.FileExists(paths.KeyFile) {
		return fmt.Errorf("not initialized. Run 'claude-code-sync init' or 'claude-code-sync import-key' first")
	}
	if !sync.FileExists(paths.RepoDir) {
		return fmt.Errorf("no repo found. Run 'claude-code-sync init <repo-url>' first")
	}

	// Load identity for decryption
	identity, err := crypto.LoadKey(paths.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load key: %w", err)
	}

	// Load config
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	g := gitpkg.New(paths.RepoDir)

	// Pull from remote
	if g.HasRemote() && !pullDryRun {
		logInfo("Pulling from remote...")
		if err := g.Pull(); err != nil {
			logWarn(fmt.Sprintf("Pull failed: %v", err))
			logWarn("You may need to resolve conflicts manually.")
		}
	}

	// Backup current config
	if sync.FileExists(paths.ClaudeDir) && !pullDryRun {
		backupPath := filepath.Join(paths.BackupDir, fmt.Sprintf("backup-%s.zip", sync.Timestamp()))
		logInfo(fmt.Sprintf("Backing up current config to %s...", backupPath))
		if err := createBackupZip(paths.ClaudeDir, paths.ClaudeJSON, backupPath); err != nil {
			logWarn(fmt.Sprintf("Backup failed: %v", err))
		}

		// Keep only last N backups
		if err := pruneBackups(paths.BackupDir, cfg.Backup.MaxCount); err != nil {
			logWarn(fmt.Sprintf("Failed to prune backups: %v", err))
		}
	}

	if !pullDryRun {
		if err := sync.EnsureDir(paths.ClaudeDir); err != nil {
			return err
		}
	}

	if pullDryRun {
		logInfo("[DRY RUN] Would restore the following files:")
	} else {
		logInfo("Restoring files...")
	}

	// Process files from repo
	files, err := sync.WalkFiles(paths.RepoDir)
	if err != nil {
		return fmt.Errorf("failed to walk repo: %w", err)
	}

	count := 0
	for _, file := range files {
		relPath := sync.RelPath(paths.RepoDir, file)

		// Skip git and manifest
		if strings.HasPrefix(relPath, ".git") || relPath == ".sync-manifest" || relPath == "README.md" {
			continue
		}

		// Check base name (without .age) against exclude patterns
		basePath := strings.TrimSuffix(relPath, ".age")
		if cfg.ShouldExclude(basePath) {
			continue
		}

		var dest string
		actualRelPath := relPath

		// Handle encrypted files
		if strings.HasSuffix(relPath, ".age") {
			actualRelPath = strings.TrimSuffix(relPath, ".age")

			// Special case for claude.json
			if actualRelPath == "claude.json" {
				dest = paths.ClaudeJSON
			} else {
				dest = filepath.Join(paths.ClaudeDir, actualRelPath)
			}

			if pullDryRun {
				logInfo(fmt.Sprintf("  [decrypt] %s", actualRelPath))
			} else {
				// Backup if different
				if sync.FileExists(dest) {
					// TODO: compare content before backing up
					backupPath, _ := sync.BackupFile(dest)
					if backupPath != "" {
						logWarn(fmt.Sprintf("Conflict: backing up %s", actualRelPath))
					}
				}

				logInfo(fmt.Sprintf("Decrypting: %s", actualRelPath))
				if err := sync.EnsureDir(filepath.Dir(dest)); err != nil {
					return err
				}
				if err := crypto.DecryptFile(identity, file, dest); err != nil {
					return fmt.Errorf("failed to decrypt %s: %w", actualRelPath, err)
				}
			}
		} else {
			dest = filepath.Join(paths.ClaudeDir, relPath)

			if pullDryRun {
				logInfo(fmt.Sprintf("  [copy] %s", relPath))
			} else {
				// Backup if different
				if sync.FileExists(dest) {
					srcHash, _ := sync.FileChecksum(file)
					dstHash, _ := sync.FileChecksum(dest)
					if srcHash != dstHash {
						backupPath, _ := sync.BackupFile(dest)
						if backupPath != "" {
							logWarn(fmt.Sprintf("Conflict: backing up %s", relPath))
						}
					}
				}

				logInfo(fmt.Sprintf("Copying: %s", relPath))
				if err := sync.CopyFile(file, dest); err != nil {
					return fmt.Errorf("failed to copy %s: %w", relPath, err)
				}
			}
		}
		count++
	}

	if pullDryRun {
		logInfo(fmt.Sprintf("[DRY RUN] Would restore %d files", count))
	} else {
		// Expand cross-platform path placeholders to local paths
		if err := expandPluginPaths(paths.ClaudeDir); err != nil {
			logWarn(fmt.Sprintf("Failed to expand plugin paths: %v", err))
		}

		logSuccess(fmt.Sprintf("Pull complete! Restored %d files.", count))
	}

	return nil
}

// expandPluginPaths converts cross-platform placeholders to local platform paths
// in plugin configuration files after pulling from the repo.
func expandPluginPaths(claudeDir string) error {
	// Find all JSON files in plugins directory that may contain path placeholders
	pluginsDir := filepath.Join(claudeDir, "plugins")
	logInfo(fmt.Sprintf("Checking for plugin paths to expand in: %s", pluginsDir))
	if !sync.FileExists(pluginsDir) {
		logInfo("Plugins directory does not exist, skipping expansion")
		return nil
	}

	files, err := sync.WalkFiles(pluginsDir)
	if err != nil {
		return err
	}

	logInfo(fmt.Sprintf("Found %d files in plugins directory", len(files)))

	for _, file := range files {
		if !strings.HasSuffix(file, ".json") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Only process if file contains the placeholder
		if !strings.Contains(string(data), sync.ClaudeDirPlaceholder) {
			continue
		}

		logInfo(fmt.Sprintf("Found placeholder in: %s", file))

		expanded := sync.ExpandPathsInJSON(data, claudeDir)
		if err := os.WriteFile(file, expanded, 0644); err != nil {
			return fmt.Errorf("failed to write expanded %s: %w", file, err)
		}

		relPath := sync.RelPath(claudeDir, file)
		logInfo(fmt.Sprintf("Expanded paths: %s", relPath))
	}

	return nil
}

// createBackupZip creates a zip backup of the claude directory
func createBackupZip(claudeDir, claudeJSON, dest string) error {
	if err := sync.EnsureDir(filepath.Dir(dest)); err != nil {
		return err
	}

	zipFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// Add claude directory
	if sync.FileExists(claudeDir) {
		err := filepath.Walk(claudeDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			relPath, _ := filepath.Rel(filepath.Dir(claudeDir), path)
			f, err := w.Create(relPath)
			if err != nil {
				return err
			}

			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close()

			_, err = io.Copy(f, src)
			return err
		})
		if err != nil {
			return err
		}
	}

	// Add claude.json
	if sync.FileExists(claudeJSON) {
		f, err := w.Create(".claude.json")
		if err != nil {
			return err
		}
		src, err := os.Open(claudeJSON)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(f, src)
		if err != nil {
			return err
		}
	}

	return nil
}

// pruneBackups keeps only the last N backups
func pruneBackups(backupDir string, maxCount int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "backup-") && strings.HasSuffix(e.Name(), ".zip") {
			backups = append(backups, filepath.Join(backupDir, e.Name()))
		}
	}

	if len(backups) <= maxCount {
		return nil
	}

	// Sort by name (which includes timestamp) - oldest first
	// Actually we want newest first, so we remove from the end
	// The names are like backup-20251219-120000.zip so alphabetical = chronological

	// Remove oldest
	for i := 0; i < len(backups)-maxCount; i++ {
		if err := os.Remove(backups[i]); err != nil {
			return err
		}
	}

	return nil
}
