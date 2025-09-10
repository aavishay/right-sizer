# Right-Sizer Helm Chart

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Version](https://img.shields.io/badge/Version-0.1.17-green.svg)](https://github.com/aavishay/right-sizer/releases)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.24%2B-326ce5)](https://kubernetes.io)
[![Helm](https://img.shields.io/badge/Helm-3.8%2B-0F1689)](https://helm.sh)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/right-sizer)](https://artifacthub.io/packages/search?repo=right-sizer)

**Intelligent Kubernetes Resource Optimization with Zero Downtime**

Right-Sizer automatically adjusts Kubernetes pod resources based on actual usage patterns, reducing costs by 20-40% while improving performance and stability.

## 🚀 TL;DR

```bash
# IMPORTANT: Install CRDs first (required)
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Add repository and install
helm repo add right-sizer https://aavishay.github.io/right-sizer
helm repo update
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --version 0.1.17
```

## 📋 Prerequisites

- **Kubernetes** 1.24+ (1.27+ for in-place pod resizing)
- **Helm** 3.8 or higher
- **Metrics Server** or **Prometheus** for resource metrics
- **Cluster admin permissions** for CRD installation

## 📦 Installation

### Step 1: Install Custom Resource Definitions (CRDs)

⚠️ **IMPORTANT**: CRDs must be installed before the Helm chart. This is a one-time operation per cluster.

```bash
# Install CRDs
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Verify CRDs are installed
kubectl get crd rightsizerconfigs.rightsizer.io
kubectl get crd rightsizerpolicies.rightsizer.io
```

### Step 2: Add Helm Repository

```bash
helm repo add right-sizer https://aavishay.github.io/right-sizer
helm repo update
```

### Step 3: Install Right-Sizer

```bash
# Install with default values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --version 0.1.17

# Install with custom values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --version 0.1.17 \
  --values custom-values.yaml
```

## ⚙️ Configuration

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.enabled` | Enable/disable right-sizing | `true` |
| `rightsizerConfig.mode` | Operating mode: adaptive, aggressive, balanced, conservative, custom | `balanced` |
| `rightsizerConfig.dryRun` | Preview changes without applying | `false` |
| `rightsizerConfig.featureGates.updateResizePolicy` | Enable in-place resizing (K8s 1.27+) | `false` |
| `rightsizerConfig.namespaceConfig.excludeNamespaces` | Namespaces to exclude | `[kube-system, kube-public, kube-node-lease]` |
| `rightsizerConfig.resourceDefaults.cpu.minRequest` | Minimum CPU request | `10m` |
| `rightsizerConfig.resourceDefaults.cpu.maxLimit` | Maximum CPU limit | `4000m` |
| `rightsizerConfig.resourceDefaults.memory.minRequest` | Minimum memory request | `64Mi` |
| `rightsizerConfig.resourceDefaults.memory.maxLimit` | Maximum memory limit | `8192Mi` |
| `rightsizerConfig.operationalConfig.resizeInterval` | Resize check interval | `5m` |
| `rightsizerConfig.sizingStrategy.algorithm` | Algorithm: percentile, peak, average | `percentile` |
| `rightsizerConfig.sizingStrategy.percentile` | Percentile value (if using percentile) | `95` |

### Example Values Files

#### Production Environment (Conservative)
```yaml
# values-production.yaml
rightsizerConfig:
  enabled: true
  mode: "conservative"
  dryRun: true  # Start with dry-run
  
  featureGates:
    updateResizePolicy: false  # Disabled by default for safety
  
  resourceDefaults:
    cpu:
      minRequest: "50m"
      maxLimit: "8000m"
    memory:
      minRequest: "128Mi"
      maxLimit: "16384Mi"
  
  sizingStrategy:
    algorithm: "percentile"
    percentile: 99
    lookbackPeriod: "14d"
    
  operationalConfig:
    resizeInterval: "30m"
    maxUpdatesPerRun: 10
    
  namespaceConfig:
    includeNamespaces:
      - "production"
    excludeNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"
      - "right-sizer"
```

#### Development Environment (Aggressive)
```yaml
# values-development.yaml
rightsizerConfig:
  enabled: true
  mode: "aggressive"
  dryRun: false
  
  featureGates:
    updateResizePolicy: true  # Enable for faster updates
  
  resourceDefaults:
    cpu:
      minRequest: "10m"
      maxLimit: "2000m"
    memory:
      minRequest: "32Mi"
      maxLimit: "4096Mi"
  
  sizingStrategy:
    algorithm: "average"
    lookbackPeriod: "1d"
    
  operationalConfig:
    resizeInterval: "5m"
    maxUpdatesPerRun: 50
    
  namespaceConfig:
    includeNamespaces:
      - "dev"
      - "staging"
```

## 🎯 Operating Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **adaptive** | Learns from workload patterns and adjusts strategy | General purpose, mixed workloads |
| **aggressive** | Minimizes resource allocation, frequent adjustments | Development, cost optimization |
| **balanced** | Moderate optimization with stability | Default, most workloads |
| **conservative** | Prioritizes stability, gradual changes | Production, critical services |
| **custom** | Full manual control over all parameters | Advanced users |

## 🚦 Feature Gates

| Feature | Default | K8s Version | Description |
|---------|---------|-------------|-------------|
| `updateResizePolicy` | `false` | 1.27+ | Enable in-place pod resizing without restarts |
| `enablePredictiveScaling` | `false` | All | ML-based predictive scaling (experimental) |
| `enableCostOptimization` | `false` | All | Cost-aware resource optimization |
| `enableMultiCluster` | `false` | All | Multi-cluster support (experimental) |

## 📊 Metrics and Monitoring

Right-Sizer exposes Prometheus metrics on port 9090:

```yaml
# ServiceMonitor for Prometheus Operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: right-sizer
  namespace: right-sizer
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: right-sizer
  endpoints:
  - port: metrics
    interval: 30s
```

Key metrics:
- `rightsizer_resize_operations_total` - Total resize operations
- `rightsizer_resize_failures_total` - Failed resize operations
- `rightsizer_resource_savings_percentage` - Estimated resource savings
- `rightsizer_pods_monitored` - Number of pods being monitored

## 🔧 Common Use Cases

### Enable for Specific Namespaces Only
```bash
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.namespaceConfig.includeNamespaces="{app1,app2,app3}"
```

### Dry-Run Mode for Testing
```bash
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.dryRun=true \
  --set rightsizerConfig.logging.level=debug
```

### Enable In-Place Resizing (K8s 1.27+)
```bash
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.featureGates.updateResizePolicy=true
```

### Custom Resource Limits
```bash
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.resourceDefaults.cpu.maxLimit="16000m" \
  --set rightsizerConfig.resourceDefaults.memory.maxLimit="32Gi"
```

## 🐛 Troubleshooting

### Issue: CRDs Not Found
```bash
Error: unable to build kubernetes objects from release manifest: 
resource mapping not found for name: "right-sizer-config" 
namespace: "" from "": no matches for kind "RightSizerConfig"
```

**Solution**: Install CRDs first (see Installation Step 1)

### Issue: Readiness Probe Failures
```bash
Warning  Unhealthy  kubelet  Readiness probe failed: Get "http://10.244.0.12:8081/readyz": 
context deadline exceeded
```

**Solution**: The operator needs time to initialize. Version 0.1.17+ includes improved probe configuration. If issues persist:
```bash
# Check operator logs
kubectl logs -n right-sizer deployment/right-sizer

# Increase resource limits if needed
helm upgrade right-sizer right-sizer/right-sizer \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=1024Mi
```

### Issue: Pods Not Being Resized
```bash
# Check if operator is enabled
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.enabled}'

# Check if in dry-run mode
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.dryRun}'

# Check operator logs
kubectl logs -n right-sizer deployment/right-sizer | grep -i error
```

## 📈 Upgrade Instructions

### Upgrade Chart
```bash
# Update repository
helm repo update

# Upgrade to latest version
helm upgrade right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --version 0.1.17
```

### Update CRDs (Manual)
```bash
# CRDs must be updated manually
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml
```

## 🗑️ Uninstallation

```bash
# Remove Helm release
helm uninstall right-sizer -n right-sizer

# Remove namespace
kubectl delete namespace right-sizer

# Remove CRDs (optional - will delete all configs)
kubectl delete crd rightsizerconfigs.rightsizer.io
kubectl delete crd rightsizerpolicies.rightsizer.io
```

## 📚 Documentation

- [GitHub Repository](https://github.com/aavishay/right-sizer)
- [Installation Guide](https://github.com/aavishay/right-sizer/blob/main/INSTALLATION_GUIDE.md)
- [Troubleshooting Guide](https://github.com/aavishay/right-sizer/blob/main/TROUBLESHOOTING_K8S.md)
- [Configuration Reference](https://github.com/aavishay/right-sizer/blob/main/docs/configuration.md)
- [API Documentation](https://github.com/aavishay/right-sizer/blob/main/docs/api/README.md)

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/aavishay/right-sizer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aavishay/right-sizer/discussions)
- **Slack**: [Join our community](https://right-sizer.slack.com)

## 📄 License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

---

**Made with ❤️ by the Right-Sizer Community**