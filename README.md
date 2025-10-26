# verifi

**Unified SSL/TLS certificate management for developers**

verifi is a command-line tool that simplifies certificate configuration across your entire development environment. Instead of configuring certificates separately for npm, pip, git, curl, and dozens of other tools, verifi does it once, centrally.

---

## The Problem

Developers working behind corporate proxies or in enterprise environments face a recurring challenge: every development tool needs to be configured with custom SSL/TLS certificates. This typically means:

- Manually configuring npm, pip, git, curl, Ruby gems, AWS CLI, and more
- Tracking down tool-specific configuration files and environment variables
- Repeating the process for each new tool or project
- Managing certificate updates across multiple configurations
- Debugging SSL errors when certificates expire or change
- Onboarding new team members through complex setup procedures

This fragmentation leads to wasted time, frustration, and inconsistent configurations across development teams.

## What verifi Does

verifi provides a centralized approach to certificate management:

- **Maintains a single certificate store** at `~/.verifi/` that serves as the source of truth
- **Generates a unified environment configuration** that works across all major development tools
- **Manages certificate lifecycle** including additions, updates, and expiration tracking
- **Provides offline-first operation** by embedding Mozilla's CA bundle directly in the binary
- **Ensures safe concurrent operations** through file locking and atomic updates

## Why Use verifi?

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

## How It Works

verifi operates on a simple principle: maintain a combined certificate bundle containing both Mozilla's trusted CAs and your custom certificates, then configure all tools to use this bundle through environment variables.

### Architecture

1. **Certificate Store**: Located at `~/.verifi/`, containing:
   - Mozilla's CA bundle (embedded in binary, optionally updated)
   - Your custom certificates (corporate, proxy, etc.)
   - A combined bundle merging both sources
   - Metadata tracking versions, hashes, and certificate details

2. **Environment Configuration**: A generated `env.sh` file that sets standard environment variables recognized by development tools:
   - `SSL_CERT_FILE` - Python, Ruby, Go, curl, wget
   - `NODE_EXTRA_CA_CERTS` - Node.js, npm, yarn, pnpm
   - `REQUESTS_CA_BUNDLE` - Python requests library
   - `GIT_SSL_CAINFO` - git
   - `AWS_CA_BUNDLE` - AWS CLI, boto3
   - `CURL_CA_BUNDLE` - curl and libcurl-based tools

3. **Unified Interface**: Simple commands for common operations:
   - Initialize the certificate store
   - Add or remove certificates
   - Update the Mozilla CA bundle
   - Check status and verify connections
   - Inspect certificate details

## Installation

> Installation instructions will be provided once releases are available.

verifi will be distributed as pre-built binaries for major platforms via GitHub Releases.

## Getting Started

The typical workflow with verifi:

1. **Initialize**: Set up the certificate store with the embedded Mozilla CA bundle
2. **Add Certificates**: Import your corporate or proxy certificates
3. **Configure Shell**: Add the generated environment file to your shell configuration
4. **Verify**: Test that HTTPS connections work correctly
5. **Use**: All your development tools now use the configured certificates automatically

## Core Principles

### Simple, Not Automatic
verifi generates configuration but doesn't automatically modify your shell files. You control what goes into your environment.

### Explicit Operations
Commands do exactly what they say with no hidden side effects. What you see is what you get.

### Fail-Safe
Operations use temporary files and atomic renames. Interrupted operations don't leave the system in an inconsistent state.

### Transparent
Clear output, detailed logging, and structured data output options (JSON) for scripting and debugging.

## Tool Support

verifi works automatically with tools that respect standard SSL environment variables:

- **Package Managers**: npm, yarn, pnpm, pip, gem, cargo
- **Version Control**: git
- **HTTP Clients**: curl, wget, HTTPie
- **Cloud CLIs**: AWS CLI, gcloud (when using Python)
- **Programming Languages**: Python (requests), Node.js, Go, Ruby
- **Infrastructure Tools**: terraform, ansible (when using Python), docker (registry operations)

No per-tool configuration required - these tools automatically recognize the environment variables verifi sets.

## Documentation

Comprehensive documentation is available in the repository:

- Command reference and usage
- Certificate management workflows
- Troubleshooting guide
- Development and contribution guidelines

## Project Status

verifi is currently in active development. The architecture and core principles are established, and implementation is underway.

## Contributing

Contributions are welcome! Please see CONTRIBUTING.md for:

- Development environment setup
- Code structure and patterns
- Testing requirements
- Pull request process

## License

> License information to be added.

## Support

For issues, questions, or feature requests, please use the GitHub issue tracker.

---

**Built for developers, by developers.**
