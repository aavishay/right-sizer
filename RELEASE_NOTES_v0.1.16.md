# Release Notes - v0.1.16

## 🎯 Overview

This release marks a significant milestone in the Right-Sizer project evolution, focusing on simplifying the codebase by removing all frontend and dashboard components. The project is now a pure Kubernetes operator focused on resource optimization, with improved documentation, better test coverage, and enhanced stability.

## 🚀 What's New

### Major Refactoring
- **Complete Frontend Removal**: All dashboard and UI-related code has been removed, making Right-Sizer a focused backend operator
- **Generic Metrics API**: Dashboard-specific metrics have been refactored to provide a generic, reusable metrics API
- **Improved Project Structure**: Better organization with dedicated directories for examples, scripts, and documentation

### Enhanced Documentation
- Comprehensive cleanup summary documenting all changes
- Detailed Minikube deployment guide
- OpenAPI specification for the REST API
- Configuration simplification proposals
- Feature flag implementation guidelines
- Resize policy implementation documentation

### New Features
- Multiple Docker image variants (minimal, simple, standard)
- Comprehensive test suite with improved coverage
- Enhanced retry logic with better error handling
- New deployment scripts for various scenarios
- Better Helm chart packaging and structure

## 💥 Breaking Changes

### API Changes
- `UpdateDashboardMetrics()` renamed to `UpdateMetrics()`
- Some API response formats have been generalized
- Dashboard-specific metric names have been updated

### Helm Chart Updates
- CRDs are now managed separately from Helm charts
- Chart structure has been reorganized
- Values file may require updates for existing deployments

## 🛠️ Improvements

### Code Quality
- ✅ Added comprehensive test coverage for all major components
- ✅ Fixed timing precision issues in retry tests
- ✅ Improved error handling with case-insensitive pattern matching
- ✅ Added support for "context deadline exceeded" errors
- ✅ Applied consistent code formatting and style

### Project Organization
- 📁 Moved test workloads to `examples/deploy/`
- 📁 Created dedicated `scripts/` directory for utility scripts
- 📁 Reorganized Helm values examples
- 📁 Added `.helmignore` for better chart packaging

### Testing
- Added webhook tests (`go/admission/webhook_test.go`)
- Added API server tests (`go/api/server_test.go`)
- Added resize policy tests (`go/controllers/resize_policy_test.go`)
- Added logger tests (`go/logger/logger_test.go`)
- Added metrics tests (`go/metrics/operator_metrics_test.go`)
- Added policy engine tests (`go/policy/engine_test.go`)
- Added retry logic tests (`go/retry/retry_test.go`)

## 📦 New Scripts

- `scripts/check-coverage.sh` - Check test coverage
- `scripts/deploy-no-metrics.sh` - Deploy without metrics server
- `scripts/deploy-rbac.sh` - Deploy with RBAC configuration
- `scripts/helm-package.sh` - Package Helm charts
- `scripts/minimal-deploy.sh` - Minimal deployment
- `scripts/monitor-deployment.sh` - Monitor deployment status
- `scripts/quick-deploy.sh` - Quick deployment for testing
- `scripts/test-all.sh` - Run all tests

## 🗑️ Removed

- Dashboard-specific code and references
- Frontend JavaScript files
- Obsolete Helm CRD files (`rightsizerconfigs.yaml`, `rightsizerpolicies.yaml`)
- UI/web component references from documentation

## 🐛 Bug Fixes

- Fixed retry logic to properly handle timeout errors
- Fixed timing precision issues in tests
- Improved error handling with proper randomization factor
- Fixed trailing whitespace and end-of-file formatting issues
- Improved test stability with better assertions

## 📋 Migration Guide

### For Users Upgrading from v0.1.15

1. **Update Metrics Collection**:
   ```go
   // Old
   metrics.UpdateDashboardMetrics(...)
   
   // New
   metrics.UpdateMetrics(...)
   ```

2. **Helm Values Update**:
   - Review your `values.yaml` file against the new structure
   - CRDs should be installed separately before Helm chart deployment

3. **API Integration**:
   - Update any integrations that relied on dashboard-specific endpoints
   - Use the generic metrics API endpoints

## 🔍 Testing

This release has been tested with:
- Kubernetes 1.27+
- Minikube
- Docker Desktop Kubernetes
- Unit test coverage: ~85%
- Integration tests: All passing

## 📝 Documentation

- [CHANGELOG.md](./CHANGELOG.md) - Detailed change log
- [CLEANUP_SUMMARY.md](./CLEANUP_SUMMARY.md) - Comprehensive cleanup documentation
- [docs/MINIKUBE_DEPLOYMENT.md](./docs/MINIKUBE_DEPLOYMENT.md) - Minikube deployment guide
- [docs/api/openapi.yaml](./docs/api/openapi.yaml) - OpenAPI specification
- [examples/README.md](./examples/README.md) - Examples and usage guide

## 🙏 Acknowledgments

Thanks to all contributors who helped make this release possible through testing, bug reports, and feedback.

## 📊 Stats

- **Files Changed**: 94
- **Insertions**: 18,221
- **Deletions**: 2,120
- **New Test Files**: 8
- **New Documentation Files**: 9
- **New Scripts**: 8

## 🚦 Compatibility

- **Kubernetes**: 1.27+
- **Helm**: 3.0+
- **Go**: 1.21+
- **Metrics Server**: v0.6.0+

## 📥 Installation

### Using Helm

```bash
helm repo add right-sizer https://github.com/aavishay/right-sizer/releases/download/v0.1.16
helm install right-sizer right-sizer/right-sizer --version 0.1.16
```

### Using kubectl

```bash
kubectl apply -f https://github.com/aavishay/right-sizer/releases/download/v0.1.16/right-sizer.yaml
```

### Docker Image

```bash
docker pull ghcr.io/aavishay/right-sizer:0.1.16
```

## ⚠️ Known Issues

- Some timing tests may occasionally fail due to system load
- Metrics aggregation may have a slight delay on first startup
- Pre-commit hooks require manual fixes for some formatting issues

## 🔮 What's Next

- Implementation of configuration simplification proposal
- Feature flag system for gradual rollout of new features
- Enhanced observability with OpenTelemetry support
- Multi-cluster support
- Advanced ML-based resource predictions

---

**Full Changelog**: https://github.com/aavishay/right-sizer/compare/v0.1.15...v0.1.16

**Container Image**: `ghcr.io/aavishay/right-sizer:0.1.16`

**Helm Chart**: `right-sizer-0.1.16.tgz`
