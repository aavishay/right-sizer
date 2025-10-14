# Runtime Testing Guide - Metrics Registration Fix

This guide provides step-by-step instructions for runtime testing of the metrics registration fix.

---

## Prerequisites

- Kubernetes cluster (1.33+) with kubectl access
- Minikube, kind, or cloud cluster
- Helm 3.0+
- Docker (for building images)

---

## Quick Start

### Option 1: Automated Testing Script

```bash
# Run the automated test script
./scripts/test-metrics-fix.sh
```

This script will:
- ✅ Verify kubectl and cluster access
- ✅ Build the operator
- ✅ Check operator deployment
- ✅ Test metrics endpoint
- ✅ Verify no panics in logs
- ✅ Check specific metrics are exposed
- ✅ Test health endpoints

### Option 2: Manual Testing

Follow the steps below for manual verification.

---

## Step 1: Build the Operator

```bash
# Build the Go binary
cd go
go build -o ../bin/right-sizer .
cd ..

# Verify build succeeded
./bin/right-sizer --version
```

**Expected Output:**
```
Right-Sizer Operator v0.2.0
```

---

## Step 2: Deploy to Kubernetes

### Option A: Using Helm (Recommended)

```bash
# Install CRDs first
kubectl apply -f helm/crds/

# Install the operator
helm install right-sizer ./helm \
  --namespace right-sizer \
  --create-namespace \
  --set image.tag=0.2.0

# Wait for deployment
kubectl rollout status deployment/right-sizer -n right-sizer
```

### Option B: Using Minikube

```bash
# Start Minikube with K8s 1.33+
minikube start --kubernetes-version=v1.33.1 --memory=4096 --cpus=2

# Build and load image
docker build -t right-sizer:test .
minikube image load right-sizer:test

# Deploy
helm install right-sizer ./helm \
  --namespace right-sizer \
  --create-namespace \
  --set image.repository=right-sizer \
  --set image.tag=test \
  --set image.pullPolicy=Never
```

### Option C: Using Make

```bash
# Deploy using Makefile
make deploy

# Or for Minikube
make mk-deploy
```

---

## Step 3: Verify Operator Startup

### Check Pod Status

```bash
# Check if pod is running
kubectl get pods -n right-sizer

# Expected output:
# NAME                           READY   STATUS    RESTARTS   AGE
# right-sizer-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

### Check Logs for Successful Startup

```bash
# View operator logs
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=100

# Look for these success indicators:
# ✅ "Right-Sizer Operator Starting..."
# ✅ "Health checker initialized"
# ✅ "Starting right-sizer operator manager..."
# ✅ No "panic" messages
# ✅ No "failed to register" errors
```

**Expected Log Output:**
```
==========================================
🚀 Right-Sizer Operator Starting...
Version: 0.2.0
Build Date: 2024-xx-xx
Git Commit: xxxxxxx
Go Version: go1.25
==========================================
✅ Health checker initialized
✅ Metrics provider initialized
✅ AdaptiveRightSizer controller initialized
🚀 Starting right-sizer operator manager...
```

### Check for Panics (Should be NONE)

```bash
# Search for panic in logs
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer | grep -i panic

# Expected output: (empty - no panics)
```

---

## Step 4: Test Metrics Endpoint

### Access Metrics Endpoint

```bash
# Port-forward to metrics port
kubectl port-forward -n right-sizer svc/right-sizer 8080:8080 &

# Wait a moment for port-forward to establish
sleep 2

# Fetch metrics
curl http://localhost:8080/metrics > metrics.txt

# View metrics
cat metrics.txt | head -50
```

### Verify Key Metrics Are Present

```bash
# Check for specific metrics
grep "rightsizer_pods_processed_total" metrics.txt
grep "rightsizer_pods_resized_total" metrics.txt
grep "rightsizer_cpu_adjustments_total" metrics.txt
grep "rightsizer_memory_adjustments_total" metrics.txt
grep "rightsizer_cpu_usage_percent" metrics.txt
grep "rightsizer_memory_usage_percent" metrics.txt
grep "rightsizer_active_pods_total" metrics.txt
```

**Expected Output (Sample):**
```
# HELP rightsizer_pods_processed_total Total number of pods processed
# TYPE rightsizer_pods_processed_total counter
rightsizer_pods_processed_total 0

# HELP rightsizer_cpu_usage_percent Current average CPU usage percent
# TYPE rightsizer_cpu_usage_percent gauge
rightsizer_cpu_usage_percent 0

# HELP rightsizer_memory_usage_percent Current average memory usage percent
# TYPE rightsizer_memory_usage_percent gauge
rightsizer_memory_usage_percent 0
```

### Test Metrics Update During Operation

```bash
# Deploy a test pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-nginx
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:latest
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi
EOF

# Wait for operator to process
sleep 30

# Check metrics again
curl http://localhost:8080/metrics | grep rightsizer_pods_processed_total

# Should show increased count
```

---

## Step 5: Test Health Endpoints

```bash
# Port-forward to health port
kubectl port-forward -n right-sizer svc/right-sizer 8081:8081 &

# Test liveness endpoint
curl http://localhost:8081/healthz
# Expected: {"status":"healthy"} or similar

# Test readiness endpoint
curl http://localhost:8081/readyz
# Expected: {"ready":true} or similar

# Test detailed health
curl http://localhost:8081/readyz/detailed
# Expected: JSON with component health status
```

---

## Step 6: Multi-Replica Testing

### Scale to Multiple Replicas

```bash
# Scale to 3 replicas
kubectl scale deployment right-sizer -n right-sizer --replicas=3

# Wait for all replicas to be ready
kubectl rollout status deployment/right-sizer -n right-sizer

# Check all pods are running
kubectl get pods -n right-sizer
```

### Verify No Registration Conflicts

```bash
# Check logs from all replicas
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --all-containers --tail=50

# Look for:
# ✅ All pods start successfully
# ✅ No "already registered" errors causing failures
# ✅ No panics
# ✅ Leader election working (if enabled)
```

### Test Metrics from Each Replica

```bash
# Get all pod names
kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer -o name

# Test metrics from each pod
for pod in $(kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer -o name); do
  echo "Testing $pod..."
  kubectl port-forward -n right-sizer $pod 8080:8080 &
  PF_PID=$!
  sleep 2
  curl -s http://localhost:8080/metrics | grep -c "rightsizer_" || echo "Metrics available"
  kill $PF_PID
done
```

---

## Step 7: Load Testing

### Deploy Multiple Test Pods

```bash
# Create 50 test pods
for i in {1..50}; do
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-$i
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:latest
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 100m
        memory: 128Mi
EOF
done

# Wait for operator to process
sleep 60

# Check metrics
curl http://localhost:8080/metrics | grep rightsizer_pods_processed_total

# Check operator resource usage
kubectl top pod -n right-sizer

# Check for any errors
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=100 | grep -i error
```

---

## Step 8: Prometheus Integration Testing

### Deploy Prometheus (if not already installed)

```bash
# Add Prometheus Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus
helm install prometheus prometheus-community/prometheus \
  --namespace monitoring \
  --create-namespace
```

### Configure Prometheus to Scrape Right-Sizer

```bash
# Apply ServiceMonitor
kubectl apply -f k8s/right-sizer-servicemonitor.yaml

# Or manually configure Prometheus scrape config
kubectl edit configmap prometheus-server -n monitoring
```

Add this scrape config:
```yaml
- job_name: 'right-sizer'
  kubernetes_sd_configs:
  - role: pod
    namespaces:
      names:
      - right-sizer
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
    action: keep
    regex: right-sizer
  - source_labels: [__meta_kubernetes_pod_ip]
    target_label: __address__
    replacement: ${1}:8080
```

### Verify Prometheus is Scraping

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-server 9090:80 &

# Open browser to http://localhost:9090
# Go to Status > Targets
# Verify right-sizer target is UP

# Query metrics
# In Prometheus UI, query: rightsizer_pods_processed_total
```

---

## Troubleshooting

### Issue: Operator Pod Not Starting

```bash
# Check pod events
kubectl describe pod -n right-sizer -l app.kubernetes.io/name=right-sizer

# Check for image pull errors
kubectl get events -n right-sizer --sort-by='.lastTimestamp'

# Check resource constraints
kubectl get resourcequota -n right-sizer
kubectl get limitrange -n right-sizer
```

### Issue: Metrics Endpoint Not Accessible

```bash
# Check if metrics port is exposed
kubectl get svc -n right-sizer

# Check if pod is listening on port 8080
kubectl exec -n right-sizer -it $(kubectl get pod -n right-sizer -l app.kubernetes.io/name=right-sizer -o name | head -1) -- netstat -tlnp | grep 8080

# Check firewall/network policies
kubectl get networkpolicies -n right-sizer
```

### Issue: Metrics Not Updating

```bash
# Check if operator is processing pods
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer | grep "Processing\|Analyzing"

# Check if metrics provider is working
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer | grep "metrics"

# Verify metrics-server is installed
kubectl get deployment metrics-server -n kube-system
```

### Issue: Panic in Logs

```bash
# Get full stack trace
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --previous

# Check for specific error
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer | grep -A 20 "panic"

# Report issue with logs
```

---

## Success Criteria Checklist

Use this checklist to verify the metrics fix is working correctly:

- [ ] Operator pod starts successfully
- [ ] No panic messages in logs
- [ ] No "failed to register" errors in logs
- [ ] Metrics endpoint accessible at :8080/metrics
- [ ] All 27 expected metrics are present
- [ ] Metrics values update during operation
- [ ] Health endpoints working (:8081/healthz, :8081/readyz)
- [ ] Multiple replicas start without conflicts
- [ ] Prometheus can scrape metrics successfully
- [ ] Operator processes pods correctly
- [ ] No memory leaks under load
- [ ] CPU usage remains reasonable

---

## Performance Benchmarks

Expected performance characteristics:

| Metric | Expected Value | Notes |
|--------|---------------|-------|
| Startup Time | < 30 seconds | Time to ready state |
| Memory Usage | ~128Mi base | Scales with pod count |
| CPU Usage | < 100m idle | Spikes during processing |
| Metrics Scrape Time | < 100ms | /metrics endpoint response |
| Pod Processing Rate | 50-100/min | Depends on cluster size |

---

## Next Steps After Successful Testing

1. **Update Documentation**
   - Mark metrics fix as verified
   - Update IMPLEMENTATION_STATUS.md
   - Add runtime test results

2. **Deploy to Production**
   - Use verified image tag
   - Monitor metrics closely
   - Set up alerts

3. **Continue with Remaining Items**
   - Item 2: API Authentication
   - Item 3: Test Coverage to 90%+
   - Item 4: Documentation

---

## Additional Resources

- [Architecture Review](./ARCHITECTURE_REVIEW.md)
- [Implementation Status](./IMPLEMENTATION_STATUS.md)
- [Quick Access Guide](./QUICK_ACCESS_GUIDE.md)
- [Troubleshooting Guide](./troubleshooting-k8s.md)

---

**Last Updated:** 2024  
**Test Script:** `scripts/test-metrics-fix.sh`
