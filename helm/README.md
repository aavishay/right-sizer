# Right-Sizer Operator

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Version](https://img.shields.io/badge/Version-0.1.3-green.svg)](https://github.com/aavishay/right-sizer/releases)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.33%2B-326ce5)](https://kubernetes.io)
[![Helm](https://img.shields.io/badge/Helm-3.0%2B-0F1689)](https://helm.sh)

**Intelligent Kubernetes Resource Optimization with Zero Downtime**

[📖 Full Documentation](https://github.com/aavishay/right-sizer) | [🐛 Issues](https://github.com/aavishay/right-sizer/issues) | [💬 Discussions](https://github.com/aavishay/right-sizer/discussions)

## 🚀 Quick Start

### Prerequisites
- Kubernetes 1.33+
- Helm 3.0+
- Metrics Server or Prometheus

### Installation

```bash
# Add the Helm repository
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update

# Install with default configuration
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace

# Install with custom values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  -f custom-values.yaml
```

### Verify Installation

```bash
# Check operator status
kubectl get pods -n right-sizer

# View operator logs
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer

# Check CRDs
kubectl get rightsizerconfigs
kubectl get rightsizerpolicies
```

## ⚙️ Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Docker image repository | `aavishay/right-sizer` |
| `image.tag` | Docker image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of operator replicas | `1` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `right-sizer` |
| `rbac.create` | Create RBAC resources | `true` |
| `metrics.enabled` | Enable Prometheus metrics | `true` |
| `webhook.enabled` | Enable admission webhooks | `false` |
| `rightsizerConfig.create` | Create default RightSizerConfig | `true` |
| `rightsizerConfig.enabled` | Enable right-sizing | `true` |
| `rightsizerConfig.mode` | Operating mode (adaptive/aggressive/conservative) | `adaptive` |
| `rightsizerConfig.dryRun` | Dry-run mode (preview only) | `false` |

### RightSizerConfig Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.resourceDefaults.cpu.minRequest` | Minimum CPU request | `10m` |
| `rightsizerConfig.resourceDefaults.cpu.maxLimit` | Maximum CPU limit | `4000m` |
| `rightsizerConfig.resourceDefaults.memory.minRequest` | Minimum memory request | `64Mi` |
| `rightsizerConfig.resourceDefaults.memory.maxLimit` | Maximum memory limit | `8Gi` |
| `rightsizerConfig.sizingStrategy.algorithm` | Algorithm (percentile/peak/average) | `percentile` |
| `rightsizerConfig.sizingStrategy.percentile` | Percentile value (if using percentile) | `95` |
| `rightsizerConfig.operationalConfig.resizeMode` | Resize mode (InPlace/Rolling) | `InPlace` |
| `rightsizerConfig.operationalConfig.resizeInterval` | How often to check resources | `5m` |
| `rightsizerConfig.namespaceConfig.excludeNamespaces` | Namespaces to exclude | `[kube-system, kube-public]` |

### Example Configuration</text>

<old_text line=75>
```yaml
# values.yaml
image:
  repository: aavishay/right-sizer
  tag: "0.1.3"
  pullPolicy: IfNotPresent

replicaCount: 1

serviceAccount:
  create: true
  name: right-sizer

rbac:
  create: true

metrics:
  enabled: true

webhook:
  enabled: false

# Custom resource configuration
config:
  enabled: true
  mode: balanced
  resizeInterval: "30s"
  dryRun: false
```

```yaml
# values.yaml
image:
  repository: aavishay/right-sizer
  tag: "0.1.3"
  pullPolicy: IfNotPresent

replicaCount: 1

serviceAccount:
  create: true
  name: right-sizer

rbac:
  create: true

metrics:
  enabled: true

webhook:
  enabled: false

# Custom resource configuration
config:
  enabled: true
  mode: balanced
  resizeInterval: "30s"
  dryRun: false
```

## 📊 Key Features

### 🚀 Core Functionality
- **Zero-downtime resizing** with Kubernetes 1.33+ in-place updates
- **Multi-strategy optimization**: Adaptive, conservative, aggressive modes
- **Multi-source metrics**: Metrics Server and Prometheus support
- **Intelligent validation**: Respects node capacity and quotas

### 🧠 Intelligence & Safety
- **CRD-based configuration** for native Kubernetes management
- **Priority-based policies** with fine-grained control
- **Safety thresholds** and configurable guardrails
- **Audit logging** for compliance and troubleshooting

### 📈 Observability
- **Prometheus metrics** for monitoring and alerting
- **Health endpoints** for liveness and readiness probes
- **Comprehensive logging** with configurable levels
- **Grafana dashboard** templates included

## 🔧 Usage Examples

### Install with Different Profiles

```bash
# Conservative mode for production
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.mode=conservative \
  --set rightsizerConfig.sizingStrategy.algorithm=peak \
  --set rightsizerConfig.operationalConfig.resizeInterval=30m

# Aggressive mode for development
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.mode=aggressive \
  --set rightsizerConfig.sizingStrategy.algorithm=average \
  --set rightsizerConfig.operationalConfig.resizeInterval=1m

# Dry-run mode for testing
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.dryRun=true \
  --set rightsizerConfig.logging.level=debug
```

### Basic Configuration
The Helm chart automatically creates a default RightSizerConfig:
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: right-sizer-config
spec:
  enabled: true
  mode: adaptive
  dryRun: false
  
  resourceDefaults:
    cpu:
      minRequest: "10m"
      maxLimit: "4000m"
    memory:
      minRequest: "64Mi"
      maxLimit: "8Gi"
  
  sizingStrategy:
    algorithm: "percentile"
    percentile: 95
  
  operationalConfig:
    resizeMode: "InPlace"
    resizeInterval: "5m"
```</text>


### Workload-Specific Policy
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
    namespaces: ["production"]
    labelSelector:
      matchLabels:
        tier: critical
```

## 📋 Requirements

- **Kubernetes**: 1.33+ (for in-place pod resizing)
- **Helm**: 3.0+
- **Metrics**: Metrics Server 0.5+ or Prometheus
- **Resources**: 2GB RAM, 1 CPU core minimum

## 🔍 Troubleshooting

### Common Issues

**Pods not resizing:**
```bash
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer
```

**Permission errors:**
```bash
kubectl apply -f helm/templates/rbac.yaml
```

**Metrics not available:**
```bash
kubectl top pods  # Check if metrics-server is working
```

### Debug Commands
```bash
# View operator status
kubectl get pods -n right-sizer

# Check CRDs
kubectl get rightsizerconfigs -A
kubectl get rightsizerpolicies -A

# View events
kubectl get events -n right-sizer --sort-by='.lastTimestamp'
```

## 📚 Documentation

- [📖 Full Documentation](https://github.com/aavishay/right-sizer)
- [🚀 Quick Start Guide](https://github.com/aavishay/right-sizer#quick-start)
- [⚙️ Configuration Guide](https://github.com/aavishay/right-sizer#configuration)
- [🔍 Troubleshooting](https://github.com/aavishay/right-sizer#troubleshooting)
- [🤝 Contributing](https://github.com/aavishay/right-sizer/blob/main/docs/CONTRIBUTING.md)

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/aavishay/right-sizer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aavishay/right-sizer/discussions)
- **Documentation**: [Full Docs](https://github.com/aavishay/right-sizer)

## 📄 License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

---

**Made with ❤️ by the Right-Sizer Community**