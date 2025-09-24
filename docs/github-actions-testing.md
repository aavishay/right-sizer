# GitHub Actions Testing with Act

This guide explains how to test the GitHub Actions workflows for the Right-Sizer project locally using `act`.

## Overview

The Right-Sizer project includes several GitHub Actions workflows:
- **Docker workflow** (`docker.yml`) - Builds and pushes Docker images
- **Helm workflow** (`helm.yml`) - Lints and publishes Helm charts
- **Release workflow** (`release.yml`) - Creates releases with binaries and Docker images
- **Test workflow** (`test.yml`) - Simple test workflow for validation

## Prerequisites

### Install act

On macOS:
```bash
brew install act
```

On Linux:
```bash
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
```

### Install Docker

Ensure Docker is running on your system as `act` uses Docker containers to simulate GitHub Actions runners.

### Verify Installation

```bash
act --version
docker info
```

## Setup

### 1. Configuration Files

The repository includes pre-configured files:

- `.actrc` - act configuration with M-series chip compatibility
- `.env.act` - Environment variables for testing (non-sensitive values only)

### 2. Test Script

Use the provided test script for easy workflow testing:

```bash
chmod +x scripts/test-github-actions.sh
./scripts/test-github-actions.sh help
```

## Available Commands

### Setup Configuration
```bash
./scripts/test-github-actions.sh setup
```

### List Available Workflows
```bash
./scripts/test-github-actions.sh list
```

### Run Local Validation (Fast)
```bash
./scripts/test-github-actions.sh local
```

### Test Individual Workflows

#### Helm Workflow (Recommended first test)
```bash
./scripts/test-github-actions.sh helm
```

#### Docker Workflow (Dry-run only)
```bash
./scripts/test-github-actions.sh docker
```

#### Go Build Workflow
```bash
./scripts/test-github-actions.sh go
```

### Run All Tests
```bash
./scripts/test-github-actions.sh all
```

## Manual act Commands

### Basic Usage

List all workflows:
```bash
act -l
```

Run specific job with dry-run:
```bash
act -j <job-name> -W .github/workflows/<workflow-file> --dryrun
```

Run specific job:
```bash
act -j <job-name> -W .github/workflows/<workflow-file>
```

### Workflow-Specific Commands

#### Test Workflow (Simple validation)
```bash
# Dry run
act -j test -W .github/workflows/test.yml --dryrun

# Full run
act -j test -W .github/workflows/test.yml
```

#### Helm Lint Workflow
```bash
# Dry run
act -j helm-lint -W .github/workflows/helm.yml --dryrun

# Full run
act -j helm-lint -W .github/workflows/helm.yml
```

#### Docker Build Workflow (Dry-run recommended)
```bash
# Dry run only (avoids pushing to registry)
act -j docker-build-and-push -W .github/workflows/docker.yml --dryrun
```

#### Release Workflow - Build Binaries
```bash
# Test Go binary building
act -j build-binaries -W .github/workflows/release.yml
```

## Configuration Details

### .actrc Configuration

```
--container-architecture linux/amd64
--platform ubuntu-latest=catthehacker/ubuntu:act-latest
--platform ubuntu-22.04=catthehacker/ubuntu:act-22.04
--platform ubuntu-20.04=catthehacker/ubuntu:act-20.04
-P ubuntu-latest=catthehacker/ubuntu:act-latest
-P ubuntu-22.04=catthehacker/ubuntu:act-22.04
-P ubuntu-20.04=catthehacker/ubuntu:act-20.04
--env-file .env.act
--artifact-server-path /tmp/artifacts
--verbose
```

### Environment Variables (.env.act)

```bash
# Test environment variables (safe for local testing)
DOCKER_USERNAME=test-user
DOCKER_PASSWORD=test-password
GITHUB_TOKEN=ghp_test_token_placeholder
REGISTRY=docker.io
IMAGE_NAME=aavishay/right-sizer
GO_VERSION=1.23
ACT_TEST=true
CI=true
GITHUB_ACTIONS=true
```

## Workflow Testing Strategies

### 1. Local Validation First

Always start with local validation before using act:

```bash
# Test Helm chart
helm lint helm/
helm template test helm/

# Test Go build and tests
cd go
go test ./...
go build -o ../right-sizer-test main.go
cd ..
```

### 2. Incremental Testing

Start with simpler workflows and gradually test more complex ones:

1. **Test workflow** - Basic validation
2. **Helm lint** - Chart validation
3. **Go build** - Binary compilation
4. **Docker build** - Container building (dry-run)

### 3. Dry-Run First

Always run dry-run mode first to validate workflow syntax and logic:

```bash
act -j <job-name> -W <workflow-file> --dryrun
```

## Common Issues and Solutions

### Issue: M-series Mac Compatibility

**Problem**: Act fails with architecture warnings on Apple M-series chips.

**Solution**: Use the `--container-architecture linux/amd64` flag (already configured in `.actrc`).

### Issue: Docker Login Failures

**Problem**: Docker login steps fail in local testing.

**Solution**: Use test credentials in `.env.act` and run Docker workflows in dry-run mode only.

### Issue: Missing Secrets

**Problem**: Workflows fail due to missing secrets.

**Solution**: Add placeholder values to `.env.act` for local testing.

### Issue: Network Connectivity

**Problem**: Action downloads fail due to network issues.

**Solution**: Ensure internet connectivity and try again. Act caches downloaded actions.

## Best Practices

### 1. Security

- Never commit real secrets to `.env.act`
- Use placeholder values for sensitive environment variables
- Test Docker workflows in dry-run mode only

### 2. Performance

- Run local validation tests first (faster than act)
- Use dry-run mode for initial validation
- Cache Docker images by running act tests multiple times

### 3. Debugging

- Use `--verbose` flag for detailed logging
- Check container logs if steps fail
- Validate individual commands locally before testing in act

## Troubleshooting

### View act logs
```bash
act -j <job-name> -W <workflow-file> --verbose
```

### Clean up containers
```bash
docker container prune
docker volume prune
```

### Reset act cache
```bash
rm -rf ~/.cache/act
```

## Integration with CI/CD

### Pre-commit Validation

Add to your development workflow:

```bash
# Before committing changes
./scripts/test-github-actions.sh local
```

### Pull Request Validation

Test specific workflows based on changes:

```bash
# If Helm charts changed
./scripts/test-github-actions.sh helm

# If Go code changed
./scripts/test-github-actions.sh go

# If Docker files changed
./scripts/test-github-actions.sh docker
```

## Workflow Descriptions

### docker.yml
- Builds multi-platform Docker images
- Pushes to Docker Hub registry
- Uses buildx for cross-platform builds
- **Local testing**: Dry-run only (avoid pushing to registry)

### helm.yml
- Lints Helm charts
- Publishes charts to GitHub Pages
- Creates Helm repository index
- **Local testing**: Safe to run fully

### release.yml
- Builds Go binaries for multiple platforms
- Creates GitHub releases
- Publishes Docker images and Helm charts
- **Local testing**: Test individual jobs only

### test.yml
- Simple validation workflow
- Tests Go build and Helm lint
- Safe for full local execution
- **Local testing**: Recommended starting point

## Example Testing Session

```bash
# 1. Setup
./scripts/test-github-actions.sh setup

# 2. Quick local validation
./scripts/test-github-actions.sh local

# 3. Test Helm workflow
./scripts/test-github-actions.sh helm

# 4. Test simple workflow
act -j test -W .github/workflows/test.yml --dryrun
act -j test -W .github/workflows/test.yml

# 5. Test Docker workflow (dry-run only)
act -j docker-build-and-push -W .github/workflows/docker.yml --dryrun
```

## References

- [act Documentation](https://github.com/nektos/act)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/)
- [Helm Documentation](https://helm.sh/docs/)
