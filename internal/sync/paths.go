package sync

import (
	"path/filepath"
	"strings"
)

// ClaudeDirPlaceholder is used to replace platform-specific paths in synced files
const ClaudeDirPlaceholder = "$CLAUDE_DIR"

// NormalizePathsInJSON replaces absolute ClaudeDir paths with a cross-platform placeholder.
// This allows plugin configs to be synced across Windows/macOS/Linux.
func NormalizePathsInJSON(data []byte, claudeDir string) []byte {
	content := string(data)

	// Handle escaped backslashes in JSON (Windows paths like C:\\Users\\...)
	escapedClaudeDir := strings.ReplaceAll(claudeDir, `\`, `\\`)
	content = strings.ReplaceAll(content, escapedClaudeDir, ClaudeDirPlaceholder)

	// Handle forward slash version (normalized paths)
	forwardSlashDir := filepath.ToSlash(claudeDir)
	content = strings.ReplaceAll(content, forwardSlashDir, ClaudeDirPlaceholder)

	// Handle raw backslash version (shouldn't normally appear in JSON, but just in case)
	content = strings.ReplaceAll(content, claudeDir, ClaudeDirPlaceholder)

	return []byte(content)
}

// ExpandPathsInJSON replaces the cross-platform placeholder with the local ClaudeDir path.
// The expanded path uses the native format for the current platform.
func ExpandPathsInJSON(data []byte, claudeDir string) []byte {
	content := string(data)

	// For JSON files, we need to use escaped backslashes on Windows
	// Check if we're on Windows by looking for backslashes in claudeDir
	if strings.Contains(claudeDir, `\`) {
		// Windows: use escaped backslashes for JSON
		escapedClaudeDir := strings.ReplaceAll(claudeDir, `\`, `\\`)
		content = strings.ReplaceAll(content, ClaudeDirPlaceholder, escapedClaudeDir)
	} else {
		// Unix: replace placeholder and convert any remaining backslashes to forward slashes
		content = strings.ReplaceAll(content, ClaudeDirPlaceholder, claudeDir)
		// Convert escaped backslashes (\\) to forward slashes
		content = strings.ReplaceAll(content, `\\`, `/`)
		// Convert single backslashes to forward slashes (shouldn't happen in JSON but just in case)
		content = strings.ReplaceAll(content, `\`, `/`)
	}

	return []byte(content)
}
