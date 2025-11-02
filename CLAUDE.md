# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**verifi** is a Go CLI tool that manages SSL/TLS certificates across multiple developer tools. It maintains a central certificate store at `~/.verifi/` and configures development environments through environment variables.

**Problem Solved**: Developers behind corporate proxies need to configure certificates for npm, pip, Git, curl, Ruby, etc. verifi does this once, centrally, by generating a single `env.sh` file that sets all necessary environment variables.

**Core Principles**:
- **Simple**: Generate env.sh file, let users source it - no auto-modification
- **Offline-first**: Embed Mozilla CA bundle in binary, no required network access
- **Concurrent-safe**: File locking for all metadata operations
- **Atomic**: Operations use temp files + atomic rename
- **Context-aware**: All long-running operations support cancellation

## Technology Stack

### Core Dependencies
- **CLI Framework**: `spf13/cobra` - Command structure, subcommands, flags, help generation
- **Configuration**: `spf13/viper` - Config file management (optional, may not need)
- **File Locking**: `gofrs/flock` - Cross-platform file locking for concurrent safety
- **Testing**: `stretchr/testify` - Assertions and test utilities

### Standard Library
- `crypto/x509` - Certificate parsing and validation
- `crypto/sha256` - Hash verification for bundle integrity
- `net/http` - Mozilla CA bundle fetching (optional updates)
- `os/user` - Home directory detection
- `path/filepath` - Cross-platform path handling (NEVER use `~` directly)
- `context` - Timeout and cancellation for long-running operations
- `log/slog` - Structured logging (Go 1.21+)
- `embed` - Embed Mozilla CA bundle in binary

### Development Tools
- `golangci-lint` - Linting
- `goreleaser` - Cross-platform binary releases

## Development Commands

```bash
# Install dependencies
go mod download

# Build the CLI
go build -o verifi cmd/verifi/main.go

# Install locally for testing
go install ./cmd/verifi

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -v -run TestAddCert ./internal/certstore/...

# Lint code
golangci-lint run

# Format code
gofmt -w .
go fmt ./...

# Generate shell completions (once implemented)
verifi completion bash > /etc/bash_completion.d/verifi
verifi completion zsh > ~/.zsh/completions/_verifi
```

## Architecture Overview

### User Directory Structure
```
~/.verifi/
├── certs/
│   ├── user/                           # Corporate/proxy certs added by users
│   │   ├── corporate-ca.pem
│   │   └── proxy-cert.pem
│   ├── bundles/
│   │   ├── mozilla-ca-bundle.pem      # Mozilla bundle (embedded on init, updated later)
│   │   └── combined-bundle.pem        # Mozilla + user certs (concatenated)
│   └── metadata.json                  # Versions, hashes, timestamps, audit trail
├── env.sh                              # Generated shell environment script
└── logs/                               # Structured logs (optional)
```

### Go Package Structure (Simplified)

```
cmd/verifi/main.go                      # Entry point

internal/
  certstore/
    store.go                            # Core Store type, initialization
    bundle.go                           # Bundle building (concatenation)
    metadata.go                         # Metadata read/write with locking
    lock.go                             # File locking wrappers
    interfaces.go                       # FileSystem, Locker interfaces for testing

  shell/
    envfile.go                          # Generate ~/.verifi/env.sh

  fetcher/
    mozilla.go                          # Fetch Mozilla bundle updates (optional)
    embedded.go                         # Embedded bundle handling
    verify.go                           # Verify bundle integrity (hash)
    interfaces.go                       # HTTPClient interface for testing
    assets/
      mozilla-ca-bundle.pem             # Embedded Mozilla bundle (go:embed)

  errors/
    errors.go                           # Custom error types with wrapping

  cli/
    root.go                             # Cobra root command setup
    cert.go                             # cert subcommands (add, list, remove, inspect)
    init.go                             # init command
    bundle.go                           # bundle subcommands (update, info, reset)
    env.go                              # env command
    status.go                           # status command
    doctor.go                           # doctor command
    clean.go                            # clean command
    output.go                           # Formatted output (tables, colors)
```

## Critical Go Patterns

### 1. Error Handling

**Use custom error types with wrapping**:

```go
// internal/errors/errors.go
package errors

import "fmt"

type VerifiError struct {
    Op   string  // Operation being performed (e.g., "add cert", "rebuild bundle")
    Path string  // File/cert path involved
    Err  error   // Underlying error
}

func (e *VerifiError) Error() string {
    if e.Path != "" {
        return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
    }
    return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *VerifiError) Unwrap() error {
    return e.Err
}

// Predefined errors
var (
    ErrCertExpired    = fmt.Errorf("certificate has expired")
    ErrInvalidPEM     = fmt.Errorf("invalid PEM format")
    ErrCertNotFound   = fmt.Errorf("certificate not found")
    ErrStoreNotInit   = fmt.Errorf("certificate store not initialized")
)

// Exit codes - use these constants in CLI commands
const (
    ExitSuccess      = 0  // Success
    ExitGeneralError = 1  // General error (file I/O, permissions)
    ExitConfigError  = 2  // Configuration error (invalid config, missing values)
    ExitCertError    = 3  // Certificate error (invalid cert, expired, verification failed)
    ExitNetworkError = 4  // Network error (failed to fetch Mozilla bundle)
)
```

**Usage in CLI commands**:
```go
// internal/cli/cert.go
func runCertAdd(cmd *cobra.Command, args []string) error {
    // ... operation
    if err != nil {
        if errors.Is(err, ErrCertExpired) || errors.Is(err, ErrInvalidPEM) {
            os.Exit(ExitCertError)
        }
        os.Exit(ExitGeneralError)
    }
    return nil
}

### 2. File Locking Pattern

**All metadata operations MUST use file locking**:

```go
// internal/certstore/lock.go
package certstore

import (
    "context"
    "time"

    "github.com/gofrs/flock"
)

type Locker interface {
    Lock(ctx context.Context) error
    Unlock() error
}

type FileLock struct {
    lock *flock.Flock
}

func NewFileLock(path string) *FileLock {
    return &FileLock{
        lock: flock.New(path + ".lock"),
    }
}

func (l *FileLock) Lock(ctx context.Context) error {
    locked, err := l.lock.TryLockContext(ctx, 100*time.Millisecond)
    if err != nil {
        return err
    }
    if !locked {
        return fmt.Errorf("failed to acquire lock")
    }
    return nil
}

func (l *FileLock) Unlock() error {
    return l.lock.Unlock()
}

// Usage in Store methods
func (s *Store) UpdateMetadata(fn func(*Metadata) error) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    lock := NewFileLock(s.metadataPath())
    if err := lock.Lock(ctx); err != nil {
        return fmt.Errorf("failed to lock metadata: %w", err)
    }
    defer lock.Unlock()

    // Read metadata
    metadata, err := s.readMetadata()
    if err != nil {
        return err
    }

    // Apply changes
    if err := fn(metadata); err != nil {
        return err
    }

    // Write metadata
    return s.writeMetadata(metadata)
}
```

### 3. Atomic Operations with Temp Files

**Use temp file + atomic rename pattern**:

```go
func (s *Store) RebuildBundle(ctx context.Context) error {
    bundlePath := filepath.Join(s.basePath, "certs", "bundles", "combined-bundle.pem")
    tempPath := bundlePath + ".tmp"

    // Write to temp file
    f, err := os.Create(tempPath)
    if err != nil {
        return fmt.Errorf("create temp bundle: %w", err)
    }

    // Write Mozilla bundle
    mozillaData, err := os.ReadFile(s.mozillaBundlePath())
    if err != nil {
        f.Close()
        os.Remove(tempPath)
        return fmt.Errorf("read mozilla bundle: %w", err)
    }
    if _, err := f.Write(mozillaData); err != nil {
        f.Close()
        os.Remove(tempPath)
        return fmt.Errorf("write mozilla bundle: %w", err)
    }

    // Append user certs
    userCerts, err := s.readUserCerts(ctx)
    if err != nil {
        f.Close()
        os.Remove(tempPath)
        return fmt.Errorf("read user certs: %w", err)
    }
    for _, certData := range userCerts {
        if _, err := f.Write(certData); err != nil {
            f.Close()
            os.Remove(tempPath)
            return fmt.Errorf("write user cert: %w", err)
        }
    }

    if err := f.Close(); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("close temp bundle: %w", err)
    }

    // Atomic rename
    if err := os.Rename(tempPath, bundlePath); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("rename bundle: %w", err)
    }

    return nil
}
```

### 4. Context Usage

**All long-running operations accept context**:

```go
// HTTP fetches
func (f *Fetcher) FetchMozillaBundle(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := f.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}

// File operations with context checks
func (s *Store) AddCert(ctx context.Context, path, name string) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // ... perform operation
}
```

### 5. Interface-Based Design for Testing

**Define interfaces for external dependencies**:

```go
// internal/certstore/interfaces.go
type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    MkdirAll(path string, perm os.FileMode) error
    Remove(path string) error
    Stat(path string) (os.FileInfo, error)
}

type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

// Production implementation
type OSFileSystem struct{}

func (fs *OSFileSystem) ReadFile(path string) ([]byte, error) {
    return os.ReadFile(path)
}

// Test implementation
type MockFileSystem struct {
    files map[string][]byte
}

func (fs *MockFileSystem) ReadFile(path string) ([]byte, error) {
    data, ok := fs.files[path]
    if !ok {
        return nil, os.ErrNotExist
    }
    return data, nil
}
```

### 6. Table-Driven Tests

```go
func TestStore_AddCert(t *testing.T) {
    tests := []struct {
        name    string
        certPEM string
        wantErr bool
        errType error
    }{
        {
            name:    "valid certificate",
            certPEM: validCertPEM,
            wantErr: false,
        },
        {
            name:    "expired certificate",
            certPEM: expiredCertPEM,
            wantErr: true,
            errType: ErrCertExpired,
        },
        {
            name:    "invalid PEM",
            certPEM: "not a cert",
            wantErr: true,
            errType: ErrInvalidPEM,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := NewStore(t.TempDir())
            err := store.AddCert(context.Background(), tt.certPEM, "test")

            if tt.wantErr {
                require.Error(t, err)
                if tt.errType != nil {
                    require.ErrorIs(t, err, tt.errType)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### 7. Structured Logging

```go
import "log/slog"

func (s *Store) AddCert(ctx context.Context, path, name string) error {
    slog.Info("adding certificate",
        "name", name,
        "path", path,
    )

    // ... operation

    slog.Info("certificate added successfully",
        "name", name,
        "fingerprint", fingerprint,
        "expires", expiryDate,
    )

    return nil
}

// In case of error
slog.Error("failed to add certificate",
    "name", name,
    "path", path,
    "error", err,
)
```

### 8. Cross-Platform Path Handling

**NEVER use `~` or hardcoded separators**:

```go
import (
    "os/user"
    "path/filepath"
)

func getVerifiHome() (string, error) {
    usr, err := user.Current()
    if err != nil {
        return "", err
    }
    return filepath.Join(usr.HomeDir, ".verifi"), nil
}

// GOOD
bundlePath := filepath.Join(basePath, "certs", "bundles", "combined-bundle.pem")

// BAD
bundlePath := basePath + "/certs/bundles/combined-bundle.pem"  // ❌ Unix-only
bundlePath := "~/.verifi/certs/bundles/combined-bundle.pem"    // ❌ ~ not expanded
```

## Code Quality Guidelines

### Avoid Redundant Wrapper Functions

**DON'T create wrapper functions that add no value**:

```go
// ❌ BAD - Redundant wrapper around standard library
func IsError(err, target error) bool {
    return errors.Is(err, target)
}

// ✅ GOOD - Use standard library directly
if errors.Is(err, ErrCertExpired) {
    // handle error
}
```

**When wrappers ARE acceptable**:
- They add meaningful business logic or validation
- They provide a domain-specific abstraction that improves readability
- They centralize complex behavior that would otherwise be duplicated

```go
// ✅ GOOD - Adds validation and domain logic
func ValidateCert(data []byte) (*x509.Certificate, error) {
    block, _ := pem.Decode(data)
    if block == nil {
        return nil, ErrInvalidPEM
    }
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("parse certificate: %w", err)
    }
    // Additional domain-specific validation
    if time.Now().After(cert.NotAfter) {
        return nil, ErrCertExpired
    }
    return cert, nil
}
```

### Eliminate Code Duplication (DRY Principle)

**Never duplicate function implementations across packages**:

```go
// ❌ BAD - Same function exists in multiple packages
// internal/certstore/store.go
func countCertificates(pemData []byte) int { ... }

// internal/fetcher/verify.go
func countCertificates(pemData []byte) int { ... }  // Duplicate!

// ✅ GOOD - Single implementation, exported from appropriate package
// internal/fetcher/verify.go
func CountCertificates(pemData []byte) int { ... }

// internal/certstore/store.go
import "github.com/princespaghetti/verifi/internal/fetcher"
count := fetcher.CountCertificates(bundleData)
```

**Guidelines for shared code**:
- Place shared utilities in the package where they're most semantically appropriate
- Export the function (capitalize) if needed by other packages
- Never copy-paste implementations

### Avoid Dead Code

**Remove unused functions immediately**:

```go
// ❌ BAD - Unused function that was never called
func RepeatString(s string, n int) string {
    return strings.Repeat(s, n)
}

// ✅ GOOD - Use standard library directly where needed
separator := strings.Repeat("=", 40)
```

**If you create a function but don't use it immediately, ask yourself**:
- Is this actually needed (YAGNI - You Aren't Gonna Need It)?
- Should this be in a utility package, or is it premature abstraction?
- Will this definitely be used soon, or am I speculating about future needs?

### Prefer Direct Standard Library Usage

**Use Go's standard library directly unless wrapping adds clear value**:

```go
// ❌ BAD - Unnecessary wrappers
func RepeatString(s string, n int) string { return strings.Repeat(s, n) }
func JoinStrings(elems []string, sep string) string { return strings.Join(elems, sep) }

// ✅ GOOD - Direct usage
separator := strings.Repeat("=", 40)
path := strings.Join(parts, "/")
```

**Exceptions where wrappers make sense**:
- Creating test doubles (interfaces for os.ReadFile, http.Client, etc.)
- Adding context-specific error handling
- Centralizing complex standard library usage patterns

### Interface Wrappers for Testing (APPROVED Pattern)

**This pattern IS recommended and should be kept**:

```go
// ✅ GOOD - Interface wrapper for testing
type FileSystem interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
}

type OSFileSystem struct{}

func (fs *OSFileSystem) ReadFile(path string) ([]byte, error) {
    return os.ReadFile(path)  // Wrapper is intentional for dependency injection
}
```

This is acceptable because:
- Enables dependency injection for testing
- Provides a seam for mocking
- Mentioned explicitly in CLAUDE.md's "Interface-Based Design for Testing" pattern

### Code Review Checklist

Before committing code, verify:
- [ ] No functions that simply wrap standard library without adding logic
- [ ] No duplicate implementations across packages
- [ ] All defined functions are actually used
- [ ] Standard library is used directly when possible
- [ ] Wrapper functions have clear justification (testing, validation, business logic)

## Embedded Mozilla Bundle Pattern

**Use `go:embed` to bundle Mozilla CA certs in the binary**:

```go
// internal/fetcher/embedded.go
package fetcher

import _ "embed"

//go:embed assets/mozilla-ca-bundle.pem
var embeddedMozillaBundle []byte

func GetEmbeddedBundle() []byte {
    return embeddedMozillaBundle
}

// Usage in verifi init
func (s *Store) Init(ctx context.Context) error {
    // ... create directory structure

    // Write embedded bundle to mozilla-ca-bundle.pem
    mozillaPath := filepath.Join(s.basePath, "certs", "bundles", "mozilla-ca-bundle.pem")
    if err := os.WriteFile(mozillaPath, GetEmbeddedBundle(), 0644); err != nil {
        return fmt.Errorf("write mozilla bundle: %w", err)
    }

    // Build combined bundle
    if err := s.RebuildBundle(ctx); err != nil {
        return fmt.Errorf("rebuild bundle: %w", err)
    }

    return nil
}
```

**Benefits**:
- `verifi init` works completely offline
- No dependency on curl.se being available
- Faster initialization (no network request)
- `verifi bundle update` optionally fetches newer versions

## CLI Command Reference

### Simplified Commands

```bash
# Initialize the certificate store (offline-first with embedded bundle)
verifi init [--force]

# Certificate management
verifi cert add <path> --name <name> [--force] [--stdin]
verifi cert list [--json] [--expired]
verifi cert remove <name> [--force]
verifi cert inspect <name>

# Mozilla CA bundle management (optional, from curl.se)
verifi bundle update [--check-only] [--url <custom-url>]
verifi bundle info [--json]

# Status and information
verifi status [--json]

# Diagnostics and cleanup
verifi doctor [--verbose] [--json]
verifi clean [--full]
```

### Cobra Command Structure

```go
// internal/cli/root.go
var rootCmd = &cobra.Command{
    Use:   "verifi",
    Short: "Unified certificate management for developers",
    Long:  `verifi manages SSL/TLS certificates across multiple developer tools.`,
}

func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(certCmd)
    rootCmd.AddCommand(bundleCmd)
    rootCmd.AddCommand(statusCmd)
    rootCmd.AddCommand(doctorCmd)
    rootCmd.AddCommand(cleanCmd)
}

// internal/cli/cert.go
var certCmd = &cobra.Command{
    Use:   "cert",
    Short: "Certificate management",
}

var certAddCmd = &cobra.Command{
    Use:   "add <path>",
    Short: "Add a certificate to the store",
    Args:  cobra.ExactArgs(1),
    RunE:  runCertAdd,
}

func init() {
    certCmd.AddCommand(certAddCmd)
    certCmd.AddCommand(certListCmd)
    certCmd.AddCommand(certRemoveCmd)
    certCmd.AddCommand(certInspectCmd)

    certAddCmd.Flags().String("name", "", "Certificate name (required)")
    certAddCmd.Flags().Bool("force", false, "Force add even if expired")
    certAddCmd.Flags().Bool("stdin", false, "Read certificate from stdin")
    certAddCmd.MarkFlagRequired("name")
}
```

## Shell Configuration Strategy

### Simple env.sh Generation (No Auto-Modification)

**verifi generates `~/.verifi/env.sh` and prints instructions**:

```go
// internal/shell/envfile.go
package shell

import (
    "fmt"
    "os"
    "path/filepath"
)

func GenerateEnvFile(verifiHome, bundlePath string) error {
    envPath := filepath.Join(verifiHome, "env.sh")

    // Convert Windows paths to forward slashes for shell compatibility
    // Even on Windows (Git Bash, WSL), shell scripts use forward slashes
    shellPath := filepath.ToSlash(bundlePath)

    content := fmt.Sprintf(`# verifi environment configuration
# Generated by verifi - do not edit manually

export SSL_CERT_FILE="%s"
export REQUESTS_CA_BUNDLE="%s"
export NODE_EXTRA_CA_CERTS="%s"
export CURL_CA_BUNDLE="%s"
export AWS_CA_BUNDLE="%s"
export GIT_SSL_CAINFO="%s"
`, shellPath, shellPath, shellPath, shellPath, shellPath, shellPath)

    return os.WriteFile(envPath, []byte(content), 0644)
}

func PrintSetupInstructions(envPath string) {
    fmt.Printf(`
✓ Certificate store initialized
✓ Environment file created: %s

To activate certificates for all tools, add this to your shell config:

  source %s

Quick setup:
  echo 'source %s' >> ~/.zshrc   # for zsh
  echo 'source %s' >> ~/.bashrc  # for bash

Then restart your shell or run:
  source ~/.zshrc  # or source ~/.bashrc

`, envPath, envPath, envPath, envPath)
}
```

**Why this approach**:
- Zero risk of breaking shell configurations
- Users understand exactly what's happening
- No backup/restore complexity
- Works with dotfile managers
- Simple copy-paste setup

## Certificate Bundle Building

**Simple PEM concatenation**:

```go
func (s *Store) RebuildBundle(ctx context.Context) error {
    // 1. Read Mozilla bundle
    // 2. Read all user certs from certs/user/*.pem
    // 3. Concatenate (PEM format is designed for this)
    // 4. Write to combined-bundle.pem using atomic rename
    // 5. Update metadata with hash and cert count
}
```

**Metadata tracking**:
```json
{
  "version": "1",
  "combined_bundle": {
    "generated": "2025-10-26T10:05:00Z",
    "sha256": "abc123...",
    "cert_count": 139,
    "sources": ["mozilla", "user"]
  },
  "mozilla_bundle": {
    "version": "2025-10-15",
    "sha256": "def456...",
    "cert_count": 137,
    "source": "embedded"
  },
  "user_certs": [
    {
      "name": "corporate-proxy",
      "path": "user/corporate-proxy.pem",
      "added": "2025-10-26T10:00:00Z",
      "fingerprint": "sha256:...",
      "subject": "CN=Corporate Proxy CA",
      "expires": "2026-10-26T10:00:00Z"
    }
  ]
}
```

## Environment Variables Set by env.sh

These environment variables work automatically for most developer tools:

| Environment Variable | Tools Supported |
|---------------------|-----------------|
| `SSL_CERT_FILE` | Python (pip, requests), Ruby (gem), curl, wget |
| `REQUESTS_CA_BUNDLE` | Python requests library |
| `NODE_EXTRA_CA_CERTS` | Node.js, npm, yarn, pnpm |
| `CURL_CA_BUNDLE` | curl, tools using libcurl |
| `AWS_CA_BUNDLE` | AWS CLI, boto3 |
| `GIT_SSL_CAINFO` | git |

**Note**: No tool-specific configuration needed. These environment variables are recognized by the tools' underlying SSL/TLS libraries.

## Important Implementation Patterns

### 1. Certificate Validation

```go
func ValidateCert(data []byte) (*x509.Certificate, error) {
    block, _ := pem.Decode(data)
    if block == nil {
        return nil, ErrInvalidPEM
    }

    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("parse certificate: %w", err)
    }

    // Check expiry
    now := time.Now()
    if now.After(cert.NotAfter) {
        return nil, ErrCertExpired
    }

    return cert, nil
}
```

### 2. Metadata Schema Versioning

```go
type Metadata struct {
    Version string `json:"version"`  // Schema version for migrations
    // ... rest of fields
}

func LoadMetadata(path string) (*Metadata, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var m Metadata
    if err := json.Unmarshal(data, &m); err != nil {
        return nil, err
    }

    // Migrate if needed
    if m.Version != currentSchemaVersion {
        if err := migrateMetadata(&m); err != nil {
            return nil, fmt.Errorf("migrate metadata: %w", err)
        }
    }

    return &m, nil
}
```

## Testing Approach

### Unit Tests
- Test each package independently
- Use interfaces for mocking (FileSystem, HTTPClient, Locker)
- Table-driven tests for validation logic
- Test error conditions thoroughly

### Integration Tests
- Test full workflows (init → add cert → status → doctor → remove → doctor → clean)
- Use temp directories for isolation
- Test concurrent operations with file locking
- Test rollback on failures

### Test Scenarios
- Valid certificate addition
- Expired certificate handling
- Invalid PEM format
- Concurrent metadata updates
- Bundle rebuild after cert add/remove
- Embedded bundle initialization
- Mozilla bundle update (network success/failure)
- Status display with various store states
- Partial operation failures (rollback verification)

## Common Development Workflows

### Initial Setup
```bash
# Initialize with embedded bundle (offline)
verifi init

# Add corporate certificate
verifi cert add /path/to/corp-ca.crt --name corporate

# Follow printed instructions to add to shell
echo 'source ~/.verifi/env.sh' >> ~/.zshrc
source ~/.zshrc

# Check status
verifi status

# Test with your actual tools
curl -v https://registry.npmjs.org
npm ping
```

### Adding Certificates
```bash
# From file
verifi cert add proxy-cert.pem --name proxy

# From URL (if accessible)
curl https://internal.corp.com/ca.crt | verifi cert add --stdin --name internal

# Inspect certificate
verifi cert inspect corporate

# List all certs
verifi cert list
```

### Updating Mozilla Bundle
```bash
# Check for updates
verifi bundle update --check-only

# Show current bundle information
verifi bundle info

# Update if available (fetches from https://curl.se/ca/cacert.pem)
verifi bundle update

# Use custom URL if needed
verifi bundle update --url https://mirror.example.com/cacert.pem
```

**Default Mozilla CA Bundle URL**: `https://curl.se/ca/cacert.pem`
- Official Mozilla CA certificate bundle in PEM format
- Maintained by the curl project
- Alternative info: https://wiki.mozilla.org/CA/Included_Certificates

### Troubleshooting
```bash
# Check overall status
verifi status

# Run diagnostics
verifi doctor --verbose

# Test with actual tools to verify certificates work
curl -v https://internal.company.com
npm config get registry
git ls-remote https://github.com/user/repo.git

# Get JSON output for scripting
verifi status --json
```

## Platform Support

**Primary**: macOS, Linux (Ubuntu, Debian, Fedora, Arch)
**Secondary**: Windows (WSL2 initially, native later)

**Platform-specific considerations**:
- Use `filepath.Join()` for all path operations (handles OS-specific separators)
- Use `filepath.ToSlash()` when writing paths to env.sh (shells always use `/`)
- Use `os/user` for home directory detection
- File locking works cross-platform with gofrs/flock

**Windows WSL2 Notes**:
- WSL2 uses Linux-style paths internally (`/home/user/.verifi`)
- Git Bash on Windows also uses forward slashes in shell scripts
- `filepath.ToSlash()` ensures compatibility across all environments

## Development Workflow

1. **Before starting a feature**: Check this document for relevant patterns
2. **Write tests first**: Define expected behavior with table-driven tests
3. **Implement with proper error handling**: Use custom error types, wrap errors with `%w`
4. **Use file locking**: All metadata operations must acquire lock first
5. **Use context**: All long-running operations accept context.Context
6. **Use temp files**: All file writes use temp file + atomic rename pattern
7. **Log with slog**: Structured logging for debugging
8. **Avoid redundant code**: Check "Code Quality Guidelines" - no unnecessary wrappers, no duplication, no dead code
9. **Run linter**: `golangci-lint run` before committing
10. **Update this file**: When you discover new patterns or make architectural decisions

## Key Design Decisions

### Why No Tool Detection?
- Environment variables work automatically for npm, pip, git, Ruby, curl, AWS CLI
- No need to detect tools or configure them individually
- Simpler codebase, same user value
- Can add later if users request it

### Why No Shell Auto-Modification?
- Adding one line to shell config is easy
- Auto-modification adds risk and complexity
- Backup/restore is unnecessary for this use case
- Users can copy-paste the command

### Why Embed Mozilla Bundle?
- Tool works completely offline on first use
- No dependency on curl.se availability
- Faster initialization
- Updates are optional, not required

### Why No Built-in Verification?
- verifi configures environment variables - testing with Go HTTP client doesn't validate that npm, pip, git, etc. will work
- Each tool (npm, pip, curl, etc.) has its own TLS/SSL implementation
- Users should test with their actual tools: `curl -v https://...`, `npm ping`, etc.
- Simple, focused tool - configuration only, not testing

## Exit Codes

Exit codes are defined as constants in `internal/errors/errors.go`:

```go
const (
    ExitSuccess      = 0  // Success
    ExitGeneralError = 1  // General error (file I/O, permissions)
    ExitConfigError  = 2  // Configuration error (invalid config, missing values)
    ExitCertError    = 3  // Certificate error (invalid cert, expired, verification failed)
    ExitNetworkError = 4  // Network error (failed to fetch Mozilla bundle)
)
```

**Always use these constants** instead of hardcoding exit codes in CLI commands.
