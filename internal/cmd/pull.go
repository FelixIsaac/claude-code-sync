package cmd

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/crypto"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var (
	pullDryRun   bool
	pullOurs     bool
	pullTheirs   bool
	pullShowDiff bool
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull and decrypt configs from GitHub",
	Long: `Pull configs from your GitHub repo and decrypt them to ~/.claude/

Conflict handling:
  By default, remote changes overwrite local (with backup).
  Use --ours to keep local versions when they differ from remote.
  Use --diff to preview differences without applying changes.`,
	RunE: runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Show what would be restored without doing it")
	pullCmd.Flags().BoolVar(&pullOurs, "ours", false, "Keep local files when they differ from remote")
	pullCmd.Flags().BoolVar(&pullTheirs, "theirs", false, "Apply remote files, backup local (default behavior)")
	pullCmd.Flags().BoolVar(&pullShowDiff, "diff", false, "Show differences between local and remote without applying")
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate mutually exclusive flags
	flagCount := 0
	if pullOurs {
		flagCount++
	}
	if pullTheirs {
		flagCount++
	}
	if pullShowDiff {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("--ours, --theirs, and --diff are mutually exclusive")
	}

	// Determine strategy (default: theirs)
	strategy := "theirs"
	if pullOurs {
		strategy = "ours"
	} else if pullShowDiff {
		strategy = "diff"
	}

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

			// Show age of local repo when pull fails
			if age := getRepoAge(paths.RepoDir); age != "" {
				logWarn(fmt.Sprintf("Using cached files from: %s", age))
			}
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
	} else if strategy == "diff" {
		logInfo("Comparing local vs remote (no changes will be applied):")
	} else if strategy == "ours" {
		logInfo("Pulling with --ours: keeping local files where they differ")
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

		// Skip platform variants for other platforms
		// e.g., on Windows, skip .unix.md files; on Unix, skip .windows.md files
		if sync.ShouldSkipForPlatform(basePath) {
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
			} else if strategy == "diff" {
				// Show diff for encrypted files (decrypt to temp, compare)
				if sync.FileExists(dest) {
					logInfo(fmt.Sprintf("  [encrypted] %s (local exists, remote differs)", actualRelPath))
				} else {
					logInfo(fmt.Sprintf("  [encrypted] %s (new file)", actualRelPath))
				}
			} else {
				// Check if local exists and differs
				localExists := sync.FileExists(dest)

				if localExists && strategy == "ours" {
					// Keep local, skip remote
					logInfo(fmt.Sprintf("Keeping local: %s", actualRelPath))
				} else {
					// theirs strategy: backup and apply
					if localExists {
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
			}
		} else {
			dest = filepath.Join(paths.ClaudeDir, relPath)

			if pullDryRun {
				logInfo(fmt.Sprintf("  [copy] %s", relPath))
			} else {
				// Check if local exists and differs
				localExists := sync.FileExists(dest)
				var differs bool
				if localExists {
					srcHash, _ := sync.FileChecksum(file)
					dstHash, _ := sync.FileChecksum(dest)
					differs = srcHash != dstHash
				}

				if strategy == "diff" {
					// Show diff
					if !localExists {
						logInfo(fmt.Sprintf("  [new] %s", relPath))
					} else if differs {
						logInfo(fmt.Sprintf("  [changed] %s", relPath))
						showFileDiff(dest, file, relPath)
					} else {
						// Same content, skip
						continue
					}
				} else if localExists && differs && strategy == "ours" {
					// Keep local, skip remote
					logInfo(fmt.Sprintf("Keeping local: %s", relPath))
				} else if !localExists || differs {
					// theirs strategy: backup and apply
					if localExists && differs {
						backupPath, _ := sync.BackupFile(dest)
						if backupPath != "" {
							logWarn(fmt.Sprintf("Conflict: backing up %s", relPath))
						}
					}

					logInfo(fmt.Sprintf("Copying: %s", relPath))
					if err := sync.CopyFile(file, dest); err != nil {
						return fmt.Errorf("failed to copy %s: %w", relPath, err)
					}
				}
			}
		}
		count++
	}

	if pullDryRun {
		logInfo(fmt.Sprintf("[DRY RUN] Would restore %d files", count))
	} else if strategy == "diff" {
		logInfo(fmt.Sprintf("Diff complete. %d files would be affected.", count))
		logInfo("Run 'sync pull' to apply changes, or 'sync pull --ours' to keep local.")
	} else if strategy == "ours" {
		logSuccess(fmt.Sprintf("Pull complete (--ours)! Kept local versions, %d files checked.", count))
	} else {
		// Expand cross-platform path placeholders to local paths
		if err := expandPluginPaths(paths.ClaudeDir); err != nil {
			logWarn(fmt.Sprintf("Failed to expand plugin paths: %v", err))
		}

		logSuccess(fmt.Sprintf("Pull complete! Restored %d files.", count))
	}

	return nil
}

// showFileDiff displays a simple diff between local and remote files
func showFileDiff(localPath, remotePath, relPath string) {
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return
	}
	remoteData, err := os.ReadFile(remotePath)
	if err != nil {
		return
	}

	localLines := strings.Split(string(localData), "\n")
	remoteLines := strings.Split(string(remoteData), "\n")

	// Simple diff: show line count difference and first few differing lines
	fmt.Printf("    Local:  %d lines\n", len(localLines))
	fmt.Printf("    Remote: %d lines\n", len(remoteLines))

	// Find first difference
	maxLines := len(localLines)
	if len(remoteLines) > maxLines {
		maxLines = len(remoteLines)
	}

	diffCount := 0
	for i := 0; i < maxLines && diffCount < 3; i++ {
		var localLine, remoteLine string
		if i < len(localLines) {
			localLine = localLines[i]
		}
		if i < len(remoteLines) {
			remoteLine = remoteLines[i]
		}
		if localLine != remoteLine {
			diffCount++
			if len(localLine) > 60 {
				localLine = localLine[:60] + "..."
			}
			if len(remoteLine) > 60 {
				remoteLine = remoteLine[:60] + "..."
			}
			fmt.Printf("    Line %d:\n", i+1)
			fmt.Printf("      - %s\n", localLine)
			fmt.Printf("      + %s\n", remoteLine)
		}
	}
	if diffCount == 0 {
		fmt.Println("    (content differs but no line-by-line diff available)")
	}
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

// getRepoAge returns a human-readable string showing when the repo was last updated
func getRepoAge(repoDir string) string {
	// Get last commit timestamp using git log
	cmd := exec.Command("git", "-C", repoDir, "log", "-1", "--format=%ai")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	timestampStr := strings.TrimSpace(string(output))
	if timestampStr == "" {
		return ""
	}

	// Parse timestamp (format: "2025-12-19 21:22:01 +0800")
	// Use only the date+time part, ignore timezone
	parts := strings.Fields(timestampStr)
	if len(parts) < 2 {
		return ""
	}
	dateTimeStr := parts[0] + " " + parts[1]

	lastCommit, err := time.Parse("2006-01-02 15:04:05", dateTimeStr)
	if err != nil {
		return ""
	}

	age := time.Since(lastCommit)

	// Format based on age
	if age < time.Hour {
		return fmt.Sprintf("minutes ago (%s)", lastCommit.Format("2006-01-02 15:04"))
	} else if age < 24*time.Hour {
		hours := int(age.Hours())
		return fmt.Sprintf("%d hour(s) ago (%s)", hours, lastCommit.Format("2006-01-02 15:04"))
	} else {
		days := int(age.Hours() / 24)
		return fmt.Sprintf("%d day(s) ago (%s)", days, lastCommit.Format("2006-01-02"))
	}
}
