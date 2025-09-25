# Kubernetes Installation Troubleshooting Guide

## Common Installation Issues

### Issue 1: "no matches for kind 'RightSizerConfig'"

**Error Message:**
```
Error: unable to build kubernetes objects from release manifest: resource mapping not found for name: "right-sizer-config" namespace: "" from "": no matches for kind "RightSizerConfig" in version "rightsizer.io/v1alpha1"
ensure CRDs are installed first
```

**Cause:** The Custom Resource Definitions (CRDs) are not installed in the cluster.

**Solution:**
```bash
# Install CRDs before installing the Helm chart
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f https://raw.githubusercontent.com/aavishay/right-sizer/main/helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Verify CRDs are installed
kubectl get crd | grep rightsizer

# Then install the Helm chart
helm upgrade --install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace
```

### Issue 2: Readiness Probe Failures

**Error Messages:**
```
Warning  Unhealthy  6s (x7 over 46s)  kubelet  Readiness probe failed: Get "http://10.244.0.12:8081/readyz": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

**Possible Causes:**
1. Operator is still initializing
2. Insufficient resources
3. Network issues
4. Missing dependencies

**Solutions:**

**Option 1: Wait for Initialization**
```bash
# Check pod status and events
kubectl describe pod -n right-sizer -l app.kubernetes.io/name=right-sizer

# Check operator logs
kubectl logs -n right-sizer deployment/right-sizer --follow
```

**Option 2: Increase Resource Limits**
```yaml
# values.yaml
resources:
  limits:
    cpu: 1000m
    memory: 1024Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

**Option 3: Use Version 0.2.0+ with Improved Probes**
```bash
helm upgrade right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --version 0.2.0 \
  --reuse-values
```

### Issue 3: Helm Package Installation Fails

**Error Message:**
```
Error: Chart.yaml file is missing
```

**Cause:** Helm packaging issue with certain filesystem configurations.

**Solution:**
```bash
# Download and extract the chart manually
wget https://github.com/aavishay/right-sizer/releases/download/v0.2.0/right-sizer-0.2.0.tgz
tar -xzf right-sizer-0.2.0.tgz

# Install from local directory
helm upgrade --install right-sizer ./right-sizer \
  --namespace right-sizer \
  --create-namespace
```

### Issue 4: Leader Election Conflicts

**Error Messages:**
```
E0911 10:00:00.000000 1 leaderelection.go:325] error retrieving resource lock right-sizer/right-sizer-leader: configmaps "right-sizer-leader" is forbidden
```

**Cause:** Multiple operator instances or insufficient RBAC permissions.

**Solution:**
```bash
# Check for multiple instances
kubectl get pods -n right-sizer

# Scale down to single replica
kubectl scale deployment right-sizer -n right-sizer --replicas=1

# Verify RBAC
kubectl get clusterrolebinding | grep right-sizer
kubectl describe clusterrole right-sizer
```

### Issue 5: Metrics Server Not Available

**Error Messages:**
```
unable to fetch metrics from metrics-server: the server could not find the requested resource
```

**Cause:** Metrics Server is not installed or not functioning.

**Solution:**
```bash
# Check if metrics-server is installed
kubectl get deployment metrics-server -n kube-system

# Install metrics-server if missing
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For development/minikube environments, may need insecure TLS
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
```

### Issue 6: Namespace Exclusion Not Working

**Symptom:** Right-Sizer is attempting to resize pods in system namespaces.

**Solution:**
```yaml
# values.yaml
rightsizerConfig:
  namespaceConfig:
    excludeNamespaces:
      - kube-system
      - kube-public
      - kube-node-lease
      - cert-manager
      - ingress-nginx
      - istio-system
      - right-sizer  # Exclude the operator's own namespace
```

## Runtime Issues

### Issue 7: Pods Not Being Resized

**Diagnostic Steps:**
```bash
# 1. Check if operator is enabled
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.enabled}'

# 2. Check if dry-run mode is active
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.dryRun}'

# 3. Check namespace configuration
kubectl get rightsizerconfig -o jsonpath='{.items[0].spec.namespaceConfig}'

# 4. Check operator logs for errors
kubectl logs -n right-sizer deployment/right-sizer | grep -i error
```

### Issue 8: In-Place Resizing Not Working

**Requirements:**
- Kubernetes 1.33+
- Feature flag enabled
- Proper restart policies

**Enable In-Place Resizing:**
```yaml
# values.yaml
rightsizerConfig:
  featureGates:
    updateResizePolicy: true  # Disabled by default in v0.2.0+
```

**Verify Kubernetes Support:**
```bash
# Check Kubernetes version
kubectl version --short

# Check if InPlacePodVerticalScaling feature gate is enabled
kubectl get nodes -o json | jq '.items[0].status.nodeInfo.kubeletVersion'
```

### Issue 9: High Memory/CPU Usage

**Symptoms:** Operator consuming excessive resources.

**Solutions:**
```yaml
# Limit concurrent operations
rightsizerConfig:
  operator:
    maxConcurrentReconciles: 3
    workerThreads: 5
  operationalConfig:
    batchSize: 2
    maxUpdatesPerRun: 20
```

### Issue 10: Webhook Certificate Issues

**Error Messages:**
```
x509: certificate signed by unknown authority
```

**Solution:**
```bash
# Disable webhook if not needed
helm upgrade right-sizer right-sizer/right-sizer \
  --set rightsizerConfig.security.enableAdmissionController=false \
  --set rightsizerConfig.security.enableMutatingWebhook=false \
  --set rightsizerConfig.security.enableValidatingWebhook=false
```

## Debugging Commands

### Check Overall Health
```bash
# Pod status
kubectl get pods -n right-sizer

# Recent events
kubectl get events -n right-sizer --sort-by='.lastTimestamp'

# Deployment status
kubectl rollout status deployment/right-sizer -n right-sizer

# Resource usage
kubectl top pod -n right-sizer
```

### View Configuration
```bash
# Current configuration
kubectl get rightsizerconfig -o yaml

# Helm values
helm get values right-sizer -n right-sizer

# All resources
kubectl get all -n right-sizer
```

### Collect Debug Information
```bash
#!/bin/bash
# debug-right-sizer.sh

NAMESPACE="right-sizer"
OUTPUT_DIR="right-sizer-debug-$(date +%Y%m%d-%H%M%S)"

mkdir -p "$OUTPUT_DIR"

# Collect pod information
kubectl get pods -n $NAMESPACE -o wide > "$OUTPUT_DIR/pods.txt"
kubectl describe pods -n $NAMESPACE > "$OUTPUT_DIR/pod-descriptions.txt"

# Collect logs
kubectl logs -n $NAMESPACE deployment/right-sizer --tail=1000 > "$OUTPUT_DIR/operator-logs.txt"
kubectl logs -n $NAMESPACE deployment/right-sizer --previous --tail=1000 > "$OUTPUT_DIR/operator-logs-previous.txt" 2>/dev/null

# Collect events
kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp' > "$OUTPUT_DIR/events.txt"

# Collect configuration
kubectl get rightsizerconfig -o yaml > "$OUTPUT_DIR/config.yaml"
kubectl get crd rightsizerconfigs.rightsizer.io -o yaml > "$OUTPUT_DIR/crd.yaml"

# Collect helm values
helm get values right-sizer -n $NAMESPACE > "$OUTPUT_DIR/helm-values.yaml"

echo "Debug information collected in $OUTPUT_DIR"
```

## Performance Tuning

### For Large Clusters (>1000 pods)
```yaml
rightsizerConfig:
  operator:
    maxConcurrentReconciles: 10
    workerThreads: 20
    qps: 50
    burst: 100
  operationalConfig:
    batchSize: 10
    maxUpdatesPerRun: 100
    resizeInterval: "10m"
```

### For Small Clusters (<100 pods)
```yaml
rightsizerConfig:
  operator:
    maxConcurrentReconciles: 2
    workerThreads: 5
    qps: 10
    burst: 20
  operationalConfig:
    batchSize: 3
    maxUpdatesPerRun: 20
    resizeInterval: "5m"
```

## Getting Help

If you continue to experience issues:

1. **Check Documentation**
   - [Installation Guide](./installation-guide.md)
   - [Configuration Reference](../README.md#⚙️-configuration)
   - [API Documentation](./api/openapi.yaml)

2. **Search Existing Issues**
   - GitHub Issues: https://github.com/aavishay/right-sizer/issues

3. **Create a New Issue**
   Include:
   - Kubernetes version (`kubectl version`)
   - Right-Sizer version (`helm list -n right-sizer`)
   - Error messages and logs
   - Debug information (use script above)

4. **Community Support**
   - Discussions: https://github.com/aavishay/right-sizer/discussions
   - Slack: [Join our Slack](https://right-sizer.slack.com)

## Quick Fixes Checklist

- [ ] CRDs installed before Helm chart
- [ ] Metrics Server or Prometheus available
- [ ] Sufficient RBAC permissions
- [ ] Correct namespace configuration
- [ ] Appropriate resource limits
- [ ] Compatible Kubernetes version
- [ ] Network policies allow communication
- [ ] No conflicting operators
- [ ] Correct feature flags for K8s version
- [ ] Logs checked for specific errors
