# Known Limitations

This document describes known limitations of the Right-Sizer Operator and provides workarounds where applicable.

---

## 1. Memory Decrease Limitation ⚠️ IMPORTANT

### Issue

**Kubernetes 1.33 in-place pod resize does not support decreasing memory limits.** This is a platform limitation, not a Right-Sizer limitation.

### Technical Background

The Kubernetes in-place pod resize feature (introduced in K8s 1.27 as alpha, stable in 1.33) has the following constraint:

- ✅ **CPU can be increased or decreased** in-place
- ✅ **Memory can be increased** in-place
- ❌ **Memory cannot be decreased** in-place

This limitation exists because:
1. Memory cannot be safely reclaimed from a running process
2. Decreasing memory could cause OOM (Out of Memory) kills
3. The kernel cannot guarantee safe memory deallocation

**Reference:** [Kubernetes Enhancement Proposal (KEP-1287)](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1287-in-place-update-pod-resources)

### Behavior in Right-Sizer

When Right-Sizer detects that memory should be decreased:

1. ⚠️ **Warning logged** to operator logs
2. ✅ **CPU changes are still applied** if needed
3. 🔒 **Memory remains at current value**
4. ✅ **Pod continues running** without restart
5. 📊 **Metrics are updated** to reflect the decision

### Example Log Output

```
⚠️  Cannot decrease memory for pod default/nginx-abc123
   Memory limit: current=512Mi, desired=256Mi (decrease not allowed)
   Memory request: current=512Mi, desired=256Mi (decrease not allowed)
   💡 Applying CPU changes only (memory decreases require pod restart)
```

### Code Location

The memory decrease handling is implemented in:
- `go/controllers/adaptive_rightsizer.go` - `updatePodInPlace()` function
- Lines ~1100-1150 (memory limit decrease detection)

```go
// Check if memory limit is being decreased (not allowed for in-place resize)
currentMemLimit := currentResources.Limits.Memory()
newMemLimit := update.NewResources.Limits.Memory()

memoryLimitDecreased := currentMemLimit != nil && newMemLimit != nil &&
                        currentMemLimit.Cmp(*newMemLimit) > 0

if memoryLimitDecreased {
    log.Printf("⚠️  Cannot decrease memory for pod %s/%s",
               update.Namespace, update.Name)
    // Keep current memory values, apply CPU changes only
}
```

---

## Workarounds

### Option 1: Manual Pod Restart (Recommended for Critical Cases)

If memory decrease is critical and must be applied:

```bash
# Delete the pod - the controller will recreate it with new resources
kubectl delete pod <pod-name> -n <namespace>

# For Deployments/StatefulSets, the controller automatically recreates the pod
# The new pod will have the desired (lower) memory allocation
```

**Pros:**
- ✅ Guaranteed to apply new memory limits
- ✅ Simple and straightforward
- ✅ Works for all workload types

**Cons:**
- ❌ Causes brief downtime (pod restart)
- ❌ May violate PodDisruptionBudget
- ❌ Requires manual intervention

### Option 2: Update Parent Resource Directly

Update the Deployment/StatefulSet/DaemonSet directly:

```bash
# For Deployments
kubectl set resources deployment <name> \
  --limits=memory=256Mi \
  --requests=memory=256Mi \
  -n <namespace>

# For StatefulSets
kubectl set resources statefulset <name> \
  --limits=memory=256Mi \
  --requests=memory=256Mi \
  -n <namespace>

# This triggers a rolling update with new memory limits
```

**Pros:**
- ✅ Controlled rolling update
- ✅ Respects PodDisruptionBudget
- ✅ No manual pod deletion needed

**Cons:**
- ❌ Causes pod restarts (rolling update)
- ❌ Takes time to complete
- ❌ May not be immediate

### Option 3: Enable RestartContainer Policy

Configure the pod's resize policy to allow container restarts for memory changes:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - name: app
    image: myapp:latest
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi
    # Add resize policy
    resizePolicy:
    - resourceName: cpu
      restartPolicy: NotRequired  # CPU changes don't require restart
    - resourceName: memory
      restartPolicy: RestartContainer  # Allow restart for memory changes
```

**How it works:**
1. Right-Sizer requests memory decrease
2. Kubernetes restarts the container with new memory limits
3. Pod stays on the same node (faster than full pod restart)
4. Other containers in the pod continue running

**Pros:**
- ✅ Automatic handling by Kubernetes
- ✅ Faster than full pod restart
- ✅ Works with Right-Sizer automatically

**Cons:**
- ❌ Still causes container restart
- ❌ Requires pod spec modification
- ❌ Not all applications handle restarts gracefully

### Option 4: Configure Right-Sizer to Prevent Memory Decreases

Prevent Right-Sizer from attempting memory decreases:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  defaultResourceStrategy:
    memory:
      # Set high scale-down threshold to prevent decreases
      scaleDownThreshold: 0.1  # Only decrease if usage < 10%

      # Or use conservative mode
  defaultMode: conservative  # Less aggressive with decreases

  globalConstraints:
    # Prevent any memory decreases
    preventMemoryDecrease: true  # Future feature (not yet implemented)
```

**Pros:**
- ✅ Prevents the issue entirely
- ✅ No unexpected behavior
- ✅ Configurable per environment

**Cons:**
- ❌ May leave pods over-provisioned
- ❌ Doesn't optimize memory usage
- ❌ Wastes cluster resources

### Option 5: Use Vertical Pod Autoscaler (VPA) with Recreate Mode

Use VPA instead of Right-Sizer for memory-intensive workloads:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: "Recreate"  # Recreate pods when resizing
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 128Mi
      maxAllowed:
        memory: 2Gi
```

**Pros:**
- ✅ Handles memory decreases via pod recreation
- ✅ Native Kubernetes solution
- ✅ Well-tested and supported

**Cons:**
- ❌ Requires VPA installation
- ❌ Causes pod restarts
- ❌ Different configuration model

---

## Best Practices

### 1. Set Appropriate Initial Memory Limits

Start with reasonable memory limits to minimize the need for decreases:

```yaml
resources:
  requests:
    memory: 256Mi  # Start with realistic baseline
  limits:
    memory: 512Mi  # 2x requests for headroom
```

### 2. Use Conservative Mode for Production

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: production
spec:
  defaultMode: conservative
  defaultResourceStrategy:
    memory:
      requestMultiplier: 1.3  # 30% buffer
      scaleDownThreshold: 0.2  # Only decrease if usage < 20%
```

### 3. Monitor Memory Trends

Use Right-Sizer's prediction engine to anticipate memory needs:

```yaml
spec:
  featureGates:
    predictionEnabled: true
  defaultResourceStrategy:
    historyWindow: "7d"  # Analyze 7 days of data
    percentile: 95  # Use 95th percentile
```

### 4. Set Up Alerts

Alert when memory cannot be decreased:

```yaml
# Prometheus alert rule
- alert: RightSizerMemoryDecreaseBlocked
  expr: |
    increase(rightsizer_memory_adjustments_blocked_total[5m]) > 0
  annotations:
    summary: "Memory decrease blocked for {{ $labels.pod }}"
    description: "Consider manual intervention or pod restart"
```

### 5. Use Dry-Run Mode First

Test Right-Sizer behavior before applying changes:

```yaml
spec:
  dryRun: true  # Log recommendations without applying
```

---

## Future Improvements

### Kubernetes Platform

The Kubernetes community is aware of this limitation. Potential future improvements:

1. **Memory Compaction** - Kernel-level memory reclamation
2. **Graceful Memory Reduction** - Cooperative memory release
3. **Memory Overcommit** - Allow temporary over-allocation

**Track Progress:**
- [KEP-1287: In-place Update of Pod Resources](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1287-in-place-update-pod-resources)
- [Kubernetes Issue #102884](https://github.com/kubernetes/kubernetes/issues/102884)

### Right-Sizer Enhancements

Planned features to better handle memory limitations:

1. **Automatic Restart Policy** - Automatically add RestartContainer policy
2. **Smart Scheduling** - Schedule memory decreases during maintenance windows
3. **Gradual Decrease** - Decrease memory over multiple cycles
4. **Memory Pressure Detection** - Only decrease when safe

---

## Related Limitations

### 2. Init Containers Not Supported

**Issue:** Init containers cannot be resized in-place.

**Workaround:** Exclude pods with init containers:
```yaml
spec:
  namespaceConfig:
    excludeNamespaces: ["init-container-workloads"]
```

### 3. Ephemeral Containers Not Supported

**Issue:** Ephemeral containers (debug containers) cannot be resized.

**Workaround:** Right-Sizer automatically skips ephemeral containers.

### 4. QoS Class Changes

**Issue:** Cannot change from Guaranteed to Burstable QoS in-place.

**Workaround:** Configure QoS preservation:
```yaml
spec:
  globalConstraints:
    preserveGuaranteedQoS: true
```

### 5. Resource Quota Constraints

**Issue:** Resizes may be blocked by namespace resource quotas.

**Workaround:** Right-Sizer validates against quotas before resizing.

---

## Troubleshooting

### Problem: Memory decrease warnings in logs

**Symptoms:**
```
⚠️  Cannot decrease memory for pod default/nginx-abc123
```

**Solution:**
1. Check if memory decrease is necessary
2. Use one of the workarounds above
3. Consider adjusting scale-down thresholds

### Problem: Pods stuck with high memory allocation

**Symptoms:**
- Pods using 20% memory but allocated 2Gi
- Right-Sizer not decreasing memory

**Solution:**
```bash
# Option 1: Manual restart
kubectl delete pod <pod-name>

# Option 2: Rolling update
kubectl rollout restart deployment/<name>

# Option 3: Adjust configuration
kubectl edit rightsizerconfig default
```

### Problem: Frequent memory decrease attempts

**Symptoms:**
- Logs full of memory decrease warnings
- No actual changes being applied

**Solution:**
```yaml
# Increase scale-down threshold
spec:
  defaultResourceStrategy:
    memory:
      scaleDownThreshold: 0.3  # Only decrease if < 30% usage
```

---

## FAQ

**Q: Will this limitation be fixed in future Kubernetes versions?**

A: The Kubernetes community is working on improvements, but there's no definitive timeline. Memory reclamation is a complex kernel-level problem.

**Q: Does this affect CPU resizing?**

A: No, CPU can be increased or decreased in-place without any limitations.

**Q: Can I force memory decreases?**

A: Not in-place. You must restart the pod/container using one of the workarounds above.

**Q: Does this affect all workload types?**

A: Yes, this is a Kubernetes platform limitation that affects all pod types (Deployments, StatefulSets, DaemonSets, standalone Pods).

**Q: What about memory requests vs limits?**

A: Both memory requests and limits cannot be decreased in-place. The limitation applies to both.

**Q: Will my pods crash if I try to decrease memory?**

A: No, Right-Sizer detects this limitation and skips the memory decrease. Your pods continue running with current memory allocation.

---

## Additional Resources

- [Kubernetes In-Place Pod Resize Documentation](https://kubernetes.io/docs/concepts/workloads/pods/pod-resize/)
- [Right-Sizer Architecture Review](./ARCHITECTURE_REVIEW.md)
- [Runtime Testing Guide](./RUNTIME_TESTING_GUIDE.md)
- [Troubleshooting Guide](./troubleshooting-k8s.md)

---

**Last Updated:** 2024
**Applies to:** Right-Sizer v0.2.0+, Kubernetes 1.33+
