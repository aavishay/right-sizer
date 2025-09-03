# Changelog

All notable changes to the Right Sizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2025-09-03

### Fixed
- **CRITICAL**: Prevent pod restarts by never updating workload controllers ([b74390a](https://github.com/aavishay/right-sizer/commit/b74390a))
  - RightSizerPolicy controller now only resizes pods directly via in-place resize
  - Never updates Deployments, StatefulSets, or DaemonSets which would trigger rolling updates
  - Guarantees zero-downtime resource optimization
  - Fixes issue where guaranteed pods were being recreated unnecessarily

- **Log Spam**: Eliminate redundant logging for no-op resize operations ([d9ecfb6](https://github.com/aavishay/right-sizer/commit/d9ecfb6))
  - Added comprehensive checks to detect no-op operations before API calls
  - Skip resize attempts when neither CPU nor memory would actually change
  - Suppress success logging for operations that don't modify resources
  - Reduces log volume by ~80% in typical deployments

### Changed
- Policy controller now finds pods matching workload selectors instead of updating workload specs
- Improved resource comparison logic to use actual pod resources instead of planned values
- Batch processing logs only shown when actual updates occur

## [0.1.1] - 2025-08-15

### Added
- In-place pod resizing support for Kubernetes 1.27+
- Guaranteed QoS preservation for pods
- Memory decrease detection and handling for Kubernetes 1.33+ limitations
- Comprehensive scaling threshold logic
- Resource validation framework
- Audit logging capability
- Health and readiness probes

### Changed
- Switched from deployment updates to in-place pod resizing as primary mechanism
- Enhanced metrics provider with support for multiple backends
- Improved error handling and retry logic with circuit breakers

### Fixed
- Memory limit calculations for various workload types
- CPU and memory resource calculation edge cases
- Metrics fetching errors for deleted pods

## [0.1.0] - 2025-07-01

### Added
- Initial release of Right Sizer operator
- Basic resource optimization based on actual usage
- Support for Deployments, StatefulSets, and DaemonSets
- Configurable thresholds for scaling decisions
- Dry-run mode for testing
- Namespace inclusion/exclusion filters
- Minimum and maximum resource constraints
- Basic metrics-server integration

### Known Issues
- Pods may restart during resource updates (fixed in 0.1.2)
- Excessive logging for unchanged resources (fixed in 0.1.2)

## [0.0.1] - 2025-06-01

### Added
- Project initialization
- Basic CRD definitions (RightSizerConfig, RightSizerPolicy)
- Helm chart structure
- Initial documentation
- GitHub Actions CI/CD pipeline
- Unit test framework

---

## Migration Guide

### Upgrading from 0.1.0/0.1.1 to 0.1.2

**Critical**: This version fixes pod restart issues. After upgrading:

1. Monitor your pods to ensure no restarts occur during resizing
2. Check logs are cleaner with reduced spam
3. Verify policies are working correctly with: `kubectl get rightsizerpolicies -A`

No configuration changes required, but recommended to:
- Review and adjust resize intervals if needed
- Consider enabling namespace filters to reduce unnecessary processing

### Upgrading from 0.0.1 to 0.1.x

1. Update CRD definitions: `kubectl apply -f helm/crds/`
2. Update RBAC permissions for new resources
3. Review new configuration options in RightSizerConfig
4. Test with dry-run mode before enabling

---

## Compatibility Matrix

| Right Sizer Version | Kubernetes Version | In-Place Resize | Notes |
|--------------------|--------------------|-----------------|-------|
| 0.1.2              | 1.27+              | ✅ Full         | Recommended |
| 0.1.2              | 1.24-1.26          | ⚠️ Partial      | Falls back to rolling updates |
| 0.1.1              | 1.27+              | ✅ Full         | Has pod restart bug |
| 0.1.0              | 1.24+              | ❌ No           | Uses rolling updates only |
| 0.0.1              | 1.24+              | ❌ No           | Alpha version |

---

*For detailed documentation, see [README.md](README.md)*  
*For troubleshooting, see [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)*