# Rate Limiting and API Server Protection Guide

## Overview

The Right Sizer operator includes comprehensive rate limiting and API server protection mechanisms to prevent overloading the Kubernetes API server. This is crucial for maintaining cluster stability, especially in large deployments.

## Why Rate Limiting Matters

Without proper rate limiting, an operator can:
- Generate hundreds or thousands of API calls per second
- Trigger API server throttling, affecting all cluster operations
- Cause cascading failures in other workloads
- Get blocked by Kubernetes' API Priority and Fairness mechanisms
- Degrade overall cluster performance

## Configuration Parameters

All rate limiting parameters are configurable through the `RightSizerConfig` CRD under the `operatorConfig` section.

### 1. Client-Side Rate Limiting

#### QPS (Queries Per Second)
- **Field**: `operatorConfig.qps`
- **Default**: `20`
- **Range**: `1-1000`
- **Description**: Maximum sustained request rate to the Kubernetes API server
- **Kubernetes Default**: `5` (we default to 20 for operator workloads)

```yaml
spec:
  operatorConfig:
    qps: 20
```

#### Burst
- **Field**: `operatorConfig.burst`
- **Default**: `30`
- **Range**: `1-1000`
- **Description**: Maximum burst capacity for short-term spikes above QPS
- **Kubernetes Default**: `10`
- **Recommendation**: Set to 1.5-2x your QPS value

```yaml
spec:
  operatorConfig:
    burst: 30
```

### 2. Controller Concurrency

#### MaxConcurrentReconciles
- **Field**: `operatorConfig.maxConcurrentReconciles`
- **Default**: `3`
- **Range**: `1-20`
- **Description**: Number of pods processed simultaneously per controller
- **Impact**: 
  - Lower values = Less API load, slower processing
  - Higher values = More API load, faster processing

```yaml
spec:
  operatorConfig:
    maxConcurrentReconciles: 3
```

### 3. Additional Rate Limiting Parameters

#### Worker Threads
- **Field**: `operatorConfig.workerThreads`
- **Default**: `5`
- **Range**: `1-50`
- **Description**: Number of worker threads for concurrent processing

#### Reconcile Interval
- **Field**: `operatorConfig.reconcileInterval`
- **Default**: `10m`
- **Description**: How often to re-reconcile resources
- **Format**: Duration string (e.g., "30s", "5m", "1h")

## Recommended Settings by Cluster Size

### Small Clusters (< 100 pods)
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: default
spec:
  operatorConfig:
    qps: 10
    burst: 15
    maxConcurrentReconciles: 2
    workerThreads: 3
```

### Medium Clusters (100-500 pods)
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: default
spec:
  operatorConfig:
    qps: 30
    burst: 50
    maxConcurrentReconciles: 5
    workerThreads: 8
```

### Large Clusters (> 500 pods)
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: default
spec:
  operatorConfig:
    qps: 50
    burst: 100
    maxConcurrentReconciles: 10
    workerThreads: 15
    reconcileInterval: "15m"  # Less frequent to reduce load
```

### Multi-Tenant/Shared Clusters
For clusters shared by multiple teams or with strict API server resource limits:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: default
spec:
  operatorConfig:
    qps: 5                       # Very conservative
    burst: 10
    maxConcurrentReconciles: 1   # Process one at a time
    workerThreads: 2
    reconcileInterval: "30m"     # Infrequent updates
```

## Built-in Protection Mechanisms

Beyond configurable parameters, the operator includes several built-in protections:

### 1. Batch Processing
- Processes maximum 50 pods per cycle
- Updates pods in batches of 5
- 2-second delay between batches
- 200ms delay between individual pod updates

### 2. Mutex Protection
- Prevents concurrent API calls
- Serializes pod update operations

### 3. Informer Caches
- Uses Kubernetes informer caches to reduce GET requests
- Watches resources instead of polling

### 4. Circuit Breaker
- Automatically backs off when errors occur
- Prevents cascade failures

## Monitoring Rate Limiting

### Check Current Configuration
```bash
kubectl get rightsizerconfig default -o yaml | grep -A 5 operatorConfig
```

### View Operator Logs
Look for rate limiting information in logs:
```bash
kubectl logs deployment/right-sizer | grep -E "Rate Limiting|QPS|Burst|Concurrent"
```

Example output:
```
2025/08/31 06:52:56 [INFO]    Rate Limiting: QPS=20, Burst=30
2025/08/31 06:52:56 [INFO]    Concurrency: MaxConcurrentReconciles=3
```

### Monitor API Server Metrics
If Prometheus is available, monitor these metrics:
- `apiserver_flowcontrol_rejected_requests_total` - Rejected requests due to flow control
- `apiserver_flowcontrol_current_inqueue_requests` - Requests currently queued
- `rest_client_request_duration_seconds` - Client request latency

## Troubleshooting

### Symptom: "Too many requests" errors
**Solution**: Reduce QPS and burst values
```yaml
spec:
  operatorConfig:
    qps: 5
    burst: 10
```

### Symptom: Slow resource processing
**Solution**: Increase concurrency carefully
```yaml
spec:
  operatorConfig:
    qps: 30
    burst: 45
    maxConcurrentReconciles: 5
```

### Symptom: API server CPU spikes
**Solution**: Reduce all rate limiting parameters and increase intervals
```yaml
spec:
  operatorConfig:
    qps: 10
    burst: 15
    maxConcurrentReconciles: 2
    reconcileInterval: "20m"
```

### Symptom: Priority and Fairness throttling
Check API server logs for priority and fairness messages, then adjust:
```yaml
spec:
  operatorConfig:
    qps: 15
    burst: 20
    maxConcurrentReconciles: 3
```

## Best Practices

1. **Start Conservative**: Begin with lower values and increase gradually
2. **Monitor Impact**: Watch API server metrics when adjusting
3. **Consider Peak Load**: Account for other workloads' API usage
4. **Use Leader Election**: For HA deployments to prevent duplicate API calls
5. **Adjust for Cluster Size**: Larger clusters can typically handle higher rates
6. **Test Changes**: Apply changes during maintenance windows first
7. **Document Settings**: Keep track of what works for your environment

## Dynamic Adjustment

The operator applies configuration changes dynamically without restart:

```bash
# Apply new configuration
kubectl apply -f rate-limiting-config.yaml

# Verify it was applied
kubectl logs deployment/right-sizer --tail=20
```

## Example: Applying Rate Limiting Configuration

1. Create configuration file:
```yaml
# rate-limiting.yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: default
spec:
  operatorConfig:
    qps: 25
    burst: 40
    maxConcurrentReconciles: 4
    logLevel: "info"
```

2. Apply configuration:
```bash
kubectl apply -f rate-limiting.yaml
```

3. Verify settings:
```bash
kubectl logs deployment/right-sizer | grep "Rate Limiting"
```

## Integration with Kubernetes API Priority and Fairness

The operator's rate limiting works alongside Kubernetes API Priority and Fairness (APF):

- **FlowSchema**: The operator's requests fall under the default flow schema
- **Priority Level**: Uses the standard priority level for workload requests
- **Queue Management**: Respects API server queue limits

To check if your requests are being throttled by APF:
```bash
kubectl get --raw /metrics | grep apiserver_flowcontrol
```

## Performance Tuning Guide

### For Fast Processing (High-Performance Clusters)
- QPS: 50-100
- Burst: 100-200
- MaxConcurrentReconciles: 10-15

### For Stability (Production Clusters)
- QPS: 20-30
- Burst: 30-50
- MaxConcurrentReconciles: 3-5

### For Minimal Impact (Sensitive Clusters)
- QPS: 5-10
- Burst: 10-20
- MaxConcurrentReconciles: 1-2

## Related Configuration

Rate limiting works in conjunction with other settings:

- **Global Constraints** (`globalConstraints.maxConcurrentResizes`): Limits actual resize operations
- **Monitoring Interval** (`monitoring.interval`): How often metrics are collected
- **Retry Configuration** (`operatorConfig.maxRetries`, `operatorConfig.retryInterval`): Failed operation handling

## Conclusion

Proper rate limiting configuration is essential for:
- Maintaining cluster stability
- Preventing API server overload
- Ensuring fair resource sharing
- Optimizing operator performance

Always monitor the impact of configuration changes and adjust based on your specific cluster characteristics and requirements.