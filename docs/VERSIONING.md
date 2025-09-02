# Versioning Strategy

## Overview

Right-Sizer follows [Semantic Versioning 2.0.0](https://semver.org/) (SemVer) for all releases. This document describes our versioning strategy, tools, and processes for managing versions across all project components.

## Version Format

```
MAJOR.MINOR.PATCH[-PRERELEASE]
```

- **MAJOR**: Incremented for incompatible API changes
- **MINOR**: Incremented for backwards-compatible functionality additions
- **PATCH**: Incremented for backwards-compatible bug fixes
- **PRERELEASE**: Optional identifier for pre-release versions (alpha, beta, rc)

### Examples
- `1.0.0` - First stable release
- `1.1.0` - New feature added
- `1.1.1` - Bug fix
- `2.0.0` - Breaking change
- `1.2.0-alpha.1` - Alpha pre-release
- `1.2.0-beta.2` - Beta pre-release
- `1.2.0-rc.1` - Release candidate

## Version Management

### Central Version File

The project version is centrally managed in the `VERSION` file at the root of the repository. This ensures consistency across all components.

```bash
$ cat VERSION
0.1.0
```

### Components Using Version

1. **Docker Images**: Tagged with the version from `VERSION` file
2. **Helm Chart**: Both `version` and `appVersion` in `Chart.yaml`
3. **Go Binary**: Version embedded at build time via ldflags
4. **Git Tags**: Prefixed with 'v' (e.g., `v1.0.0`)
5. **Release Artifacts**: Named with version suffix

## Tools and Commands

### Using the Makefile

The project includes a comprehensive Makefile for version management:

```bash
# Display current version
make version

# Bump versions
make version-bump-patch  # 0.1.0 -> 0.1.1
make version-bump-minor  # 0.1.0 -> 0.2.0
make version-bump-major  # 0.1.0 -> 1.0.0

# Update version in all files
make version-update

# Build with version
make build              # Build for current platform
make docker-build       # Build Docker image with version
make helm-package       # Package Helm chart with version

# Full release preparation
make release-prepare    # Prepare for release (test, lint, update versions)
make release-tag        # Create git tag for current version
```

### Using the Version Script

For more advanced version management, use the dedicated script:

```bash
# Show current version
./scripts/version/version.sh current

# Bump versions
./scripts/version/version.sh bump patch        # 0.1.0 -> 0.1.1
./scripts/version/version.sh bump minor        # 0.1.0 -> 0.2.0
./scripts/version/version.sh bump major        # 0.1.0 -> 1.0.0

# Pre-release versions
./scripts/version/version.sh bump prerelease alpha  # 0.1.0 -> 0.1.1-alpha.1
./scripts/version/version.sh bump prerelease beta   # 0.1.0 -> 0.1.1-beta.1
./scripts/version/version.sh bump prerelease rc     # 0.1.0 -> 0.1.1-rc.1

# Set specific version
./scripts/version/version.sh set 1.2.3

# Create git tag
./scripts/version/version.sh tag
./scripts/version/version.sh tag --push    # Also push to remote

# Full release cycle
./scripts/version/version.sh release patch --push

# Check version consistency
./scripts/version/version.sh check
```

## Release Process

### 1. Prepare Release

```bash
# 1. Ensure you're on main branch with latest changes
git checkout main
git pull origin main

# 2. Decide on version bump type (patch/minor/major)
# For example, for a minor release:
make version-bump-minor

# 3. Run release preparation
make release-prepare

# This will:
# - Update VERSION file
# - Update helm/Chart.yaml
# - Run tests
# - Run linters
# - Format code
```

### 2. Review Changes

```bash
# Review the version changes
git diff

# Check version consistency
./scripts/version/version.sh check
```

### 3. Commit and Tag

```bash
# Commit version changes
git add -A
git commit -m "chore: bump version to $(cat VERSION)"

# Create annotated tag
git tag -a "v$(cat VERSION)" -m "Release v$(cat VERSION)"
```

### 4. Build and Test

```bash
# Build all artifacts
make build-all
make docker-build
make helm-package

# Run final tests
make test
```

### 5. Push and Publish

```bash
# Push commits and tags
git push origin main
git push origin "v$(cat VERSION)"

# The GitHub Actions workflow will automatically:
# - Build multi-platform Docker images
# - Push to Docker Hub
# - Create GitHub release
# - Upload release artifacts
# - Update Helm repository
```

## Version Locations

The following files contain version information and are automatically updated:

| File | Field | Description |
|------|-------|-------------|
| `VERSION` | - | Master version file |
| `helm/Chart.yaml` | `version` | Helm chart version |
| `helm/Chart.yaml` | `appVersion` | Application version |
| `helm/values.yaml` | `image.tag` | Default Docker image tag (empty = use appVersion) |

## Docker Image Tagging

Docker images are tagged with multiple identifiers for flexibility:

```bash
# Semantic version tags
aavishay/right-sizer:1.0.0
aavishay/right-sizer:v1.0.0

# Major/minor version tags (automatically updated)
aavishay/right-sizer:1.0
aavishay/right-sizer:1

# Latest tag (for main branch releases)
aavishay/right-sizer:latest

# Git commit SHA (for tracking)
aavishay/right-sizer:sha-abc1234

# Build number (CI/CD specific)
aavishay/right-sizer:v42
```

## Helm Chart Versioning

The Helm chart follows the same version as the application for simplicity:

```yaml
# helm/Chart.yaml
apiVersion: v2
name: right-sizer
version: 1.0.0      # Chart version (same as app)
appVersion: "1.0.0" # Application version
```

When installing the chart, the version is automatically used:

```bash
# Install specific version
helm install right-sizer ./helm --set image.tag=1.0.0

# Install using chart's appVersion (default)
helm install right-sizer ./helm
```

## Pre-release Versions

Pre-releases are useful for testing before official releases:

### Alpha Releases
- For internal testing
- May have incomplete features
- Not recommended for production

```bash
./scripts/version/version.sh bump prerelease alpha
# Creates: 1.0.1-alpha.1, 1.0.1-alpha.2, etc.
```

### Beta Releases
- For external testing
- Feature complete but may have bugs
- Can be used in staging environments

```bash
./scripts/version/version.sh bump prerelease beta
# Creates: 1.0.1-beta.1, 1.0.1-beta.2, etc.
```

### Release Candidates
- Final testing before release
- Should be production ready
- Used for final validation

```bash
./scripts/version/version.sh bump prerelease rc
# Creates: 1.0.1-rc.1, 1.0.1-rc.2, etc.
```

## Version Compatibility Matrix

| Right-Sizer Version | Kubernetes Version | Helm Version | Go Version |
|--------------------|--------------------|--------------|------------|
| 0.1.x | 1.33+ | 3.0+ | 1.24 |
| 1.0.x | 1.33+ | 3.0+ | 1.24 |

## Best Practices

1. **Always bump version** before creating a release
2. **Use semantic versioning** strictly
3. **Test thoroughly** before major version bumps
4. **Document breaking changes** in CHANGELOG.md
5. **Tag all releases** in git
6. **Keep VERSION file** as single source of truth
7. **Automate version updates** using provided tools
8. **Verify version consistency** before releases

## Troubleshooting

### Version Mismatch

If versions are inconsistent across files:

```bash
# Check current state
./scripts/version/version.sh check

# Fix by setting version explicitly
./scripts/version/version.sh set $(cat VERSION)

# Verify fix
./scripts/version/version.sh check
```

### Failed Release

If a release fails after tagging:

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag (if pushed)
git push origin :refs/tags/v1.0.0

# Fix issues and retry
make release-prepare
```

### Docker Image Tag Issues

If Docker image has wrong tag:

```bash
# Rebuild with correct version
make docker-build DOCKER_TAG=$(cat VERSION)

# Or manually
docker build --build-arg VERSION=$(cat VERSION) \
  -t aavishay/right-sizer:$(cat VERSION) .
```

## FAQ

**Q: When should I bump the major version?**
A: When making backwards-incompatible changes to:
- CRD schemas
- API contracts
- Configuration format
- Behavioral changes that could break existing deployments

**Q: Can I have different versions for chart and app?**
A: While possible, we maintain the same version for simplicity. The chart version changes even for documentation-only updates.

**Q: How do I handle hotfixes?**
A: Create a branch from the release tag, apply fix, bump patch version, and follow normal release process:
```bash
git checkout -b hotfix/1.0.1 v1.0.0
# Apply fixes
./scripts/version/version.sh bump patch
# Continue with release process
```

**Q: What about nightly builds?**
A: Nightly builds use the commit SHA or timestamp:
```bash
# Example nightly tag
aavishay/right-sizer:nightly-20240102
aavishay/right-sizer:main-abc1234
```

## References

- [Semantic Versioning Specification](https://semver.org/)
- [Helm Chart Best Practices](https://helm.sh/docs/chart_best_practices/conventions/)
- [Docker Tagging Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Go Module Versioning](https://go.dev/doc/modules/version-numbers)