package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/felixisaac/claude-code-sync/internal/config"
	"github.com/felixisaac/claude-code-sync/internal/crypto"
	"github.com/felixisaac/claude-code-sync/internal/sync"
	"github.com/spf13/cobra"
)

// keyCmd is the parent command for key management
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Encryption key management",
	Long: `Encryption key management commands.

Subcommands:
  import    Import private key from another machine
  export    Display private key for backup`,
}

// keyImportCmd imports a private key
var keyImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import private key from another machine",
	Long: `Import your age private key to set up sync on a new machine.

The key starts with AGE-SECRET-KEY- and was shown during init.
You can also include the comment line with the public key.`,
	RunE: runKeyImport,
}

// keyExportCmd exports the private key
var keyExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Display private key for backup",
	Long:  `Display your private key so you can save it securely or import on another machine.`,
	RunE:  runKeyExport,
}

func init() {
	keyCmd.AddCommand(keyImportCmd)
	keyCmd.AddCommand(keyExportCmd)
}

func runKeyImport(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if err := sync.EnsureDir(paths.SyncDir); err != nil {
		return err
	}

	if sync.FileExists(paths.KeyFile) {
		logWarn(fmt.Sprintf("Key already exists at %s", paths.KeyFile))
		fmt.Print("Overwrite? (y/N) ")

		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			return fmt.Errorf("aborted")
		}
	}

	fmt.Println("Paste your age private key (starts with AGE-SECRET-KEY-):")
	fmt.Println("Press Ctrl+D (Unix) or Ctrl+Z then Enter (Windows) when done.")
	fmt.Println()

	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	keyContent := strings.Join(lines, "\n")

	// Validate key format
	if err := crypto.ValidateKeyContent(keyContent); err != nil {
		return fmt.Errorf("invalid key format: %w", err)
	}

	// Write key file
	if err := os.WriteFile(paths.KeyFile, []byte(keyContent+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	logSuccess("Key imported successfully!")

	pubKey, err := crypto.GetPublicKeyFromContent(keyContent)
	if err == nil {
		logInfo(fmt.Sprintf("Public key: %s", pubKey))
	}

	return nil
}

func runKeyExport(cmd *cobra.Command, args []string) error {
	paths := config.GetPaths()

	if !sync.FileExists(paths.KeyFile) {
		return fmt.Errorf("no key found. Run 'claude-code-sync config init' first")
	}

	content, err := os.ReadFile(paths.KeyFile)
	if err != nil {
		return err
	}

	fmt.Println()
	color.Yellow("=== Your Private Key ===")
	fmt.Println()
	fmt.Print(string(content))
	fmt.Println()
	color.Yellow("Keep this secure!")

	return nil
}
