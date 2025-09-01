# GitHub Actions Testing Documentation

## Overview

This document provides a comprehensive guide for testing all GitHub Actions workflows in the right-sizer project. All workflows have been configured to use Go 1.24 consistently.

## Workflow Files

The project includes three main GitHub Actions workflows:

### 1. Docker Build and Push (`docker-build.yml`)
- **Triggers**: Push to main, pull requests, tags (v*.*.*), manual dispatch
- **Purpose**: Builds and pushes multi-platform Docker images
- **Platforms**: linux/amd64, linux/arm64
- **Registry**: GitHub Container Registry (ghcr.io)
- **Features**:
  - Multi-platform builds with QEMU
  - Vulnerability scanning with Trivy
  - SBOM generation
  - Fallback builds for Alpine failures
  - Image testing and validation

### 2. Test Workflow (`test.yml`)
- **Triggers**: Push to main/develop/feature/release branches, pull requests, manual dispatch
- **Go Version**: 1.24 (single version, no matrix)
- **Jobs**:
  - **test**: Unit tests with race detection and coverage
  - **integration-test**: Kubernetes integration tests with Kind
  - **benchmark**: Performance benchmarks
  - **lint**: Code quality with golangci-lint
  - **security-scan**: Security scanning with gosec and govulncheck
  - **all-tests-passed**: Final validation job
- **Features**:
  - Code coverage reporting to Codecov
  - Test artifacts upload
  - Comprehensive security scanning
  - SARIF report generation

### 3. Release Workflow (`release.yml`)
- **Triggers**: Version tags (v*.*.*), manual dispatch with tag input
- **Go Version**: 1.24
- **Jobs**:
  - **build-binaries**: Cross-platform binary builds (Linux, macOS, Windows)
  - **build-docker**: Multi-platform Docker image publishing
  - **build-helm-chart**: Helm chart packaging
  - **create-release**: GitHub release creation with artifacts
  - **publish-helm-chart**: Helm chart publishing to GitHub Pages
- **Platforms**: 
  - linux/amd64, linux/arm64
  - darwin/amd64, darwin/arm64
  - windows/amd64

## Testing Tools

### 1. Local Testing with act

The project includes a comprehensive test script at `scripts/test-github-actions.sh` that uses [act](https://github.com/nektos/act) to run workflows locally.

#### Installation
```bash
# macOS
brew install act

# Linux
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Windows
choco install act-cli
```

#### Usage
```bash
# Test all workflows
./scripts/test-github-actions.sh all

# Test specific workflow
./scripts/test-github-actions.sh docker-build
./scripts/test-github-actions.sh test
./scripts/test-github-actions.sh release

# List available workflows
./scripts/test-github-actions.sh list

# Dry run mode
DRY_RUN=true ./scripts/test-github-actions.sh all

# Verbose output
VERBOSE=true ./scripts/test-github-actions.sh test
```

#### Configuration Files
- `.env.act`: Environment variables for act
- `.secrets.act`: Secrets for act (create with actual tokens if needed)
- `.github/events/push.json`: Sample push event for testing

### 2. Workflow Validation

The project includes validation scripts to ensure workflow correctness:

#### Go Version Validation (`scripts/validate-go-version.sh`)
Validates that all workflows use the correct Go version (1.24).

```bash
./scripts/validate-go-version.sh
```

#### Workflow Structure Validation (`scripts/validate-workflows.sh`)
Comprehensive validation including:
- YAML syntax checking
- Workflow structure validation
- Go version consistency
- Common issues detection
- Best practices enforcement

```bash
./scripts/validate-workflows.sh
```

## Go Version Configuration

All workflows and build files are configured to use **Go 1.24**:

- **Environment Variable**: `GO_VERSION: "1.24"`
- **Matrix Configuration**: `go-version: ["1.24"]`
- **go.mod**: `go 1.24`

### Verification
```bash
# Check Go versions in workflows
grep -E "GO_VERSION|go-version" .github/workflows/*.yml

# Check go.mod
grep "^go " go/go.mod
```

## Common Testing Scenarios

### 1. Pull Request Testing
```bash
# Simulate PR event
act pull_request -W .github/workflows/test.yml
```

### 2. Release Testing
```bash
# Test release workflow (dry-run recommended)
DRY_RUN=true act push -W .github/workflows/release.yml -e .github/events/tag.json
```

### 3. Docker Build Testing
```bash
# Test Docker build locally
act push -W .github/workflows/docker-build.yml -j build
```

## Troubleshooting

### Common Issues

1. **Authentication Errors with act**
   - Issue: `authentication required: Invalid username or token`
   - Solution: This is normal for act when accessing GitHub Actions. Use `--pull=false` to use cached actions.

2. **Platform Architecture Issues**
   - Issue: Apple Silicon compatibility warnings
   - Solution: Use `--container-architecture linux/amd64` flag

3. **Docker Socket Errors**
   - Issue: Cannot connect to Docker daemon
   - Solution: Ensure Docker Desktop is running

4. **Go Version Mismatches**
   - Issue: Different Go versions in workflows
   - Solution: Run `scripts/validate-go-version.sh` to identify and fix inconsistencies

### Debugging Commands

```bash
# List all jobs in workflows
act -l

# Run specific job with verbose output
act -j test -v

# Check workflow syntax
yamllint .github/workflows/*.yml

# Validate with actionlint
actionlint .github/workflows/*.yml
```

## Best Practices

1. **Always Test Locally First**
   - Use act to test workflows before pushing
   - Run validation scripts to catch issues early

2. **Version Consistency**
   - Maintain consistent Go version across all workflows
   - Keep go.mod in sync with workflow configurations

3. **Security**
   - Never commit real secrets to `.secrets.act`
   - Use GitHub Secrets for sensitive data
   - Run security scans regularly

4. **Performance**
   - Set appropriate timeout-minutes for jobs
   - Use matrix strategies judiciously
   - Cache dependencies when possible

5. **Documentation**
   - Document workflow triggers and purposes
   - Keep this guide updated with workflow changes
   - Add comments in complex workflow sections

## Workflow Dependencies

### Required GitHub Actions
- `actions/checkout@v4`
- `actions/setup-go@v5`
- `docker/setup-qemu-action@v3`
- `docker/setup-buildx-action@v3`
- `docker/login-action@v3`
- `docker/build-push-action@v5`
- `docker/metadata-action@v5`
- `actions/upload-artifact@v4`
- `actions/download-artifact@v4`
- `codecov/codecov-action@v4`
- `golangci/golangci-lint-action@v6`
- `github/codeql-action/upload-sarif@v3`
- `aquasecurity/trivy-action@master`
- `anchore/sbom-action@v0`
- `softprops/action-gh-release@v2`
- `helm/kind-action@v1`
- `azure/setup-kubectl@v4`
- `azure/setup-helm@v4`

### External Services
- GitHub Container Registry (ghcr.io)
- Docker Hub (optional)
- Codecov (optional)

## Continuous Improvement

### Monitoring
- Review workflow run times regularly
- Monitor failure rates and patterns
- Track resource usage

### Updates
- Keep actions updated to latest versions
- Review and update Go version as needed
- Update security scanning tools

### Testing Coverage
- Ensure all workflow paths are tested
- Add integration tests for new features
- Maintain high code coverage (>70%)

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [act Documentation](https://github.com/nektos/act)
- [actionlint](https://github.com/rhysd/actionlint)
- [Go 1.24 Release Notes](https://go.dev/doc/go1.24)
- [Docker Build Action](https://github.com/docker/build-push-action)
- [Helm Chart Testing](https://helm.sh/docs/topics/chart_tests/)

## Support

For issues or questions about GitHub Actions testing:
1. Check the troubleshooting section above
2. Review workflow logs in GitHub Actions tab
3. Run local validation scripts
4. Test with act in verbose mode
5. Consult the project maintainers

---

*Last updated: December 2024*  
*Go Version: 1.24*  
*Project: right-sizer*