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

## Installation

### macOS

**Homebrew**:
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

## Quick Start

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

- [Usage Guide](USAGE.md) - Detailed workflows, common operations, and integration examples
- [Design Principles](PRINCIPLES.md) - Core principles and architecture
- [Contributing](CONTRIBUTING.md) - Development setup and contribution guidelines
- [Implementation Plan](PLAN.md) - Complete implementation roadmap

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development environment setup
- Code structure and patterns
- Testing requirements
- Pull request process

## Support

For issues, questions, or feature requests, please use the [GitHub issue tracker](https://github.com/princespaghetti/verifi/issues).

## License

MIT License - see [LICENSE](LICENSE) for details.
