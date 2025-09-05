# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.13] - 2025-09-05

## What's Changed

- Remove version backup directory
- chore: update Chart.yaml version to 0.3.0 and sync documentation
- feat: remove auto version bump workflow from CI/CD
- chore: bump version to 0.3.0 (minor) [skip-version-bump]
- feat: remove 'memory decrease skipped' message from CPU-only resize logs
- chore: bump version to 0.2.0 (minor) [skip-version-bump]
- feat: update GitHub Actions workflows for proper versioning

## [0.1.12] - 2025-09-05

## What's Changed

- chore: bump version to 0.1.12
- chore: remove obsolete files

## [0.1.11] - 2025-09-05

## What's Changed

- chore: bump version to 0.1.11
- Add 'adaptive' mode to RightSizer configuration
- Ensure Helm chart is created for every Docker image build
- Remove -helm suffix from documentation and workflow
- Fix Helm chart publishing version conflicts
- Fix Helm chart publishing to handle existing versions
- Fix YAML parsing issues in GitHub Actions workflows

## [0.1.10] - 2025-09-05

## What's Changed

- chore: bump version to 0.1.10
- Fix CI/CD duplicate Docker build issue

## [0.1.9] - 2025-09-05

## What's Changed

- fix: update VERSION file to 0.1.9
- chore: update version to 0.1.9
- feat: Add automatic version bumping on every push
- fix: remove shell commands from distroless image testing

## [0.1.8] - 2025-09-05

## What's Changed

- bump version to 0.1.8
- fix: resolve security issues and stacktrace errors
- feat: Add caching mechanism to prevent repetitive log messages
- feat: Add pod phase filtering to exclude non-resizable pods

## [0.1.7] - 2025-09-05

## What's Changed

* feat: add GitHub Container Registry as backup OCI registry (Avishay Ashkenazi)
* fix: improve OCI Helm chart publishing reliability (Avishay Ashkenazi)
* fix: correct Docker Hub credentials for OCI Helm publishing (Avishay Ashkenazi)
* feat: add complete OCI Helm registry publishing support (Avishay Ashkenazi)
* chore: update version to 0.1.7 (Avishay Ashkenazi)
* feat: add automated version management system (Avishay Ashkenazi)

## [0.1.6] - 2025-09-05

## What's Changed

* Update documentation and CRD for v0.1.6 release (Avishay Ashkenazi)
* Fix OCI registry URL for Helm chart installation (Avishay Ashkenazi)
* Update Helm chart to v0.1.6 with OCI registry support (Avishay Ashkenazi)
* Add rightsizerconfig templates and full example configuration (Avishay Ashkenazi)
* feat: Add memory decrease restriction handling and conservative config (Avishay Ashkenazi)
* Create comprehensive RightSizerConfig template with all configuration fields (Avishay Ashkenazi)
* Fix resource removal error by preserving all resource types (Avishay Ashkenazi)
* fix: Helm chart installation issues (Avishay Ashkenazi)
* fix: Remove edit artifacts from Helm README (Avishay Ashkenazi)

## [0.1.5] - 2025-09-04

## What's Changed

* fix: CI/CD pipeline to use semantic versioning properly (Avishay Ashkenazi)
* fix: Critical RBAC and RightSizerConfig template fixes (Avishay Ashkenazi)


**Full Changelog**: https://github.com/aavishay/right-sizer/compare/v0.1.4...v0.1.5

## [0.1.4] - 2025-09-04

## What's Changed

* chore: Bump version to 0.1.4 and add Helm chart publishing script (Avishay Ashkenazi)
* style: Minor formatting fixes in documentation (Avishay Ashkenazi)
* feat: Add default RightSizerConfig to Helm chart (Avishay Ashkenazi)
* Rename: Standardize all references from right-sizer-test to right-sizer (Avishay Ashkenazi)
* Fix: Only modify existing resource types during in-place pod resizing (Avishay Ashkenazi)
* 🐛 Fix resource removal error in adaptive rightsizer (Avishay Ashkenazi)
* 🐛 Fix resource removal error in pod resize (Avishay Ashkenazi)
* 🔗 Fix GitHub Discussions link to correct repository URL (Avishay Ashkenazi)
* 🔗 Fix GitHub Issues link to correct repository URL (Avishay Ashkenazi)
* 📊 Streamline README: Simplify architecture diagrams, remove complex sections (Avishay Ashkenazi)
* docs: Add README.md for Artifact Hub (Avishay Ashkenazi)
* chore: Update VERSION to 0.1.3 (Avishay Ashkenazi)
* docs: Update CHANGELOG for version 0.1.3 release (Avishay Ashkenazi)
* chore: Bump version to 0.1.3 (Avishay Ashkenazi)

## [0.1.3] - 2025-09-04

## What's Changed

* feat: Add icon for Artifact Hub visibility (Avishay Ashkenazi)
* docs: Update documentation to reflect current project state (Avishay Ashkenazi)
* Fix CI/CD pipeline: Update Dockerfiles to use distroless debug image (Avishay Ashkenazi)
* Fix CI/CD pipeline issues (Avishay Ashkenazi)
* feat: Implement real metrics fetching and notification system (Avishay Ashkenazi)
* Merge pull request #1 from aavishay/fix/helm-chart-publish-workflow (Avishay Ashkenazi)
* fix: helm chart publishing workflow (Avishay Ashkenazi)

## [0.1.2] - 2025-09-03

This is the first release!