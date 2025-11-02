package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestVerifiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *VerifiError
		wantText string
	}{
		{
			name: "error with path",
			err: &VerifiError{
				Op:   "read certificate",
				Path: "/path/to/cert.pem",
				Err:  fmt.Errorf("file not found"),
			},
			wantText: "read certificate /path/to/cert.pem: file not found",
		},
		{
			name: "error without path",
			err: &VerifiError{
				Op:  "add certificate",
				Err: fmt.Errorf("store not initialized"),
			},
			wantText: "add certificate: store not initialized",
		},
		{
			name: "error with empty path",
			err: &VerifiError{
				Op:   "write file",
				Path: "",
				Err:  fmt.Errorf("permission denied"),
			},
			wantText: "write file: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantText {
				t.Errorf("Error() = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestVerifiError_Unwrap(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")
	verifiErr := &VerifiError{
		Op:  "test operation",
		Err: underlyingErr,
	}

	unwrapped := verifiErr.Unwrap()
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}
}

func TestPredefinedErrors(t *testing.T) {
	// Verify all predefined errors are distinct
	errors := []error{
		ErrCertExpired,
		ErrInvalidPEM,
		ErrCertNotFound,
		ErrStoreNotInit,
	}

	for i, err1 := range errors {
		for j, err2 := range errors {
			if i != j && err1 == err2 {
				t.Errorf("Errors at index %d and %d are the same: %v", i, j, err1)
			}
		}
	}

	// Verify error messages are descriptive
	tests := []struct {
		err         error
		wantContain string
	}{
		{ErrCertExpired, "expired"},
		{ErrInvalidPEM, "PEM"},
		{ErrCertNotFound, "not found"},
		{ErrStoreNotInit, "not initialized"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			msg := tt.err.Error()
			if !strings.Contains(strings.ToLower(msg), strings.ToLower(tt.wantContain)) {
				t.Errorf("Error message %q does not contain %q", msg, tt.wantContain)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test that VerifiError properly wraps underlying errors
	baseErr := errors.New("base error")
	wrappedErr := &VerifiError{
		Op:   "test operation",
		Path: "/test/path",
		Err:  baseErr,
	}

	// Test errors.Is() works with wrapped error
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is() should find base error in wrapped error")
	}

	// Test errors.As() works
	var verifiErr *VerifiError
	if !errors.As(wrappedErr, &verifiErr) {
		t.Error("errors.As() should match VerifiError type")
	}

	if verifiErr.Op != "test operation" {
		t.Errorf("errors.As() extracted wrong VerifiError: got Op=%q, want %q", verifiErr.Op, "test operation")
	}
}

func TestExitCodes(t *testing.T) {
	// Verify exit codes are distinct and in expected range
	codes := map[string]int{
		"ExitSuccess":      ExitSuccess,
		"ExitGeneralError": ExitGeneralError,
		"ExitConfigError":  ExitConfigError,
		"ExitCertError":    ExitCertError,
		"ExitNetworkError": ExitNetworkError,
	}

	// Check all codes are distinct
	seen := make(map[int]string)
	for name, code := range codes {
		if prevName, exists := seen[code]; exists {
			t.Errorf("Exit codes %s and %s have the same value: %d", name, prevName, code)
		}
		seen[code] = name
	}

	// Check success code is 0
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}

	// Check error codes are non-zero
	errorCodes := []struct {
		name string
		code int
	}{
		{"ExitGeneralError", ExitGeneralError},
		{"ExitConfigError", ExitConfigError},
		{"ExitCertError", ExitCertError},
		{"ExitNetworkError", ExitNetworkError},
	}

	for _, tc := range errorCodes {
		if tc.code == 0 {
			t.Errorf("%s = 0, should be non-zero", tc.name)
		}
		if tc.code < 0 || tc.code > 255 {
			t.Errorf("%s = %d, should be in range 0-255", tc.name, tc.code)
		}
	}
}
