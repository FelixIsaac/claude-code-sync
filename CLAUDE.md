# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`claude-code-sync` is a CLI tool for syncing Claude Code configs (`~/.claude/`) across machines via GitHub with age encryption. Core functionality: selective encryption (sensitive files like `settings.json` encrypted, non-sensitive like `CLAUDE.md` plain text), cross-platform single binary (Go), native age encryption (no external CLI).

## Build Commands

```bash
# Build for current platform
go build -o claude-code-sync.exe ./cmd/claude-code-sync/  # Windows
go build -o claude-code-sync ./cmd/claude-code-sync/      # Unix

# Build with version (for development)
go build -ldflags="-X main.version=0.2.1-dev" -o claude-code-sync.exe ./cmd/claude-code-sync/

# Install to system PATH
cp claude-code-sync.exe C:\Users\<user>\.local\bin\  # Windows
sudo mv claude-code-sync /usr/local/bin/             # Unix

# Run tests
go test ./...
go test -v ./internal/config/  # Specific package

# Format and lint
go fmt ./...
go vet ./...
```

## Architecture

### Entry Point & Version Injection

- `cmd/claude-code-sync/main.go`: Entry point with `var version = "dev"` (overridden by goreleaser's `-ldflags "-X main.version={{.Version}}"`)
- Calls `cmd.SetVersion(version)` then `cmd.Execute()`

### Core Layers

1. **CLI Layer** (`internal/cmd/`): Cobra commands (root.go registers all subcommands)
2. **Business Logic**: Config patterns (`internal/config/`), crypto (`internal/crypto/`), git wrapper (`internal/git/`), sync engine (`internal/sync/`)
3. **External**: Shells out to `git` CLI, native age lib for encryption

### Key Architectural Decisions

**Git wrapper vs go-git**: Shells out to `git` CLI for simplicity (users already have it, easier debugging). All git ops in `internal/git/git.go` with `exec.Command("git", "-C", repoDir, args...)`.

**Native age encryption**: Uses `filippo.io/age` Go library directly (no external `age` CLI). Streaming I/O via `age.Encrypt()`/`age.Decrypt()` - no full files in memory.

**Pattern matching**: Two systems:
- **Encrypt patterns** (`internal/config/config.go:ShouldEncrypt`): Filename match (e.g., `settings.json`) or path wildcard (e.g., `skills/*/resources/*`)
- **Exclude patterns** (`internal/config/config.go:ShouldExclude`): Directory prefix (e.g., `plans/` matches `plans/foo/bar.md`) or filename wildcard (e.g., `*.log`)

**Data flow (Push)**:
1. Walk `~/.claude/` (`internal/sync/sync.go`)
2. For each file: check exclude → check encrypt → copy plain or encrypt to `~/.claude-sync/repo/`
3. Generate `.sync-manifest` (SHA256 checksums)
4. `git add -A && git commit && git push` (`internal/git/git.go`)

**Data flow (Pull)**:
1. Backup current `~/.claude/` to `~/.claude-sync/backups/TIMESTAMP/`
2. `git pull` (retries with `--allow-unrelated-histories` if needed)
3. Walk repo, decrypt `.age` files or copy plain to `~/.claude/`
4. Verify SHA256 checksums

### File Locations

```
~/.claude-sync/
├── config          # Repo URL (plain text)
├── identity.key    # age private key (chmod 600)
├── backups/        # Auto backups before pull
└── repo/           # Git clone
    ├── CLAUDE.md                    # Plain
    ├── commands/, agents/, skills/  # Plain (except skills/*/resources/* encrypted)
    ├── settings.json.age            # Encrypted
    └── .sync-manifest               # SHA256 checksums
```

### Patterns (internal/config/config.go)

**DefaultEncryptPatterns**:
```go
"settings.json", "settings.local.json", "claude.json",
".credentials.json", "client_secret_*.json",
"skills/*/resources/*"
```

**DefaultExcludePatterns** (note specific exclusions):
```go
"plans", "projects", "local", "statsig", "todos", "debug",
"file-history", "ide", "shell-snapshots", "telemetry", "sessionStorage",
"plugins/cache", "plugins/marketplaces",  // But NOT plugins/ itself
"history.jsonl", "stats-cache.json",
"*.log", "*.tmp", "*.cache", "*.local-backup-*", ".git"
```

## Critical Implementation Details

### Error Handling

Return errors up the stack (don't panic), wrap with context: `fmt.Errorf("context: %w", err)`. User-facing errors via `logError()` in `internal/cmd/root.go`.

### Cross-Platform Paths

`toUnixPath()` in `internal/cmd/init.go` converts backslashes to forward slashes for Git Bash compatibility. Always use `filepath.ToSlash()` when displaying paths.

### Git Operations

All git commands in `internal/git/git.go`:
- `run()`: Executes git, returns stderr on error
- `Pull()`: Auto-retries with `--allow-unrelated-histories` if unrelated histories error
- `IsValidRepoURL()`, `CheckRemote()`: Validate before cloning

### Encryption

- Key generation: `age.GenerateX25519Identity()` returns identity with both private key and public recipient
- Public key derivation: `identity.Recipient().String()`
- Key file format: Comment lines + `AGE-SECRET-KEY-...`
- Encrypt: Stream `io.Copy(age.Encrypt(out, recipient), in)`
- Decrypt: Stream `io.Copy(out, age.Decrypt(in, identity))`

## Adding New Commands

1. Create `internal/cmd/newcommand.go`:
```go
var newCmd = &cobra.Command{
    Use:   "new",
    Short: "Description",
    RunE:  runNew,
}

func runNew(cmd *cobra.Command, args []string) error {
    // Implementation
}
```

2. Register in `internal/cmd/root.go` init():
```go
rootCmd.AddCommand(newCmd)
```

## Release Process

Triggered by git tag push:
```bash
git tag -a v0.3.0 -m "Release v0.3.0"
git push origin v0.3.0
```

GitHub Actions (`.github/workflows/release.yaml`) runs goreleaser:
- Builds for Windows/macOS/Linux (amd64/arm64)
- Injects version via `-ldflags "-X main.version={{.Version}}"`
- Creates GitHub release
- Updates Homebrew tap and Scoop bucket

## Known Limitations

- No tests yet (see ARCHITECTURE.md technical debt)
- No custom encrypt/exclude patterns (hardcoded in `config.go`)
- No lock file (concurrent operations possible but risky)
- Shells out to git (requires git in PATH)

## Next Release: v0.3.0

New features to test before release:
- `pull --ours/--theirs/--diff` - conflict handling strategies
- `push --no-platform-check` - platform detection warnings
- Platform variant support (`.windows.md`/`.unix.md` suffixes)
