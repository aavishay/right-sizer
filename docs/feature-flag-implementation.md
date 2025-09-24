# Restart Policy Feature Flag Implementation

## Overview

This document describes the implementation of a feature flag that controls whether the Right-Sizer operator automatically patches Kubernetes resources with restart policies. The restart policy determines whether containers need to be restarted when their resource limits are changed.

## Problem Statement

The Right-Sizer operator was unconditionally patching restart policies on pods and their parent resources (Deployments, StatefulSets, DaemonSets) with `RestartPolicy: NotRequired` for both CPU and memory resources. This behavior should be configurable based on user preference and Kubernetes version compatibility.

## Solution

Implemented a feature flag `UpdateResizePolicy` in the CRD that controls whether restart policies are patched. When enabled, the operator will add resize policies to containers to enable in-place resizing without pod restarts. When disabled, the operator skips all restart policy patching operations.

## Implementation Details

### 1. Configuration Structure

The feature flag is part of the `featureGates` section in the RightSizerConfig CRD:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  featureGates:
    UpdateResizePolicy: true  # Enable/disable restart policy patching
```

### 2. Code Changes

#### Modified Structs

**AdaptiveRightSizer** (`controllers/adaptive_rightsizer.go`):
- Added `Config *config.Config` field to access feature flags
- Modified initialization in `SetupAdaptiveRightSizer` to include config

**InPlaceRightSizer** (`controllers/inplace_rightsizer.go`):
- Added `Config *config.Config` field to access feature flags
- Modified initialization in `SetupInPlaceRightSizer` to include config

#### Modified Methods

**AdaptiveRightSizer.ensureParentHasResizePolicy**:
- Added check for `UpdateResizePolicy` flag at the beginning
- Returns early if flag is disabled
- Logs skip message when disabled

**AdaptiveRightSizer.applyResizePolicyForContainer**:
- Added check for `UpdateResizePolicy` flag at the beginning
- Returns early if flag is disabled
- Logs skip message when disabled

**AdaptiveRightSizer.updatePodInPlace**:
- Wrapped call to `ensureParentHasResizePolicy` with feature flag check
- Only attempts to update parent resources when flag is enabled

**InPlaceRightSizer.applyResizePolicy**:
- Added check for `UpdateResizePolicy` flag at the beginning
- Returns early if flag is disabled
- Logs skip message when disabled

### 3. Type Changes

Both `AdaptiveRightSizer` and `InPlaceRightSizer` now use `kubernetes.Interface` instead of `*kubernetes.Clientset` for the `ClientSet` field. This allows for better testing with fake clients.

### 4. Configuration Flow

1. User sets `UpdateResizePolicy` in the CRD
2. `RightSizerConfigReconciler` reads the feature flag from the CRD
3. Configuration is passed to the global `config.Config` struct
4. Rightsizers access the config through their `Config` field
5. Methods check the flag before performing restart policy operations

## Testing

### Test Coverage

Created comprehensive tests in `controllers/resize_policy_test.go`:

1. **TestResizePolicyWithFeatureFlag**: Tests that resize policies are only applied to pods when the feature flag is enabled
   - Tests both `InPlaceRightSizer` and `AdaptiveRightSizer`
   - Verifies no patches occur when flag is disabled
   - Verifies correct patches occur when flag is enabled

2. **TestEnsureParentHasResizePolicyWithFeatureFlag**: Tests that parent resources (Deployments) are only updated when the feature flag is enabled
   - Verifies deployment updates occur when flag is enabled
   - Verifies no updates occur when flag is disabled

### Test Results

All tests pass successfully:
- Feature flag enabled: Restart policies are correctly applied
- Feature flag disabled: No restart policy patches are made

## Default Behavior

- Default value: `false` (disabled)
- When disabled: No restart policies are patched
- When enabled: Restart policies set to `NotRequired` for CPU and memory

## Benefits

1. **Backward Compatibility**: Users can disable the feature if their Kubernetes version doesn't support in-place resizing
2. **Flexibility**: Administrators can control whether pods should restart on resource changes
3. **Safety**: Prevents unexpected behavior in environments where restart policies might cause issues
4. **Compliance**: Meets requirements where restart policies should only be set when explicitly configured

## Usage Example

To enable restart policy patching:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  featureGates:
    UpdateResizePolicy: true
  # ... other configuration
```

To disable (default):

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  featureGates:
    UpdateResizePolicy: false
  # ... other configuration
```

## Migration Guide

For existing deployments:
1. The feature is disabled by default, maintaining current behavior for safety
2. To enable in-place resizing, explicitly set `UpdateResizePolicy: true` in the CRD
3. Monitor pods after enabling to ensure resize operations work as expected
4. The feature requires Kubernetes 1.33+ for full in-place resize support

## Related Files

- `go/controllers/adaptive_rightsizer.go` - Main adaptive rightsizer implementation
- `go/controllers/inplace_rightsizer.go` - In-place rightsizer implementation
- `go/config/config.go` - Configuration structure and defaults
- `go/controllers/rightsizerconfig_controller.go` - CRD controller that reads feature flags
- `go/controllers/resize_policy_test.go` - Test coverage for the feature
- `helm/crds/rightsizer.io_rightsizerconfigs.yaml` - CRD definition
