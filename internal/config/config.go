package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Paths returns all the standard paths used by claude-code-sync
type Paths struct {
	ClaudeDir  string // ~/.claude
	ClaudeJSON string // ~/.claude.json
	SyncDir    string // ~/.claude-sync
	ConfigFile string // ~/.claude-sync/config.yaml
	KeyFile    string // ~/.claude-sync/identity.key
	RepoDir    string // ~/.claude-sync/repo
	BackupDir  string // ~/.claude-sync/backups
	LockFile   string // ~/.claude-sync/.lock
}

// GetPaths returns the standard paths for the current user
func GetPaths() Paths {
	home, _ := os.UserHomeDir()
	syncDir := filepath.Join(home, ".claude-sync")

	return Paths{
		ClaudeDir:  filepath.Join(home, ".claude"),
		ClaudeJSON: filepath.Join(home, ".claude.json"),
		SyncDir:    syncDir,
		ConfigFile: filepath.Join(syncDir, "config.yaml"),
		KeyFile:    filepath.Join(syncDir, "identity.key"),
		RepoDir:    filepath.Join(syncDir, "repo"),
		BackupDir:  filepath.Join(syncDir, "backups"),
		LockFile:   filepath.Join(syncDir, ".lock"),
	}
}

// Config represents the user configuration file
type Config struct {
	EncryptPatterns []string `yaml:"encrypt_patterns,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
	Backup          struct {
		MaxCount int `yaml:"max_count,omitempty"`
	} `yaml:"backup,omitempty"`
}

// DefaultEncryptPatterns are files that should be encrypted
var DefaultEncryptPatterns = []string{
	"settings.json",
	"settings.local.json",
	"claude.json",
	".credentials.json",
	"client_secret_*.json",
	"skills/*/resources/*",
}

// DefaultExcludePatterns are files/dirs that should not be synced
var DefaultExcludePatterns = []string{
	// Directories
	"plans",
	"projects",
	"local",
	"statsig",
	"todos",
	"debug",
	"file-history",
	"ide",
	"plugins",
	"shell-snapshots",
	"telemetry",
	"sessionStorage",
	// Files
	"history.jsonl",
	"stats-cache.json",
	"*.log",
	"*.tmp",
	"*.cache",
	"*.local-backup-*",
	".git",
}

// Load reads the config file or returns defaults
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults
			cfg.EncryptPatterns = DefaultEncryptPatterns
			cfg.ExcludePatterns = DefaultExcludePatterns
			cfg.Backup.MaxCount = 5
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Merge with defaults
	if len(cfg.EncryptPatterns) == 0 {
		cfg.EncryptPatterns = DefaultEncryptPatterns
	}
	if len(cfg.ExcludePatterns) == 0 {
		cfg.ExcludePatterns = DefaultExcludePatterns
	}
	if cfg.Backup.MaxCount == 0 {
		cfg.Backup.MaxCount = 5
	}

	return cfg, nil
}

// ShouldEncrypt checks if a file should be encrypted
func (c *Config) ShouldEncrypt(relPath string) bool {
	filename := filepath.Base(relPath)
	relPathNorm := filepath.ToSlash(relPath)

	for _, pattern := range c.EncryptPatterns {
		if strings.Contains(pattern, "*") {
			// Wildcard pattern
			if matchWildcard(filename, pattern) || matchWildcard(relPathNorm, pattern) {
				return true
			}
		} else {
			// Exact match
			if filename == pattern {
				return true
			}
		}
	}
	return false
}

// ShouldExclude checks if a file should be excluded from sync
func (c *Config) ShouldExclude(relPath string) bool {
	filename := filepath.Base(relPath)
	relPathNorm := strings.ToLower(filepath.ToSlash(relPath))

	for _, pattern := range c.ExcludePatterns {
		patternLower := strings.ToLower(pattern)

		if strings.Contains(pattern, "*") {
			// Wildcard pattern - match against filename
			if matchWildcard(strings.ToLower(filename), patternLower) {
				return true
			}
		} else {
			// Directory/file name - match if relPath starts with pattern/ or equals pattern
			if relPathNorm == patternLower || strings.HasPrefix(relPathNorm, patternLower+"/") {
				return true
			}
			// Exact filename match
			if strings.ToLower(filename) == patternLower {
				return true
			}
		}
	}
	return false
}

// matchWildcard performs simple glob matching (* matches any characters)
func matchWildcard(s, pattern string) bool {
	// Simple glob matching - could use filepath.Match but it's stricter
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		// Simple prefix/suffix match
		return strings.HasPrefix(s, parts[0]) && strings.HasSuffix(s, parts[1])
	}

	// Use filepath.Match for complex patterns
	matched, _ := filepath.Match(pattern, s)
	return matched
}
