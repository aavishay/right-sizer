# Release Checklist - v0.1.16

## Pre-Release Verification ✅

### Code Quality
- [x] All tests passing (with minor known timing issues)
- [x] Code formatted and linted
- [x] No dashboard/frontend references remaining
- [x] Documentation updated
- [x] CHANGELOG.md updated
- [x] Release notes prepared

### Version Updates
- [x] VERSION file updated to 0.1.16
- [x] helm/Chart.yaml version updated to 0.1.16
- [x] helm/Chart.yaml appVersion updated to 0.1.16
- [x] Git tag v0.1.16 created

### Build Artifacts
- [x] Helm chart package created: `dist/right-sizer-0.1.16.tgz`
- [ ] Docker images built and tested
- [ ] Binary builds for multiple platforms

## Release Process 🚀

### 1. Git Operations
```bash
# Push commits to main branch
git push origin main

# Push release tag
git push origin v0.1.16
```

### 2. GitHub Release
- [ ] Go to https://github.com/aavishay/right-sizer/releases
- [ ] Click "Draft a new release"
- [ ] Select tag: v0.1.16
- [ ] Title: "Right-Sizer v0.1.16 - Major Cleanup Release"
- [ ] Copy content from RELEASE_NOTES_v0.1.16.md
- [ ] Attach artifacts:
  - [ ] `dist/right-sizer-0.1.16.tgz` (Helm chart)
  - [ ] Binary builds (if available)
- [ ] Set as latest release
- [ ] Publish release

### 3. Docker Images
```bash
# Build multi-platform images
docker buildx create --use --name right-sizer-builder
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/aavishay/right-sizer:0.1.16 \
  -t ghcr.io/aavishay/right-sizer:latest \
  --push .

# Alternative: Build and push separately
docker build -t ghcr.io/aavishay/right-sizer:0.1.16 .
docker push ghcr.io/aavishay/right-sizer:0.1.16
docker tag ghcr.io/aavishay/right-sizer:0.1.16 ghcr.io/aavishay/right-sizer:latest
docker push ghcr.io/aavishay/right-sizer:latest
```

### 4. Helm Repository
```bash
# If using GitHub Pages for Helm repo
cd dist
helm repo index . --url https://github.com/aavishay/right-sizer/releases/download/v0.1.16
# Upload index.yaml to GitHub Pages or release assets
```

### 5. Documentation Updates
- [ ] Update main README.md with new version if needed
- [ ] Update installation instructions with new version
- [ ] Update any version references in documentation
- [ ] Verify all links in documentation work

## Post-Release Tasks 📋

### Verification
- [ ] Docker image pulls successfully
- [ ] Helm chart installs correctly
- [ ] Basic smoke test on Kubernetes cluster
- [ ] Metrics API endpoints respond correctly

### Communication
- [ ] Update project website (if applicable)
- [ ] Post release announcement (if applicable)
- [ ] Update any dependent projects
- [ ] Notify users of breaking changes

### Monitoring
- [ ] Monitor GitHub issues for bug reports
- [ ] Check CI/CD pipelines are green
- [ ] Verify automated tests pass with new release

## Rollback Plan 🔄

If issues are discovered:
1. Delete the GitHub release (keep tag for history)
2. Fix issues in new commits
3. Create new patch version (0.1.17)
4. Re-run release process

## Testing Commands 🧪

### Quick Deployment Test
```bash
# Using Helm
helm install test-release dist/right-sizer-0.1.16.tgz \
  --create-namespace \
  --namespace right-sizer-test

# Verify deployment
kubectl get all -n right-sizer-test

# Check logs
kubectl logs -n right-sizer-test deployment/test-release-right-sizer

# Cleanup
helm uninstall test-release -n right-sizer-test
kubectl delete namespace right-sizer-test
```

### Docker Test
```bash
# Test the image
docker run --rm ghcr.io/aavishay/right-sizer:0.1.16 --version
docker run --rm ghcr.io/aavishay/right-sizer:0.1.16 --help
```

## Known Issues ⚠️

1. Some timing tests may fail intermittently
2. Helm packaging from source directory has xattr issues on macOS
3. Pre-commit hooks may require manual fixes

## Breaking Changes 💥

Users upgrading from v0.1.15 should note:
- `UpdateDashboardMetrics` renamed to `UpdateMetrics`
- Helm CRDs moved out of chart
- Some API response formats changed

## Support Information 📞

- GitHub Issues: https://github.com/aavishay/right-sizer/issues
- Documentation: https://github.com/aavishay/right-sizer/tree/v0.1.16/docs
- Examples: https://github.com/aavishay/right-sizer/tree/v0.1.16/examples

---

## Sign-off

- [ ] Release manager approval
- [ ] Technical review completed
- [ ] Documentation review completed
- [ ] All checklist items addressed

**Release Date**: 2024-12-20
**Released By**: [Your Name]
**Version**: 0.1.16
**Type**: Minor Release (Major Refactoring)