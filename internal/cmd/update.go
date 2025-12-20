package cmd

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	repoOwner = "felixisaac"
	repoName  = "claude-code-sync"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var checkUpdateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check for newer version",
	RunE:  runCheckUpdate,
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and install latest version",
	RunE:  runUpdate,
}

var updateAutoConfirm bool

func init() {
	updateCmd.Flags().BoolVarP(&updateAutoConfirm, "yes", "y", false, "Auto-confirm update without prompting")
}

func runCheckUpdate(cmd *cobra.Command, args []string) error {
	logInfo("Checking for updates...")

	latest, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVer := strings.TrimPrefix(latest.TagName, "v")
	currentVer := version

	if latestVer == currentVer {
		logSuccess(fmt.Sprintf("You're on the latest version (v%s)", currentVer))
		return nil
	}

	// Simple version comparison (works for semver)
	if compareVersions(latestVer, currentVer) > 0 {
		fmt.Println()
		color.Yellow("Update available: v%s → v%s", currentVer, latestVer)
		fmt.Println()
		fmt.Println("Download:")
		fmt.Printf("  %s\n", latest.HTMLURL)
		fmt.Println()

		// Show direct download link for current platform
		assetName := getAssetName()
		for _, asset := range latest.Assets {
			if asset.Name == assetName {
				fmt.Println("Direct download for your platform:")
				fmt.Printf("  %s\n", asset.BrowserDownloadURL)
				break
			}
		}
		fmt.Println()
		logInfo("To update: download and replace your current binary")
	} else {
		logSuccess(fmt.Sprintf("You're on the latest version (v%s)", currentVer))
	}

	return nil
}

func getLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func getAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	ext := ".tar.gz"
	if os == "windows" {
		ext = ".zip"
	}

	return fmt.Sprintf("claude-code-sync_%s_%s%s", os, arch, ext)
}

// compareVersions returns >0 if a > b, <0 if a < b, 0 if equal
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		var aNum, bNum int
		fmt.Sscanf(aParts[i], "%d", &aNum)
		fmt.Sscanf(bParts[i], "%d", &bNum)

		if aNum > bNum {
			return 1
		}
		if aNum < bNum {
			return -1
		}
	}

	return len(aParts) - len(bParts)
}

// runUpdate handles the automatic update flow
func runUpdate(cmd *cobra.Command, args []string) error {
	logInfo("Checking for updates...")

	// Check for latest release
	latest, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVer := strings.TrimPrefix(latest.TagName, "v")
	currentVer := version

	// Check if update is needed
	if compareVersions(latestVer, currentVer) <= 0 {
		logSuccess(fmt.Sprintf("Already on latest version (v%s)", currentVer))
		return nil
	}

	// Prompt user unless --yes flag
	if !updateAutoConfirm {
		fmt.Printf("Update available: v%s → v%s\n", currentVer, latestVer)
		fmt.Printf("Update to v%s? [Y/n]: ", latestVer)

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "" && response != "y" && response != "yes" {
			logInfo("Update cancelled")
			return nil
		}
	}

	// Get asset info
	assetName := getAssetName()
	var downloadURL string
	for _, asset := range latest.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	logInfo(fmt.Sprintf("Downloading %s...", assetName))
	tmpFile, err := downloadToTemp(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpFile)

	// Extract binary
	logInfo("Extracting binary...")
	extractedBinary, err := extractBinary(tmpFile)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(extractedBinary))

	// Get current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current binary: %w", err)
	}

	// Check permissions
	logInfo("Installing update...")
	if err := checkWritePermission(currentBinary); err != nil {
		return fmt.Errorf("insufficient permissions: %w", err)
	}

	// Create backup
	backup := currentBinary + ".old"
	if err := os.Rename(currentBinary, backup); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(extractedBinary, currentBinary); err != nil {
		// Restore backup on failure
		os.Rename(backup, currentBinary)
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Ensure executable permissions
	os.Chmod(currentBinary, 0755)

	// Clean up backup
	os.Remove(backup)

	logSuccess(fmt.Sprintf("Updated to v%s!", latestVer))
	return nil
}

// downloadToTemp downloads a file from URL to a temp file
func downloadToTemp(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "claude-code-sync-*.tmp")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// extractBinary extracts the binary from the archive
func extractBinary(archivePath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "update-")
	if err != nil {
		return "", err
	}

	var binaryPath string

	if strings.HasSuffix(archivePath, ".zip") {
		binaryPath, err = extractZip(archivePath, tmpDir)
	} else {
		binaryPath, err = extractTarGz(archivePath, tmpDir)
	}

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return binaryPath, nil
}

// extractZip extracts binary from zip archive
func extractZip(zipPath, destDir string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.Contains(file.Name, "claude-code-sync") && !strings.Contains(file.Name, "/") {
			// Found the binary at root level
			src, err := file.Open()
			if err != nil {
				return "", err
			}
			defer src.Close()

			destPath := filepath.Join(destDir, file.Name)
			dest, err := os.Create(destPath)
			if err != nil {
				return "", err
			}
			defer dest.Close()

			if _, err := io.Copy(dest, src); err != nil {
				return "", err
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// extractTarGz extracts binary from tar.gz archive
func extractTarGz(tarPath, destDir string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if strings.Contains(header.Name, "claude-code-sync") && !strings.Contains(header.Name, "/") {
			// Found the binary
			destPath := filepath.Join(destDir, header.Name)
			dest, err := os.Create(destPath)
			if err != nil {
				return "", err
			}
			defer dest.Close()

			if _, err := io.Copy(dest, tr); err != nil {
				return "", err
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// checkWritePermission checks if we can write to the binary location
func checkWritePermission(binaryPath string) error {
	parent := filepath.Dir(binaryPath)
	tmpFile := filepath.Join(parent, ".write-test")

	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("try: sudo %s update", os.Args[0])
		}
		return err
	}

	os.Remove(tmpFile)
	return nil
}
