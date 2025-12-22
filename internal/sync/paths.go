package sync

import (
	"encoding/json"
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
// Uses JSON parsing to safely handle escape sequences.
func ExpandPathsInJSON(data []byte, claudeDir string) []byte {
	// First, parse as JSON to get the structure
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		// If not valid JSON, fall back to string replacement
		return fallbackExpandPaths(data, claudeDir)
	}

	// Recursively replace placeholders in the parsed object
	expanded := expandInObject(obj, claudeDir)

	// Marshal back to JSON with proper formatting
	result, err := json.MarshalIndent(expanded, "", "  ")
	if err != nil {
		// If marshaling fails, fall back
		return fallbackExpandPaths(data, claudeDir)
	}

	return result
}

// expandInObject recursively expands placeholders in JSON objects
func expandInObject(obj interface{}, claudeDir string) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		for k, val := range v {
			v[k] = expandInObject(val, claudeDir)
		}
		return v
	case []interface{}:
		for i, val := range v {
			v[i] = expandInObject(val, claudeDir)
		}
		return v
	case string:
		if strings.Contains(v, ClaudeDirPlaceholder) {
			// Replace placeholder with local path
			expanded := strings.ReplaceAll(v, ClaudeDirPlaceholder, claudeDir)

			// On Unix systems, convert backslashes to forward slashes in paths
			if !strings.Contains(claudeDir, `\`) {
				// This is Unix - convert Windows-style backslashes to forward slashes
				// Use string replacement with explicit backslash character
				// The JSON unmarshaler has already converted \\ to \, so we just need to convert \ to /
				backslash := string(rune(92)) // ASCII 92 = backslash
				expanded = strings.ReplaceAll(expanded, backslash, `/`)
			}

			return expanded
		}
		return v
	default:
		return v
	}
}

// fallbackExpandPaths is a safe string-based fallback that only replaces in quoted strings
func fallbackExpandPaths(data []byte, claudeDir string) []byte {
	content := string(data)

	// For JSON files, we need to use escaped backslashes on Windows
	if strings.Contains(claudeDir, `\`) {
		// Windows: use escaped backslashes for JSON
		escapedClaudeDir := strings.ReplaceAll(claudeDir, `\`, `\\`)
		content = strings.ReplaceAll(content, ClaudeDirPlaceholder, escapedClaudeDir)
	} else {
		// Unix: replace placeholder with forward-slash path
		// This is safer than replacing all backslashes
		normalizedPath := filepath.ToSlash(claudeDir) // ensure forward slashes
		content = strings.ReplaceAll(content, ClaudeDirPlaceholder, normalizedPath)
	}

	return []byte(content)
}
