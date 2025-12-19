package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var (
	resetKeepKey bool
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete all sync data",
	Long:  `Delete all claude-code-sync data. Use --keep-key to preserve your private key.`,
	RunE:  runReset,
}

func init() {
	resetCmd.Flags().BoolVarP(&resetKeepKey, "keep-key", "k", false, "Preserve your private key")
}

func runReset(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.SyncDir) {
		logInfo("Nothing to reset - ~/.claude-sync does not exist.")
		return nil
	}

	fmt.Println()
	color.Yellow("This will delete:")
	if resetKeepKey {
		fmt.Printf("  - %s (local repo)\n", paths.RepoDir)
		fmt.Printf("  - %s (config)\n", paths.ConfigFile)
		fmt.Printf("  - %s (backups)\n", paths.BackupDir)
		fmt.Println()
		color.Green("Your key will be PRESERVED at %s", paths.KeyFile)
	} else {
		color.Red("  - %s (everything including your private key!)", paths.SyncDir)
		fmt.Println()
		color.Red("WARNING: If you haven't backed up your key, you will lose access")
		color.Red("to any encrypted configs in your repo!")
	}
	fmt.Println()

	fmt.Print("Type 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)

	if confirm != "yes" {
		logInfo("Aborted.")
		return nil
	}

	if resetKeepKey {
		// Remove everything except key
		if sync.FileExists(paths.RepoDir) {
			os.RemoveAll(paths.RepoDir)
		}
		if sync.FileExists(paths.ConfigFile) {
			os.Remove(paths.ConfigFile)
		}
		if sync.FileExists(paths.BackupDir) {
			os.RemoveAll(paths.BackupDir)
		}
		if sync.FileExists(paths.LockFile) {
			os.Remove(paths.LockFile)
		}
		logSuccess("Reset complete. Key preserved. Run 'claude-code-sync init <repo-url>' to reconnect.")
	} else {
		os.RemoveAll(paths.SyncDir)
		logSuccess("Reset complete. All sync data removed.")
	}

	return nil
}
