# Feature Flag Renaming: EnableInPlaceResize → UpdateResizePolicy

## Summary

The feature flag `EnableInPlaceResize` has been renamed to `UpdateResizePolicy` throughout the Right-Sizer codebase to better reflect its actual functionality.

## Rationale for Change

The original name `EnableInPlaceResize` was misleading because:
- It suggested the flag enables in-place resizing functionality
- In reality, it only controls whether resize policies are patched on containers
- The actual in-place resize capability depends on Kubernetes version and configuration

The new name `UpdateResizePolicy` is more accurate because:
- It clearly indicates the flag controls updating/patching of resize policies
- It doesn't imply enabling or disabling the resize feature itself
- It's more descriptive of the actual operation being performed

## Files Changed

### Go Source Files
- `go/config/config.go` - Configuration struct and methods
- `go/config/config_test.go` - Configuration tests
- `go/controllers/adaptive_rightsizer.go` - Adaptive rightsizer implementation
- `go/controllers/inplace_rightsizer.go` - In-place rightsizer implementation
- `go/controllers/rightsizerconfig_controller.go` - CRD controller
- `go/controllers/resize_policy_test.go` - Resize policy tests
- `go/controllers/resize_test.go` - Resize operation tests

### YAML Configuration Files
- `examples/rightsizerconfig-conservative.yaml` - Conservative configuration example
- `examples/rightsizerconfig-full.yaml` - Full configuration example
- `helm/templates/rightsizerconfig.yaml` - Helm template
- `helm/values.yaml` - Helm values
- `tests/fixtures/test-rate-config.yaml` - Test configuration

### Documentation Files
- `FEATURE_FLAG_IMPLEMENTATION.md` - Feature documentation
- `CONFIG_SIMPLIFICATION_PROPOSAL.md` - Configuration proposal

## Usage Changes

### Before
```yaml
spec:
  featureGates:
    EnableInPlaceResize: true
```

### After
```yaml
spec:
  featureGates:
    UpdateResizePolicy: true
```

## Code Changes

### Configuration Field
```go
// Before
EnableInPlaceResize bool // Enable in-place pod resizing (Kubernetes 1.33+)

// After
UpdateResizePolicy bool // Update resize policy for in-place pod resizing (Kubernetes 1.27+)
```

### Usage in Controllers
```go
// Before
if r.Config != nil && r.Config.EnableInPlaceResize {
    // Apply resize policy
}

// After
if r.Config != nil && r.Config.UpdateResizePolicy {
    // Apply resize policy
}
```

### Log Messages
```go
// Before
log.Printf("📝 Skipping resize policy patch - EnableInPlaceResize feature flag is disabled")

// After
log.Printf("📝 Skipping resize policy patch - UpdateResizePolicy feature flag is disabled")
```

## Backward Compatibility

This is a **breaking change** for existing configurations. Users will need to update their CRD configurations to use the new flag name.

### Migration Path

1. Update your RightSizerConfig CRD to use `UpdateResizePolicy` instead of `EnableInPlaceResize`
2. The default value remains `false` (disabled) for safety
3. No functional changes - the behavior remains the same

### Migration Script
```bash
# Update existing configurations
kubectl get rightsizerconfig -o yaml | \
  sed 's/EnableInPlaceResize/UpdateResizePolicy/g' | \
  kubectl apply -f -
```

## Test Results

All tests pass with the new naming:
- ✅ `TestResizePolicyWithFeatureFlag` - Verifies resize policies are applied based on flag
- ✅ `TestEnsureParentHasResizePolicyWithFeatureFlag` - Verifies parent resources are updated based on flag
- ✅ Code compilation successful
- ✅ No functional regressions

## Conclusion

The renaming from `EnableInPlaceResize` to `UpdateResizePolicy` provides clearer semantics about what the feature flag actually controls. While this is a breaking change, it improves code clarity and reduces confusion for users and developers.
