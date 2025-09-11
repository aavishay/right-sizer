# Configuration Simplification Proposal

## Current State

The Right-Sizer operator currently has two separate configuration flags that control resource modification behavior:

1. **`dryRun`**: Global flag that prevents ANY changes from being applied (only logs what would be done)
2. **`UpdateResizePolicy`**: Feature flag that controls whether restart policies are patched on containers

## Problem Statement

Having two separate flags creates complexity:
- Users need to understand the interaction between both flags
- Code has multiple conditional checks that could be simplified
- The naming could be clearer about what the flag actually does (updating resize policies vs enabling resize functionality)

## Valid Use Cases to Consider

Before simplifying, we must ensure these scenarios remain supported:

1. **Preview Mode**: See what changes would be made without applying them
2. **Resource-Only Updates**: Update CPU/memory without modifying restart policies (for K8s < 1.33)
3. **Full In-Place Support**: Update resources AND add restart policies for seamless resizing
4. **Gradual Rollout**: Test resource changes before enabling restart policy modifications

## Proposed Solutions

### Option 1: Rename and Clarify (Minimal Change)

Keep both flags but rename for clarity:

```yaml
spec:
  dryRun: false                    # Global preview mode
  patchRestartPolicy: true         # Whether to add restart policies to containers
```

**Pros:**
- Clear naming
- Minimal code changes
- Preserves all use cases

**Cons:**
- Still two flags to manage

### Option 2: Single Mode Selector

Replace both flags with a single `operationMode`:

```yaml
spec:
  operationMode: "resize-with-policies"  # Options below
  # - "dry-run": Preview only, no changes
  # - "resize-only": Update resources without restart policies
  # - "resize-with-policies": Update resources and add restart policies
```

**Pros:**
- Single configuration point
- Clear operational modes
- Easy to understand

**Cons:**
- Breaking change for existing configurations
- Less granular control

### Option 3: Nested Configuration (Recommended)

Organize related settings together:

```yaml
spec:
  execution:
    dryRun: false              # Preview mode (overrides everything)

  resizePolicy:
    enabled: true              # Whether to resize at all
    patchRestartPolicy: true  # Whether to add restart policies
    strategy: "in-place"       # Future: could support "rolling", "recreate"
```

**Pros:**
- Logical grouping
- Extensible for future strategies
- Clear hierarchy (dryRun overrides all)
- Backward compatible with adapter layer

**Cons:**
- Slightly more verbose
- Requires migration path

### Option 4: Smart Defaults Based on K8s Version

Automatically detect capabilities and set defaults:

```yaml
spec:
  dryRun: false
  resizeStrategy: "auto"  # Options: "auto", "legacy", "in-place"
  # - "auto": Detect K8s version and use best strategy
  # - "legacy": Never patch restart policies (for K8s < 1.33)
  # - "in-place": Always patch restart policies (requires K8s 1.33+)
```

**Pros:**
- User-friendly defaults
- Automatic optimization
- Single strategy selector

**Cons:**
- Version detection complexity
- May surprise users with automatic behavior

## Recommendation

**Adopt Option 3 (Nested Configuration)** with a migration path:

### Phase 1: Add New Structure (Backward Compatible)
1. Add new nested structure
2. Map old flags to new structure internally
3. Deprecation warnings for old flags

### Phase 2: Migration Period
1. Support both old and new configurations
2. Documentation updates
3. Migration tooling

### Phase 3: Remove Old Flags
1. Remove deprecated flags in next major version
2. Clean up compatibility code

## Implementation Details

### Configuration Mapping

```go
// Internal configuration resolution
func resolveConfig(spec *RightSizerConfigSpec) *ResolvedConfig {
    config := &ResolvedConfig{}

    // Handle legacy flags (deprecated)
    if spec.DryRun != nil {
        config.DryRun = *spec.DryRun
        log.Warn("spec.dryRun is deprecated, use spec.execution.dryRun")
    }

    // New structure takes precedence
    if spec.Execution != nil {
        config.DryRun = spec.Execution.DryRun
    }

    // Resolve resize behavior
    if config.DryRun {
        config.ApplyChanges = false
        config.PatchPolicies = false
    } else if spec.ResizePolicy != nil {
        config.ApplyChanges = spec.ResizePolicy.Enabled
        config.PatchPolicies = spec.ResizePolicy.PatchRestartPolicy
    }

    return config
}
```

### Migration Helper

```bash
#!/bin/bash
# migrate-config.sh - Helps migrate old config to new format

kubectl get rightsizerconfig -o yaml | \
  yq eval '
    .spec.execution.dryRun = .spec.dryRun |
    .spec.resizePolicy.enabled = .spec.enabled |
    .spec.resizePolicy.patchRestartPolicy = .spec.featureGates.UpdateResizePolicy |
    del(.spec.dryRun) |
    del(.spec.featureGates.UpdateResizePolicy)
  ' - | \
  kubectl apply -f -
```

## Benefits of Recommended Approach

1. **Clear Semantics**: Each flag has a single, clear purpose
2. **Extensibility**: Easy to add new strategies or options
3. **Backward Compatible**: Existing deployments continue working
4. **Future-Proof**: Structure supports upcoming features like rolling updates
5. **User-Friendly**: Logical grouping makes configuration intuitive

## Timeline

- **Week 1-2**: Implement new configuration structure with backward compatibility
- **Week 3-4**: Update documentation and create migration guides
- **Month 2-3**: Migration period with deprecation warnings
- **Next Major Release**: Remove deprecated flags

## Conclusion

While the current dual-flag approach works, adopting a nested configuration structure will:
- Improve user experience
- Reduce code complexity
- Provide better extensibility
- Maintain all current use cases

The migration path ensures zero disruption to existing deployments while moving toward a cleaner architecture.
