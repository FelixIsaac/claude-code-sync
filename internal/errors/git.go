// Package errors provides user-friendly error translation and types.
package errors

import (
	"regexp"
	"strings"
)

// gitErrorPattern maps a regex pattern to user-friendly error info
type gitErrorPattern struct {
	pattern *regexp.Regexp
	code    ErrorCode
	message string
	action  string
}

// gitPatterns are checked in order; first match wins
var gitPatterns = []gitErrorPattern{
	// Authentication errors
	{
		pattern: regexp.MustCompile(`(?i)(permission denied|publickey|authentication failed|could not read.*repository)`),
		code:    ErrGitAuth,
		message: "Git authentication failed",
		action:  "Run 'gh auth login' or configure SSH keys. See: https://docs.github.com/en/authentication",
	},
	{
		pattern: regexp.MustCompile(`(?i)(invalid username or password|bad credentials)`),
		code:    ErrGitAuth,
		message: "Invalid Git credentials",
		action:  "Update your credentials with 'gh auth login' or check your SSH key passphrase",
	},

	// Network errors
	{
		pattern: regexp.MustCompile(`(?i)(could not resolve host|network is unreachable|connection timed out|connection refused)`),
		code:    ErrGitNetwork,
		message: "Network connection failed",
		action:  "Check your internet connection and try again",
	},
	{
		pattern: regexp.MustCompile(`(?i)(ssl|tls|certificate)`),
		code:    ErrGitNetwork,
		message: "SSL/TLS connection error",
		action:  "Check your network proxy settings or try: git config --global http.sslVerify true",
	},
	{
		pattern: regexp.MustCompile(`(?i)(timeout|timed out)`),
		code:    ErrTimeout,
		message: "Operation timed out",
		action:  "Check your internet connection and try again. For slow connections, try: git config --global http.postBuffer 524288000",
	},

	// Repository errors
	{
		pattern: regexp.MustCompile(`(?i)(repository not found|does not appear to be a git repository)`),
		code:    ErrGitNotFound,
		message: "Repository not found or inaccessible",
		action:  "Check the repository URL and ensure you have access. For private repos, verify your authentication",
	},
	{
		pattern: regexp.MustCompile(`(?i)(remote.*already exists)`),
		code:    ErrValidation,
		message: "Remote already configured",
		action:  "Use 'git remote set-url origin <url>' to update, or 'git remote remove origin' to remove",
	},

	// Merge/conflict errors
	{
		pattern: regexp.MustCompile(`(?i)(refusing to merge unrelated histories)`),
		code:    ErrGitUnrelated,
		message: "Repositories have different histories",
		action:  "This happens when syncing a fresh repo. Run 'pull' again to force merge, or 'reset' to start fresh",
	},
	{
		pattern: regexp.MustCompile(`(?i)(conflict|merge conflict|automatic merge failed)`),
		code:    ErrGitConflict,
		message: "Merge conflict detected",
		action:  "Resolve conflicts manually, then run 'push' again. Use 'sync conflicts' for help",
	},
	{
		pattern: regexp.MustCompile(`(?i)(your local changes.*would be overwritten)`),
		code:    ErrGitConflict,
		message: "Local changes would be overwritten",
		action:  "Commit or stash your local changes first, then try again",
	},
	{
		pattern: regexp.MustCompile(`(?i)(not possible to fast-forward)`),
		code:    ErrGitConflict,
		message: "Remote has changes that conflict with local",
		action:  "Run 'pull' first to merge remote changes, then 'push'",
	},

	// State errors
	{
		pattern: regexp.MustCompile(`(?i)(head detached|detached head)`),
		code:    ErrGitDetachedHead,
		message: "Repository is in detached HEAD state",
		action:  "Run 'git checkout main' (or master) to return to a branch",
	},
	{
		pattern: regexp.MustCompile(`(?i)(index\.lock|another git process)`),
		code:    ErrGitLocked,
		message: "Git repository is locked",
		action:  "Another git process may be running. Wait or delete .git/index.lock if stuck",
	},

	// Permission errors
	{
		pattern: regexp.MustCompile(`(?i)(permission denied|access denied|protected branch)`),
		code:    ErrGitPermission,
		message: "Permission denied",
		action:  "Check repository permissions. You may not have write access to this branch",
	},

	// Disk/space errors
	{
		pattern: regexp.MustCompile(`(?i)(no space left|disk quota|out of memory)`),
		code:    ErrDiskSpace,
		message: "Insufficient disk space or memory",
		action:  "Free up disk space and try again",
	},
}

// TranslateGitError converts a git error message to a user-friendly UserError
func TranslateGitError(gitErr error) *UserError {
	if gitErr == nil {
		return nil
	}

	errStr := gitErr.Error()

	for _, p := range gitPatterns {
		if p.pattern.MatchString(errStr) {
			return &UserError{
				Code:    p.code,
				Message: p.message,
				Action:  p.action,
				Cause:   gitErr,
				Debug:   errStr,
			}
		}
	}

	// No pattern matched, return generic error
	return &UserError{
		Code:    ErrUnknown,
		Message: "Git operation failed",
		Action:  "Run with --debug for more details. Check 'doctor' for configuration issues",
		Cause:   gitErr,
		Debug:   errStr,
	}
}

// TranslateGitOutput checks git stderr/stdout for error patterns
func TranslateGitOutput(output string, originalErr error) *UserError {
	if output == "" && originalErr == nil {
		return nil
	}

	// Check output for patterns
	for _, p := range gitPatterns {
		if p.pattern.MatchString(output) {
			return &UserError{
				Code:    p.code,
				Message: p.message,
				Action:  p.action,
				Cause:   originalErr,
				Debug:   output,
			}
		}
	}

	// Fall back to checking the error itself
	if originalErr != nil {
		return TranslateGitError(originalErr)
	}

	return nil
}

// IsAuthError checks if error is authentication-related
func IsAuthError(err error) bool {
	if ue, ok := err.(*UserError); ok {
		return ue.Code == ErrGitAuth
	}
	// Check raw error string
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "publickey")
}

// IsNetworkError checks if error is network-related
func IsNetworkError(err error) bool {
	if ue, ok := err.(*UserError); ok {
		return ue.Code == ErrGitNetwork || ue.Code == ErrTimeout
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "could not resolve")
}

// IsConflictError checks if error is merge/conflict-related
func IsConflictError(err error) bool {
	if ue, ok := err.(*UserError); ok {
		return ue.Code == ErrGitConflict || ue.Code == ErrGitUnrelated
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "conflict") ||
		strings.Contains(errStr, "unrelated histories")
}
