# Right-Sizer Scripts

This directory contains utility scripts and automation tools for the Right-Sizer project.

## 📁 Available Scripts

### Core Testing & CI/CD

#### `test-github-actions.sh`
**Purpose**: Local testing of GitHub Actions workflows using `act`

**Description**: Comprehensive script for validating CI/CD workflows locally before pushing changes to GitHub. Provides both automated testing and manual validation options.

**Usage**:
```bash
# Make executable
chmod +x scripts/test-github-actions.sh

# Show help and available commands
./scripts/test-github-actions.sh help

# Setup act configuration
./scripts/test-github-actions.sh setup

# Run local validation (fast, no Docker)
./scripts/test-github-actions.sh local

# Test specific workflows
./scripts/test-github-actions.sh helm
./scripts/test-github-actions.sh docker
./scripts/test-github-actions.sh go

# Run comprehensive tests
./scripts/test-github-actions.sh all
```

**Features**:
- ✅ Interactive testing with progress feedback
- ✅ Safety checks to prevent accidental deployments
- ✅ Apple M-series chip compatibility
- ✅ Comprehensive error handling and logging
- ✅ Local validation without Docker overhead
- ✅ Incremental testing from simple to complex workflows

**Requirements**:
- `act` (GitHub Actions local runner)
- `docker` (Container runtime)
- `helm` (For chart validation)
- `go` (For Go build testing)

#### `test.sh`
**Purpose**: Basic Go testing and validation

**Usage**:
```bash
./scripts/test.sh
```

#### `test-all.sh`
**Purpose**: Comprehensive test suite execution

**Usage**:
```bash
./scripts/test-all.sh
```

#### `quick-test.sh`
**Purpose**: Fast validation for development workflow

**Usage**:
```bash
./scripts/quick-test.sh
```

#### `check-coverage.sh`
**Purpose**: Generate and analyze test coverage reports

**Usage**:
```bash
./scripts/check-coverage.sh
```

### Deployment & Installation

#### `quick-deploy.sh`
**Purpose**: Rapid deployment for development and testing

**Usage**:
```bash
./scripts/quick-deploy.sh
```

#### `minimal-deploy.sh`
**Purpose**: Minimal deployment with essential components only

**Usage**:
```bash
./scripts/minimal-deploy.sh
```

#### `deploy-no-metrics.sh`
**Purpose**: Deploy without metrics dependencies

**Usage**:
```bash
./scripts/deploy-no-metrics.sh
```

#### `deploy-rbac.sh`
**Purpose**: Deploy with RBAC configuration

**Usage**:
```bash
./scripts/deploy-rbac.sh
```

#### `verify-deployment.sh`
**Purpose**: Validate deployment status and health

**Usage**:
```bash
./scripts/verify-deployment.sh
```

#### `monitor-deployment.sh`
**Purpose**: Monitor deployment progress and logs

**Usage**:
```bash
./scripts/monitor-deployment.sh
```

### Helm & Packaging

#### `helm-package.sh`
**Purpose**: Package Helm charts for distribution

**Usage**:
```bash
./scripts/helm-package.sh
```

#### `publish-helm-chart.sh`
**Purpose**: Publish Helm charts to repository

**Usage**:
```bash
./scripts/publish-helm-chart.sh
```

### Release & Versioning

#### `bump-version.sh`
**Purpose**: Increment version numbers across the project

**Usage**:
```bash
./scripts/bump-version.sh [major|minor|patch]
```

#### `create-release.sh`
**Purpose**: Create and publish project releases

**Usage**:
```bash
./scripts/create-release.sh
```

#### `update-versions.sh`
**Purpose**: Update version references in all files

**Usage**:
```bash
./scripts/update-versions.sh
```

### Validation & Compliance

#### `check-k8s-compliance.sh`
**Purpose**: Validate Kubernetes resource compliance

**Usage**:
```bash
./scripts/check-k8s-compliance.sh
```

#### `test-metrics.sh`
**Purpose**: Test metrics collection and validation

**Usage**:
```bash
./scripts/test-metrics.sh
```

### Build & Development

#### `make.sh`
**Purpose**: Build automation and compilation

**Usage**:
```bash
./scripts/make.sh [target]
```

## 🚀 Quick Start

### First Time Setup
```bash
# 1. Navigate to project root
cd right-sizer

# 2. Setup testing environment
./scripts/test-github-actions.sh setup

# 3. Run quick validation
./scripts/test-github-actions.sh local
```

### Regular Development Workflow
```bash
# Before committing changes
./scripts/test-github-actions.sh local

# Test specific workflow based on changes
./scripts/test-github-actions.sh helm    # If Helm charts changed
./scripts/test-github-actions.sh go     # If Go code changed
./scripts/test-github-actions.sh docker # If Dockerfile changed
```

## 📋 Script Details

### test-github-actions.sh Commands

### Script Categories

| Category | Scripts | Purpose |
|----------|---------|---------|
| **Testing** | `test-github-actions.sh`, `test.sh`, `test-all.sh`, `quick-test.sh`, `check-coverage.sh` | Validation and testing |
| **Deployment** | `quick-deploy.sh`, `minimal-deploy.sh`, `deploy-no-metrics.sh`, `deploy-rbac.sh` | Installation and deployment |
| **Monitoring** | `verify-deployment.sh`, `monitor-deployment.sh`, `test-metrics.sh` | Health and monitoring |
| **Packaging** | `helm-package.sh`, `publish-helm-chart.sh` | Distribution and packaging |
| **Release** | `bump-version.sh`, `create-release.sh`, `update-versions.sh` | Version and release management |
| **Compliance** | `check-k8s-compliance.sh` | Validation and compliance |
| **Build** | `make.sh` | Build automation |

### test-github-actions.sh Commands

| Command | Description | Duration | Safety |
|---------|-------------|----------|--------|
| `help` | Show usage information | Instant | ✅ Safe |
| `setup` | Configure act and Docker | ~30s | ✅ Safe |
| `list` | List available workflows | ~5s | ✅ Safe |
| `local` | Run local validation tests | ~30s | ✅ Safe |
| `helm` | Test Helm workflow | ~2-3min | ✅ Safe |
| `docker` | Test Docker workflow (dry-run) | ~3-4min | ✅ Safe |
| `go` | Test Go build workflow | ~2-3min | ✅ Safe |
| `all` | Run comprehensive test suite | ~5-10min | ⚠️ Interactive |

### Configuration Files Created

The script automatically creates configuration files:
- `.actrc` - act runner configuration
- `.env.act` - Environment variables for testing

### Safety Features

- **Dry-run first policy** - Always validates before execution
- **Test credentials** - No real secrets used in local testing
- **Registry protection** - Docker workflows run in dry-run mode only
- **Interactive prompts** - User confirmation for potentially lengthy operations
- **Error handling** - Graceful failure with helpful error messages

## 🔧 Troubleshooting

### Common Issues

**Issue**: `act: command not found`
```bash
# Install act
brew install act  # macOS
# or
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash  # Linux
```

**Issue**: Docker connection errors
```bash
# Ensure Docker is running
docker info
# Restart Docker if needed
```

**Issue**: M-series Mac compatibility
```bash
# Configuration automatically handles this with:
--container-architecture linux/amd64
```

**Issue**: Network connectivity for action downloads
```bash
# Check internet connection and retry
# act caches downloaded actions for faster subsequent runs
```

### Debug Mode

Enable verbose logging for troubleshooting:
```bash
# Run with debug information
./scripts/test-github-actions.sh local 2>&1 | tee debug.log
```

## 🛠️ Development

### Script Usage Patterns

#### Development Workflow
```bash
# 1. Quick validation during development
./scripts/quick-test.sh

# 2. Full testing before commit
./scripts/test-all.sh

# 3. Local deployment for testing
./scripts/quick-deploy.sh

# 4. Verify deployment health
./scripts/verify-deployment.sh
```

#### Release Workflow
```bash
# 1. Bump version
./scripts/bump-version.sh patch

# 2. Update all version references
./scripts/update-versions.sh

# 3. Package Helm charts
./scripts/helm-package.sh

# 4. Create release
./scripts/create-release.sh
```

#### CI/CD Testing
```bash
# 1. Setup local testing
./scripts/test-github-actions.sh setup

# 2. Run local validation
./scripts/test-github-actions.sh local

# 3. Test workflows
./scripts/test-github-actions.sh all
```

## 🛠️ Development

### Adding New Scripts

When adding new scripts to this directory:

1. **Make executable**: `chmod +x scripts/new-script.sh`
2. **Add shebang**: `#!/bin/bash` at the top
3. **Add error handling**: Use `set -e` for fail-fast behavior
4. **Include help**: Provide usage information with `--help`
5. **Update this README**: Document the new script
6. **Test thoroughly**: Ensure script works in different environments

### Script Standards

- Use bash for shell scripts
- Include comprehensive error handling
- Provide clear usage instructions
- Use consistent output formatting
- Include safety checks for destructive operations
- Document all dependencies and requirements

## 📖 Related Documentation

- **[GitHub Actions Testing Guide](../docs/github-actions-testing.md)** - Comprehensive act usage guide
- **[Act Testing Summary](../docs/act-testing-summary.md)** - Test results and validation status
- **[Installation Guide](../docs/installation-guide.md)** - Project setup instructions

## 🤝 Contributing

When contributing new scripts or improving existing ones:

1. Follow the established patterns and conventions
2. Include comprehensive error handling and user feedback
3. Test on multiple environments (macOS, Linux)
4. Update documentation in this README
5. Add examples and usage instructions
6. Consider safety and security implications

---

*For more information, see the [main documentation](../docs/README.md)*
