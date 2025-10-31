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

### macOS

**Homebrew** (coming soon):
```bash
brew tap princespaghetti/verifi https://github.com/princespaghetti/verifi
brew install verifi
```

**Manual installation**:
```bash
# For M1/M2 Macs (arm64):
curl -LO https://github.com/princespaghetti/verifi/releases/latest/download/verifi_*_darwin_arm64.tar.gz
tar -xzf verifi_*_darwin_arm64.tar.gz
sudo mv verifi /usr/local/bin/
sudo chmod +x /usr/local/bin/verifi

# For Intel Macs (amd64):
curl -LO https://github.com/princespaghetti/verifi/releases/latest/download/verifi_*_darwin_amd64.tar.gz
tar -xzf verifi_*_darwin_amd64.tar.gz
sudo mv verifi /usr/local/bin/
sudo chmod +x /usr/local/bin/verifi
```

### Linux

```bash
# For amd64:
curl -LO https://github.com/princespaghetti/verifi/releases/latest/download/verifi_*_linux_amd64.tar.gz
tar -xzf verifi_*_linux_amd64.tar.gz
sudo mv verifi /usr/local/bin/
sudo chmod +x /usr/local/bin/verifi

# For arm64:
curl -LO https://github.com/princespaghetti/verifi/releases/latest/download/verifi_*_linux_arm64.tar.gz
tar -xzf verifi_*_linux_arm64.tar.gz
sudo mv verifi /usr/local/bin/
sudo chmod +x /usr/local/bin/verifi
```

### Verify Installation

```bash
verifi version
```

## Getting Started

### Quick Start

```bash
# 1. Initialize the certificate store
verifi init

# 2. Add your corporate certificate
verifi cert add /path/to/corporate-ca.pem --name corporate

# 3. Configure your shell (add to ~/.zshrc or ~/.bashrc)
echo 'source ~/.verifi/env.sh' >> ~/.zshrc
source ~/.zshrc

# 4. Verify everything works
verifi status
curl -v https://your-internal-site.corp.com
npm ping
```

### Detailed Workflow

#### 1. Initialize the Certificate Store

```bash
verifi init
```

This creates `~/.verifi/` with:
- Mozilla's CA bundle (embedded offline)
- Directory structure for your certificates
- Environment configuration file (`env.sh`)
- Metadata for tracking changes

#### 2. Add Your Certificates

```bash
# Add from file
verifi cert add /path/to/cert.pem --name corporate

# Add from URL (if accessible)
curl https://internal.corp.com/ca.crt | verifi cert add --stdin --name internal

# Add expired certificate (for testing)
verifi cert add old-cert.pem --name legacy --force
```

#### 3. Configure Your Shell

Add verifi's environment file to your shell startup:

**For zsh** (macOS default):
```bash
echo 'source ~/.verifi/env.sh' >> ~/.zshrc
source ~/.zshrc
```

**For bash**:
```bash
echo 'source ~/.verifi/env.sh' >> ~/.bashrc
source ~/.bashrc
```

#### 4. Verify Configuration

```bash
# Check verifi status
verifi status

# List certificates
verifi cert list

# Test with real tools
curl -v https://internal.corp.com
npm config get registry
git ls-remote https://github.com/user/repo.git
pip install --index-url https://pypi.internal.corp.com somepackage
```

### Common Operations

#### Managing Certificates

```bash
# List all certificates
verifi cert list

# List only expired certificates
verifi cert list --expired

# Inspect certificate details
verifi cert inspect corporate

# Remove a certificate
verifi cert remove corporate

# Add certificate from clipboard
pbpaste | verifi cert add --stdin --name clipboard-cert
```

#### Mozilla CA Bundle Management

```bash
# Check current Mozilla bundle info
verifi bundle info

# Update to latest Mozilla bundle (optional)
verifi bundle update

# Reset to embedded version
verifi bundle reset
```

#### Diagnostics & Maintenance

```bash
# Run comprehensive diagnostics
verifi doctor

# Show detailed diagnostic info
verifi doctor --verbose

# Get JSON output for scripting
verifi status --json
verifi cert list --json

# Clean up temporary files
verifi clean

# Remove entire store (with confirmation)
verifi clean --full
```

#### Shell Completion

```bash
# For bash
verifi completion bash > /usr/local/etc/bash_completion.d/verifi

# For zsh
verifi completion zsh > "${fpath[1]}/_verifi"
```

### Integration Examples

#### Python (pip & requests)

```python
import requests

# Works automatically - verifi configures REQUESTS_CA_BUNDLE
response = requests.get('https://internal.api.corp.com')
print(response.status_code)
```

```bash
# pip also works automatically
pip install --index-url https://pypi.internal.corp.com package-name
```

#### Node.js (npm, yarn)

```bash
# All npm operations work automatically
npm install
npm publish
npm ping

# yarn works too
yarn install
yarn publish
```

#### Git

```bash
# Git operations work automatically
git clone https://internal.gitlab.corp.com/org/repo.git
git push origin main
```

#### AWS CLI

```bash
# AWS CLI respects AWS_CA_BUNDLE
aws s3 ls
aws ec2 describe-instances
```

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

verifi is production-ready and actively maintained. All core features are implemented and tested:

- ✅ Certificate store initialization and management
- ✅ User certificate addition, removal, and inspection
- ✅ Mozilla CA bundle updates and management
- ✅ Environment configuration generation
- ✅ Comprehensive diagnostics and status reporting
- ✅ Maintenance and cleanup operations
- ✅ Shell completion support (bash/zsh)
- ✅ Cross-platform releases (macOS/Linux)

See [PLAN.md](PLAN.md) for the complete implementation roadmap.

## Contributing

Contributions are welcome! Please see CONTRIBUTING.md for:

- Development environment setup
- Code structure and patterns
- Testing requirements
- Pull request process

## Support

For issues, questions, or feature requests, please use the GitHub issue tracker.

---

**Built for developers, by developers.**
