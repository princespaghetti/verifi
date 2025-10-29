# Phased Implementation Plan for verifi CLI

## Phase 1: Foundation & Project Setup ✅
**Goal**: Establish project structure with error handling and basic CLI framework

**Status**: COMPLETED

**Deliverables**:
- Go module initialization (`go.mod`)
- Error handling package (`internal/errors/errors.go`) with custom types and exit codes
- Basic Cobra CLI structure (`internal/cli/root.go`, `cmd/verifi/main.go`)
- Makefile/build scripts for common tasks

**Verification**:
- `go build` succeeds
- `verifi --help` shows basic usage
- `verifi --version` shows version info

**Files to Create**:
- `go.mod`
- `cmd/verifi/main.go`
- `internal/errors/errors.go`
- `internal/cli/root.go`

---

## Phase 2: Core Certificate Store & Initialization ✅
**Goal**: Implement directory structure creation with embedded Mozilla bundle

**Status**: COMPLETED

**Deliverables**:
- Obtain Mozilla CA bundle for embedding (one-time development step: download from https://curl.se/ca/cacert.pem to `internal/fetcher/assets/mozilla-ca-bundle.pem` and commit to repo)
- Embedded bundle handling using `go:embed` - completely offline at runtime (`internal/fetcher/embedded.go`)
- File locking implementation (`internal/certstore/lock.go`, `interfaces.go`)
- Core Store type (`internal/certstore/store.go`)
- Metadata management (`internal/certstore/metadata.go`)
- `verifi init` command (`internal/cli/init.go`)

**Verification**:
- `verifi init` creates `~/.verifi/certs/{user,bundles}` directories
- Mozilla bundle extracted from embedded data to `~/.verifi/certs/bundles/mozilla-ca-bundle.pem` (completely offline)
- Initial `metadata.json` created with bundle info
- Running `verifi init` twice shows appropriate error (already initialized)

**Files to Create**:
- `internal/fetcher/assets/mozilla-ca-bundle.pem` (one-time: download from curl.se, commit to repo, embedded at build time)
- `internal/fetcher/embedded.go`
- `internal/certstore/interfaces.go`
- `internal/certstore/lock.go`
- `internal/certstore/store.go`
- `internal/certstore/metadata.go`
- `internal/cli/init.go`

---

## Phase 3: Certificate Management ✅
**Goal**: Add, list, and manage user certificates with bundle building

**Status**: COMPLETED

**Deliverables**:
- Certificate validation (`internal/certstore/validate.go`)
- Bundle building/concatenation (`internal/certstore/bundle.go`)
- `verifi cert add` command
- `verifi cert list` command
- Unit tests for cert validation and bundle building

**Verification**:
- `verifi cert add <path> --name test` adds certificate successfully
- `verifi cert add` rejects expired certs (unless `--force`)
- `verifi cert add` rejects invalid PEM format
- `verifi cert list` shows added certificates with details
- `verifi cert list --json` outputs valid JSON
- `verifi cert list --expired` filters expired certificates
- `combined-bundle.pem` contains Mozilla bundle + user certs
- `metadata.json` tracks all user certificates

**Files Created**:
- `internal/certstore/validate.go`
- `internal/certstore/bundle.go`
- `internal/cli/cert.go`
- `internal/certstore/validate_test.go`
- `internal/certstore/bundle_test.go`

---

## Phase 3.5: Comprehensive Test Coverage for Foundation ✅
**Goal**: Add thorough test coverage for Phases 1-3 before building Phase 4+

**Status**: COMPLETED

**Deliverables**:
- Error handling tests (`internal/errors/errors_test.go`)
- Embedded bundle tests (`internal/fetcher/embedded_test.go`)
- File locking tests (`internal/certstore/lock_test.go`)
- Metadata operation tests (`internal/certstore/metadata_test.go`)
- Store operation tests (`internal/certstore/store_test.go`)
- CLI init command tests (`internal/cli/init_test.go`)

**Verification**:
- All 53+ test cases passing
- Coverage: certstore 81.6%, errors 100%, fetcher 100%
- Concurrent access safety validated (goroutine tests)
- Context cancellation handling verified
- Atomic file operations confirmed
- Error wrapping and unwrapping working correctly

**Files Created**:
- `internal/errors/errors_test.go` (6 tests)
- `internal/fetcher/embedded_test.go` (6 tests)
- `internal/certstore/lock_test.go` (7 tests)
- `internal/certstore/metadata_test.go` (13 tests)
- `internal/certstore/store_test.go` (18 tests)
- `internal/cli/init_test.go` (10 tests)

---

## Phase 4: Shell Environment Generation ✅
**Goal**: Generate env.sh file for environment variable configuration

**Status**: COMPLETED

**Deliverables**:
- Shell environment file generation (`internal/shell/envfile.go`)
- Update `verifi init` to generate env.sh
- `verifi env` command for regenerating env.sh
- Setup instructions printer

**Verification**:
- `verifi init` generates `~/.verifi/env.sh`
- `verifi env` regenerates `~/.verifi/env.sh` (requires initialized store)
- env.sh contains all required environment variables pointing to combined-bundle.pem
- Sourcing env.sh sets `SSL_CERT_FILE`, `NODE_EXTRA_CA_CERTS`, etc.
- `verifi init` prints clear instructions for adding to shell config
- Paths work cross-platform (using `filepath.ToSlash()`)

**Files Created**:
- `internal/shell/envfile.go` - env.sh generation with all 6 environment variables
- `internal/shell/envfile_test.go` - 8 comprehensive tests (100% coverage)
- `internal/cli/env.go` - verifi env command implementation

**Files Modified**:
- `internal/cli/init.go` - Added env.sh generation after store initialization

---

## Phase 5: Status & Information Display ✅
**Goal**: Show certificate store status and configuration (no connection testing)

**Status**: COMPLETED

**Deliverables**:
- `verifi status [--json]` command to display store information
- Output formatting utilities (`internal/cli/output.go`) - optional, may inline

**Status Information Displayed**:
- Certificate store location (~/.verifi)
- Store initialization status
- Number of user certificates (from metadata)
- Combined bundle details (cert count, file path, size)
- env.sh status (exists, location)
- Mozilla bundle info (source: embedded, cert count)
- Last updated timestamps (from metadata)
- JSON output option for scripting/automation

**Verification**:
- `verifi status` displays all store information in clear, readable format
- `verifi status --json` outputs valid, parseable JSON
- Shows helpful message if store not initialized
- No network connections or HTTPS testing (fast, offline-first)
- All information from local file reads only

**Files Created**:
- `internal/cli/status.go` - Status command implementation with human and JSON output
- `internal/cli/status_test.go` - Comprehensive tests (10 tests)

**Files Modified**:
- `internal/certstore/store.go` - Added GetMetadata() public method

**Design Note**: Removed `verifi verify` command from original plan. HTTPS connection
testing with Go doesn't validate that npm, pip, git, curl, etc. will work since those
tools read environment variables independently. Users should test with their actual
tools (e.g., `curl -v https://registry.npmjs.org`, `npm ping`, etc.).

---

## Phase 6: Mozilla Bundle Updates
**Goal**: Fetch and update Mozilla CA bundle from network

**Deliverables**:
- HTTP client for fetching bundles (`internal/fetcher/mozilla.go`)
- Bundle integrity verification (`internal/fetcher/verify.go`)
- `verifi bundle update [--check-only] [--url <custom>]` command
- `verifi bundle info` command to show bundle version and details

**Verification**:
- `verifi bundle update --check-only` shows if newer bundle available
- `verifi bundle update` downloads and replaces Mozilla bundle
- `verifi bundle info` displays bundle version, cert count, SHA256
- Update uses atomic rename pattern (temp file + rename)
- Failed download doesn't corrupt existing bundle
- `metadata.json` updated with new bundle version and SHA256
- `combined-bundle.pem` rebuilt after update

**Files to Create**:
- `internal/fetcher/mozilla.go`
- `internal/fetcher/verify.go`
- `internal/fetcher/interfaces.go`
- `internal/cli/bundle.go`

---

## Phase 7: Maintenance Commands
**Goal**: Complete CLI with certificate removal, inspection, and diagnostics

**Deliverables**:
- `verifi cert remove <name>` command
- `verifi cert inspect <name>` command
- `verifi doctor [--verbose]` diagnostics command
- `verifi clean [--full]` cleanup command
- Integration tests for full workflows

**Verification**:
- `verifi cert remove` removes cert and rebuilds bundle
- `verifi cert inspect` shows detailed cert info (subject, expiry, fingerprint)
- `verifi doctor` validates store integrity, checks for issues
- `verifi clean` removes temp files; `--full` removes entire store
- Integration test: `init → add cert → verify → remove → clean`

**Files to Create**:
- Update `internal/cli/cert.go` with remove/inspect commands
- `internal/cli/doctor.go`
- `internal/cli/clean.go`
- `internal/certstore/store_integration_test.go`

---

## Phase 8: Polish & Distribution
**Goal**: Production-ready release with cross-platform support

**Deliverables**:
- goreleaser configuration for macOS/Linux builds
- Shell completion generation (bash, zsh)
- Enhanced output formatting (colors, tables)
- `--stdin` flag for `cert add`
- Comprehensive README with examples
- CI/CD setup (optional)

**Verification**:
- `goreleaser build --snapshot` produces binaries for multiple platforms
- Shell completions work in bash/zsh
- `verifi cert add --stdin` reads cert from stdin
- All commands have consistent, attractive output
- README guides users through common workflows

**Files to Create**:
- `.goreleaser.yml`
- `internal/cli/completion.go`
- Update `internal/cli/output.go` with colors/tables
- `README.md` (comprehensive)

---

## Phase 9: Java Keystore Integration (Future)
**Goal**: Add Java keystore management using keytool for complete ecosystem coverage

**Deliverables**:
- Java installation detection
- `verifi java add <name>` - Add certificates to Java keystores using keytool
- `verifi java list` - Show certificates in Java keystores
- `verifi java remove <name>` - Remove certificates from Java keystores
- Support for multiple Java versions/installations

**Verification**:
- Detects Java installations automatically
- `verifi java add` imports certificates into cacerts keystore
- `verifi java list` shows certificates in keystore
- Works with multiple Java versions (Oracle, OpenJDK, etc.)
- Handles keystore passwords appropriately

**Files to Create**:
- `internal/java/detect.go` - Java installation detection
- `internal/java/keystore.go` - Keytool operations
- `internal/cli/java.go` - Java command implementations

---

## Testing Strategy Per Phase

Each phase includes:
- **Unit tests**: For new packages/functions (table-driven where appropriate)
- **Error scenarios**: Test failure modes and edge cases
- **Manual verification**: Run commands to verify expected behavior
- **Concurrent safety**: File locking tests in Phase 2+

---

## Key Dependencies Between Phases

```
Phase 1 (Foundation)
  ↓
Phase 2 (Store Init) ← Phase 4 (env.sh) ← Phase 5 (Verify/Status)
  ↓                         ↑
Phase 3 (Cert Mgmt) --------+
  ↓
Phase 6 (Updates)
  ↓
Phase 7 (Maintenance) ← Everything
  ↓
Phase 8 (Polish)
```
