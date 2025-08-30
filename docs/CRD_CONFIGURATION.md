# Right Sizer CRD-Based Configuration

## Overview

Right Sizer has been updated to use Custom Resource Definitions (CRDs) for all configuration, eliminating the need for environment variables and hardcoded policy rules. This provides a more Kubernetes-native way to manage the operator's configuration and policies.

## Key Changes

### What's Been Removed
- ❌ Environment variable configuration
- ❌ Hardcoded policy rules
- ❌ ConfigMap-based policies
- ❌ Static configuration files

### What's New
- ✅ `RightSizerConfig` CRD for global configuration
- ✅ `RightSizerPolicy` CRD for sizing policies
- ✅ Dynamic configuration updates without operator restart
- ✅ Namespace-scoped configuration
- ✅ Policy-based resource management

## Architecture

```
┌─────────────────────────────────────────┐
│         RightSizerConfig CRD            │
│   (Global Operator Configuration)       │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│        Right Sizer Operator             │
│  - Watches Config & Policy CRDs         │
│  - Applies configuration dynamically    │
│  - No environment variables needed      │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│        RightSizerPolicy CRDs            │
│   (Resource-specific policies)          │
└─────────────────────────────────────────┘
```

## CRD Specifications

### RightSizerConfig

The `RightSizerConfig` CRD manages the global configuration for the Right Sizer operator.

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: right-sizer-system
spec:
  # Core Configuration
  enabled: true                    # Enable/disable the operator globally
  defaultMode: "balanced"          # Default sizing mode: aggressive, balanced, conservative
  resizeInterval: "30s"            # How often to check and resize
  dryRun: false                    # If true, only log recommendations

  # Default Resource Strategy
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 1.2       # Multiply usage by this for requests
      requestAddition: 0           # Add this many millicores
      limitMultiplier: 2.0         # Multiply request by this for limits
      limitAddition: 0             # Add this many millicores
      minRequest: 10               # Minimum CPU request (millicores)
      maxLimit: 4000               # Maximum CPU limit (millicores)
    memory:
      requestMultiplier: 1.2       # Multiply usage by this for requests
      requestAddition: 0           # Add this many MB
      limitMultiplier: 2.0         # Multiply request by this for limits
      limitAddition: 0             # Add this many MB
      minRequest: 64               # Minimum memory request (MB)
      maxLimit: 8192               # Maximum memory limit (MB)
    historyWindow: "7d"            # How much history to consider
    percentile: 95                 # Percentile for calculations (50, 90, 95, 99)
    updateMode: "rolling"          # Update mode: immediate, rolling, scheduled

  # Global Constraints
  globalConstraints:
    maxChangePercentage: 50        # Max resource change per adjustment (%)
    minChangeThreshold: 5          # Min change to trigger update (%)
    cooldownPeriod: "5m"           # Wait between adjustments
    maxConcurrentResizes: 10       # Max simultaneous resizes
    maxRestartsPerHour: 3          # Max pod restarts per hour
    respectPDB: true               # Respect PodDisruptionBudgets
    respectHPA: true               # Don't conflict with HPA
    respectVPA: true               # Don't conflict with VPA

  # Metrics Configuration
  metricsConfig:
    provider: "metrics-server"     # metrics-server or prometheus
    prometheusEndpoint: ""         # Prometheus URL if using prometheus
    scrapeInterval: "30s"          # Metrics scrape interval
    retentionPeriod: "7d"          # Metrics retention period

  # Observability Configuration
  observabilityConfig:
    logLevel: "info"               # debug, info, warn, error
    logFormat: "json"              # json or text
    enableAuditLog: true           # Enable audit logging
    auditLogPath: "/var/log/right-sizer/audit.log"
    enableMetricsExport: true      # Enable Prometheus metrics
    metricsPort: 9090              # Metrics server port
    enableTracing: false           # Enable distributed tracing
    tracingEndpoint: ""            # Jaeger/Zipkin endpoint
    enableEvents: true             # Create Kubernetes events

  # Security Configuration
  securityConfig:
    enableAdmissionController: false  # Enable admission webhook
    admissionWebhookPort: 8443       # Webhook server port
    requireAnnotation: false          # Require annotation to resize
    annotationKey: "rightsizer.io/enable"
    enableMutatingWebhook: false     # Enable mutation
    enableValidatingWebhook: true    # Enable validation

  # Operator Configuration
  operatorConfig:
    leaderElection: false          # Enable leader election
    leaderElectionNamespace: ""    # Namespace for leader election
    maxRetries: 3                  # Max retries for operations
    retryInterval: "5s"            # Retry interval
    enableCircuitBreaker: true     # Enable circuit breaker
    circuitBreakerThreshold: 5     # Failure threshold
    reconcileInterval: "1m"        # Reconciliation interval
    workerThreads: 10              # Number of worker threads

  # Namespace Configuration
  namespaceConfig:
    includeNamespaces: []          # Empty = all namespaces
    excludeNamespaces:             # Namespaces to exclude
      - kube-system
      - kube-public
    systemNamespaces:              # Always excluded
      - kube-system
      - kube-public
      - kube-node-lease

  # Feature Gates
  featureGates:
    EnableInPlaceResize: "false"   # Enable in-place resizing (K8s 1.33+)
    EnablePredictiveScaling: "false"
    EnableCostOptimization: "false"
```

### RightSizerPolicy

The `RightSizerPolicy` CRD defines specific sizing policies for resources.

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-policy
  namespace: right-sizer-system
spec:
  enabled: true                    # Enable this policy
  priority: 100                    # Higher priority wins (0-1000)
  mode: "conservative"             # Sizing mode for this policy
  dryRun: false                    # Override global dry-run

  # Target Selection
  targetRef:
    kind: "Deployment"             # Deployment, StatefulSet, DaemonSet, Pod
    apiVersion: "apps/v1"          # API version
    namespaces:                    # Target namespaces
      - production
      - staging
    excludeNamespaces:             # Exclude these namespaces
      - test
    labelSelector:                 # Select by labels
      matchLabels:
        environment: production
        tier: backend
    annotationSelector:            # Select by annotations
      rightsizer.io/profile: "high-performance"
    names:                         # Specific resource names
      - critical-app
    excludeNames:                  # Exclude specific names
      - debug-pod

  # Resource Strategy (overrides global)
  resourceStrategy:
    cpu:
      requestMultiplier: 1.5
      limitMultiplier: 2.5
      minRequest: 100
      maxLimit: 8000
      targetUtilization: 70        # Target CPU utilization %
    memory:
      requestMultiplier: 1.3
      limitMultiplier: 1.8
      minRequest: 256
      maxLimit: 16384
      targetUtilization: 80        # Target memory utilization %
    metricsSource: "prometheus"    # Override metrics source
    prometheusConfig:
      url: "http://prometheus:9090"
      cpuQuery: "rate(container_cpu_usage_seconds_total[5m])"
      memoryQuery: "container_memory_working_set_bytes"
    historyWindow: "14d"
    percentile: 99
    updateMode: "scheduled"

  # Scheduling
  schedule:
    interval: "5m"                 # Evaluation interval
    cronSchedule: "*/30 * * * *"  # Alternative: cron schedule
    timeWindows:                   # Active time windows
      - start: "08:00"
        end: "18:00"
        daysOfWeek:
          - Monday
          - Tuesday
          - Wednesday
          - Thursday
          - Friday
        timezone: "America/New_York"

  # Constraints (overrides global)
  constraints:
    maxChangePercentage: 25
    minChangeThreshold: 10
    cooldownPeriod: "15m"
    maxRestartsPerHour: 2
    respectPDB: true
    respectHPA: true
    respectVPA: false

  # Webhooks for notifications
  webhooks:
    - url: "https://hooks.slack.com/services/xxx"
      events:
        - resize
        - error
      headers:
        Content-Type: "application/json"
      retryPolicy:
        maxRetries: 3
        retryInterval: "5s"

  # Annotations to add to resized resources
  resourceAnnotations:
    rightsizer.io/last-resize: "{{ .Timestamp }}"
    rightsizer.io/policy: "{{ .PolicyName }}"
```

## Installation

### 1. Install CRDs

```bash
kubectl apply -f helm/crds/
```

### 2. Deploy with Helm

```bash
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace \
  --set createDefaultConfig=true
```

### 3. Verify Installation

```bash
# Check CRDs
kubectl get crd | grep rightsizer

# Check operator
kubectl get pods -n right-sizer-system

# Check default config
kubectl get rightsizerconfig -n right-sizer-system

# Check policies
kubectl get rightsizerpolicy -A
```

## Usage Examples

### Example 1: Basic Configuration

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: basic-config
spec:
  enabled: true
  resizeInterval: "1m"
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 1.3
      limitMultiplier: 2.0
    memory:
      requestMultiplier: 1.2
      limitMultiplier: 1.5
```

### Example 2: Development Environment Policy

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: dev-policy
spec:
  enabled: true
  priority: 50
  mode: "aggressive"
  targetRef:
    kind: "Deployment"
    namespaces: ["development"]
  resourceStrategy:
    cpu:
      requestMultiplier: 1.1
      maxLimit: 1000
    memory:
      requestMultiplier: 1.1
      maxLimit: 2048
  schedule:
    interval: "2m"
```

### Example 3: Production Database Policy

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: database-policy
spec:
  enabled: true
  priority: 200
  mode: "conservative"
  targetRef:
    kind: "StatefulSet"
    labelSelector:
      matchLabels:
        app: postgresql
  resourceStrategy:
    cpu:
      requestMultiplier: 2.0
      minRequest: 500
    memory:
      requestMultiplier: 1.5
      minRequest: 2048
  constraints:
    maxChangePercentage: 10
    cooldownPeriod: "30m"
    maxRestartsPerHour: 1
```

### Example 4: Time-based Policy

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: business-hours-policy
spec:
  enabled: true
  priority: 150
  targetRef:
    kind: "Deployment"
    annotationSelector:
      workload-type: "business-app"
  resourceStrategy:
    cpu:
      requestMultiplier: 1.5  # Higher during business hours
  schedule:
    timeWindows:
      - start: "08:00"
        end: "18:00"
        daysOfWeek: ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
        timezone: "Europe/London"
```

## Monitoring and Troubleshooting

### Check Configuration Status

```bash
# Get config status
kubectl get rightsizerconfig default -o yaml

# Check if configuration is applied
kubectl describe rightsizerconfig default

# View operator logs
kubectl logs -n right-sizer-system deployment/right-sizer
```

### Check Policy Status

```bash
# List all policies
kubectl get rightsizerpolicy -A

# Check policy details
kubectl describe rightsizerpolicy my-policy

# View policy status
kubectl get rightsizerpolicy my-policy -o jsonpath='{.status}'
```

### Common Issues

1. **Configuration not applied**
   - Check operator logs for errors
   - Verify CRD is valid: `kubectl explain rightsizerconfig.spec`
   - Ensure operator has proper RBAC permissions

2. **Policies not working**
   - Check policy priority (higher wins)
   - Verify target selectors match resources
   - Check if policy is enabled
   - Review constraints and cooldown periods

3. **No resource changes**
   - Check if dry-run mode is enabled
   - Verify metrics provider is working
   - Check minimum change thresholds
   - Review operator logs for decisions

## Migration from Environment Variables

If you're migrating from the environment variable-based configuration:

1. **Create RightSizerConfig CRD** with your previous settings:
   ```yaml
   apiVersion: rightsizer.io/v1alpha1
   kind: RightSizerConfig
   metadata:
     name: migrated-config
   spec:
     # Transfer your env var settings here
     defaultResourceStrategy:
       cpu:
         requestMultiplier: 1.2  # Was CPU_REQUEST_MULTIPLIER
         requestAddition: 0      # Was CPU_REQUEST_ADDITION
       # ... etc
   ```

2. **Remove environment variables** from your deployment
3. **Apply the CRD**: `kubectl apply -f config.yaml`
4. **Verify**: Check operator logs to confirm configuration is loaded

## Benefits of CRD-Based Configuration

1. **Dynamic Updates**: Change configuration without restarting the operator
2. **GitOps Compatible**: Store configuration as Kubernetes manifests
3. **Namespace Scoped**: Different configurations per namespace
4. **Policy-Based**: Fine-grained control over different workloads
5. **Validation**: CRD schema validation prevents invalid configurations
6. **Auditable**: Configuration changes tracked in Kubernetes audit logs
7. **Declarative**: Configuration as code, version controlled
8. **No Secrets in Env**: Sensitive data can use Kubernetes secrets references

## Advanced Features

### Multi-Cluster Configuration

Deploy different configurations per cluster:

```yaml
# Production cluster
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: prod-config
spec:
  defaultMode: "conservative"
  dryRun: false
  # ... production settings

---
# Development cluster
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: dev-config
spec:
  defaultMode: "aggressive"
  dryRun: true  # Test mode in dev
  # ... development settings
```

### Policy Inheritance

Policies can override global configuration:

```
Global Config (RightSizerConfig)
    ↓
Policy 1 (Priority: 100) - Overrides for specific workloads
    ↓
Policy 2 (Priority: 200) - Higher priority, wins conflicts
```

### Integration with CI/CD

```yaml
# ci/cd pipeline
steps:
  - name: Deploy Config
    run: |
      kubectl apply -f configs/rightsizer-config.yaml
      kubectl wait --for=condition=Active \
        rightsizerconfig/default -n right-sizer-system
  
  - name: Deploy Policies
    run: |
      kubectl apply -f policies/
      kubectl get rightsizerpolicy -A
```

## Best Practices

1. **Start with dry-run mode** to observe recommendations
2. **Use conservative settings** for production workloads
3. **Set appropriate cooldown periods** to avoid thrashing
4. **Monitor metrics** after enabling policies
5. **Use priority levels** to manage policy conflicts
6. **Document your policies** with annotations
7. **Test policies** in development before production
8. **Set resource limits** to prevent runaway scaling
9. **Use time windows** for predictable workload patterns
10. **Enable audit logging** for compliance and debugging

## Support

For issues or questions:
- Check operator logs: `kubectl logs -n right-sizer-system deployment/right-sizer`
- Review CRD documentation: `kubectl explain rightsizerconfig`
- File issues: [GitHub Issues](https://github.com/right-sizer/right-sizer/issues)