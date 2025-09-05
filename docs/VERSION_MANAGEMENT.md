# 🏷️ Version Management Guide

This document explains the version management strategy and automation for the Right-Sizer project.

## 📊 Version Schema

### Semantic Versioning
Right-Sizer follows [Semantic Versioning 2.0.0](https://semver.org/):

```
MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

- **MAJOR**: Incompatible API changes
- **MINOR**: New functionality, backwards compatible
- **PATCH**: Bug fixes, backwards compatible
- **PRERELEASE**: Optional pre-release identifier (alpha, beta, rc)
- **BUILD**: Optional build metadata

### Examples
- `0.1.6` - Current stable version
- `1.0.0` - Major release
- `1.2.3-beta.1` - Beta pre-release
- `1.2.3-alpha.2+build.123` - Alpha with build metadata

## 🐳 Docker Image Tags

### Tagging Strategy
| Tag Format | Example | Description | Use Case |
|------------|---------|-------------|----------|
| `MAJOR.MINOR.PATCH` | `0.1.6` | Semantic version | Production deployments |
| `MAJOR.MINOR.PATCH-vBUILD` | `0.1.6-v123` | Version + build number | Specific build tracking |
| `vBUILD` | `v123` | Build number only | CI/CD references |
| `latest` | `latest` | Latest stable | Development/testing |
| `main` | `main` | Main branch build | Development |
| `sha-COMMIT` | `sha-a1b2c3d` | Specific commit | Debugging/rollbacks |

### Registry Locations
- **Primary**: `docker.io/aavishay/right-sizer:TAG`
- **OCI**: `registry-1.docker.io/aavishay/right-sizer:TAG`

## 📦 Helm Chart Versioning

### Chart Versioning
- **Chart Version**: Follows semantic versioning independently
- **App Version**: Matches the application version being packaged

```yaml
# helm/Chart.yaml
apiVersion: v2
name: right-sizer
version: 0.1.6      # Chart version
appVersion: "0.1.6" # Application version
```

### Chart Distribution
- **Repository**: `https://aavishay.github.io/right-sizer/charts`
- **OCI Registry**: `oci://registry-1.docker.io/aavishay/right-sizer`

## 🤖 Automated Version Management

### 1. Manual Version Update Workflow

Use the GitHub Actions workflow to update versions across all files:

```bash
# Go to GitHub → Actions → "Update Versions" → "Run workflow"
# Enter new version: 0.1.7
# Choose: Create Pull Request = true
```

**What it does:**
- ✅ Updates all README files with new version references
- ✅ Updates Docker image tags in examples
- ✅ Updates Helm chart version and appVersion
- ✅ Updates installation examples
- ✅ Updates version badges
- ✅ Creates a Pull Request for review

### 2. Release Process

After merging the version update PR:

```bash
# Create and push git tag
git tag v0.1.7
git push origin v0.1.7
```

**This automatically triggers:**
- 🐳 Multi-arch Docker image builds
- 📦 Helm chart packaging and publishing
- 📋 GitHub release creation with binaries
- 🌍 Distribution to all registries

### 3. Script Usage

You can also run the version update script locally:

```bash
# Update to new version
./scripts/update-versions.sh 0.1.7

# Review changes
git diff

# Commit and push
git add .
git commit -m "chore: update version to 0.1.7"
git push origin main
```

## 📁 Files Updated by Version Script

The automated version update affects these files:

| File | Updates |
|------|---------|
| `README.md` | Version badges, Docker tags, Helm versions, examples |
| `helm/README.md` | Version badges, installation examples |
| `helm/Chart.yaml` | Chart version and appVersion |
| `helm/values.yaml` | Comments and default tag references |
| `docs/CHANGELOG.md` | New version entry (if exists) |

## 🔍 Version Reference Patterns

### README Files
```markdown
[![Version](https://img.shields.io/badge/Version-0.1.6-green.svg)]
docker pull aavishay/right-sizer:0.1.6
helm install right-sizer right-sizer/right-sizer --version 0.1.6
--set image.tag=0.1.6
targetRevision: 0.1.6
```

### Helm Chart
```yaml
version: 0.1.6
appVersion: "0.1.6"
```

### Docker Commands
```bash
docker pull aavishay/right-sizer:0.1.6
docker pull aavishay/right-sizer:0.1.6-v123
docker pull aavishay/right-sizer:latest
```

## 🚀 Release Checklist

### Before Release
- [ ] All tests pass
- [ ] Documentation is updated
- [ ] Breaking changes are documented
- [ ] Version follows semantic versioning rules

### Version Update Process
1. [ ] Run "Update Versions" workflow or local script
2. [ ] Review generated Pull Request
3. [ ] Merge PR after approval
4. [ ] Create and push git tag: `git tag vX.Y.Z && git push origin vX.Y.Z`

### After Release
- [ ] Verify Docker images are published
- [ ] Verify Helm chart is available
- [ ] Verify GitHub release is created
- [ ] Update any external documentation
- [ ] Announce release if significant

## 🔧 Troubleshooting

### Version Update Script Fails
```bash
# Check script permissions
chmod +x scripts/update-versions.sh

# Run with verbose output
bash -x scripts/update-versions.sh 0.1.7

# Restore from backup if needed
cp version-backup-*/README.md ./README.md
```

### Missing Version References
```bash
# Find all version references
grep -r "0.1.6" . --exclude-dir=.git

# Check specific patterns
grep -r "Version-[0-9]" . --include="*.md"
grep -r "aavishay/right-sizer:[0-9]" . --include="*.md"
```

### Docker Tags Not Building
- Check GitHub Actions workflows
- Verify Docker Hub credentials
- Check tag format (must be `v*.*.*`)

### Helm Chart Not Publishing
- Check `gh-pages` branch exists
- Verify GitHub Pages is enabled
- Check Helm chart syntax: `helm lint helm/`

## 📚 Best Practices

1. **Always test locally** before creating releases
2. **Use Pull Requests** for version updates (allows review)
3. **Follow semantic versioning** strictly
4. **Tag releases immediately** after merging version updates
5. **Monitor CI/CD pipelines** after tagging
6. **Keep backups** during version updates (script creates them automatically)
7. **Document breaking changes** in release notes
8. **Test installation** from published artifacts

## 🔗 Related Links

- [Semantic Versioning](https://semver.org/)
- [Docker Tag Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Helm Chart Versioning](https://helm.sh/docs/topics/charts/#the-chartyaml-file)
- [GitHub Actions Workflows](../.github/workflows/)
- [Version Update Script](../scripts/update-versions.sh)