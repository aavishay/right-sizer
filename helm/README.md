# Right-Sizer Helm Chart

## 🎯 Overview

Right-Sizer is a Kubernetes operator that automatically optimizes pod resource allocations based on actual usage patterns. Using advanced algorithms and Kubernetes 1.33+ in-place resize capabilities, it ensures your applications have the resources they need while minimizing waste and reducing costs.

### Key Features

- **🚀 Zero-Downtime Resizing**: Leverages Kubernetes 1.33+ in-place pod resizing - no pod restarts required
- **💰 Cost Optimization**: Typical 30-50% reduction in cloud infrastructure costs
- **🎯 Intelligent Optimization**: Multiple sizing strategies (adaptive, conservative, aggressive)
- **📊 Multi-Source Metrics**: Supports both Metrics Server and Prometheus
- **🔒 Enterprise-Ready**: Comprehensive security, audit logging, and RBAC
- **⚡ High Performance**: Handles thousands of pods with minimal overhead

## 📋 Prerequisites

- Kubernetes 1.33 or higher (required for in-place pod resizing)
- Metrics Server or Prometheus installed in your cluster
- Helm 3.0 or higher

## 🚀 Installation

### Add Helm Repository

```bash
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update
```

### Install Chart

```bash
# Install with default values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace

# Install with custom values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --values values.yaml
```

### Install with Quick Configuration Profiles

```bash
# Development environment (aggressive optimization)
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --set config.defaultMode=aggressive \
  --set config.resizeInterval=5m

# Production environment (conservative approach)
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --set config.defaultMode=conservative \
  --set config.resizeInterval=30m \
  --set config.dryRun=true
```

## ⚙️ Configuration

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Container image repository | `aavishay/right-sizer` |
| `image.tag` | Container image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `config.enabled` | Enable the operator | `true` |
| `config.defaultMode` | Default optimization mode (adaptive/conservative/aggressive) | `balanced` |
| `config.resizeInterval` | How often to check for resize opportunities | `15m` |
| `config.dryRun` | Run in dry-run mode (no actual resizing) | `false` |
| `config.metricsSource` | Metrics source (metrics-server/prometheus) | `metrics-server` |
| `resources.limits.cpu` | CPU limit for operator | `500m` |
| `resources.limits.memory` | Memory limit for operator | `512Mi` |
| `resources.requests.cpu` | CPU request for operator | `100m` |
| `resources.requests.memory` | Memory request for operator | `128Mi` |
| `nodeSelector` | Node selector for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity rules for pod assignment | `{}` |

### View Default Values

```bash
helm show values right-sizer/right-sizer
```

### Custom Values File Example

Create a `values.yaml` file:

```yaml
config:
  enabled: true
  defaultMode: balanced
  resizeInterval: 10m
  dryRun: false
  
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 0.8
      limitMultiplier: 1.2
      minRequest: "10m"
      maxLimit: "4000m"
    memory:
      requestMultiplier: 0.9
      limitMultiplier: 1.1
      minRequest: "128Mi"
      maxLimit: "8Gi"

  globalConstraints:
    maxChangePercentage: 20
    cooldownPeriod: 5m
    maxConcurrentResizes: 10
    respectPDB: true
    respectHPA: true

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## 📊 Usage Examples

### Basic Usage

Once installed, Right-Sizer will automatically start monitoring and optimizing your workloads based on the default configuration.

### Creating Custom Policies

Create workload-specific optimization policies:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-critical
spec:
  enabled: true
  priority: 100
  mode: conservative
  targetRef:
    kind: Deployment
    namespaces:
      - production
    labelSelector:
      matchLabels:
        tier: critical
  resourceStrategy:
    cpu:
      requestMultiplier: 0.9
      limitMultiplier: 1.5
      targetUtilization: 70
    memory:
      requestMultiplier: 0.95
      limitMultiplier: 1.3
      targetUtilization: 80
  constraints:
    maxChangePercentage: 10
    cooldownPeriod: 30m
```

Apply the policy:

```bash
kubectl apply -f policy.yaml
```

### Namespace-Specific Configuration

Configure Right-Sizer for specific namespaces:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: namespace-config
spec:
  namespaceConfig:
    includeNamespaces:
      - production
      - staging
    excludeNamespaces:
      - kube-system
      - kube-public
```

## 🔍 Monitoring

### Check Operator Status

```bash
# Check if the operator is running
kubectl get pods -n right-sizer

# View operator logs
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer

# Check applied policies
kubectl get rightsizerpolicies -A

# View resize recommendations (dry-run mode)
kubectl get events -n your-namespace | grep RightSizer
```

### Metrics Endpoints

Right-Sizer exposes Prometheus metrics on port 8080:

- `/metrics` - Prometheus metrics
- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint

### Key Metrics

- `rightsizer_resize_operations_total` - Total number of resize operations
- `rightsizer_resize_operations_success` - Successful resize operations
- `rightsizer_resize_operations_failed` - Failed resize operations
- `rightsizer_resource_savings_percentage` - Estimated resource savings
- `rightsizer_pods_monitored` - Number of pods being monitored
- `rightsizer_policies_active` - Number of active policies

## 🆙 Upgrading

```bash
# Update the repository
helm repo update

# Upgrade the release
helm upgrade right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --values values.yaml
```

## 🗑️ Uninstalling

```bash
# Uninstall the release
helm uninstall right-sizer --namespace right-sizer

# Delete the namespace (optional)
kubectl delete namespace right-sizer

# Delete CRDs (if you want to completely remove Right-Sizer)
kubectl delete crd rightsizerpolicies.rightsizer.io
kubectl delete crd rightsizerconfigs.rightsizer.io
```

## ⚠️ Important Notes

### Requirements

- **Kubernetes 1.33+**: Required for in-place pod resizing feature
- **Metrics Source**: Either Metrics Server or Prometheus must be available
- **RBAC**: The operator requires cluster-wide permissions to monitor and resize pods

### Limitations

- Only supports Deployments, StatefulSets, and DaemonSets
- Requires containers to have InPlaceResizePolicy set (defaults to RestartPolicy in most cases)
- Some workloads may not be suitable for automatic resizing

### Safety Features

- **Dry-Run Mode**: Test configurations without making actual changes
- **Gradual Adjustments**: Incremental resource changes to prevent disruption
- **Respect for Constraints**: Honors PodDisruptionBudgets and HorizontalPodAutoscalers
- **Validation**: Ensures changes don't exceed node capacity or resource quotas

## 📖 Documentation

- [GitHub Repository](https://github.com/aavishay/right-sizer)
- [Full Documentation](https://github.com/aavishay/right-sizer/tree/main/docs)
- [Examples](https://github.com/aavishay/right-sizer/tree/main/examples)
- [Troubleshooting Guide](https://github.com/aavishay/right-sizer/blob/main/docs/TROUBLESHOOTING.md)
- [Contributing](https://github.com/aavishay/right-sizer/blob/main/docs/CONTRIBUTING.md)

## 🤝 Support

- **Issues**: [GitHub Issues](https://github.com/aavishay/right-sizer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aavishay/right-sizer/discussions)
- **Security**: Report security vulnerabilities to security@right-sizer.io

## 📜 License

This software is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

For commercial licensing options, please contact licensing@right-sizer.io.

## 🌟 Star History

If you find Right-Sizer useful, please consider giving us a star on [GitHub](https://github.com/aavishay/right-sizer)!

---

**Made with ❤️ by the Right-Sizer Community**