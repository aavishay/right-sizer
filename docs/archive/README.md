# Archived Documentation

This directory contains historical documentation and files that are no longer actively maintained but are preserved for reference.

## Archived Files

### Implementation Documentation
These files document specific fixes and improvements that have already been implemented in the codebase:

- **`DUPLICATE_LOGGING_FIX.md`** - Documentation for a logging issue that was resolved. The fix has been implemented and is now part of the standard codebase.
- **`INFO_PREFIX_REMOVAL.md`** - Details about removing the [INFO] prefix from log messages for cleaner output. This change has been completed.
- **`KUBERNETES-1.33-UPGRADE.md`** - Upgrade guide for Kubernetes 1.33 support. While containing valuable information, this is version-specific documentation that users on 1.33+ no longer need.

### Configuration Files
- **`fix-rbac.yaml`** - Standalone RBAC configuration file that is now redundant as the Helm chart contains the complete RBAC configuration.

## Why These Files Were Archived

1. **Completed Improvements**: Files documenting fixes that have been fully implemented and integrated into the codebase.
2. **Version-Specific Guides**: Documentation for specific version upgrades that are no longer relevant for most users.
3. **Duplicate Configurations**: Configuration files that duplicate functionality now properly maintained in the Helm charts.

## Accessing Archived Information

While these files are archived, they may still be useful for:
- Understanding the history of certain features
- Debugging issues in older deployments
- Reference for similar future improvements

If you need information from these archived documents, they remain accessible here but should be understood as historical references rather than current documentation.

## Archive Date

Files archived on: 2024 (during project cleanup and organization)