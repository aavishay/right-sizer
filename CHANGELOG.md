# Changelog

All notable changes to the Right-Sizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security
- **BREAKING**: API tokens and secrets are now stored in Kubernetes Secrets instead of plain text in values.yaml
  - Added secure secret management for dashboard API tokens
  - Added secret management for cluster credentials
  - Added migration script for upgrading existing installations
  - See [helm/SECRETS.md](helm/SECRETS.md) for detailed migration guide

## [0.2.0] - 2024-09-26

### 🎉 Major Release - ARM64 Support & Enhanced Testing

This release marks a significant milestone with full ARM64 architecture support, comprehensive CI/CD testing framework, and improved deployment experience for Minikube users.

### Added

#### 🏗️ Architecture Support
- **Full ARM64/AARCH64 Support**: Native support for Apple Silicon (M1/M2/M3) and ARM-based servers
- **Multi-platform Docker Images**: Automatic building for both `linux/amd64` and `linux/arm64`
- **Platform-specific Deployment Scripts**: Dedicated `deploy-arm64.sh` for ARM64 systems
- **Cross-platform Compatibility**: Unified codebase working seamlessly across architectures

#### 📚 Documentation
- **Comprehensive CI Testing Guide** (`docs/ci-testing/README.md`): Complete testing documentation
- **Quick Start Guide** (`docs/ci-testing/QUICK_START.md`): Fast onboarding for developers
- **Advanced Testing Guide** (`docs/ci-testing/ADVANCED_TESTING.md`): Complex testing scenarios
- **IDE Setup Guide** (`docs/ci-testing/IDE_SETUP.md`): Configuration for popular IDEs
- **ARM64 Deployment Guide** (`docs/ARM64_DEPLOYMENT_SUCCESS.md`): ARM-specific deployment instructions
- **Metrics Server Guide** (`docs/METRICS_SERVER_DEPLOYMENT.md`): Metrics integration documentation
- **Minikube Deployment Guide**: Enhanced with multi-platform support

#### 🚀 Deployment Improvements
- **Quick Deploy Scripts**: One-command deployment with `deploy-minikube-quick.sh`
- **Wrapper Script**: Simplified `deploy.sh` for common operations
- **Metrics Server Automation**: `scripts/deploy-metrics-server.sh` for automatic setup
- **Test Workload Templates**: Ready-to-use test deployments in `test-workload.yaml`

#### 🧪 Testing Framework
- **Chaos Testing**: Resilience testing under adverse conditions
- **Performance Regression Testing**: Automated performance benchmarking
- **Multi-cluster Testing**: Cross-cluster consistency validation
- **Compliance Testing**: Kubernetes specification compliance checks
- **Mutation Testing**: Code quality validation through mutation analysis
- **Contract Testing**: API contract validation with Pact
- **Load Testing**: Stress testing with configurable scenarios
- **Security Testing**: Vulnerability scanning and penetration testing

#### 🛠️ CI/CD Enhancements
- **Pre-commit Hooks**: Early detection with comprehensive checks
- **GitHub Actions Workflows**: Updated for Go 1.25 and multi-platform builds
- **Coverage Reporting**: Automated coverage with HTML reports
- **Benchmark Testing**: Performance tracking with benchstat
- **Integration Testing**: Full Kubernetes integration test suite

### Changed

#### 🔧 Technical Updates
- **Go Version**: Upgraded from 1.23 to 1.25 across all components
- **Docker Build Process**: Migrated to buildx for multi-platform support
- **Makefile Targets**: Enhanced with multi-platform build targets
- **Helm Charts**: Updated to version 0.2.0 with improved defaults
- **Base Images**: Updated to latest secure base images

#### 📊 Metrics Integration
- **Metrics Server**: Improved integration with automatic deployment
- **Resource Monitoring**: Enhanced resource usage tracking
- **Optimization Logic**: Better recommendation algorithms
- **Self-protection**: Validated and documented self-protection mechanisms

### Fixed

#### 🐛 Bug Fixes
- **ARM64 Exec Format Error**: Resolved binary architecture mismatch issues
- **Multi-platform Build**: Fixed buildx compatibility with Minikube
- **Metrics Collection**: Improved reliability of metrics gathering
- **Resource Calculations**: More accurate resource recommendations
- **Pod Resize Logic**: Better handling of edge cases

### Security

- **Go 1.25**: Latest Go version with security patches
- **Distroless Images**: Using minimal attack surface containers
- **Non-root User**: Running as user 65532 for enhanced security
- **Security Scanning**: Integrated vulnerability scanning in CI
- **RBAC**: Minimal required permissions documented

### Performance

- **Build Time**: Reduced Docker build time with better caching
- **Image Size**: Optimized image size (~35.9MB)
- **Startup Time**: Faster operator initialization
- **Memory Usage**: Reduced memory footprint
- **CPU Efficiency**: Optimized resource calculation algorithms

### Testing Coverage

- **Unit Tests**: Comprehensive coverage for all packages
- **Integration Tests**: Full Kubernetes API integration testing
- **E2E Tests**: End-to-end testing with real workloads
- **Compliance Tests**: Kubernetes specification compliance validation
- **Performance Tests**: Automated benchmark suite
- **Security Tests**: Vulnerability and penetration testing

### Developer Experience

- **IDE Support**: Configuration for VS Code, GoLand, Vim, Emacs, Sublime
- **Debugging**: Enhanced debugging capabilities with Delve
- **Local Development**: Improved Minikube development workflow
- **Documentation**: Extensive guides and troubleshooting resources
- **Scripts**: Automation scripts for common tasks

### Known Issues

- **Minikube on Windows**: Some users may need to adjust Docker Desktop settings
- **Metrics Delay**: Initial metrics may take 30-60 seconds to appear
- **Stress Test Image**: The `progrium/stress` image may not be available in all regions

### Migration Guide

To upgrade from 0.1.x to 0.2.0:

1. **Update Helm Repository**:
   ```bash
   helm repo update
   ```

2. **Backup Current Configuration**:
   ```bash
   helm get values right-sizer -n right-sizer > values-backup.yaml
   ```

3. **Upgrade to 0.2.0**:
   ```bash
   helm upgrade right-sizer right-sizer/right-sizer \
     --version 0.2.0 \
     --namespace right-sizer \
     -f values-backup.yaml
   ```

4. **For ARM64 Systems**:
   ```bash
   # Use the dedicated ARM64 deployment script
   ./scripts/deploy-arm64.sh
   ```

### Contributors

Special thanks to all contributors who made this release possible:
- ARM64 architecture support and testing
- Comprehensive CI/CD framework implementation
- Documentation improvements
- Testing enhancements
- Bug fixes and optimizations

### Compatibility Matrix

| Component | Version | Notes |
|-----------|---------|-------|
| Kubernetes | 1.33+ | In-place resize support required |
| Go | 1.25 | Build requirement |
| Helm | 3.0+ | Deployment tool |
| Docker | 20.10+ | With buildx support |
| Minikube | 1.30+ | Local development |
| Metrics Server | 0.6+ | Resource metrics |

---

## [0.1.20] - 2025-09-24

### Changed
- Pre-release version used for testing 0.2.0 features

## [0.1.19] - 2025-09-23

### Initial Release
- Core Right-Sizer operator functionality
- Basic Kubernetes pod resource optimization
- Helm chart deployment
- Docker image distribution
- Basic documentation

---

[0.2.0]: https://github.com/aavishay/right-sizer/releases/tag/v0.2.0
[0.1.20]: https://github.com/aavishay/right-sizer/releases/tag/v0.1.20
[0.1.19]: https://github.com/aavishay/right-sizer/releases/tag/v0.1.19
