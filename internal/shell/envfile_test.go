package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "certs", "bundles", "combined-bundle.pem")

	err := GenerateEnvFile(tmpDir, bundlePath)
	if err != nil {
		t.Fatalf("GenerateEnvFile() failed: %v", err)
	}

	// Verify file was created
	envPath := filepath.Join(tmpDir, "env.sh")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Fatalf("env.sh was not created at %s", envPath)
	}

	// Read and verify content
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read env.sh: %v", err)
	}

	contentStr := string(content)

	// Verify all required environment variables are set
	requiredVars := []string{
		"SSL_CERT_FILE",
		"REQUESTS_CA_BUNDLE",
		"NODE_EXTRA_CA_CERTS",
		"CURL_CA_BUNDLE",
		"AWS_CA_BUNDLE",
		"GIT_SSL_CAINFO",
	}

	for _, varName := range requiredVars {
		if !strings.Contains(contentStr, "export "+varName) {
			t.Errorf("env.sh missing variable: %s", varName)
		}
	}

	// Verify bundle path is in the file (converted to forward slashes)
	shellPath := filepath.ToSlash(bundlePath)
	if !strings.Contains(contentStr, shellPath) {
		t.Errorf("env.sh missing bundle path: %s", shellPath)
	}

	// Verify each variable is set to the bundle path
	for _, varName := range requiredVars {
		expected := fmt.Sprintf("export %s=\"%s\"", varName, shellPath)
		if !strings.Contains(contentStr, expected) {
			t.Errorf("env.sh variable %s not set to bundle path\nExpected: %s\nContent:\n%s",
				varName, expected, contentStr)
		}
	}
}

func TestGenerateEnvFile_WindowsPath(t *testing.T) {
	tmpDir := t.TempDir()
	// Simulate a Windows-style path with backslashes
	bundlePath := "C:\\Users\\test\\.verifi\\certs\\bundles\\combined-bundle.pem"

	err := GenerateEnvFile(tmpDir, bundlePath)
	if err != nil {
		t.Fatalf("GenerateEnvFile() failed: %v", err)
	}

	// Read content
	envPath := filepath.Join(tmpDir, "env.sh")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read env.sh: %v", err)
	}

	contentStr := string(content)

	// Verify path was converted to forward slashes
	expectedPath := "C:/Users/test/.verifi/certs/bundles/combined-bundle.pem"
	if !strings.Contains(contentStr, expectedPath) {
		t.Errorf("Windows path not converted to forward slashes\nExpected: %s\nContent:\n%s",
			expectedPath, contentStr)
	}

	// Verify no backslashes remain
	if strings.Contains(contentStr, "\\") {
		t.Errorf("env.sh contains backslashes (should be converted to forward slashes):\n%s", contentStr)
	}
}

func TestGenerateEnvFile_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()

	err := GenerateEnvFile(tmpDir, "")
	if err != nil {
		t.Fatalf("GenerateEnvFile() with empty bundle path failed: %v", err)
	}

	// Verify file was still created (even with empty path)
	envPath := filepath.Join(tmpDir, "env.sh")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Fatalf("env.sh was not created with empty bundle path")
	}

	// Read content
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read env.sh: %v", err)
	}

	// Verify variables are still exported (even if empty)
	contentStr := string(content)
	if !strings.Contains(contentStr, "export SSL_CERT_FILE") {
		t.Errorf("env.sh missing SSL_CERT_FILE export with empty path")
	}
}

func TestGenerateEnvFile_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.sh")

	// Create an initial env.sh file
	initialContent := "# Old content\n"
	if err := os.WriteFile(envPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial env.sh: %v", err)
	}

	// Generate new env.sh (should overwrite)
	bundlePath := filepath.Join(tmpDir, "certs", "bundles", "combined-bundle.pem")
	err := GenerateEnvFile(tmpDir, bundlePath)
	if err != nil {
		t.Fatalf("GenerateEnvFile() failed: %v", err)
	}

	// Verify content was overwritten
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read env.sh: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "Old content") {
		t.Errorf("env.sh was not overwritten (still contains old content)")
	}

	if !strings.Contains(contentStr, "export SSL_CERT_FILE") {
		t.Errorf("env.sh does not contain new content after overwrite")
	}
}

func TestGenerateEnvFile_InvalidDirectory(t *testing.T) {
	// Try to write to a non-existent directory
	invalidDir := "/nonexistent/invalid/path/that/does/not/exist"
	bundlePath := filepath.Join(invalidDir, "bundle.pem")

	err := GenerateEnvFile(invalidDir, bundlePath)
	if err == nil {
		t.Errorf("GenerateEnvFile() should fail with invalid directory, but succeeded")
	}
}

func TestEnvFilePath(t *testing.T) {
	tests := []struct {
		name        string
		verifiHome  string
		expected    string
	}{
		{
			name:       "unix path",
			verifiHome: "/home/user/.verifi",
			expected:   "/home/user/.verifi/env.sh",
		},
		{
			name:       "windows path",
			verifiHome: "C:\\Users\\test\\.verifi",
			expected:   filepath.Join("C:\\Users\\test\\.verifi", "env.sh"),
		},
		{
			name:       "relative path",
			verifiHome: ".verifi",
			expected:   filepath.Join(".verifi", "env.sh"),
		},
		{
			name:       "empty path",
			verifiHome: "",
			expected:   "env.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnvFilePath(tt.verifiHome)
			if result != tt.expected {
				t.Errorf("EnvFilePath(%q) = %q, want %q", tt.verifiHome, result, tt.expected)
			}
		})
	}
}

func TestPrintSetupInstructions(t *testing.T) {
	// This test just ensures PrintSetupInstructions doesn't panic
	// We can't easily test the output without capturing stdout
	envPath := "/home/user/.verifi/env.sh"

	// Should not panic
	PrintSetupInstructions(envPath)
}

func TestGenerateEnvFile_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "bundle.pem")

	err := GenerateEnvFile(tmpDir, bundlePath)
	if err != nil {
		t.Fatalf("GenerateEnvFile() failed: %v", err)
	}

	// Check file permissions
	envPath := filepath.Join(tmpDir, "env.sh")
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("Failed to stat env.sh: %v", err)
	}

	// Verify file is readable (at least user-readable)
	mode := info.Mode()
	if mode&0400 == 0 {
		t.Errorf("env.sh is not user-readable (mode: %o)", mode)
	}
}
