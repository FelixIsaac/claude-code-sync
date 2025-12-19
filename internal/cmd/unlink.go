package cmd

import (
	"fmt"
	"os"

	"github.com/felixisaac/claude-code-sync/internal/config"
	gitpkg "github.com/felixisaac/claude-code-sync/internal/git"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Disconnect from remote repo",
	Long:  `Remove the remote origin connection while keeping local data.`,
	RunE:  runUnlink,
}

func runUnlink(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.RepoDir) {
		logInfo("No repo linked.")
		return nil
	}

	g := gitpkg.New(paths.RepoDir)

	if g.HasRemote() {
		if err := g.RemoveRemote("origin"); err != nil {
			return fmt.Errorf("failed to remove remote: %w", err)
		}
		if sync.FileExists(paths.ConfigFile) {
			os.Remove(paths.ConfigFile)
		}
		logSuccess(fmt.Sprintf("Unlinked from remote. Local repo preserved at %s", paths.RepoDir))
		logInfo(fmt.Sprintf("To link to a new repo: git -C %s remote add origin <new-url>", paths.RepoDir))
	} else {
		logInfo("No remote configured.")
	}

	return nil
}
