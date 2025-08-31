# Understanding Right-Sizer Logs

This guide explains how to interpret the log output from the Right-Sizer operator, including metrics, scaling decisions, and resource adjustments.

## Log Format Overview

Right-Sizer uses emoji prefixes to categorize different types of log messages:

| Emoji | Category | Description |
|-------|----------|-------------|
| 🚀 | Startup | Operator initialization and configuration |
| 📊 | Metrics | Resource usage statistics and measurements |
| 🔍 | Analysis | Scaling decision analysis |
| 📈 | Planning | Planned resource changes |
| 🔧 | Action | Active resizing operations |
| ✅ | Success | Successful operations |
| ⚠️ | Warning | Potential issues or skipped operations |
| ❌ | Error | Failed operations |
| ⏭️ | Skip | Intentionally skipped operations |
| 🔄 | Batch | Batch processing information |

## Common Log Patterns

### 1. Scaling Analysis

```
🔍 Scaling analysis for default/my-app - CPU: scale down (usage: 75m, limit: 500m, 15.0%), Memory: scale up (usage: 355Mi, limit: 256Mi, 138.7%)
```

**What it means:**
- **Pod**: `default/my-app` (namespace/name)
- **CPU Decision**: Scale down (usage is only 15% of limit)
- **Memory Decision**: Scale up (usage is 138.7% of limit - over-utilized!)
- **Current Usage**: CPU=75m, Memory=355Mi
- **Current Limits**: CPU=500m, Memory=256Mi

### 2. Planned Resource Changes

```
📈 Container default/my-app/main will be resized - CPU: 100m→82m, Memory: 128Mi→426Mi
```

**What it means:**
- The container `main` in pod `default/my-app`
- CPU request will decrease: 100m → 82m
- Memory request will increase: 128Mi → 426Mi
- These are the actual changes that will be applied

### 3. Resize Operation Details

```
🔧 Resizing pod default/my-app:
  Container main: CPU request 100m→82m, Memory request 128Mi→426Mi
```

**What it means:**
- Detailed breakdown of changes per container
- Shows old → new values for each resource
- Multiple containers will be listed separately

### 4. Successful Resize

```
✅ Successfully resized pod default/my-app using in-place resize
```

**What it means:**
- The resize operation completed successfully
- "in-place" means the pod wasn't restarted
- Resources are now adjusted to the new values

## Understanding Metrics

### CPU Metrics

CPU is measured in:
- **m** (millicores): 1000m = 1 CPU core
- **Usage**: Actual CPU consumption
- **Request**: Guaranteed CPU allocation
- **Limit**: Maximum CPU allowed

Examples:
- `100m` = 0.1 CPU cores = 10% of one core
- `2000m` = 2 CPU cores
- `500m` = 0.5 CPU cores = 50% of one core

### Memory Metrics

Memory is measured in:
- **Mi** (Mebibytes): 1024 Mi = 1 Gi
- **Gi** (Gibibytes): 1024 Gi = 1 Ti
- **Usage**: Actual memory consumption
- **Request**: Guaranteed memory allocation
- **Limit**: Maximum memory allowed (OOM if exceeded)

Examples:
- `128Mi` = 128 Megabytes
- `1Gi` = 1024 Megabytes
- `512Mi` = 0.5 Gigabytes

## Scaling Decisions Explained

### Scale Up Triggers

```
🔍 Scaling analysis - CPU: scale up (usage: 450m, limit: 500m, 90.0%)
```

**Triggered when:**
- CPU usage > 80% of limit (default)
- Memory usage > 85% of limit (default)
- Indicates resource pressure

### Scale Down Triggers

```
🔍 Scaling analysis - CPU: scale down (usage: 50m, limit: 500m, 10.0%)
```

**Triggered when:**
- CPU usage < 30% of limit (default)
- Memory usage < 40% of limit (default)
- Indicates over-provisioning

### No Change

```
🔍 Scaling analysis - CPU: no change (usage: 250m, limit: 500m, 50.0%)
```

**When:**
- Usage is within acceptable range
- Between scale-down and scale-up thresholds

## Resource Calculation Logic

### Request Calculations

New requests are calculated using:
```
New Request = (Current Usage × Multiplier) + Addition
```

Default multipliers:
- **CPU Scale Up**: 1.2× usage + 50m
- **CPU Scale Down**: 1.1× usage + 50m
- **Memory Scale Up**: 1.3× usage + 100Mi
- **Memory Scale Down**: 1.1× usage + 100Mi

### Limit Calculations

New limits are calculated using:
```
New Limit = (New Request × Limit Multiplier) + Limit Addition
```

Default multipliers:
- **CPU Limit**: 2.0× request
- **Memory Limit**: 1.5× request

## Common Log Sequences

### Successful Resize Flow

```
1. 🔍 Scaling analysis for default/my-app - CPU: scale down (usage: 75m, limit: 500m, 15.0%)
2. 📈 Container default/my-app/main will be resized - CPU: 100m→82m
3. 🔧 Resizing pod default/my-app
4. ✅ Successfully resized pod default/my-app using in-place resize
```

### Skipped Resize (Memory Only Reduction)

```
1. 🔍 Scaling analysis for default/my-app - Memory: scale down (usage: 100Mi, limit: 1Gi, 10.0%)
2. ⏭️ Skipping resize for pod default/my-app: CPU doesn't need update and memory would be reduced
```

**Why skipped:** Memory-only reductions may require pod restart in some Kubernetes versions.

### Batch Processing

```
1. 📊 Found 10 resources needing adjustment
2. 🔄 Processing 10 pod updates in 2 batches (batch size: 5)
3. 📦 Processing batch 1/2 (5 pods)
4. 📦 Processing batch 2/2 (5 pods)
5. ✅ Completed processing all 10 pod updates
```

## Warning Messages

### Metrics Not Available

```
⚠️ Metrics not available for pod default/my-app: metrics-server not ready
```

**Causes:**
- Metrics-server is still initializing
- Pod just started (metrics need ~30s)
- Metrics-server addon not enabled

### Resource Constraints

```
⚠️ Cannot scale up default/my-app: would exceed namespace resource quota
```

**Causes:**
- Namespace ResourceQuota limits
- Node capacity constraints
- Cluster resource limits

### Skip Conditions

```
⏭️ Skipping pod kube-system/coredns: system namespace
```

**Common skip reasons:**
- System namespaces (kube-system, kube-public)
- Pods with `rightsizer.io/disable: true` annotation
- Pods not in Running state
- Static pods

## Error Messages

### Resize Failures

```
❌ Failed to resize pod default/my-app: Operation cannot be fulfilled: pod is being deleted
```

**Common causes:**
- Pod is terminating
- Deployment is updating
- Resource conflicts

### Metrics Errors

```
❌ Failed to fetch metrics for default/my-app: context deadline exceeded
```

**Common causes:**
- Metrics-server overloaded
- Network issues
- API server timeout

## Debugging Tips

### 1. Verbose Logging

Enable debug logging for more details:
```yaml
env:
- name: LOG_LEVEL
  value: "debug"
```

### 2. Check Metrics Availability

```bash
# Check if metrics are available for a pod
kubectl top pod my-app -n default

# Check metrics-server status
kubectl get pods -n kube-system | grep metrics-server
```

### 3. Verify Scaling Decisions

Look for this pattern to understand why scaling occurred:
```
🔍 Scaling analysis - CPU: scale up (usage: 450m, limit: 500m, 90.0%)
```

The percentage (90.0%) shows usage relative to limit, helping verify threshold triggers.

### 4. Track Resource Changes

Follow the change progression:
```
Old Request → New Request
100m → 82m  (CPU decreased by 18%)
128Mi → 426Mi (Memory increased by 233%)
```

### 5. Batch Processing Issues

If seeing delays, check batch logs:
```
🔄 Processing 100 pod updates in 20 batches (batch size: 5)
```

Large numbers of updates may take time. Consider adjusting batch size.

## Configuration Impact on Logs

### Dry Run Mode

When `dryRun: true`:
```
🔍 [DRY RUN] Would resize pod default/my-app - CPU: 100m→82m
```

### Namespace Filtering

With namespace restrictions:
```
⏭️ Skipping pod other-namespace/app: namespace not in watch list
```

### Resource Thresholds

Different thresholds produce different scaling patterns:
- Lower thresholds = more frequent scaling
- Higher thresholds = less sensitive to spikes

## Monitoring Patterns

### Healthy Operation

```
✅ Regular scaling decisions
✅ Successful resizes
✅ Balanced scale up/down operations
✅ No repeated failures
```

### Potential Issues

```
⚠️ Repeated scale up/down for same pod (flapping)
⚠️ Many skip messages (configuration issue?)
⚠️ No scaling decisions (metrics issue?)
❌ Consistent resize failures
```

## Log Aggregation Queries

### Prometheus/Loki Queries

Find all resize operations:
```
{app="right-sizer"} |= "Successfully resized"
```

Find scaling decisions:
```
{app="right-sizer"} |= "Scaling analysis"
```

Find errors:
```
{app="right-sizer"} |= "Failed to resize"
```

### kubectl Commands

Recent resize operations:
```bash
kubectl logs -n right-sizer-system deployment/right-sizer | grep "📈"
```

Scaling decisions in last hour:
```bash
kubectl logs -n right-sizer-system deployment/right-sizer --since=1h | grep "🔍"
```

## Summary

Understanding Right-Sizer logs helps you:
1. **Verify** that pods are being analyzed correctly
2. **Understand** why certain scaling decisions are made
3. **Track** resource adjustments over time
4. **Debug** issues when resizing fails
5. **Optimize** configuration based on patterns

The key is following the emoji indicators and understanding the flow:
`Analysis (🔍) → Planning (📈) → Action (🔧) → Result (✅/❌)`
