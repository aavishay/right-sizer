# Scaling Thresholds Configuration Guide

## Overview

Right-Sizer now supports **configurable scaling thresholds** that allow you to precisely control when pods should scale up or down based on resource utilization. This feature gives you fine-grained control over resource optimization while preventing unnecessary scaling operations.

## What Are Scaling Thresholds?

Scaling thresholds define the resource usage percentages that trigger scaling decisions:

- **Scale-Up Threshold**: The usage percentage that triggers resource increase
- **Scale-Down Threshold**: The usage percentage that triggers resource decrease
- **Hysteresis Gap**: The difference between thresholds prevents oscillation

## How It Works

### Decision Logic

Right-Sizer monitors pod resource usage and makes scaling decisions based on configured thresholds:

1. **Scale Up**: Triggered when **either** CPU or memory usage exceeds their respective scale-up thresholds
2. **Scale Down**: Triggered when **both** CPU and memory usage are below their respective scale-down thresholds
3. **No Change**: When usage is between the thresholds, resources remain unchanged

### Example Scenario

```
Pod Current State:
- CPU Limit: 1000m
- Memory Limit: 2048MB

Current Usage:
- CPU: 850m (85% of limit)
- Memory: 1638MB (80% of limit)

Thresholds:
- CPU Scale Up: 80%
- Memory Scale Up: 80%

Decision: SCALE UP (CPU at 85% > 80% threshold)
```

## Configuration

### Using RightSizerConfig CRD

Configure scaling thresholds through the RightSizerConfig Custom Resource:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
  namespace: right-sizer-system
spec:
  defaultResourceStrategy:
    cpu:
      # CPU resource calculations
      requestMultiplier: 1.2
      limitMultiplier: 2.0
      minRequest: 10
      maxLimit: 4000
      
      # CPU scaling thresholds
      scaleUpThreshold: 0.8      # Scale up at 80% CPU usage
      scaleDownThreshold: 0.3    # Scale down at 30% CPU usage
      
    memory:
      # Memory resource calculations
      requestMultiplier: 1.2
      limitMultiplier: 2.0
      minRequest: 64
      maxLimit: 8192
      
      # Memory scaling thresholds
      scaleUpThreshold: 0.8      # Scale up at 80% memory usage
      scaleDownThreshold: 0.3    # Scale down at 30% memory usage
```

### Default Values

If not specified, Right-Sizer uses these default thresholds:

| Resource | Scale Up Threshold | Scale Down Threshold |
|----------|-------------------|---------------------|
| CPU      | 80% (0.8)         | 30% (0.3)          |
| Memory   | 80% (0.8)         | 30% (0.3)          |

## Configuration Examples

### Production Environment (Conservative)

Prioritize stability with early scale-up and cautious scale-down:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: production-config
spec:
  defaultResourceStrategy:
    cpu:
      scaleUpThreshold: 0.7      # Scale up early at 70%
      scaleDownThreshold: 0.2    # Only scale down at 20%
    memory:
      scaleUpThreshold: 0.7      # Scale up early at 70%
      scaleDownThreshold: 0.2    # Only scale down at 20%
```

**Benefits:**
- Prevents resource exhaustion by scaling up early
- Avoids frequent scaling by requiring very low usage for scale-down
- Maintains performance headroom

### Development Environment (Aggressive)

Optimize costs with tighter resource allocation:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: development-config
spec:
  defaultResourceStrategy:
    cpu:
      scaleUpThreshold: 0.9      # Only scale up at 90%
      scaleDownThreshold: 0.5    # Scale down at 50%
    memory:
      scaleUpThreshold: 0.9      # Only scale up at 90%
      scaleDownThreshold: 0.5    # Scale down at 50%
```

**Benefits:**
- Minimizes resource waste
- Reduces costs in non-critical environments
- Allows higher utilization before scaling

### Batch Processing Workloads

Handle burst workloads with appropriate thresholds:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: batch-processing-config
spec:
  defaultResourceStrategy:
    cpu:
      scaleUpThreshold: 0.85     # Scale up at 85%
      scaleDownThreshold: 0.4    # Scale down at 40%
    memory:
      scaleUpThreshold: 0.75     # Scale up earlier for memory
      scaleDownThreshold: 0.3    # Keep memory longer
```

### Memory-Intensive Applications

Prioritize memory availability:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: memory-intensive-config
spec:
  defaultResourceStrategy:
    cpu:
      scaleUpThreshold: 0.8      # Standard CPU threshold
      scaleDownThreshold: 0.3
    memory:
      scaleUpThreshold: 0.65     # Scale up memory early at 65%
      scaleDownThreshold: 0.25   # Keep memory allocated longer
```

## Best Practices

### 1. Maintain Hysteresis Gap

Always keep a significant gap between scale-up and scale-down thresholds:

```yaml
# GOOD: 50% gap prevents oscillation
scaleUpThreshold: 0.8
scaleDownThreshold: 0.3

# BAD: Too narrow, may cause frequent scaling
scaleUpThreshold: 0.8
scaleDownThreshold: 0.7
```

### 2. Consider Workload Patterns

Match thresholds to your application behavior:

- **Steady workloads**: Wider thresholds (0.8/0.3)
- **Bursty workloads**: Earlier scale-up (0.7/0.3)
- **Predictable patterns**: Tighter thresholds (0.85/0.4)

### 3. Account for Startup Time

If your application has slow startup:

```yaml
cpu:
  scaleUpThreshold: 0.7      # Scale up early
  scaleDownThreshold: 0.2    # Avoid frequent restarts
```

### 4. Monitor and Adjust

Start with conservative thresholds and adjust based on metrics:

```bash
# Check current resource usage
kubectl top pods -n <namespace>

# View Right-Sizer logs for scaling decisions
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i scale

# Monitor scaling events
kubectl get events -n <namespace> | grep -i resize
```

### 5. Different Thresholds for Different Resources

CPU and memory often have different usage patterns:

```yaml
cpu:
  scaleUpThreshold: 0.8      # CPU can handle higher utilization
  scaleDownThreshold: 0.3
memory:
  scaleUpThreshold: 0.7      # Scale memory earlier to prevent OOM
  scaleDownThreshold: 0.25   # Keep memory allocated longer
```

## Validation Rules

Right-Sizer enforces these validation rules for thresholds:

1. **Range**: Values must be between 0.1 and 1.0
2. **Ordering**: Scale-down threshold must be less than scale-up threshold
3. **Minimum Gap**: Recommended minimum gap of 0.2 between thresholds

## Troubleshooting

### Pods Not Scaling Up

Check if usage is actually exceeding thresholds:

```bash
# Check pod metrics
kubectl top pod <pod-name> -n <namespace>

# Check Right-Sizer logs
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep <pod-name>
```

### Frequent Scaling Operations

Increase the gap between thresholds:

```yaml
# Before (too narrow)
scaleUpThreshold: 0.8
scaleDownThreshold: 0.6

# After (better hysteresis)
scaleUpThreshold: 0.8
scaleDownThreshold: 0.3
```

### Pods Running Out of Resources

Lower the scale-up threshold:

```yaml
# Before (might be too late)
scaleUpThreshold: 0.9

# After (more proactive)
scaleUpThreshold: 0.75
```

## Integration with Other Features

### With Global Constraints

Thresholds work alongside global constraints:

```yaml
spec:
  defaultResourceStrategy:
    cpu:
      scaleUpThreshold: 0.8
      scaleDownThreshold: 0.3
  globalConstraints:
    cooldownPeriod: "5m"        # Wait between scaling operations
    minChangeThreshold: 5        # Ignore small changes
```

### With Resource Multipliers

Thresholds determine **when** to scale, multipliers determine **how much**:

```yaml
spec:
  defaultResourceStrategy:
    cpu:
      # When to scale
      scaleUpThreshold: 0.8
      scaleDownThreshold: 0.3
      
      # How much to scale
      requestMultiplier: 1.2     # Add 20% buffer
      limitMultiplier: 2.0       # Allow 2x burst
```

## Monitoring and Metrics

### Prometheus Metrics

Right-Sizer exposes threshold-related metrics:

```promql
# Current usage percentage
rightsizer_pod_cpu_usage_percent{namespace="default", pod="my-app"}

# Threshold violations
rightsizer_scaling_threshold_exceeded_total{threshold="cpu_up"}

# Scaling decisions
rightsizer_scaling_decisions_total{decision="scale_up", reason="memory_threshold"}
```

### Grafana Dashboard

Monitor threshold effectiveness:

1. Usage percentage over time
2. Threshold crossing events
3. Scaling decision distribution
4. Resource utilization efficiency

## Advanced Use Cases

### Time-Based Thresholds

While not directly supported, you can achieve time-based behavior by updating the CRD:

```bash
# Business hours configuration
kubectl apply -f production-thresholds.yaml

# After hours configuration (via CronJob)
kubectl apply -f after-hours-thresholds.yaml
```

### Namespace-Specific Thresholds

Apply different thresholds to different namespaces using multiple configs:

```yaml
# Production namespace
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-policy
spec:
  selector:
    namespaceSelector:
      matchLabels:
        environment: production
  resourceStrategy:
    cpu:
      scaleUpThreshold: 0.7
      scaleDownThreshold: 0.2
```

## Summary

Configurable scaling thresholds provide:

- **Precision Control**: Define exactly when scaling should occur
- **Workload Optimization**: Tailor thresholds to application needs
- **Cost Management**: Balance performance and resource efficiency
- **Stability**: Prevent oscillation with hysteresis
- **Flexibility**: Different strategies for different environments

By properly configuring scaling thresholds, you can achieve optimal resource utilization while maintaining application performance and stability.