// Package errors provides user-friendly error translation and types.
package errors

import (
	"fmt"
)

// ErrorCode represents a category of error for programmatic handling
type ErrorCode string

const (
	// Git-related errors
	ErrGitAuth         ErrorCode = "git_auth"
	ErrGitNetwork      ErrorCode = "git_network"
	ErrGitConflict     ErrorCode = "git_conflict"
	ErrGitNotFound     ErrorCode = "git_not_found"
	ErrGitUnrelated    ErrorCode = "git_unrelated_histories"
	ErrGitPermission   ErrorCode = "git_permission"
	ErrGitLocked       ErrorCode = "git_locked"
	ErrGitDetachedHead ErrorCode = "git_detached_head"

	// Encryption errors
	ErrEncryptionKey     ErrorCode = "encryption_key"
	ErrEncryptionDecrypt ErrorCode = "encryption_decrypt"

	// File system errors
	ErrPermission ErrorCode = "permission"
	ErrDiskSpace  ErrorCode = "disk_space"
	ErrNotFound   ErrorCode = "not_found"

	// Operation errors
	ErrConcurrent  ErrorCode = "concurrent_operation"
	ErrValidation  ErrorCode = "validation"
	ErrPrerequisite ErrorCode = "prerequisite"

	// Network errors
	ErrNetwork ErrorCode = "network"
	ErrTimeout ErrorCode = "timeout"

	// Unknown
	ErrUnknown ErrorCode = "unknown"
)

// UserError is an error with user-friendly messaging and suggested actions
type UserError struct {
	// Code is the error category for programmatic handling
	Code ErrorCode

	// Message is a user-friendly error description
	Message string

	// Action is a suggested fix or next step
	Action string

	// Cause is the original underlying error
	Cause error

	// Debug contains detailed technical information (shown with --debug)
	Debug string
}

// Error implements the error interface
func (e *UserError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause for errors.Is/As
func (e *UserError) Unwrap() error {
	return e.Cause
}

// New creates a new UserError
func New(code ErrorCode, message, action string, cause error) *UserError {
	debug := ""
	if cause != nil {
		debug = cause.Error()
	}
	return &UserError{
		Code:    code,
		Message: message,
		Action:  action,
		Cause:   cause,
		Debug:   debug,
	}
}

// Wrap wraps an existing error with user-friendly messaging
func Wrap(cause error, code ErrorCode, message, action string) *UserError {
	return &UserError{
		Code:    code,
		Message: message,
		Action:  action,
		Cause:   cause,
		Debug:   cause.Error(),
	}
}

// Format returns a formatted error message with action suggestion
func (e *UserError) Format() string {
	if e.Action != "" {
		return fmt.Sprintf("%s\n\nSuggested action: %s", e.Message, e.Action)
	}
	return e.Message
}

// FormatWithDebug returns the error with debug info
func (e *UserError) FormatWithDebug() string {
	result := e.Format()
	if e.Debug != "" {
		result += fmt.Sprintf("\n\nDebug info:\n%s", e.Debug)
	}
	return result
}

// IsUserError checks if an error is a UserError
func IsUserError(err error) bool {
	_, ok := err.(*UserError)
	return ok
}

// AsUserError attempts to convert an error to a UserError
func AsUserError(err error) (*UserError, bool) {
	ue, ok := err.(*UserError)
	return ue, ok
}
