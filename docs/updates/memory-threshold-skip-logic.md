# Memory Threshold Configuration Update

## Overview

This document describes the implementation of a new feature that prevents unnecessary pod resizing when CPU resources don't require updates but memory resources would be reduced.

## Problem Statement

Previously, the RightSizer would resize pods whenever either CPU or memory metrics indicated a need for scaling. This could lead to unnecessary pod disruptions when:
- CPU usage is within acceptable thresholds (no change needed)
- Memory usage is below the scale-down threshold (reduction suggested)

In such cases, reducing only memory while CPU remains stable doesn't provide significant benefit and causes unnecessary pod disruption.

## Solution

The implementation now makes independent scaling decisions for CPU and memory resources and skips resizing when:
- CPU scaling decision is `ScaleNone` (no change needed)
- Memory scaling decision is `ScaleDown` (reduction suggested)

This prevents unnecessary pod restarts when the only change would be a memory reduction without CPU adjustment.

## Implementation Details

### 1. New Data Structure

A new `ResourceScalingDecision` struct tracks independent scaling decisions:

```go
type ResourceScalingDecision struct {
    CPU    ScalingDecision
    Memory ScalingDecision
}
```

### 2. Independent Threshold Checking

The `checkScalingThresholds` function now:
- Evaluates CPU and memory usage independently against their respective thresholds
- Returns separate scaling decisions for each resource type
- Logs detailed decisions for better observability

### 3. Skip Logic

In the `rightSizePod` function, the following logic prevents unnecessary resizing:

```go
// Skip if CPU should not be updated but memory should be reduced
if scalingDecision.CPU == ScaleNone && scalingDecision.Memory == ScaleDown {
    log.Printf("⏭️  Skipping resize for pod %s/%s: CPU doesn't need update and memory would be reduced",
        pod.Namespace, pod.Name)
    return false, nil
}
```

## Configuration

The feature uses existing configuration parameters:

```yaml
cpuScaleUpThreshold: 0.8      # Scale up when CPU > 80%
cpuScaleDownThreshold: 0.3    # Scale down when CPU < 30%
memoryScaleUpThreshold: 0.8   # Scale up when memory > 80%
memoryScaleDownThreshold: 0.3 # Scale down when memory < 30%
```

## Behavior Examples

### Example 1: Skip Resize
- CPU Usage: 50% (between 30-80% thresholds)
- Memory Usage: 20% (below 30% threshold)
- **Decision**: Skip resize (CPU=NoChange, Memory=ScaleDown)
- **Result**: No pod disruption

### Example 2: Perform Resize - Scale Up
- CPU Usage: 85% (above 80% threshold)
- Memory Usage: 20% (below 30% threshold)
- **Decision**: Resize pod (CPU=ScaleUp, Memory=ScaleDown)
- **Result**: Pod resources adjusted

### Example 3: Perform Resize - Both Scale Down
- CPU Usage: 25% (below 30% threshold)
- Memory Usage: 20% (below 30% threshold)
- **Decision**: Resize pod (CPU=ScaleDown, Memory=ScaleDown)
- **Result**: Pod resources reduced

### Example 4: Perform Resize - Memory Scale Up
- CPU Usage: 50% (between thresholds)
- Memory Usage: 85% (above 80% threshold)
- **Decision**: Resize pod (CPU=NoChange, Memory=ScaleUp)
- **Result**: Pod memory increased

## Benefits

1. **Reduced Pod Disruptions**: Avoids unnecessary restarts when only memory reduction is suggested
2. **Better Resource Stability**: Maintains stable CPU allocations when they're already appropriate
3. **Improved Application Performance**: Fewer disruptions mean better application availability
4. **Cost Optimization**: Still allows for resource reduction when both CPU and memory can be scaled down

## Testing

The implementation includes comprehensive unit tests covering:
- Independent CPU and memory scaling decisions
- Skip logic validation
- Edge cases with various threshold combinations
- Multi-container pod scenarios

Test cases validate:
- `skip_resize_cpu_none_memory_down`: Verifies skipping when CPU doesn't need changes but memory would be reduced
- `scale_cpu_up_memory_down`: Verifies resizing proceeds when CPU needs scaling up even if memory scales down
- Various threshold combinations to ensure correct behavior

## Migration Notes

This change is backward compatible and requires no configuration changes. The feature activates automatically with the updated operator version.

## Monitoring

The operator logs detailed information about scaling decisions:

```
📊 Pod namespace/pod-name scaling decision - CPU: no change (50.0%), Memory: scale down (20.0%)
⏭️  Skipping resize for pod namespace/pod-name: CPU doesn't need update and memory would be reduced
```

Monitor these logs to understand:
- How often resize operations are skipped
- Resource utilization patterns
- Threshold effectiveness

## Future Enhancements

Potential improvements for consideration:
1. Configurable skip behavior (allow users to disable this logic if needed)
2. Separate skip thresholds for CPU and memory
3. Metrics/dashboards for skipped resize operations
4. More granular control over which resource combinations trigger skips