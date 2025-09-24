# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.19] - 2025-01-13

### Changed
- Updated version to 0.1.19 across all components
- Synchronized dashboard version with main operator version

### Updated
- All documentation and examples updated to use version 0.1.19
- Helm charts updated to version 0.1.19
- Docker image tags updated to 0.1.19
- Package.json version updated to 0.1.19
- OpenAPI specification version updated to 0.1.19

## [0.1.16] - 2024-12-20

### Changed
- Refactored metrics API to be generic instead of dashboard-specific
- Renamed `UpdateDashboardMetrics` to `UpdateMetrics` throughout codebase
- Reorganized project structure with better example and deployment organization
- Updated all test fixtures to use generic names instead of frontend-specific terminology
- Improved Helm chart structure and removed obsolete CRD files from helm/crds/
- Enhanced retry logic with case-insensitive pattern matching
- Moved test workloads to `examples/deploy/` directory
- Moved Helm values examples to `examples/` directory

### Added
- Comprehensive documentation in `docs/` directory:

  - `MINIKUBE_DEPLOYMENT.md` - Minikube deployment guide
  - `RESIZE_POLICY_IMPLEMENTATION.md` - Resize policy implementation details
  - `api/openapi.yaml` - OpenAPI specification
- New configuration and feature documentation:

  - `FEATURE_FLAG_IMPLEMENTATION.md` - Feature flag implementation guide
  - `RENAME_SUMMARY.md` - Summary of naming changes

- New deployment and utility scripts:
  - `scripts/check-coverage.sh` - Test coverage checker
  - `scripts/deploy-no-metrics.sh` - Deployment without metrics server
  - `scripts/deploy-rbac.sh` - RBAC deployment script
  - `scripts/helm-package.sh` - Helm chart packaging
  - `scripts/minimal-deploy.sh` - Minimal deployment script
  - `scripts/monitor-deployment.sh` - Deployment monitoring
  - `scripts/quick-deploy.sh` - Quick deployment script
  - `scripts/test-all.sh` - Comprehensive test runner
- Comprehensive test coverage:
  - `go/admission/webhook_test.go` - Webhook tests
  - `go/api/server_test.go` - API server tests
  - `go/controllers/resize_policy_test.go` - Resize policy tests
  - `go/controllers/resize_test.go` - Resize functionality tests
  - `go/logger/logger_test.go` - Logger tests
  - `go/metrics/operator_metrics_test.go` - Metrics tests
  - `go/policy/engine_test.go` - Policy engine tests
  - `go/retry/retry_test.go` - Retry logic tests
- Docker build variants:
  - `Dockerfile.minimal` - Minimal image variant
  - `Dockerfile.simple` - Simple build variant
- `.helmignore` for better Helm chart packaging
- Comprehensive examples in `examples/deploy/` directory

### Removed
- All dashboard and frontend-related references from code and documentation
- Dashboard-specific metrics and API comments
- Frontend JavaScript file from archive directory
- Obsolete Helm CRD files:
  - `helm/crds/rightsizerconfigs.yaml`
  - `helm/crds/rightsizerpolicies.yaml`

### Fixed
- Retry logic now properly handles "context deadline exceeded" errors
- Fixed timing precision issues in retry tests
- Improved error handling with proper randomization factor support
- Applied trailing whitespace and end-of-file formatting fixes
- Test stability improvements with better assertions

### Security
- Updated `.gitignore` to properly exclude build artifacts
- Improved pre-commit hooks configuration for better code quality

## [0.1.15] - 2024-12-19

### Fixed
- Corrected Helm template syntax in service.yaml
- Removed mock data from optimization events API

### Added
- Test workloads for better testing capabilities

## [0.1.14] - 2024-12-18

_Previous releases not documented in this file. See git history for details._

### Deprecated / Removed (Documentation Cleanup - 2025-09-24)
- Removed obsolete or superseded internal markdown documents:
  - CLEANUP_SUMMARY.md
  - CONFIG_SIMPLIFICATION_PROPOSAL.md
  - DASHBOARD_MIGRATION_NOTICE.md
  - FIX_SUMMARY_UPDATE_RESIZE_POLICY.md
  - IMPLEMENTATION_SUMMARY.md
  - K8S_COMPLIANCE_IMPLEMENTATION_SUMMARY.md
  - K8S_INPLACE_RESIZE_COMPLIANCE_REPORT.md
  - PROXY_ENDPOINTS_FIX_SUMMARY.md
  - TEST_REPORT.md
  - e2e-test-summary.md
  - inplace-resize-test-report.md
  - docs/HELM_CLEANUP_SUMMARY.md
  - docs/ACTIONS_MONITORING.md
  - examples/k8s-compliance-testing/README.md
- Rationale: Consolidated to a Helm‑only deployment model and reduced outdated narrative docs. Core retained docs now live in README.md, INSTALLATION_GUIDE.md, RESIZE_POLICY_IMPLEMENTATION.md, FEATURE_FLAG_IMPLEMENTATION.md, TROUBLESHOOTING_K8S.md, and relevant files under docs/.
- No functional changes to operator logic; purely documentation hygiene.
