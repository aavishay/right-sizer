# Right Sizer CRD Migration Complete ✅

## Migration Summary

The Right Sizer operator has been successfully migrated from environment variable-based configuration to a fully CRD-based configuration system. This is a breaking change that provides significant improvements in flexibility, maintainability, and Kubernetes-native operations.

## What Changed

### 🔄 Configuration System
- **Before**: Configuration via environment variables in deployment
- **After**: Configuration via `RightSizerConfig` CRD

### 🔄 Policy Management
- **Before**: Hardcoded policy rules in the operator code
- **After**: Dynamic policies via `RightSizerPolicy` CRD

### 🔄 Updates
- **Before**: Restart required for configuration changes
- **After**: Dynamic updates without restarts

## Key Files Modified

### Core Changes
- `go/config/config.go` - Complete rewrite for CRD-based config
- `go/main.go` - Removed env vars, added CRD controllers
- `go/controllers/rightsizerconfig_controller.go` - Handles config CRD
- `go/controllers/rightsizerpolicy_controller.go` - Handles policy CRD

### Helm Chart Updates
- `helm/templates/deployment.yaml` - Removed all config env vars
- `helm/values.yaml` - Restructured for CRD configuration
- `helm/templates/config/rightsizerconfig.yaml` - Default config template
- `helm/templates/policies/example-policies.yaml` - Example policies

### New CRDs
- `helm/crds/rightsizer.io_rightsizerconfigs.yaml`
- `helm/crds/rightsizer.io_rightsizerpolicies.yaml`

## Quick Start Guide

### 1. Deploy the Operator

```bash
# Install CRDs
kubectl apply -f helm/crds/

# Deploy with Helm
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace \
  --set createDefaultConfig=true
```

### 2. Verify Installation

```bash
# Check operator is running
kubectl get pods -n right-sizer-system

# Check default configuration
kubectl get rightsizerconfig -n right-sizer-system

# View configuration details
kubectl describe rightsizerconfig default -n right-sizer-system
```

### 3. Create Custom Configuration

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: production-config
  namespace: right-sizer-system
spec:
  enabled: true
  resizeInterval: "5m"
  dryRun: false
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 1.3
      limitMultiplier: 2.0
      minRequest: 100
      maxLimit: 4000
    memory:
      requestMultiplier: 1.2
      limitMultiplier: 1.8
      minRequest: 256
      maxLimit: 8192
  observabilityConfig:
    logLevel: "info"
    enableMetricsExport: true
    metricsPort: 9090
    enableAuditLog: true
  namespaceConfig:
    excludeNamespaces:
      - kube-system
      - kube-public
```

### 4. Create Sizing Policies

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: web-apps-policy
  namespace: right-sizer-system
spec:
  enabled: true
  priority: 100
  mode: "balanced"
  targetRef:
    kind: "Deployment"
    labelSelector:
      matchLabels:
        app-type: "web"
  resourceStrategy:
    cpu:
      requestMultiplier: 1.2
      targetUtilization: 70
    memory:
      requestMultiplier: 1.3
      targetUtilization: 80
  schedule:
    interval: "2m"
  constraints:
    maxChangePercentage: 30
    cooldownPeriod: "10m"
```

## Migration from Environment Variables

If you're upgrading from the previous version:

### Step 1: Map Your Environment Variables

| Old Environment Variable | New CRD Field |
|-------------------------|---------------|
| `CPU_REQUEST_MULTIPLIER` | `spec.defaultResourceStrategy.cpu.requestMultiplier` |
| `MEMORY_REQUEST_MULTIPLIER` | `spec.defaultResourceStrategy.memory.requestMultiplier` |
| `CPU_LIMIT_MULTIPLIER` | `spec.defaultResourceStrategy.cpu.limitMultiplier` |
| `MEMORY_LIMIT_MULTIPLIER` | `spec.defaultResourceStrategy.memory.limitMultiplier` |
| `MIN_CPU_REQUEST` | `spec.defaultResourceStrategy.cpu.minRequest` |
| `MIN_MEMORY_REQUEST` | `spec.defaultResourceStrategy.memory.minRequest` |
| `MAX_CPU_LIMIT` | `spec.defaultResourceStrategy.cpu.maxLimit` |
| `MAX_MEMORY_LIMIT` | `spec.defaultResourceStrategy.memory.maxLimit` |
| `RESIZE_INTERVAL` | `spec.resizeInterval` |
| `LOG_LEVEL` | `spec.observabilityConfig.logLevel` |
| `DRY_RUN` | `spec.dryRun` |
| `KUBE_NAMESPACE_INCLUDE` | `spec.namespaceConfig.includeNamespaces` |
| `KUBE_NAMESPACE_EXCLUDE` | `spec.namespaceConfig.excludeNamespaces` |

### Step 2: Create Your Config CRD

Create a `rightsizerconfig.yaml` with your settings and apply it:

```bash
kubectl apply -f rightsizerconfig.yaml
```

### Step 3: Update Helm Deployment

Remove the old deployment and install the new version:

```bash
# Uninstall old version
helm uninstall right-sizer -n right-sizer-system

# Install new version
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --values your-values.yaml
```

## Benefits of CRD-Based Configuration

### 🚀 Dynamic Updates
- Change configuration without restarting the operator
- Instant policy updates
- No downtime for configuration changes

### 🎯 GitOps Ready
- Configuration as code in Git
- Version controlled changes
- Easy rollbacks

### 🔒 Better Security
- No sensitive data in environment variables
- RBAC-controlled configuration access
- Audit trail for all changes

### 📊 Enhanced Monitoring
- Configuration status in Kubernetes resources
- Events for configuration changes
- Metrics for configuration health

### 🎨 Flexible Policies
- Multiple policies with priorities
- Namespace-specific configurations
- Time-based policy scheduling
- Fine-grained resource targeting

## Testing the New System

### Run Integration Tests

```bash
# Run the CRD configuration test
./tests/integration/test-crd-config.sh

# Run unit tests
cd go && go test ./... -v
```

### Manual Testing

1. Deploy a test application:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  labels:
    app: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: app
        image: nginx
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

2. Watch for resource adjustments:
```bash
kubectl get pods -w -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory
```

## Troubleshooting

### Configuration Not Applied
```bash
# Check operator logs
kubectl logs -n right-sizer-system deployment/right-sizer

# Check config status
kubectl get rightsizerconfig -A
kubectl describe rightsizerconfig default -n right-sizer-system
```

### Policies Not Working
```bash
# List all policies
kubectl get rightsizerpolicy -A

# Check policy status
kubectl describe rightsizerpolicy <policy-name> -n right-sizer-system

# Check if resources match policy selectors
kubectl get deployments --show-labels
```

### No Resource Changes
- Verify dry-run mode is disabled
- Check minimum change thresholds
- Review cooldown periods
- Ensure metrics provider is working

## Documentation

- **CRD Configuration Guide**: `docs/CRD_CONFIGURATION.md`
- **API Reference**: `kubectl explain rightsizerconfig.spec`
- **Examples**: `helm/templates/config/` and `helm/templates/policies/`
- **Migration Guide**: This document

## Support

For issues or questions:
- Review the [CRD Configuration Guide](docs/CRD_CONFIGURATION.md)
- Check operator logs: `kubectl logs -n right-sizer-system deployment/right-sizer`
- File issues on GitHub with the `crd-config` label

## Next Steps

1. **Deploy to Development**: Test the new configuration system
2. **Create Policies**: Define policies for your workloads
3. **Monitor**: Watch resource adjustments and metrics
4. **Optimize**: Fine-tune multipliers and thresholds
5. **Scale**: Apply to production workloads

---

**Commit**: `bc46d75` - feat: migrate to CRD-based configuration, remove env vars and policy rules

**Status**: ✅ Successfully migrated and tested

**Date**: 2024