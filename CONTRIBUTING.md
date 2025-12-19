# Contributing to claude-code-sync

Thank you for considering contributing to `claude-code-sync`! This document outlines the contribution process and guidelines.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
- [Development Setup](#development-setup)
- [Submitting Changes](#submitting-changes)
- [Coding Guidelines](#coding-guidelines)
- [Testing](#testing)
- [Documentation](#documentation)
- [Release Process](#release-process)

---

## Code of Conduct

This project adheres to a code of conduct based on the [Contributor Covenant](https://www.contributor-covenant.org/). By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

**In short:**
- Be respectful and inclusive
- Provide constructive feedback
- Focus on what is best for the community
- Show empathy towards other community members

---

## How Can I Contribute?

### Reporting Bugs

Before creating a bug report, please check the [existing issues](https://github.com/felixisaac/claude-code-sync/issues) to avoid duplicates.

**When filing a bug report, include:**

- **Clear title** describing the issue
- **Steps to reproduce** the behavior
- **Expected behavior** vs **actual behavior**
- **Environment details:**
  - OS (Windows/macOS/Linux)
  - Version of `claude-code-sync` (`claude-code-sync version`)
  - Git version (`git --version`)
- **Logs** (run with `--verbose` if available)
- **Screenshots** if applicable

[Create a bug report](https://github.com/felixisaac/claude-code-sync/issues/new?labels=bug)

### Suggesting Enhancements

Enhancement suggestions are welcome! Before submitting:

1. Check if the feature already exists or is planned
2. Search [existing issues](https://github.com/felixisaac/claude-code-sync/issues) for similar requests

**When suggesting an enhancement, include:**

- **Clear title** summarizing the feature
- **Use case** - Why is this feature needed?
- **Proposed solution** - How should it work?
- **Alternatives considered** - Other ways to achieve the goal
- **Additional context** - Examples, mockups, etc.

[Request a feature](https://github.com/felixisaac/claude-code-sync/issues/new?labels=enhancement)

### Improving Documentation

Documentation improvements are highly appreciated! You can:

- Fix typos or unclear wording
- Add examples
- Improve clarity
- Translate documentation (future)
- Add missing information

[Improve documentation](https://github.com/felixisaac/claude-code-sync/issues/new?labels=documentation)

### Contributing Code

See [Development Setup](#development-setup) and [Submitting Changes](#submitting-changes) below.

---

## Development Setup

### Prerequisites

- **Go 1.21+** ([download](https://golang.org/dl/))
- **Git 2.0+**
- **age** (for testing encryption) - `brew install age` or download from [releases](https://github.com/FiloSottile/age/releases)

### Clone the Repository

```bash
git clone https://github.com/felixisaac/claude-code-sync.git
cd claude-code-sync
```

### Install Dependencies

```bash
go mod download
```

### Build

```bash
# Build for your current platform
go build -o claude-code-sync ./cmd/claude-code-sync/

# Or use the Makefile (if available)
make build
```

### Run

```bash
./claude-code-sync --help
```

### Development Build with Version

```bash
go build -ldflags="-X main.version=dev" -o claude-code-sync ./cmd/claude-code-sync/
```

---

## Submitting Changes

### Workflow

1. **Fork** the repository
2. **Create a branch** from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```
3. **Make your changes**
4. **Test** your changes (see [Testing](#testing))
5. **Commit** with clear messages (see [Commit Messages](#commit-messages))
6. **Push** to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
7. **Open a Pull Request** to the `main` branch

### Pull Request Guidelines

**Before submitting:**

- [ ] Code follows the [Coding Guidelines](#coding-guidelines)
- [ ] All tests pass (`go test ./...`)
- [ ] Added tests for new functionality
- [ ] Updated documentation (README, code comments)
- [ ] Ran `go fmt` and `go vet`
- [ ] Checked for compiler warnings
- [ ] Tested on your platform (mention in PR description)

**PR Description should include:**

- **What** does this PR do?
- **Why** is this change needed?
- **How** does it work?
- **Testing** - How did you test this?
- **Screenshots** (if UI changes)
- **Related issues** - Closes #123

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]

[optional footer]
```

**Types:**

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `refactor:` Code refactoring (no behavior change)
- `test:` Adding or updating tests
- `chore:` Build process, tooling, dependencies

**Examples:**

```
feat: add custom encryption patterns in config

Allow users to specify custom patterns for encryption
in ~/.claude-sync/config.yaml

Closes #45
```

```
fix: handle unrelated histories in git pull

Automatically retry with --allow-unrelated-histories
when pulling fails due to divergent branches

Fixes #67
```

---

## Coding Guidelines

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting (or `goimports`)
- Run `go vet` to catch common issues
- Keep functions small and focused
- Prefer clarity over cleverness

### Code Organization

```
claude-code-sync/
├── cmd/claude-code-sync/  # Main entry point
│   └── main.go
├── internal/              # Internal packages (not importable)
│   ├── cmd/               # Cobra commands
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── push.go
│   │   └── pull.go
│   ├── config/            # Configuration logic
│   ├── crypto/            # Encryption/decryption
│   ├── git/               # Git operations
│   └── sync/              # Sync logic
├── .github/               # GitHub Actions
└── README.md
```

### Error Handling

- Return errors, don't panic (except in `main`)
- Wrap errors with context: `fmt.Errorf("failed to encrypt: %w", err)`
- Use descriptive error messages

**Good:**
```go
if err != nil {
    return fmt.Errorf("failed to read file %s: %w", path, err)
}
```

**Bad:**
```go
if err != nil {
    return err  // Lost context
}
```

### Logging

Use the `logInfo`, `logSuccess`, `logWarn`, `logError` helpers:

```go
logInfo("Initializing sync...")
logSuccess("Push complete!")
logWarn("Key already exists")
logError("Failed to connect to repo")
```

### Comments

- Write comments for exported functions/types (godoc format)
- Explain *why*, not *what* (code should be self-explanatory)
- Keep comments up-to-date

**Good:**
```go
// ShouldEncrypt returns true if the file should be encrypted
// based on the configured encryption patterns.
func (c *Config) ShouldEncrypt(path string) bool {
```

**Bad:**
```go
// This function checks if file should be encrypted
func (c *Config) ShouldEncrypt(path string) bool {
```

---

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests verbosely
go test -v ./...

# Run specific test
go test -run TestShouldEncrypt ./internal/config/
```

### Writing Tests

- Place tests in `*_test.go` files next to the code
- Use table-driven tests for multiple cases
- Mock external dependencies (git, filesystem)

**Example:**

```go
func TestShouldEncrypt(t *testing.T) {
    tests := []struct {
        name     string
        path     string
        expected bool
    }{
        {"settings.json", "settings.json", true},
        {"CLAUDE.md", "CLAUDE.md", false},
        {"skill resource", "skills/pdf/resources/key.txt", true},
    }

    cfg := &Config{EncryptPatterns: DefaultEncryptPatterns}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := cfg.ShouldEncrypt(tt.path)
            if got != tt.expected {
                t.Errorf("ShouldEncrypt(%q) = %v, want %v", tt.path, got, tt.expected)
            }
        })
    }
}
```

### Integration Tests

Test with a real git repo (use temp directories):

```go
func TestPushPull(t *testing.T) {
    // Create temp directories
    tmpDir := t.TempDir()
    // ... test push/pull flow
}
```

---

## Documentation

### README

The README should be:

- Comprehensive yet scannable
- Include quickstart examples
- Explain all concepts clearly
- Link to relevant external docs

When making changes:

- Update relevant sections
- Keep table of contents in sync
- Add examples for new features

### Code Comments

- Document exported functions, types, constants
- Use godoc format
- Include examples in comments when helpful

### Inline Comments

- Explain non-obvious code
- Document workarounds or edge cases
- Clarify intent when code might be confusing

---

## Release Process

> **Note:** This section is for maintainers.

### Versioning

We use [Semantic Versioning](https://semver.org/):

- `MAJOR.MINOR.PATCH`
- `MAJOR`: Breaking changes
- `MINOR`: New features (backward-compatible)
- `PATCH`: Bug fixes

### Creating a Release

1. **Update version** in relevant places (if not automated)
2. **Update CHANGELOG** (or generate from commits)
3. **Create git tag:**
   ```bash
   git tag -a v0.3.0 -m "Release v0.3.0"
   git push origin v0.3.0
   ```
4. **GitHub Actions** automatically:
   - Builds binaries for all platforms
   - Creates GitHub release
   - Updates Homebrew tap
   - Updates Scoop bucket

### Manual Release (if Actions fail)

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# Run release (dry-run)
goreleaser release --snapshot --clean

# Run actual release
export GITHUB_TOKEN=your_token
goreleaser release --clean
```

---

## Questions?

If you have questions:

- Check existing [issues](https://github.com/felixisaac/claude-code-sync/issues) and [discussions](https://github.com/felixisaac/claude-code-sync/discussions)
- Open a [new discussion](https://github.com/felixisaac/claude-code-sync/discussions/new)
- Reach out to maintainers

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to `claude-code-sync`! 🎉
