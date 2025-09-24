# 📊 Metrics Server Deployment Guide for Right-Sizer

## Overview

The metrics-server is a critical component for Right-Sizer to function properly. It collects resource metrics from Kubernetes nodes and pods, which Right-Sizer uses to make intelligent resource optimization decisions.

## ✅ Current Status

**Metrics-Server**: Successfully Deployed and Operational
- **Version**: v0.8.0
- **Namespace**: kube-system
- **Status**: Running
- **Metrics Available**: Yes
- **Integration with Right-Sizer**: Fully Functional

## 🚀 Quick Deployment

### Option 1: Minikube Addon (Recommended for Minikube)

```bash
# Enable metrics-server addon
minikube -p right-sizer addons enable metrics-server

# Verify deployment
kubectl get deployment metrics-server -n kube-system

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=120s
```

### Option 2: Using Deployment Script

```bash
# Deploy using the provided script
./scripts/deploy-metrics-server.sh

# Deploy standalone version (for non-Minikube clusters)
./scripts/deploy-metrics-server.sh --standalone

# Test existing deployment
./scripts/deploy-metrics-server.sh --test-only
```

### Option 3: Manual Deployment

```bash
# Apply the official metrics-server manifest
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For Minikube, you may need to add insecure TLS flag
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
```

## 📈 Metrics Available

### Node Metrics
```bash
kubectl top nodes
```

**Example Output:**
```
NAME          CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
right-sizer   193m         1%       1078Mi          13%
```

### Pod Metrics

#### Right-Sizer Operator
```bash
kubectl top pods -n right-sizer
```
**Output:**
```
NAME                           CPU(cores)   MEMORY(bytes)
right-sizer-7c84949577-vw8ww   3m           15Mi
```

#### Test Workloads
```bash
kubectl top pods -n test-workloads
```
**Output:**
```
NAME                          CPU(cores)   MEMORY(bytes)
nginx-demo-64c5cf9bb6-n4dzs   0m           13Mi
nginx-demo-64c5cf9bb6-s98ld   0m           8Mi
nginx-demo-64c5cf9bb6-xl4mg   0m           8Mi
redis-cache-d79dd5d54-qfx8c   7m           5Mi
```

## 🔄 Integration with Right-Sizer

### How Right-Sizer Uses Metrics

1. **Data Collection**: Right-Sizer queries metrics-server every 30 seconds
2. **Analysis**: Analyzes CPU and memory usage patterns
3. **Recommendations**: Generates resize recommendations based on actual usage
4. **Optimization**: Applies resource adjustments according to configured policies

### Example Right-Sizer Analysis

```log
📈 Container test-workloads/nginx-demo-64c5cf9bb6-s98ld/nginx will be resized
   - CPU: 10m→10m (no change needed)
   - Memory: 128Mi→10Mi (optimized based on actual usage)

📊 Found 3 resources needing adjustment
   - nginx pods: Memory reduced from 128Mi to 10-16Mi based on usage
   - redis pod: Maintained current resources (already optimized)
```

## 🛠️ Configuration

### Metrics-Server Arguments

Key configuration options for metrics-server:

```yaml
args:
  - --cert-dir=/tmp
  - --secure-port=10250
  - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
  - --kubelet-use-node-status-port
  - --metric-resolution=15s              # How often metrics are scraped
  - --kubelet-insecure-tls              # Required for Minikube
```

### Resource Requirements

```yaml
resources:
  requests:
    cpu: 100m
    memory: 200Mi
  limits:
    cpu: 500m
    memory: 500Mi
```

## 🔍 Verification Commands

### Check Deployment Status
```bash
kubectl get deployment metrics-server -n kube-system
```

### Check Pod Status
```bash
kubectl get pods -n kube-system -l k8s-app=metrics-server
```

### Check API Service
```bash
kubectl get apiservice v1beta1.metrics.k8s.io
```

### View Logs
```bash
kubectl logs -n kube-system -l k8s-app=metrics-server
```

### Test Metrics API
```bash
# Test node metrics
kubectl get --raw /apis/metrics.k8s.io/v1beta1/nodes

# Test pod metrics
kubectl get --raw /apis/metrics.k8s.io/v1beta1/pods
```

## 🚨 Troubleshooting

### Issue: Metrics Not Available

**Symptoms:**
```
error: Metrics API not available
```

**Solutions:**
1. Wait 60 seconds after deployment for initial metrics collection
2. Check pod status: `kubectl get pods -n kube-system -l k8s-app=metrics-server`
3. Check logs: `kubectl logs -n kube-system -l k8s-app=metrics-server`

### Issue: TLS Certificate Errors

**Symptoms:**
```
x509: cannot validate certificate
```

**Solution for Minikube:**
```bash
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
```

### Issue: High Memory Usage

**Symptoms:**
- metrics-server pod using excessive memory

**Solutions:**
1. Adjust metric resolution: `--metric-resolution=30s`
2. Increase memory limits if needed
3. Check for memory leaks in logs

### Issue: Incomplete Metrics

**Symptoms:**
- Some pods show no metrics

**Solutions:**
1. Ensure pods have been running for at least 15 seconds
2. Check if pods have resource requests defined
3. Verify kubelet is running on all nodes

## 📋 Monitoring Checklist

- [ ] **Deployment**: metrics-server deployment is running
- [ ] **Pod**: metrics-server pod is in Running state
- [ ] **API Service**: v1beta1.metrics.k8s.io is Available
- [ ] **Node Metrics**: `kubectl top nodes` returns data
- [ ] **Pod Metrics**: `kubectl top pods -A` returns data
- [ ] **Right-Sizer Integration**: Right-Sizer logs show metric collection
- [ ] **Resource Optimization**: Right-Sizer is making resize recommendations

## 🎯 Best Practices

1. **Resource Allocation**: Ensure metrics-server has sufficient resources
2. **Monitoring**: Regularly check metrics-server health
3. **Updates**: Keep metrics-server updated to latest stable version
4. **High Availability**: Consider running multiple replicas in production
5. **Metric Resolution**: Adjust based on cluster size and requirements

## 📚 Additional Resources

- [Metrics Server GitHub](https://github.com/kubernetes-sigs/metrics-server)
- [Kubernetes Metrics API](https://kubernetes.io/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/)
- [Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [Right-Sizer Documentation](https://github.com/aavishay/right-sizer)

## Summary

The metrics-server is successfully deployed and integrated with Right-Sizer. It's collecting metrics from all nodes and pods, enabling Right-Sizer to make intelligent resource optimization decisions. The system is currently monitoring test workloads and providing optimization recommendations based on actual resource usage patterns.

**Current Metrics Collection Status:**
- ✅ Node metrics: Available
- ✅ System pod metrics: Available
- ✅ Application pod metrics: Available
- ✅ Right-Sizer integration: Functional
- ✅ Optimization recommendations: Being generated

---

*Last Updated: September 24, 2025*
*Metrics-Server Version: v0.8.0*
*Kubernetes Version: 1.34+*
*Platform: ARM64/Minikube*
