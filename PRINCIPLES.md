# Design Principles

## Core Principles

### Simple, Not Automatic
verifi generates configuration but doesn't automatically modify your shell files. You control what goes into your environment.

### Explicit Operations
Commands do exactly what they say with no hidden side effects. What you see is what you get.

### Fail-Safe
Operations use temporary files and atomic renames. Interrupted operations don't leave the system in an inconsistent state.

### Transparent
Clear output, detailed logging, and structured data output options (JSON) for scripting and debugging.

## Architecture

verifi operates on a simple principle: maintain a combined certificate bundle containing both Mozilla's trusted CAs and your custom certificates, then configure all tools to use this bundle through environment variables.

### Certificate Store

Located at `~/.verifi/`, containing:
- Mozilla's CA bundle (embedded in binary, optionally updated)
- Your custom certificates (corporate, proxy, etc.)
- A combined bundle merging both sources
- Metadata tracking versions, hashes, and certificate details

### Environment Configuration

A generated `env.sh` file that sets standard environment variables recognized by development tools:
- `SSL_CERT_FILE` - Python, Ruby, Go, curl, wget
- `NODE_EXTRA_CA_CERTS` - Node.js, npm, yarn, pnpm
- `REQUESTS_CA_BUNDLE` - Python requests library
- `GIT_SSL_CAINFO` - git
- `AWS_CA_BUNDLE` - AWS CLI, boto3
- `CURL_CA_BUNDLE` - curl and libcurl-based tools

### Unified Interface

Simple commands for common operations:
- Initialize the certificate store
- Add or remove certificates
- Update the Mozilla CA bundle
- Check status and verify connections
- Inspect certificate details

## Design Benefits

### Simplicity
Configure once, use everywhere. Add your corporate certificates to verifi, source one environment file, and all your tools work automatically.

### Reliability
Built on proven patterns including atomic file operations, proper file locking, and comprehensive error handling. Operations are designed to be safe even when interrupted.

### Offline-First
Works completely offline from day one. The Mozilla CA bundle is embedded in the binary, so you don't need network access to initialize or use verifi.

### Cross-Platform
Designed to work on macOS, Linux, and Windows (via WSL2), with platform-specific considerations handled automatically.

### Developer-Focused
Respects your shell configuration, provides clear output, supports JSON for scripting, and integrates naturally into existing workflows.
