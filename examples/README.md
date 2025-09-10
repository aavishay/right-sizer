# Right-Sizer Examples

This directory contains example configurations and deployment files for the Right-Sizer Kubernetes operator.

## 📁 Directory Structure

```
examples/
├── deploy/                    # Deployment examples
├── values-examples.yaml       # Comprehensive Helm values examples
├── helm-values-custom.yaml    # Custom Helm values example
├── rightsizerconfig-*.yaml    # RightSizerConfig CRD examples
└── README.md                  # This file
```

## 🚀 Quick Start Examples

### Basic Installation

```bash
# Install with default values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace

# Install with conservative mode
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --set rightsizerConfig.mode=conservative
```

### Using Example Values Files

```bash
# Extract and use a specific configuration from values-examples.yaml
yq eval '.conservative-production' values-examples.yaml > my-values.yaml
helm install right-sizer right-sizer/right-sizer -f my-values.yaml

# Or use helm-values-custom.yaml directly
helm install right-sizer right-sizer/right-sizer -f helm-values-custom.yaml
```

## 📋 Available Examples

### Helm Values Examples (`values-examples.yaml`)

This file contains multiple configuration examples:

1. **Conservative Production** - Stable configuration for production environments
2. **Aggressive Cost Optimization** - Maximum resource savings for dev/test
3. **Adaptive with Prometheus** - Uses Prometheus for metrics
4. **Dry-Run Mode** - Preview changes without applying them
5. **Namespace-Specific** - Different settings per namespace
6. **Scheduled Maintenance** - Only resize during maintenance windows
7. **Development Minimal** - Minimal resources for development
8. **High Availability** - Multi-replica setup for production

### RightSizerConfig Examples

#### `rightsizerconfig-conservative.yaml`
Conservative configuration suitable for production workloads with safety-first approach:
- Uses peak values for sizing
- Large safety margins
- Slow scaling down
- Longer observation periods

```bash
kubectl apply -f rightsizerconfig-conservative.yaml
```

#### `rightsizerconfig-full.yaml`
Complete configuration example showing all available options:
- All configurable fields documented
- Comprehensive policy settings
- Advanced operational controls
- Monitoring and notification setup

```bash
kubectl apply -f rightsizerconfig-full.yaml
```

### Deployment Examples (`deploy/`)

Various Kubernetes deployment manifests for different scenarios:

#### Test Workloads
- `test-workloads.yaml` - Sample workloads for testing right-sizing
- `test-workloads-quick.yaml` - Quick test deployment
- `stress-test.yaml` - Stress testing configuration

#### Real Data Collection
- `real-events-collector.yaml` - Collect real metrics data
- `real-events-collector-v2.yaml` - Updated metrics collector
- `real-test-workloads.yaml` - Real workload simulation

#### API and Optimization
- `api-proxy.yaml` - API proxy configuration
- `api-proxy-real.yaml` - Production API proxy
- `optimization-events-server.yaml` - Optimization events server
- `optimization-events-minikube.yaml` - Minikube-specific optimization

#### Right-Sizer Deployments
- `right-sizer-deployment.yaml` - Standard deployment
- `right-sizer-no-mock.yaml` - Production deployment without mocks
- `right-sizer-real-data.yaml` - Deployment with real data collection

## 🎯 Common Use Cases

### 1. Development Environment

```bash
# Use aggressive optimization for development
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.mode=aggressive \
  --set rightsizerConfig.operationalConfig.resizeInterval=30s \
  --set rightsizerConfig.dryRun=true
```

### 2. Production Environment

```bash
# Conservative production setup with monitoring
kubectl apply -f rightsizerconfig-conservative.yaml

# Or use Helm with production values
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.mode=conservative \
  --set rightsizerConfig.sizingStrategy.algorithm=peak \
  --set rightsizerConfig.operationalConfig.resizeMode=Rolling \
  --set rightsizerConfig.operationalConfig.resizeInterval=30m
```

### 3. Testing and Validation

```bash
# Deploy test workloads
kubectl apply -f deploy/test-workloads.yaml

# Install Right-Sizer in dry-run mode
helm install right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.dryRun=true \
  --set rightsizerConfig.logging.level=debug
```

### 4. Namespace-Specific Configuration

```yaml
# Create different policies for different namespaces
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-policy
spec:
  enabled: true
  priority: 100
  mode: conservative
  targetRef:
    namespaces: ["production"]
---
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: development-policy
spec:
  enabled: true
  priority: 50
  mode: aggressive
  targetRef:
    namespaces: ["development"]
```

## 🔧 Customization Tips

### Adjusting Resource Limits

```yaml
# In your values file
rightsizerConfig:
  resourceDefaults:
    cpu:
      minRequest: "50m"      # Minimum CPU request
      maxLimit: "4000m"      # Maximum CPU limit
    memory:
      minRequest: "128Mi"    # Minimum memory request
      maxLimit: "8Gi"        # Maximum memory limit
```

### Configuring Scaling Behavior

```yaml
rightsizerConfig:
  sizingStrategy:
    scalingFactors:
      scaleUpMultiplier: 1.5    # How much to scale up (50% increase)
      scaleDownMultiplier: 0.8  # How much to scale down (20% decrease)
      minChangeThreshold: 10    # Minimum % change to trigger resize
```

### Setting Up Maintenance Windows

```yaml
rightsizerConfig:
  operationalConfig:
    maintenanceWindow:
      enabled: true
      schedule: "0 2 * * 1,3,5"  # Mon, Wed, Fri at 2 AM
      duration: "3h"
      timezone: "America/New_York"
```

## 📊 Monitoring and Observability

### Prometheus Integration

```yaml
# From values-examples.yaml
monitoring:
  metricsProvider: "prometheus"
  prometheusURL: "http://prometheus.monitoring.svc.cluster.local:9090"
  scrapeInterval: "15s"
```

### Viewing Recommendations

```bash
# Check logs for recommendations (dry-run mode)
kubectl logs -n right-sizer deployment/right-sizer | grep -i recommendation

# View current RightSizerConfig
kubectl get rightsizerconfig -A -o yaml

# Check metrics
kubectl port-forward -n right-sizer svc/right-sizer 9090:9090
# Visit http://localhost:9090/metrics
```

## 🤝 Contributing

To add new examples:

1. Create your example configuration
2. Test it thoroughly in a development environment
3. Document the use case and configuration options
4. Submit a pull request with the new example

## 📚 Additional Resources

- [Right-Sizer Documentation](https://github.com/aavishay/right-sizer)
- [Helm Chart Documentation](https://github.com/aavishay/right-sizer/tree/main/helm)
- [API Reference](https://github.com/aavishay/right-sizer/tree/main/docs/api)
- [Troubleshooting Guide](https://github.com/aavishay/right-sizer#troubleshooting)

## ⚠️ Important Notes

- Always test configurations in a non-production environment first
- Use `dryRun: true` to preview changes before applying them
- Monitor resource usage after enabling right-sizing
- Keep backups of your original resource configurations
- Review logs regularly for optimization recommendations

## 📄 License

These examples are provided under the same license as the Right-Sizer project (AGPL-3.0).
