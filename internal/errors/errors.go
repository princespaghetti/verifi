// Package errors provides custom error types and exit codes for verifi.
package errors

import (
	"errors"
	"fmt"
)

// VerifiError is a custom error type that provides context about operations.
type VerifiError struct {
	Op   string // Operation being performed (e.g., "add cert", "rebuild bundle")
	Path string // File/cert path involved
	Err  error  // Underlying error
}

// Error implements the error interface.
func (e *VerifiError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for error chain inspection.
func (e *VerifiError) Unwrap() error {
	return e.Err
}

// Predefined errors for common scenarios.
var (
	ErrCertExpired      = fmt.Errorf("certificate has expired")
	ErrInvalidPEM       = fmt.Errorf("invalid PEM format")
	ErrCertNotFound     = fmt.Errorf("certificate not found")
	ErrStoreNotInit     = fmt.Errorf("certificate store not initialized")
	ErrStoreAlreadyInit = fmt.Errorf("certificate store already initialized")
)

// Exit codes - use these constants in CLI commands instead of hardcoding values.
const (
	ExitSuccess      = 0 // Success
	ExitGeneralError = 1 // General error (file I/O, permissions)
	ExitConfigError  = 2 // Configuration error (invalid config, missing values)
	ExitCertError    = 3 // Certificate error (invalid cert, expired, verification failed)
	ExitNetworkError = 4 // Network error (failed to fetch Mozilla bundle)
)

// IsError checks if the given error matches the target error using errors.Is.
func IsError(err, target error) bool {
	return errors.Is(err, target)
}
