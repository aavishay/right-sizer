# Right-Sizer Installation Guide

## Table of Contents
- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
  - [Method 1: Helm (Recommended)](#method-1-helm-recommended)
  - [Method 2: Manual Installation](#method-2-manual-installation)
- [Configuration](#configuration)
- [Verification](#verification)
- [Upgrade Instructions](#upgrade-instructions)
- [Uninstallation](#uninstallation)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required
- Kubernetes cluster version 1.24 or higher
- kubectl configured to access your cluster
- Helm 3.8+ (for Helm installation method)
- Cluster admin permissions for CRD installation

### Optional
- Kubernetes 1.27+ for in-place pod resizing support
- Metrics Server or Prometheus for resource metrics
- cert-manager (if using webhook features)

## Installation Methods

### Method 1: Helm (Recommended)

#### Step 1: Add the Helm Repository

```bash
# Add the Right-Sizer Helm repository
helm repo add right-sizer https://aavishay.github.io/right-sizer

# Update your local Helm chart repository cache
helm repo update
```

#### Step 2: Install CRDs

**Important:** CRDs must be installed before the main chart. This is a one-time operation per cluster.

```bash
# Download and install CRDs
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Verify CRDs are installed
kubectl get crd rightsizerconfigs.rightsizer.io
kubectl get crd rightsizerpolicies.rightsizer.io
```

#### Step 3: Install Right-Sizer

```bash
# Install with default values
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace

# Or install with custom values file
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --values custom-values.yaml
```

### Method 2: Manual Installation

#### Step 1: Clone the Repository

```bash
git clone https://github.com/aavishay/right-sizer.git
cd right-sizer
```

#### Step 2: Install CRDs

```bash
kubectl apply -f helm/crds/
```

#### Step 3: Install Using Local Helm Chart

```bash
helm upgrade --install right-sizer ./helm \
  --namespace right-sizer \
  --create-namespace
```

## Configuration

### Basic Configuration

Create a `values.yaml` file to customize your installation:

```yaml
# values.yaml
rightsizerConfig:
  # Enable or disable right-sizing
  enabled: true
  
  # Operating mode: adaptive, aggressive, balanced, conservative, custom
  mode: "balanced"
  
  # Enable dry-run mode (no actual changes)
  dryRun: false
  
  # Feature flags
  featureGates:
    # Enable in-place pod resizing (requires K8s 1.27+)
    # Default is false for safety
    updateResizePolicy: false

  # Namespace configuration
  namespaceConfig:
    # Exclude system namespaces
    excludeNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - cert-manager
      - ingress-nginx
```

### Advanced Configuration

#### Enable In-Place Pod Resizing

For Kubernetes 1.27+, you can enable in-place pod resizing:

```yaml
rightsizerConfig:
  featureGates:
    updateResizePolicy: true
```

#### Configure Resource Limits

```yaml
rightsizerConfig:
  resourceDefaults:
    cpu:
      minRequest: "10m"
      maxLimit: "4000m"
    memory:
      minRequest: "64Mi"
      maxLimit: "8192Mi"
```

#### Enable Notifications

```yaml
rightsizerConfig:
  notifications:
    enabled: true
    slack:
      webhookURL: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
      channel: "#right-sizer"
```

### Common Installation Scenarios

#### Production Installation

```bash
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  --set rightsizerConfig.dryRun=true \
  --set rightsizerConfig.mode=conservative \
  --set rightsizerConfig.featureGates.updateResizePolicy=false
```

#### Development/Testing Installation

```bash
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer-dev \
  --create-namespace \
  --set rightsizerConfig.mode=aggressive \
  --set rightsizerConfig.namespaceConfig.includeNamespaces={dev,test}
```

## Verification

### 1. Check Deployment Status

```bash
# Check if the operator is running
kubectl get deployment -n right-sizer
kubectl get pods -n right-sizer

# Check logs
kubectl logs -n right-sizer deployment/right-sizer
```

### 2. Verify CRDs

```bash
# List CRDs
kubectl get crd | grep rightsizer

# Check RightSizerConfig
kubectl get rightsizerconfig
kubectl describe rightsizerconfig
```

### 3. Test Configuration

```bash
# View the configuration
kubectl get rightsizerconfig -o yaml

# Check if right-sizing is active
kubectl logs -n right-sizer deployment/right-sizer | grep -i "starting"
```

## Upgrade Instructions

### Upgrade Helm Release

```bash
# Update repository
helm repo update

# Upgrade the release
helm upgrade right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --reuse-values
```

### Upgrade CRDs

CRDs are not automatically upgraded by Helm. Update them manually:

```bash
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml
```

## Uninstallation

### Remove Helm Release

```bash
# Uninstall the release
helm uninstall right-sizer -n right-sizer

# Delete the namespace
kubectl delete namespace right-sizer
```

### Remove CRDs (Optional)

⚠️ **Warning:** This will delete all RightSizerConfig and RightSizerPolicy resources.

```bash
kubectl delete crd rightsizerconfigs.rightsizer.io
kubectl delete crd rightsizerpolicies.rightsizer.io
```

## Troubleshooting

### Issue: "no matches for kind 'RightSizerConfig'"

**Cause:** CRDs are not installed.

**Solution:**
```bash
# Install CRDs
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Then retry the helm install
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace
```

### Issue: Operator Not Starting

**Check logs:**
```bash
kubectl logs -n right-sizer deployment/right-sizer
```

**Common causes:**
- Missing RBAC permissions
- Invalid configuration
- Metrics server not available

### Issue: No Pods Being Right-Sized

**Check configuration:**
```bash
# Verify enabled
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.enabled}'

# Check dry-run mode
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.dryRun}'

# Check namespace configuration
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.namespaceConfig}'
```

### Issue: Helm Repository Not Found

**Solution:**
```bash
# Remove and re-add repository
helm repo remove right-sizer
helm repo add right-sizer https://aavishay.github.io/right-sizer
helm repo update
```

## Support

For additional help:
- Check the [documentation](https://github.com/aavishay/right-sizer/tree/main/docs)
- Open an [issue](https://github.com/aavishay/right-sizer/issues)
- Review [examples](https://github.com/aavishay/right-sizer/tree/main/examples)

## Next Steps

After installation:
1. Monitor the operator logs to ensure it's running correctly
2. Start with dry-run mode to observe recommendations
3. Gradually enable right-sizing for specific namespaces
4. Adjust configuration based on your workload patterns
5. Consider enabling in-place resizing if using Kubernetes 1.27+