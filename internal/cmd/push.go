package cmd

import (
	"fmt"
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
	pushDryRun          bool
	pushNoPlatformCheck bool
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Encrypt and push configs to GitHub",
	Long: `Sync local ~/.claude/ configs to your GitHub repo.

Platform detection:
  By default, warns if files contain platform-specific content without variants.
  Use --no-platform-check to skip this detection.`,
	RunE: runPush,
}

func init() {
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "Show what would be synced without doing it")
	pushCmd.Flags().BoolVar(&pushNoPlatformCheck, "no-platform-check", false, "Skip platform-specific content detection")
}

func runPush(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	// Check prerequisites
	if !sync.FileExists(paths.KeyFile) {
		return fmt.Errorf("not initialized. Run 'claude-code-sync init' first")
	}
	if !sync.FileExists(paths.ClaudeDir) {
		return fmt.Errorf("no ~/.claude directory found. Nothing to sync")
	}

	// Load config
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get public key
	pubKey, err := crypto.GetPublicKey(paths.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}

	if pushDryRun {
		logInfo("[DRY RUN] Would sync the following files:")
	} else {
		logInfo("Syncing files to repo...")
	}

	// Process ~/.claude directory
	files, err := sync.WalkFiles(paths.ClaudeDir)
	if err != nil {
		return fmt.Errorf("failed to walk claude dir: %w", err)
	}

	count := 0
	for _, file := range files {
		relPath := sync.RelPath(paths.ClaudeDir, file)

		// Skip excluded files
		if cfg.ShouldExclude(relPath) {
			continue
		}

		dest := filepath.Join(paths.RepoDir, relPath)

		if cfg.ShouldEncrypt(relPath) {
			if pushDryRun {
				logInfo(fmt.Sprintf("  [encrypt] %s", relPath))
			} else {
				logInfo(fmt.Sprintf("Encrypting: %s", relPath))
				if err := sync.EnsureDir(filepath.Dir(dest + ".age")); err != nil {
					return err
				}
				if err := crypto.EncryptFile(pubKey, file, dest+".age"); err != nil {
					return fmt.Errorf("failed to encrypt %s: %w", relPath, err)
				}
			}
		} else {
			if pushDryRun {
				logInfo(fmt.Sprintf("  [copy] %s", relPath))
			} else {
				logInfo(fmt.Sprintf("Copying: %s", relPath))
				if err := sync.CopyFile(file, dest); err != nil {
					return fmt.Errorf("failed to copy %s: %w", relPath, err)
				}
			}
		}
		count++
	}

	// Also sync ~/.claude.json if it exists
	if sync.FileExists(paths.ClaudeJSON) {
		dest := filepath.Join(paths.RepoDir, "claude.json.age")
		if pushDryRun {
			logInfo("  [encrypt] ~/.claude.json")
		} else {
			logInfo("Encrypting: claude.json")
			if err := crypto.EncryptFile(pubKey, paths.ClaudeJSON, dest); err != nil {
				return fmt.Errorf("failed to encrypt claude.json: %w", err)
			}
		}
		count++
	}

	if pushDryRun {
		logInfo(fmt.Sprintf("[DRY RUN] Would sync %d files", count))
		return nil
	}

	// Normalize paths in plugin config files for cross-platform compatibility
	if err := normalizePluginPaths(paths.RepoDir, paths.ClaudeDir); err != nil {
		logWarn(fmt.Sprintf("Failed to normalize plugin paths: %v", err))
	}

	// Check for platform-specific content without variants
	if !pushNoPlatformCheck {
		repoFiles, err := sync.WalkFiles(paths.RepoDir)
		if err == nil {
			warnings := sync.CheckPlatformVariants(paths.RepoDir, repoFiles)
			if len(warnings) > 0 {
				logWarn("Platform-specific content detected without variants:")
				for _, w := range warnings {
					logWarn(fmt.Sprintf("  %s contains %s syntax (%s)", w.File, w.Platform, w.Pattern))
					otherPlatform := "windows"
					if w.Platform == "windows" {
						otherPlatform = "unix"
					}
					variantName := sync.GetPlatformVariantName(w.File, otherPlatform)
					logInfo(fmt.Sprintf("    Consider creating: %s", variantName))
				}
				logInfo("Use --no-platform-check to skip this warning")
			}
		}
	}

	// Generate manifest
	logInfo("Generating manifest...")
	entries, err := sync.GenerateManifest(paths.RepoDir)
	if err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}
	manifestPath := filepath.Join(paths.RepoDir, ".sync-manifest")
	if err := sync.WriteManifest(manifestPath, entries); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Git commit and push
	g := gitpkg.New(paths.RepoDir)

	logInfo("Committing changes...")
	if err := g.AddAll(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	hasChanges, err := g.HasChanges()
	if err != nil {
		return err
	}

	if !hasChanges {
		logInfo("No changes to commit.")
	} else {
		if err := g.Commit(fmt.Sprintf("Sync %s", sync.Timestamp())); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}

		if g.HasRemote() {
			logInfo("Pushing to remote...")
			if err := g.Push(); err != nil {
				return fmt.Errorf("git push failed: %w", err)
			}
			logSuccess(fmt.Sprintf("Pushed %d files to remote.", count))
		} else {
			logWarn("No remote configured. Changes committed locally only.")
			logInfo(fmt.Sprintf("Add a remote with: git -C %s remote add origin <url>", paths.RepoDir))
		}
	}

	logSuccess("Push complete!")
	return nil
}

// normalizePluginPaths converts platform-specific paths to cross-platform placeholders
// in plugin configuration files for seamless syncing across Windows/macOS/Linux.
func normalizePluginPaths(repoDir, claudeDir string) error {
	// Find all JSON files in plugins directory that may contain paths
	pluginsDir := filepath.Join(repoDir, "plugins")
	if !sync.FileExists(pluginsDir) {
		return nil
	}

	files, err := sync.WalkFiles(pluginsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !strings.HasSuffix(file, ".json") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Only process if file contains the claude dir path
		if !strings.Contains(string(data), claudeDir) &&
			!strings.Contains(string(data), filepath.ToSlash(claudeDir)) &&
			!strings.Contains(string(data), strings.ReplaceAll(claudeDir, `\`, `\\`)) {
			continue
		}

		normalized := sync.NormalizePathsInJSON(data, claudeDir)
		if err := os.WriteFile(file, normalized, 0644); err != nil {
			return fmt.Errorf("failed to write normalized %s: %w", file, err)
		}

		relPath := sync.RelPath(repoDir, file)
		logInfo(fmt.Sprintf("Normalized paths: %s", relPath))
	}

	return nil
}
