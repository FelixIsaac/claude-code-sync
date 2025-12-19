package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Timestamp returns a formatted timestamp for backups/commits
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileChecksum computes SHA256 of a file
func FileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// WalkFiles walks a directory and returns all file paths
func WalkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// RelPath returns the relative path from base to path
func RelPath(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rel
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ManifestEntry represents a single file in the manifest
type ManifestEntry struct {
	Checksum string
	Path     string
}

// GenerateManifest creates a manifest of all files in a directory
func GenerateManifest(repoDir string) ([]ManifestEntry, error) {
	var entries []ManifestEntry

	files, err := WalkFiles(repoDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		relPath := RelPath(repoDir, file)

		// Skip git and manifest files
		if strings.HasPrefix(relPath, ".git") || relPath == ".sync-manifest" {
			continue
		}

		checksum, err := FileChecksum(file)
		if err != nil {
			return nil, err
		}

		entries = append(entries, ManifestEntry{
			Checksum: checksum,
			Path:     relPath,
		})
	}

	return entries, nil
}

// WriteManifest writes the manifest to a file
func WriteManifest(path string, entries []ManifestEntry) error {
	var lines []string
	lines = append(lines, fmt.Sprintf("# claude-code-sync manifest - %s", time.Now().Format(time.RFC3339)))
	lines = append(lines, "# Format: checksum  path")

	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s  %s", e.Checksum, e.Path))
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// ReadManifest reads the manifest from a file
func ReadManifest(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []ManifestEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}

		entries = append(entries, ManifestEntry{
			Checksum: parts[0],
			Path:     parts[1],
		})
	}

	return entries, nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// BackupFile creates a backup of a file with timestamp
func BackupFile(src string) (string, error) {
	if !FileExists(src) {
		return "", nil
	}

	backupPath := fmt.Sprintf("%s.local-backup-%s", src, Timestamp())
	return backupPath, CopyFile(src, backupPath)
}
