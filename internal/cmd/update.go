package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
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
