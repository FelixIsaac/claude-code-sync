package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git wraps git CLI commands
type Git struct {
	repoDir string
}

// New creates a Git wrapper for the given repo directory
func New(repoDir string) *Git {
	return &Git{repoDir: repoDir}
}

// run executes a git command and returns stdout
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.repoDir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// runSilent executes a git command, ignoring stderr
func (g *Git) runSilent(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.repoDir}, args...)...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

// Init initializes a new git repository
func (g *Git) Init() error {
	if err := os.MkdirAll(g.repoDir, 0755); err != nil {
		return err
	}
	_, err := g.run("init")
	return err
}

// Clone clones a remote repository
func Clone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AddAll stages all changes
func (g *Git) AddAll() error {
	_, err := g.run("add", "-A")
	return err
}

// Commit creates a commit with the given message
func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

// HasChanges checks if there are staged changes to commit
func (g *Git) HasChanges() (bool, error) {
	_, err := g.runSilent("diff", "--cached", "--quiet")
	if err != nil {
		// Non-zero exit means there are changes
		return true, nil
	}
	return false, nil
}

// Push pushes to remote
func (g *Git) Push() error {
	_, err := g.run("push", "origin", "HEAD")
	return err
}

// Pull pulls from remote
func (g *Git) Pull() error {
	_, err := g.run("pull", "origin", "HEAD")
	if err != nil && strings.Contains(err.Error(), "unrelated histories") {
		// Retry with --allow-unrelated-histories
		_, err = g.run("pull", "origin", "HEAD", "--allow-unrelated-histories")
	}
	return err
}

// Fetch fetches from remote
func (g *Git) Fetch() error {
	_, _ = g.runSilent("fetch", "origin")
	return nil // Ignore errors, fetch is best-effort
}

// HasRemote checks if origin remote exists
func (g *Git) HasRemote() bool {
	out, _ := g.runSilent("remote")
	return strings.Contains(out, "origin")
}

// AddRemote adds a remote
func (g *Git) AddRemote(name, url string) error {
	_, err := g.run("remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote
func (g *Git) RemoveRemote(name string) error {
	_, err := g.run("remote", "remove", name)
	return err
}

// GetLocalCommit returns the current HEAD commit hash
func (g *Git) GetLocalCommit() (string, error) {
	return g.runSilent("rev-parse", "HEAD")
}

// GetRemoteCommit returns the origin/HEAD commit hash
func (g *Git) GetRemoteCommit() (string, error) {
	return g.runSilent("rev-parse", "origin/HEAD")
}

// IsRepo checks if the directory is a git repository
func (g *Git) IsRepo() bool {
	_, err := os.Stat(filepath.Join(g.repoDir, ".git"))
	return err == nil
}

// IsInstalled checks if git is available
func IsInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsValidRepoURL checks if a string looks like a valid git repo URL
func IsValidRepoURL(url string) bool {
	// HTTPS URLs
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return strings.Contains(url, "/")
	}
	// SSH URLs (git@host:user/repo)
	if strings.HasPrefix(url, "git@") {
		return strings.Contains(url, ":")
	}
	// SSH URLs (ssh://git@host/user/repo)
	if strings.HasPrefix(url, "ssh://") {
		return strings.Contains(url, "/")
	}
	return false
}

// CheckRemote verifies a remote URL is accessible
func CheckRemote(url string) error {
	cmd := exec.Command("git", "ls-remote", "--exit-code", url)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("repository not found or not accessible")
	}
	return nil
}

// CreateInitialCommit creates a README and initial commit
func (g *Git) CreateInitialCommit() error {
	readme := filepath.Join(g.repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Claude Code Sync\n"), 0644); err != nil {
		return err
	}

	if _, err := g.run("add", "README.md"); err != nil {
		return err
	}

	return g.Commit("Initial commit")
}
