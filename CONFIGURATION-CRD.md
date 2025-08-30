# Right Sizer CRD-Based Configuration Guide

## Overview

The Right Sizer operator uses Kubernetes Custom Resource Definitions (CRDs) for all configuration management, providing a native Kubernetes approach to policy-based resource optimization. This guide covers the two primary CRDs: `RightSizerConfig` for global configuration and `RightSizerPolicy` for resource sizing policies.

## Why CRDs Instead of ConfigMaps?

- **Schema Validation**: CRDs provide OpenAPI schema validation ensuring configuration correctness
- **Versioning**: Built-in API versioning for smooth upgrades
- **RBAC Integration**: Fine-grained access control using standard Kubernetes RBAC
- **Status Tracking**: Native status subresources for observability
- **Kubectl Support**: Full kubectl integration with explain, describe, and custom columns
- **Admission Control**: Webhook validation and mutation support
- **Watch Capabilities**: Real-time configuration updates via Kubernetes watch API

## Prerequisites

Before configuring the Right Sizer operator, you must install the CRDs:

```bash
# Install CRDs
./scripts/install-crds.sh

# Or manually
kubectl apply -f helm/crds/rightsizer-crds.yaml

# Verify installation
kubectl get crd | grep rightsizer
```

## RightSizerConfig CRD

The `RightSizerConfig` CRD defines global configuration for the Right Sizer operator.

### Schema Overview

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config  # Singleton - only one instance should exist
spec:
  global:           # Global settings
  defaults:         # Default multipliers and boundaries
  features:         # Feature flags
  safety:           # Safety and reliability settings
  namespaceFilters: # Namespace inclusion/exclusion
  observability:    # Metrics and health checks
status:            # Managed by operator
```

### Global Settings

```yaml
spec:
  global:
    enabled: true                # Enable/disable the operator globally
    dryRun: false               # Log recommendations without applying
    resizeInterval: "30s"       # How often to evaluate pods
    metricsProvider: "kubernetes"  # "kubernetes" or "prometheus"
    prometheusUrl: "http://prometheus:9090"  # If using Prometheus
    logLevel: "info"            # debug, info, warn, error
```

### Default Resource Calculations

```yaml
spec:
  defaults:
    # Request calculation: actual_usage * multiplier
    cpuRequestMultiplier: 1.2      # 20% buffer for CPU requests
    memoryRequestMultiplier: 1.2   # 20% buffer for memory requests
    
    # Limit calculation: request * multiplier
    cpuLimitMultiplier: 2.0        # 2x burst capacity for CPU
    memoryLimitMultiplier: 2.0     # 2x burst capacity for memory
    
    # Absolute boundaries
    maxCpuLimit: "4000m"          # Maximum 4 cores
    maxMemoryLimit: "8Gi"         # Maximum 8GB
    minCpuRequest: "10m"          # Minimum 10 millicores
    minMemoryRequest: "64Mi"      # Minimum 64MB
```

### Feature Flags

```yaml
spec:
  features:
    enableInPlaceResize: true     # Use K8s 1.27+ in-place resize
    policyBasedSizing: true       # Enable policy evaluation
    historicalAnalysis: true      # Use historical trends
    admissionController: false    # Enable admission webhooks
    auditLogging: true           # Enable audit logs
    customMetrics:               # Additional metrics to consider
      - "requests_per_second"
      - "queue_depth"
```

### Safety Configuration

```yaml
spec:
  safety:
    maxRetries: 3                 # Retry failed operations
    retryInterval: "5s"          # Wait between retries
    safetyThreshold: 0.5         # Max 50% resource change
    historyDays: 7               # Days of history to retain
    circuitBreaker:
      enabled: true
      threshold: 5               # Failures before circuit opens
      timeout: "30s"             # Time before retry
```

### Namespace Filtering

```yaml
spec:
  namespaceFilters:
    include:                     # Only process these namespaces
      - production
      - staging
    exclude:                     # Never process these namespaces
      - kube-system
      - kube-public
      - kube-node-lease
```

### Complete Example

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  global:
    enabled: true
    dryRun: false
    resizeInterval: "30s"
    metricsProvider: "kubernetes"
    logLevel: "info"
  defaults:
    cpuRequestMultiplier: 1.2
    memoryRequestMultiplier: 1.2
    cpuLimitMultiplier: 2.0
    memoryLimitMultiplier: 2.0
    maxCpuLimit: "4000m"
    maxMemoryLimit: "8Gi"
    minCpuRequest: "10m"
    minMemoryRequest: "64Mi"
  features:
    enableInPlaceResize: true
    policyBasedSizing: true
    historicalAnalysis: true
    admissionController: false
    auditLogging: true
  safety:
    maxRetries: 3
    retryInterval: "5s"
    safetyThreshold: 0.5
    historyDays: 7
  namespaceFilters:
    include: []  # Empty means all namespaces
    exclude:
      - kube-system
      - kube-public
```

## RightSizerPolicy CRD

The `RightSizerPolicy` CRD defines resource sizing policies with sophisticated matching rules.

### Schema Overview

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: policy-name
spec:
  priority: 100          # Higher values take precedence
  enabled: true          # Enable/disable this policy
  description: ""        # Human-readable description
  selectors:            # Pod matching criteria
  actions:              # Resource sizing actions
  schedule:             # Time-based activation (optional)
status:                 # Managed by operator
```

### Selectors

Selectors determine which pods a policy applies to:

```yaml
spec:
  selectors:
    # Namespace matching
    namespaces: ["production", "staging"]
    excludeNamespaces: ["test"]
    
    # Label and annotation matching
    labels:
      app: nginx
      tier: frontend
    annotations:
      rightsizer.io/profile: high-performance
    
    # Workload type matching
    workloadTypes:
      - Deployment
      - StatefulSet
    
    # Container name patterns (supports wildcards)
    containerNames:
      - "nginx"
      - "*-sidecar"
    
    # Pod name regex
    podNameRegex: "^web-.*"
    
    # QoS class matching
    qosClass: "Guaranteed"  # Guaranteed, Burstable, BestEffort
```

### Actions

Actions define how resources are sized:

```yaml
spec:
  actions:
    # Skip processing entirely
    skip: false
    
    # Multipliers (override defaults)
    cpuMultiplier: 1.5
    memoryMultiplier: 2.0
    
    # Absolute boundaries
    minCPU: "100m"
    maxCPU: "8000m"
    minMemory: "256Mi"
    maxMemory: "16Gi"
    
    # Fixed values (bypass calculations)
    setCPURequest: "500m"
    setCPULimit: "2000m"
    setMemoryRequest: "1Gi"
    setMemoryLimit: "4Gi"
```

### Time-Based Scheduling

Policies can be activated based on time:

```yaml
spec:
  schedule:
    timeRanges:
      - start: "08:00"
        end: "18:00"
    daysOfWeek:
      - Monday
      - Tuesday
      - Wednesday
      - Thursday
      - Friday
    timezone: "America/New_York"
```

### Policy Examples

#### 1. Skip System Namespaces

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: skip-system-pods
spec:
  priority: 1000  # Highest priority
  enabled: true
  description: "Never resize system components"
  selectors:
    namespaces:
      - kube-system
      - kube-public
      - cert-manager
      - ingress-nginx
  actions:
    skip: true
```

#### 2. Production Database Workloads

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-databases
spec:
  priority: 500
  enabled: true
  description: "Enhanced resources for production databases"
  selectors:
    namespaces: ["production"]
    labels:
      app.kubernetes.io/component: database
    workloadTypes: ["StatefulSet"]
    qosClass: "Guaranteed"
  actions:
    cpuMultiplier: 1.5
    memoryMultiplier: 2.5
    minMemory: "2Gi"
    maxMemory: "32Gi"
```

#### 3. Development Environment

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: development-conservative
spec:
  priority: 100
  enabled: true
  description: "Conservative resources for development"
  selectors:
    namespaces: ["dev", "development"]
  actions:
    cpuMultiplier: 1.0
    memoryMultiplier: 1.1
    maxCPU: "1000m"
    maxMemory: "2Gi"
```

#### 4. Business Hours Scaling

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: business-hours-boost
spec:
  priority: 200
  enabled: true
  description: "Increased resources during business hours"
  selectors:
    namespaces: ["production"]
    labels:
      scaling: time-based
  schedule:
    timeRanges:
      - start: "08:00"
        end: "18:00"
    daysOfWeek: ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
    timezone: "America/New_York"
  actions:
    cpuMultiplier: 1.5
    memoryMultiplier: 1.3
```

#### 5. Sidecar Containers

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: minimal-sidecars
spec:
  priority: 150
  enabled: true
  description: "Minimal resources for sidecar containers"
  selectors:
    containerNames:
      - "*-sidecar"
      - "istio-proxy"
      - "envoy"
  actions:
    cpuMultiplier: 0.8
    memoryMultiplier: 0.9
    maxCPU: "200m"
    maxMemory: "256Mi"
```

## Managing CRDs

### Create Configuration

```bash
# Apply global configuration
kubectl apply -f - <<EOF
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  global:
    enabled: true
    dryRun: false
    resizeInterval: "30s"
EOF

# Apply a policy
kubectl apply -f - <<EOF
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: my-policy
spec:
  priority: 100
  enabled: true
  selectors:
    namespaces: ["default"]
  actions:
    cpuMultiplier: 1.2
EOF
```

### View Resources

```bash
# List all policies
kubectl get rightsizerpolicies
kubectl get rsp  # Using short name

# View specific policy
kubectl describe rsp my-policy

# View configuration
kubectl get rightsizerconfig
kubectl get rsc  # Using short name

# Get policies in YAML
kubectl get rsp -o yaml

# Watch for changes
kubectl get rsp -w
```

### Edit Resources

```bash
# Edit configuration
kubectl edit rsc right-sizer-config

# Edit policy
kubectl edit rsp my-policy

# Patch specific field
kubectl patch rsp my-policy --type='merge' -p '{"spec":{"enabled":false}}'
```

### Delete Resources

```bash
# Delete a policy
kubectl delete rsp my-policy

# Delete all policies
kubectl delete rsp --all

# Delete configuration (will use defaults)
kubectl delete rsc right-sizer-config
```

## Policy Evaluation Order

Policies are evaluated in priority order (highest first):

1. **Matching**: Pod is checked against all policy selectors
2. **Priority**: Highest priority matching policy wins
3. **Actions**: Winner's actions are applied
4. **Defaults**: Unspecified actions use RightSizerConfig defaults

Example evaluation:
- Pod matches policies with priorities: 100, 200, 50
- Policy with priority 200 is selected
- Its actions override defaults

## Status and Observability

### RightSizerConfig Status

```yaml
status:
  phase: Active              # Active, Updating, Error
  observedGeneration: 2      # Last processed generation
  lastReconciled: "2024-01-15T10:30:00Z"
  policiesLoaded: 15         # Number of active policies
  message: "Configuration active"
```

### RightSizerPolicy Status

```yaml
status:
  phase: Active              # Active, Inactive, Error
  lastApplied: "2024-01-15T10:30:00Z"
  matchedPods: 25           # Currently matching pods
  message: "Policy active and matching pods"
```

### Monitoring

```bash
# View policy metrics
kubectl top rsp

# Get events
kubectl get events --field-selector involvedObject.kind=RightSizerPolicy

# Check operator logs
kubectl logs -n right-sizer-system deployment/right-sizer
```

## Migration from Environment Variables

If migrating from environment variable configuration:

```bash
# 1. Create RightSizerConfig from environment variables
cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  defaults:
    cpuRequestMultiplier: ${CPU_REQUEST_MULTIPLIER:-1.2}
    memoryRequestMultiplier: ${MEMORY_REQUEST_MULTIPLIER:-1.2}
    cpuLimitMultiplier: ${CPU_LIMIT_MULTIPLIER:-2.0}
    memoryLimitMultiplier: ${MEMORY_LIMIT_MULTIPLIER:-2.0}
    maxCpuLimit: "${MAX_CPU_LIMIT:-4000m}"
    maxMemoryLimit: "${MAX_MEMORY_LIMIT:-8Gi}"
    minCpuRequest: "${MIN_CPU_REQUEST:-10m}"
    minMemoryRequest: "${MIN_MEMORY_REQUEST:-64Mi}"
EOF

# 2. Remove environment variables from deployment
kubectl set env deployment/right-sizer -n right-sizer-system --all-
```

## Best Practices

### 1. Policy Design

- **Start with high-priority skip rules** for system namespaces
- **Use specific selectors** to avoid unintended matches
- **Test with dryRun** before enabling policies
- **Document policies** with clear descriptions
- **Version control** all CRD configurations

### 2. Priority Guidelines

- 900-1000: System exclusions
- 500-899: Production critical workloads
- 200-499: Standard production workloads
- 100-199: Non-production environments
- 0-99: Default and fallback policies

### 3. Safety Measures

- Set conservative `safetyThreshold` initially
- Enable `circuitBreaker` for production
- Use `historicalAnalysis` for stable workloads
- Monitor `matchedPods` in policy status
- Review audit logs regularly

### 4. Resource Boundaries

- Always set `minCPU` and `minMemory` for critical workloads
- Use `maxCPU` and `maxMemory` to prevent runaway scaling
- Consider node capacity when setting limits
- Account for resource quotas in namespaces

### 5. Monitoring

- Watch policy status for errors
- Monitor the operator's metrics endpoint
- Set up alerts for circuit breaker trips
- Track resource utilization trends
- Review audit logs for unexpected changes

## Troubleshooting

### Policy Not Matching

```bash
# Check policy status
kubectl describe rsp my-policy

# Verify selectors
kubectl get pods -n target-namespace --show-labels

# Check operator logs
kubectl logs -n right-sizer-system deployment/right-sizer | grep my-policy
```

### Configuration Not Applied

```bash
# Check config status
kubectl get rsc right-sizer-config -o jsonpath='{.status}'

# Verify CRD installation
kubectl get crd rightsizerpolicies.rightsizer.io

# Check operator permissions
kubectl auth can-i get rightsizerpolicies --as=system:serviceaccount:right-sizer-system:right-sizer
```

### Validation Errors

```bash
# Test policy syntax
kubectl apply --dry-run=client -f my-policy.yaml

# View OpenAPI schema
kubectl explain rsp.spec.selectors

# Validate with kubectl
kubectl create --dry-run=server -f my-policy.yaml
```

## Advanced Topics

### Custom Metrics Integration

```yaml
spec:
  features:
    customMetrics:
      - "myapp_requests_per_second"
      - "myapp_queue_depth"
```

### Webhook Validation

Enable admission controller to validate resource changes:

```yaml
spec:
  features:
    admissionController: true
```

### Multi-Cluster Configuration

Use GitOps tools like ArgoCD or Flux to manage CRDs across clusters:

```yaml
# kustomization.yaml
resources:
  - rightsizer-config.yaml
  - policies/
patches:
  - target:
      kind: RightSizerConfig
      name: right-sizer-config
    patch: |-
      - op: replace
        path: /spec/global/dryRun
        value: true  # Dry-run in staging
```

## Summary

CRD-based configuration provides a robust, type-safe, and Kubernetes-native way to manage the Right Sizer operator. By leveraging CRDs instead of ConfigMaps, you gain:

- Strong schema validation
- RBAC integration
- Version control
- Status tracking
- Native kubectl support
- Real-time updates via watch API

For more information, see:
- [Policy Examples](examples/policy-rules-example.yaml)
- [RBAC Configuration](docs/RBAC.md)
- [Troubleshooting Guide](docs/RBAC-TROUBLESHOOTING.md)