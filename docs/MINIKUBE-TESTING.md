# Minikube Testing Guide for Right-Sizer Configuration

This guide provides comprehensive instructions for testing the right-sizer operator with configurable multipliers in a Minikube environment.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Testing](#detailed-testing)
- [Configuration Validation](#configuration-validation)
- [Troubleshooting](#troubleshooting)
- [Test Scenarios](#test-scenarios)

## Prerequisites

### Required Tools
- **Minikube**: v1.30+ ([Installation Guide](https://minikube.sigs.k8s.io/docs/start/))
- **kubectl**: v1.28+ ([Installation Guide](https://kubernetes.io/docs/tasks/tools/))
- **Docker**: v20.10+ ([Installation Guide](https://docs.docker.com/get-docker/))
- **Go**: v1.22+ (for building from source)

### System Requirements
- Minimum 4GB RAM available for Minikube
- Minimum 2 CPU cores
- 20GB free disk space

## Quick Start

### 1. Start Minikube and Build

```bash
# Start Minikube with sufficient resources
minikube start --memory=4096 --cpus=2 --kubernetes-version=v1.31.0

# Use Minikube's Docker daemon
eval $(minikube docker-env)

# Build the right-sizer image
docker build -t right-sizer:config-test .

# Run the quick test script
./quick-test-config.sh
```

### 2. Deploy with Custom Configuration

```bash
# Create namespace
kubectl create namespace rightsizer-test

# Apply RBAC (use the provided rbac.yaml)
kubectl apply -f rbac.yaml -n rightsizer-test

# Deploy with custom environment variables
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: rightsizer-test
spec:
  selector:
    matchLabels:
      app: right-sizer
  template:
    metadata:
      labels:
        app: right-sizer
    spec:
      serviceAccountName: right-sizer
      containers:
        - name: right-sizer
          image: right-sizer:config-test
          imagePullPolicy: Never
          env:
            - name: CPU_REQUEST_MULTIPLIER
              value: "1.5"
            - name: MEMORY_REQUEST_MULTIPLIER
              value: "1.3"
            - name: CPU_LIMIT_MULTIPLIER
              value: "2.5"
            - name: MEMORY_LIMIT_MULTIPLIER
              value: "2.0"
            - name: MAX_CPU_LIMIT
              value: "8000"
            - name: MAX_MEMORY_LIMIT
              value: "16384"
EOF
```

### 3. Verify Configuration

```bash
# Check logs for configuration loading
kubectl logs -n rightsizer-test deployment/right-sizer | grep -E "Configuration|Multiplier"

# Expected output:
# CPU_REQUEST_MULTIPLIER set to: 1.50
# MEMORY_REQUEST_MULTIPLIER set to: 1.30
# CPU_LIMIT_MULTIPLIER set to: 2.50
# MEMORY_LIMIT_MULTIPLIER set to: 2.00
# MAX_CPU_LIMIT set to: 8000 millicores
# MAX_MEMORY_LIMIT set to: 16384 MB
```

## Detailed Testing

### Running the Comprehensive Test Script

The `minikube-config-test.sh` script provides a complete testing environment:

```bash
# Make the script executable
chmod +x minikube-config-test.sh

# Run the comprehensive test
./minikube-config-test.sh
```

This script will:
1. ✅ Verify prerequisites (Minikube, kubectl, Docker)
2. ✅ Start Minikube if not running
3. ✅ Build the Docker image in Minikube's daemon
4. ✅ Create a test namespace with RBAC
5. ✅ Deploy right-sizer with custom configuration
6. ✅ Deploy test workloads
7. ✅ Verify configuration is loaded correctly
8. ✅ Show expected vs actual resource calculations

### Manual Testing Steps

#### Step 1: Prepare Minikube Environment

```bash
# Check Minikube status
minikube status

# If not running, start with specific version
minikube start --kubernetes-version=v1.31.0 --memory=4096

# Configure Docker to use Minikube's daemon
eval $(minikube docker-env)

# Verify Docker is using Minikube
docker ps | head -2
```

#### Step 2: Build and Verify Image

```bash
# Build the image
docker build -t right-sizer:test .

# Verify image exists in Minikube
docker images | grep right-sizer

# Expected output:
# right-sizer    test    <hash>    <time>    <size>
```

#### Step 3: Deploy Test Configuration

Create a test deployment with specific multipliers:

```yaml
# test-config-deployment.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: config-test
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
  namespace: config-test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: config-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer
  template:
    metadata:
      labels:
        app: right-sizer
    spec:
      serviceAccountName: right-sizer
      containers:
        - name: right-sizer
          image: right-sizer:test
          imagePullPolicy: Never
          env:
            # Test Configuration
            - name: CPU_REQUEST_MULTIPLIER
              value: "1.7"
            - name: MEMORY_REQUEST_MULTIPLIER
              value: "1.5"
            - name: CPU_LIMIT_MULTIPLIER
              value: "3.0"
            - name: MEMORY_LIMIT_MULTIPLIER
              value: "2.5"
            - name: MAX_CPU_LIMIT
              value: "10000"
            - name: MAX_MEMORY_LIMIT
              value: "20480"
            - name: MIN_CPU_REQUEST
              value: "30"
            - name: MIN_MEMORY_REQUEST
              value: "256"
```

Apply and verify:

```bash
kubectl apply -f test-config-deployment.yaml
kubectl wait --for=condition=available deployment/right-sizer -n config-test
kubectl logs -n config-test deployment/right-sizer | head -40
```

## Configuration Validation

### Test Scenarios with Expected Results

#### Scenario 1: Default Configuration
```bash
# No environment variables set
# Expected behavior:
# CPU Request = Usage × 1.2
# Memory Request = Usage × 1.2
# CPU Limit = Request × 2.0
# Memory Limit = Request × 2.0
```

#### Scenario 2: Conservative Configuration
```bash
export CPU_REQUEST_MULTIPLIER=1.5
export MEMORY_REQUEST_MULTIPLIER=1.4
export CPU_LIMIT_MULTIPLIER=2.5
export MEMORY_LIMIT_MULTIPLIER=2.0

# For 100m CPU, 100Mi memory usage:
# CPU Request: 150m (100 × 1.5)
# Memory Request: 140Mi (100 × 1.4)
# CPU Limit: 375m (150 × 2.5)
# Memory Limit: 280Mi (140 × 2.0)
```

#### Scenario 3: Aggressive Cost Optimization
```bash
export CPU_REQUEST_MULTIPLIER=1.1
export MEMORY_REQUEST_MULTIPLIER=1.1
export CPU_LIMIT_MULTIPLIER=1.5
export MEMORY_LIMIT_MULTIPLIER=1.5

# For 100m CPU, 100Mi memory usage:
# CPU Request: 110m (100 × 1.1)
# Memory Request: 110Mi (100 × 1.1)
# CPU Limit: 165m (110 × 1.5)
# Memory Limit: 165Mi (110 × 1.5)
```

### Validation Commands

```bash
# Watch operator logs for configuration
kubectl logs -n config-test deployment/right-sizer -f | grep -E "Configuration|Multiplier|Calculating"

# Check if test workloads are being analyzed
kubectl get deployments -n config-test -o wide

# View events for resource updates
kubectl get events -n config-test --sort-by='.lastTimestamp' | grep -i resource

# Check pod resource specifications
kubectl get pods -n config-test -o custom-columns=\
NAME:.metadata.name,\
CPU_REQ:.spec.containers[0].resources.requests.cpu,\
MEM_REQ:.spec.containers[0].resources.requests.memory,\
CPU_LIM:.spec.containers[0].resources.limits.cpu,\
MEM_LIM:.spec.containers[0].resources.limits.memory
```

## Test Workload Deployment

Deploy a test application to see resource adjustments:

```yaml
# test-app.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: config-test
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
        rightsizer: enabled
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 100m
              memory: 128Mi
```

Apply and monitor:

```bash
# Deploy test app
kubectl apply -f test-app.yaml

# Watch for resource adjustments (may take a few minutes)
watch -n 5 "kubectl get deployment test-app -n config-test -o yaml | grep -A4 resources:"

# Generate some load
POD=$(kubectl get pod -n config-test -l app=test-app -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n config-test $POD -- sh -c "while true; do echo test > /dev/null; done" &

# Check if resources are adjusted based on load
kubectl logs -n config-test deployment/right-sizer | tail -20
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Image Pull Error: ErrImageNeverPull

**Problem**: Pod shows error "ErrImageNeverPull"

**Solution**:
```bash
# Ensure you're using Minikube's Docker daemon
eval $(minikube docker-env)

# Rebuild the image
docker build -t right-sizer:test .

# Verify image exists
docker images | grep right-sizer
```

#### 2. Configuration Not Loading

**Problem**: Environment variables not showing in logs

**Solution**:
```bash
# Check pod environment variables directly
kubectl exec -n config-test deployment/right-sizer -- env | grep MULTIPLIER

# Check deployment spec
kubectl get deployment right-sizer -n config-test -o yaml | grep -A20 env:
```

#### 3. Metrics Server Not Available

**Problem**: Operator can't fetch metrics

**Solution**:
```bash
# Enable metrics-server addon
minikube addons enable metrics-server

# Wait for metrics-server to be ready
kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=300s

# Verify metrics are available
kubectl top nodes
kubectl top pods --all-namespaces
```

#### 4. Permissions Issues

**Problem**: RBAC errors in logs

**Solution**:
```bash
# Apply comprehensive RBAC
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer-full
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer-full
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer-full
subjects:
  - kind: ServiceAccount
    name: right-sizer
    namespace: config-test
EOF
```

### Debug Commands

```bash
# Check all resources in test namespace
kubectl get all -n config-test

# Describe right-sizer pod for detailed status
kubectl describe pod -l app=right-sizer -n config-test

# Check recent events
kubectl get events -n config-test --sort-by='.lastTimestamp' | tail -20

# View full operator logs
kubectl logs -n config-test deployment/right-sizer --tail=100

# Check if operator is connecting to API server
kubectl logs -n config-test deployment/right-sizer | grep -i "error\|fail\|unable"

# Verify service account permissions
kubectl auth can-i '*' '*' --as=system:serviceaccount:config-test:right-sizer
```

## Cleanup

After testing, clean up resources:

```bash
# Delete test namespace (removes all resources in it)
kubectl delete namespace config-test
kubectl delete namespace rightsizer-test

# Remove cluster-wide resources if created
kubectl delete clusterrole right-sizer-full
kubectl delete clusterrolebinding right-sizer-full

# Stop Minikube (optional)
minikube stop

# Delete Minikube cluster (complete cleanup)
minikube delete
```

## Advanced Testing

### Load Testing with Multiple Workloads

```bash
# Deploy multiple test applications with different resource profiles
for i in {1..5}; do
  kubectl create deployment test-app-$i --image=nginx:alpine -n config-test
  kubectl set resources deployment test-app-$i -n config-test --requests=cpu=${i}0m,memory=${i}0Mi
done

# Monitor resource adjustments
watch -n 10 "kubectl get deployments -n config-test -o custom-columns=NAME:.metadata.name,REPLICAS:.spec.replicas,CPU_REQ:.spec.template.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.template.spec.containers[0].resources.requests.memory"
```

### Testing Different Configurations

```bash
# Test script for multiple configurations
for config in "1.1,1.1,1.5,1.5" "1.3,1.2,2.0,2.0" "1.8,1.5,3.0,2.5"; do
  IFS=',' read -r cpu_req mem_req cpu_lim mem_lim <<< "$config"
  echo "Testing config: CPU_REQ=$cpu_req, MEM_REQ=$mem_req, CPU_LIM=$cpu_lim, MEM_LIM=$mem_lim"
  
  kubectl set env deployment/right-sizer -n config-test \
    CPU_REQUEST_MULTIPLIER=$cpu_req \
    MEMORY_REQUEST_MULTIPLIER=$mem_req \
    CPU_LIMIT_MULTIPLIER=$cpu_lim \
    MEMORY_LIMIT_MULTIPLIER=$mem_lim
  
  sleep 10
  kubectl logs -n config-test deployment/right-sizer --tail=20 | grep -E "Configuration|Multiplier"
  echo "---"
done
```

## Summary

The right-sizer operator with configurable multipliers allows fine-tuning of resource allocation strategies through environment variables. Testing in Minikube provides a safe, isolated environment to validate different configurations before deploying to production clusters.

Key testing points:
1. ✅ Configuration values are loaded correctly from environment variables
2. ✅ Multipliers are applied correctly to calculate requests and limits
3. ✅ Minimum and maximum boundaries are enforced
4. ✅ Different configuration profiles (conservative, aggressive, performance) work as expected
5. ✅ The operator continues to function correctly with custom configurations

For production deployment, thoroughly test your chosen configuration values in a staging environment that mirrors your production workload patterns.