# Release Notes - v0.1.16

## Release Date
September 11, 2024

## Overview
This patch release updates the default configuration for the `UpdateResizePolicy` feature flag in the Helm chart to improve safety and stability.

## Changes

### Configuration Updates

#### UpdateResizePolicy Default Value Changed
- **Changed**: The `UpdateResizePolicy` feature flag is now **disabled by default** (`false`)
- **Previous**: Was enabled by default (`true`)
- **Impact**: In-place pod resizing functionality will be disabled unless explicitly enabled
- **Rationale**: This change prioritizes stability and safety, requiring users to explicitly opt-in to the resize policy patching feature

### Helm Chart Improvements

#### Fixed Feature Gates Templating
- **Fixed**: All feature gates in the Helm chart now properly use values from `values.yaml`
- **Previous**: Feature gates were hardcoded in the template
- **Impact**: Users can now properly configure all feature gates through the Helm values file

## Breaking Changes
None. This is a configuration default change only.

## Migration Guide

### For Existing Users
If you were relying on the default enabled state of `UpdateResizePolicy`:

1. **Option 1**: Explicitly enable it in your `values.yaml`:
   ```yaml
   rightsizerConfig:
     featureGates:
       updateResizePolicy: true
   ```

2. **Option 2**: Override during Helm install/upgrade:
   ```bash
   helm upgrade right-sizer right-sizer/right-sizer \
     --set rightsizerConfig.featureGates.updateResizePolicy=true
   ```

### For New Users
No action required. The feature will be disabled by default, which is the recommended safe configuration.

## Technical Details

### Files Modified
- `helm/values.yaml`: Changed `updateResizePolicy` default from `true` to `false`
- `helm/templates/rightsizerconfig.yaml`: Updated to use templated values for all feature gates

### Feature Flag Details
The `UpdateResizePolicy` feature flag controls whether the Right-Sizer operator patches resize policies on containers to enable in-place resizing without pod restarts. When disabled (now default), the operator skips all restart policy patching operations.

## Recommendations
- Test in-place resizing in a non-production environment before enabling in production
- Monitor pods after enabling to ensure resize operations work as expected
- Review Kubernetes version compatibility (requires K8s 1.27+)

## Known Issues
None in this release.

## Contributors
- Right-Sizer Contributors

## Support
For questions or issues, please open an issue at: https://github.com/aavishay/right-sizer/issues
