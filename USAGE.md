# Usage Guide

## Detailed Workflow

### 1. Initialize the Certificate Store

```bash
verifi init
```

This creates `~/.verifi/` with:
- Mozilla's CA bundle (embedded offline)
- Directory structure for your certificates
- Environment configuration file (`env.sh`)
- Metadata for tracking changes

### 2. Add Your Certificates

```bash
# Add from file
verifi cert add /path/to/cert.pem --name corporate

# Add from URL (if accessible)
curl https://internal.corp.com/ca.crt | verifi cert add --stdin --name internal

# Add expired certificate (for testing)
verifi cert add old-cert.pem --name legacy --force
```

### 3. Configure Your Shell

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

### 4. Verify Configuration

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

## Common Operations

### Managing Certificates

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

### Mozilla CA Bundle Management

```bash
# Check current Mozilla bundle info
verifi bundle info

# Update to latest Mozilla bundle (optional)
verifi bundle update

# Reset to embedded version
verifi bundle reset
```

### Diagnostics & Maintenance

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

### Shell Completion

```bash
# For bash
verifi completion bash > /usr/local/etc/bash_completion.d/verifi

# For zsh
verifi completion zsh > "${fpath[1]}/_verifi"
```

## Integration Examples

### Python (pip & requests)

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

### Node.js (npm, yarn)

```bash
# All npm operations work automatically
npm install
npm publish
npm ping

# yarn works too
yarn install
yarn publish
```

### Git

```bash
# Git operations work automatically
git clone https://internal.gitlab.corp.com/org/repo.git
git push origin main
```

### AWS CLI

```bash
# AWS CLI respects AWS_CA_BUNDLE
aws s3 ls
aws ec2 describe-instances
```
