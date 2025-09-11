# Fix Summary: Set UpdateResizePolicy Default to False

## Issue
The GitHub Actions workflow was failing because the webhook was unconditionally adding resize policies directly to pods, which should only be done when the `UpdateResizePolicy` feature flag is explicitly enabled.

## Root Cause
The admission webhook's `generateResourcePatches` function was adding resize policies to pod containers without checking the `UpdateResizePolicy` feature flag. This could cause issues in environments where direct pod patching for resize policies is not supported or desired.

## Changes Made

### 1. Fixed Webhook to Respect Feature Flag
**File:** `go/admission/webhook.go`
- Added a condition to check `ws.config.UpdateResizePolicy` before adding resize policy patches
- The resize policy is now only added when the feature flag is explicitly enabled
- This prevents unintended modifications to pods when the feature is disabled

### 2. Verified Default Configuration
**Files:**
- `go/config/config.go` - Default value: `false`
- `helm/values.yaml` - Default value: `false`
- `helm/templates/rightsizerconfig.yaml` - Uses default: `false`

All configuration files correctly set `UpdateResizePolicy` to `false` by default.

### 3. Added Test Coverage
**File:** `go/admission/resize_policy_test.go`
- Created comprehensive tests to verify resize policy behavior with feature flag
- Tests confirm that resize policies are only added when `UpdateResizePolicy: true`
- Tests verify the default value is `false`

## Key Principles Applied

1. **Feature Flag Control**: Resize policy modifications are now properly gated behind the `UpdateResizePolicy` feature flag
2. **Safe Defaults**: The feature defaults to `false` for backward compatibility and safety
3. **Parent Resource Policy**: Resize policies should be set on parent resources (Deployments, StatefulSets, DaemonSets), not directly on pods
4. **Kubernetes Compatibility**: Respects Kubernetes limitations where pods cannot be directly patched for certain fields after creation

## Testing
All tests pass successfully:
- Resize policy is added only when feature flag is enabled
- No resize policy patches are generated when feature flag is disabled
- Default configuration correctly sets the flag to false

## Impact
This fix ensures that:
1. The right-sizer operator doesn't attempt unsupported pod modifications
2. Users can opt-in to the resize policy feature when their Kubernetes version supports it (1.27+)
3. The default behavior is safe and compatible with all Kubernetes versions
