# Changelog

All notable changes to the Right Sizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive RBAC documentation in `docs/RBAC.md` with detailed permission explanations
- Automated RBAC verification script (`scripts/rbac/verify-permissions.sh`) to check all required permissions
- Automated RBAC fix application script (`scripts/rbac/apply-rbac-fix.sh`) for quick permission repairs
- Support for both metrics.k8s.io v1 and v1beta1 APIs for broader compatibility
- Explicit permissions for PodMetrics and NodeMetrics resources
- Storage resource permissions (PVCs, PVs, StorageClasses) for comprehensive validation
- Network policy permissions for better network segmentation awareness
- PriorityClass permissions for scheduling priority information
- Scale subresource permissions for workload controllers

### Fixed
- **Critical RBAC Issues**: Resolved permission errors preventing the operator from accessing required resources
  - Fixed "nodes is forbidden" error by ensuring proper ClusterRole configuration
  - Fixed metrics API access issues by adding explicit metrics.k8s.io permissions
  - Added missing `watch` verb for metrics resources
  - Ensured compatibility with different Kubernetes metrics API versions
- Enhanced pod/resize subresource permissions for Kubernetes 1.27+ in-place resizing
- Corrected service account automounting configuration
- **Duplicate Logging Issues**: Resolved redundant log messages in resource adjustment operations
  - Consolidated scaling analysis logs to appear only when resize is needed
  - Removed duplicate success messages in batch processing
  - Eliminated redundant resize notifications for the same pod operations
  - Achieved ~40-50% reduction in log volume during resize operations
  - Improved log readability with clear progression: analysis → decision → action → result
- **Log Formatting**: Removed `[INFO]` prefix from informational log messages
  - Info and Success level messages no longer show `[INFO]` prefix for cleaner output
  - Warning, Error, and Debug messages retain their severity prefixes for proper identification
  - Emoji indicators (🔍, 📈, ✅) provide visual context instead of text prefixes
  - Reduces log verbosity and improves readability

### Changed
- Updated RBAC configuration to follow principle of least privilege more strictly
- Improved metrics API permissions to include both resource names and lowercase variants
- Enhanced workload controller permissions to include scale subresources
- Expanded troubleshooting documentation with detailed RBAC fix procedures

### Security
- Implemented comprehensive RBAC with minimal required permissions
- Added security best practices documentation for RBAC configuration
- Included network policy examples for additional security hardening

## [0.2.0] - 2024-01-15

### Added
- Kubernetes 1.33 support with in-place pod resizing capabilities
- Enhanced metrics collection from multiple sources (metrics-server, Prometheus)
- Admission webhook for resource validation and mutation
- Comprehensive audit logging system
- Multiple optimization strategies (adaptive, non-disruptive, deployment-focused)

### Changed
- Improved resource calculation algorithms for better accuracy
- Updated controller-runtime to latest version
- Enhanced error handling and recovery mechanisms

### Fixed
- Resource calculation edge cases for init containers
- Memory leak in metrics collection
- Race conditions in concurrent pod updates

## [0.1.0] - 2023-12-01

### Added
- Initial release of Right Sizer operator
- Basic pod resource optimization
- HPA and VPA conflict detection
- Helm chart for easy deployment
- Basic metrics collection from metrics-server
- Namespace filtering capabilities
- Resource quota and limit range validation

### Security
- Initial RBAC implementation
- Service account creation and management
- Basic security policies

## Notes

### Version Schema
- MAJOR version: Incompatible API changes or major feature additions
- MINOR version: Backwards-compatible functionality additions
- PATCH version: Backwards-compatible bug fixes

### Upcoming Features
- [ ] Multi-cluster support
- [ ] Advanced ML-based prediction models
- [ ] Cost optimization reporting
- [ ] Integration with cloud provider pricing APIs
- [ ] Graphical UI for policy management