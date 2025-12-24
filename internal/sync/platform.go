package sync

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Platform constants
const (
	PlatformWindows = "windows"
	PlatformUnix    = "unix"
)

// GetPlatform returns the current platform identifier
func GetPlatform() string {
	if runtime.GOOS == "windows" {
		return PlatformWindows
	}
	return PlatformUnix
}

// IsPlatformVariant checks if a filename is a platform-specific variant
// e.g., "deploy.windows.md" or "deploy.unix.md"
func IsPlatformVariant(filename string) bool {
	base := filepath.Base(filename)
	return strings.Contains(base, ".windows.") || strings.Contains(base, ".unix.")
}

// GetPlatformSuffix extracts the platform suffix from a filename
// Returns "" if not a platform variant
func GetPlatformSuffix(filename string) string {
	base := filepath.Base(filename)
	if strings.Contains(base, ".windows.") {
		return PlatformWindows
	}
	if strings.Contains(base, ".unix.") {
		return PlatformUnix
	}
	return ""
}

// GetBaseName returns the base name without platform suffix
// e.g., "deploy.windows.md" -> "deploy.md"
func GetBaseName(filename string) string {
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)

	// Remove platform suffix
	base = strings.Replace(base, ".windows.", ".", 1)
	base = strings.Replace(base, ".unix.", ".", 1)

	if dir == "." {
		return base
	}
	return filepath.Join(dir, base)
}

// GetPlatformVariantName returns the platform-specific variant filename
// e.g., "deploy.md", "windows" -> "deploy.windows.md"
func GetPlatformVariantName(filename, platform string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	return base + "." + platform + ext
}

// ShouldSkipForPlatform determines if a file should be skipped on current platform
// Returns true if the file is a variant for a different platform
func ShouldSkipForPlatform(filename string) bool {
	suffix := GetPlatformSuffix(filename)
	if suffix == "" {
		return false // Not a platform variant, don't skip
	}
	return suffix != GetPlatform()
}

// FindBestVariant finds the best file variant for the current platform
// Priority: platform-specific > base file
// Returns the relative path to use and whether a variant was found
func FindBestVariant(files []string, targetPath string) (string, bool) {
	currentPlatform := GetPlatform()
	platformVariant := GetPlatformVariantName(targetPath, currentPlatform)

	// Check if platform-specific variant exists
	for _, f := range files {
		if f == platformVariant {
			return platformVariant, true
		}
	}

	// Check if base file exists
	for _, f := range files {
		if f == targetPath {
			return targetPath, true
		}
	}

	return "", false
}

// PlatformWarning represents a detected platform-specific pattern
type PlatformWarning struct {
	File     string
	Platform string
	Pattern  string
}

// Unix-specific patterns
var unixPatterns = []*regexp.Regexp{
	regexp.MustCompile(`#!/bin/(?:ba)?sh`),
	regexp.MustCompile(`#!/usr/bin/env\s+(?:ba)?sh`),
	regexp.MustCompile(`\bgrep\s+`),
	regexp.MustCompile(`\bsed\s+`),
	regexp.MustCompile(`\bawk\s+`),
	regexp.MustCompile(`\bchmod\s+`),
	regexp.MustCompile(`\bchown\s+`),
	regexp.MustCompile(`\$HOME\b`),
	regexp.MustCompile(`\$USER\b`),
	regexp.MustCompile(`/usr/(?:local/)?bin/`),
}

// Windows-specific patterns
var windowsPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bpowershell\b`),
	regexp.MustCompile(`(?i)\bpwsh\b`),
	regexp.MustCompile(`(?i)\bcmd\s*/c\b`),
	regexp.MustCompile(`(?i)\bGet-\w+`),
	regexp.MustCompile(`(?i)\bSet-\w+`),
	regexp.MustCompile(`(?i)\bNew-\w+`),
	regexp.MustCompile(`(?i)\bRemove-\w+`),
	regexp.MustCompile(`%USERPROFILE%`),
	regexp.MustCompile(`%APPDATA%`),
	regexp.MustCompile(`%LOCALAPPDATA%`),
	regexp.MustCompile(`(?i)\.exe\b`),
}

// DetectPlatformContent scans a file for platform-specific patterns
// Returns the detected platform ("unix", "windows", or "") and the first matching pattern
func DetectPlatformContent(filePath string) (string, string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", ""
	}

	content := string(data)

	// Check Unix patterns
	for _, p := range unixPatterns {
		if match := p.FindString(content); match != "" {
			return PlatformUnix, match
		}
	}

	// Check Windows patterns
	for _, p := range windowsPatterns {
		if match := p.FindString(content); match != "" {
			return PlatformWindows, match
		}
	}

	return "", ""
}

// CheckPlatformVariants checks if platform variants exist for files with platform-specific content
// Returns warnings for files that have platform-specific content but no variant for the other platform
func CheckPlatformVariants(repoDir string, files []string) []PlatformWarning {
	var warnings []PlatformWarning

	// Build a set of all files for quick lookup
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	for _, file := range files {
		relPath := RelPath(repoDir, file)

		// Skip non-markdown and non-script files
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".md" && ext != ".sh" && ext != ".ps1" && ext != ".bat" && ext != ".cmd" {
			continue
		}

		// Skip if already a platform variant
		if IsPlatformVariant(relPath) {
			continue
		}

		// Detect platform-specific content
		platform, pattern := DetectPlatformContent(file)
		if platform == "" {
			continue
		}

		// Determine which variant should exist
		var missingPlatform string
		if platform == PlatformUnix {
			missingPlatform = PlatformWindows
		} else {
			missingPlatform = PlatformUnix
		}

		// Check if variant exists
		variantPath := GetPlatformVariantName(relPath, missingPlatform)
		variantFullPath := filepath.Join(repoDir, variantPath)

		if !fileSet[variantFullPath] {
			warnings = append(warnings, PlatformWarning{
				File:     relPath,
				Platform: platform,
				Pattern:  pattern,
			})
		}
	}

	return warnings
}
