# ResizePolicy Implementation for Kubernetes 1.33

## Overview

This document describes the implementation of pod resource resizing with `resizePolicy: NotRequired` according to Kubernetes 1.33 specifications. The Right-Sizer operator now supports in-place pod resource updates without container restarts by implementing a two-step resize process and properly setting resize policies.

## Key Changes

### 1. Two-Step Resize Process

According to Kubernetes 1.33 best practices, pod resource resizing should be performed in two separate operations to ensure stability and proper resource allocation:

1. **Step 1: CPU Resize** - Update CPU requests and limits
2. **Step 2: Memory Resize** - Update memory requests and limits

This approach prevents potential issues that could arise from simultaneous CPU and memory changes, ensuring each resource type is handled independently.

### 2. ResizePolicy Configuration

The operator now automatically adds `resizePolicy: NotRequired` to all containers marked for resizing, enabling in-place updates without container restarts:

```yaml
resizePolicy:
  - resourceName: cpu
    restartPolicy: NotRequired
  - resourceName: memory
    restartPolicy: NotRequired
```

## Implementation Details

### InPlaceRightSizer (`inplace_rightsizer.go`)

The `applyInPlaceResize` method has been refactored to perform resizing in three distinct steps:

```go
// Step 1: Apply resize policy to enable in-place updates
func (r *InPlaceRightSizer) applyInPlaceResize(ctx context.Context, pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) error {
    // First, set resize policy
    r.applyResizePolicy(ctx, pod)

    // Step 2: Resize CPU for all containers
    // ... CPU resize logic ...

    // Step 3: Resize Memory for all containers
    // ... Memory resize logic ...
}
```

#### Key Features:
- **Resize Policy Check**: Verifies if containers already have the correct resize policy before applying
- **Sequential Operations**: Ensures CPU is resized before memory with a small delay between operations
- **State Refresh**: Refreshes pod state between operations to ensure consistency
- **Error Handling**: Continues with memory resize even if CPU resize fails (partial success)

### AdaptiveRightSizer (`adaptive_rightsizer.go`)

The `updatePodInPlace` method follows the same two-step approach using JSON patches:

```go
func (r *AdaptiveRightSizer) updatePodInPlace(ctx context.Context, update ResourceUpdate) (string, error) {
    // Step 1: Apply resize policy
    r.applyResizePolicyForContainer(ctx, &pod, containerIndex)

    // Step 2: Resize CPU
    // Creates JSON patch with CPU values only, keeping memory unchanged

    // Step 3: Resize Memory
    // Creates JSON patch with memory values only, keeping CPU at new values
}
```

#### Key Features:
- **JSON Patch Operations**: Uses precise JSON patches for each resource type
- **Container-Specific**: Applies resize policy per container, not per pod
- **Resource Isolation**: Each patch contains only the resource being changed

### Webhook Mutations (`admission/webhook.go`)

The admission webhook now automatically adds resize policies when mutating pods:

```go
func (ws *WebhookServer) generateResourcePatches(pod *corev1.Pod) []JSONPatch {
    // Add resize policy for each container
    resizePolicy := []corev1.ContainerResizePolicy{
        {
            ResourceName:  corev1.ResourceCPU,
            RestartPolicy: corev1.NotRequired,
        },
        {
            ResourceName:  corev1.ResourceMemory,
            RestartPolicy: corev1.NotRequired,
        },
    }
    // ... patch generation logic ...
}
```

## Benefits

### 1. Zero-Downtime Updates
With `resizePolicy: NotRequired`, pods can have their resources adjusted without restarting containers, maintaining application availability.

### 2. Improved Stability
The two-step resize process ensures:
- CPU pressure is resolved before memory adjustments
- Each resource type is validated independently
- Partial success is possible (CPU succeeds even if memory fails)

### 3. Better Observability
The implementation provides detailed logging for each step:
```
📝 Step 1: Setting resize policy for pod namespace/pod-name
⚡ Step 2: Resizing CPU for pod namespace/pod-name
💾 Step 3: Resizing Memory for pod namespace/pod-name
🎯 All resize operations completed for pod namespace/pod-name
```

### 4. Kubernetes 1.33 Compliance
The implementation follows the official Kubernetes 1.33 documentation for in-place pod vertical scaling.

## Configuration

No additional configuration is required. The operator automatically:
1. Detects pods that need resizing
2. Applies the appropriate resize policy
3. Performs the two-step resize operation

## Error Handling

The implementation includes robust error handling:

- **Memory Decrease Restrictions**: Detects when memory cannot be decreased and logs appropriate warnings
- **Node Capacity Limits**: Validates against node capacity before attempting resize
- **Partial Success**: CPU resize can succeed even if memory resize fails
- **Policy Application Failures**: Continues with resize even if policy application fails (it might already be set)

## Testing

The implementation includes comprehensive tests (`resize_test.go`):

1. **TestTwoStepResize**: Verifies CPU and memory are resized separately
2. **TestResizePolicy**: Ensures resize policies are correctly applied
3. **TestAdaptiveRightSizerTwoStepResize**: Tests the adaptive rightsizer's implementation

## Migration Guide

For existing deployments:

1. **No Manual Intervention Required**: The operator automatically adds resize policies to pods
2. **Gradual Rollout**: Resize policies are applied during the next resize operation
3. **Backwards Compatible**: Works with pods that already have resize policies set

## Monitoring

Monitor resize operations through:

1. **Operator Logs**: Detailed step-by-step logging of resize operations
2. **Metrics**: Track successful vs failed resize operations
3. **Events**: Kubernetes events for resize operations

## Limitations

1. **Memory Decrease**: Some workloads may not support memory decrease without restart
2. **Node Resources**: Resize is limited by available node capacity
3. **Kubernetes Version**: Requires Kubernetes 1.33+ for in-place resize support

## Future Enhancements

1. **Configurable Delays**: Make the delay between CPU and memory resize configurable
2. **Rollback Support**: Automatic rollback if resize operations fail
3. **Priority-Based Resizing**: Resize critical workloads first
4. **Batch Operations**: Group resize operations for efficiency

## References

- [Kubernetes 1.33 In-Place Pod Vertical Scaling](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#resize-policy)
- [KEP-1287: In-Place Update of Pod Resources](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1287-in-place-update-pod-resources)
- [Right-Sizer Documentation](https://github.com/aavishay/right-sizer)

## Conclusion

The implementation of `resizePolicy: NotRequired` with a two-step resize process ensures reliable, zero-downtime resource adjustments for Kubernetes pods. This approach follows Kubernetes 1.33 best practices and provides a robust foundation for automatic resource optimization.
