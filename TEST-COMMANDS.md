# Right-Sizer Testing Commands - Quick Start

## Prerequisites Check

```bash
# Check required tools
minikube version
kubectl version --client
docker version
helm version

# Check Minikube status
minikube status
```

## 1. Start Minikube with Enhanced Resources

```bash
# Start Minikube with sufficient resources for all features
minikube start \
  --memory=8192 \
  --cpus=4 \
  --kubernetes-version=v1.33.1 \
  --extra-config=apiserver.enable-admission-plugins=ValidatingAdmissionWebhook,MutatingAdmissionWebhook

# Enable required addons
minikube addons enable metrics-server
minikube addons enable ingress

# Verify cluster is ready
kubectl cluster-info
kubectl get nodes
```

## 2. Build and Deploy Right-Sizer

```bash
# Navigate to project root
cd right-sizer

# Use Minikube's Docker daemon
eval $(minikube docker-env)

# Build the enhanced image
docker build -t right-sizer:test .

# Verify image
docker images | grep right-sizer

# Create namespace
kubectl create namespace right-sizer-system

# Deploy with Helm (basic configuration first)
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --set image.repository=right-sizer \
  --set image.tag=test \
  --set image.pullPolicy=Never \
  --set config.logLevel=debug \
  --set config.resizeInterval=30s
```

## 3. Verify Basic Deployment

```bash
# Check pod status
kubectl get pods -n right-sizer-system

# Wait for pod to be ready
kubectl wait --for=condition=ready pod -l app=right-sizer -n right-sizer-system --timeout=300s

# Check logs for successful startup
kubectl logs -n right-sizer-system deployment/right-sizer-operator | head -50

# Look for configuration loading
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep "Configuration Loaded"
```

## 4. Test Health and Metrics

```bash
# Test health endpoints
kubectl port-forward -n right-sizer-system service/right-sizer-operator 8081:8081 &
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
pkill -f "kubectl port-forward.*8081:8081"

# Test metrics endpoint
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &
curl http://localhost:9090/metrics | grep rightsizer_ | head -10
pkill -f "kubectl port-forward.*9090:9090"
```

## 5. Deploy Test Workloads

```bash
# Create a basic test application
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
  labels:
    app: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
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
EOF

# Wait for deployment
kubectl wait --for=condition=available deployment/test-app --timeout=120s

# Check initial resources
kubectl get deployment test-app -o yaml | grep -A 10 resources:
```

## 6. Monitor Resource Processing

```bash
# Watch operator logs for processing
kubectl logs -n right-sizer-system deployment/right-sizer-operator -f | grep -E "Processing|pods|resource"

# In another terminal, check for resource changes after a few minutes
kubectl get deployment test-app -o yaml | grep -A 10 resources:

# Check events
kubectl get events --sort-by='.lastTimestamp' | grep -i resource | tail -5
```

## 7. Test Enhanced Features

### Enable Policy-Based Sizing

```bash
# Apply policy rules
kubectl apply -f examples/policy-rules-example.yaml

# Enable policy engine
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set config.policyBasedSizing=true \
  --set policyEngine.enabled=true

# Deploy high-priority workload
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: high-priority-app
  namespace: default
  labels:
    priority: high
    environment: production
spec:
  replicas: 1
  selector:
    matchLabels:
      app: high-priority-app
  template:
    metadata:
      labels:
        app: high-priority-app
        priority: high
        environment: production
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
EOF

# Check for policy applications
sleep 60
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "policy.*applied"
```

### Test Resource Validation

```bash
# Try to create pod with excessive resources
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: resource-test
  namespace: default
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sleep", "300"]
    resources:
      requests:
        cpu: "8"      # High CPU
        memory: "16Gi" # High memory
EOF

# Check validation logs
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "validation\|threshold"
```

### Enable Audit Logging

```bash
# Enable audit logging
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set observability.auditEnabled=true

# Check audit logs
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "audit"

# Check for Kubernetes events created by auditing
kubectl get events --field-selector source=right-sizer
```

## 8. Test Metrics and Observability

```bash
# Get comprehensive metrics
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &

# Check different metric types
echo "=== Pod Processing Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_pods_processed_total

echo "=== Resource Change Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_resource_change

echo "=== Safety Threshold Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_safety_threshold

echo "=== Performance Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_processing_duration

pkill -f "kubectl port-forward.*9090:9090"
```

## 9. Test Circuit Breaker and Retry Logic

```bash
# Check retry metrics
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &
curl -s http://localhost:9090/metrics | grep -E "rightsizer.*(retry|circuit)"
pkill -f "kubectl port-forward.*9090:9090"

# Check logs for retry activity
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "retry\|circuit"
```

## 10. Performance Testing

```bash
# Create multiple workloads for performance testing
for i in {1..5}; do
  kubectl create deployment perf-test-$i --image=nginx:alpine --replicas=2
  kubectl set resources deployment perf-test-$i --requests=cpu=${i}0m,memory=$((i*64))Mi
done

# Monitor processing performance
kubectl logs -n right-sizer-system deployment/right-sizer-operator -f | grep -E "Processing|duration"

# Check performance metrics
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &
curl -s http://localhost:9090/metrics | grep rightsizer_processing_duration_seconds
pkill -f "kubectl port-forward.*9090:9090"

# Cleanup performance test
for i in {1..5}; do
  kubectl delete deployment perf-test-$i
done
```

## 11. Test Failure Recovery

```bash
# Test operator recovery by restarting pod
kubectl delete pod -n right-sizer-system -l app=right-sizer

# Wait for new pod to be ready
kubectl wait --for=condition=ready pod -l app=right-sizer -n right-sizer-system --timeout=180s

# Verify operator reinitializes correctly
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep "Starting.*right-sizer"
```

## 12. Run Automated Test Suite

```bash
# Make test script executable
chmod +x scripts/minikube-test.sh

# Run comprehensive test suite
./scripts/minikube-test.sh

# Or run quick tests only
./scripts/minikube-test.sh --quick
```

## Troubleshooting Commands

### Check Overall Status
```bash
# Get all resources in right-sizer namespace
kubectl get all -n right-sizer-system

# Describe operator pod
kubectl describe pod -l app=right-sizer -n right-sizer-system

# Check recent events
kubectl get events -n right-sizer-system --sort-by='.lastTimestamp' | tail -10
```

### Debug Configuration Issues
```bash
# Check environment variables
kubectl get deployment right-sizer-operator -n right-sizer-system -o yaml | grep -A 20 env:

# Check ConfigMap
kubectl get configmap right-sizer-config -n right-sizer-system -o yaml

# Check for configuration errors
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "error\|fail\|invalid"
```

### Debug Processing Issues
```bash
# Check if metrics-server is working
kubectl top nodes
kubectl top pods --all-namespaces

# Verify RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:right-sizer-system:right-sizer-operator
kubectl auth can-i patch pods --as=system:serviceaccount:right-sizer-system:right-sizer-operator

# Check processing logs with debug level
kubectl logs -n right-sizer-system deployment/right-sizer-operator | grep -i "debug\|processing"
```

## Cleanup

```bash
# Remove test workloads
kubectl delete deployment test-app high-priority-app resource-test --ignore-not-found=true
kubectl delete pod resource-test --ignore-not-found=true

# Remove right-sizer
helm uninstall right-sizer -n right-sizer-system

# Remove namespace
kubectl delete namespace right-sizer-system

# Stop Minikube (optional)
minikube stop

# Delete Minikube cluster (complete cleanup)
# minikube delete
```

## Expected Results

After running these tests, you should see:

1. **Configuration Loading**: Logs showing multipliers and settings loaded
2. **Pod Processing**: Operator processing pods every 30 seconds (default interval)
3. **Resource Adjustments**: Changes to pod resource requests/limits based on usage
4. **Policy Applications**: Higher priority workloads getting more resources
5. **Metrics**: Prometheus metrics showing operator activity
6. **Audit Events**: Logs and Kubernetes events tracking changes
7. **Health Endpoints**: Responding with healthy status
8. **Validation**: Warnings/errors for invalid resource configurations

## Key Metrics to Monitor

```bash
# Core processing metrics
rightsizer_pods_processed_total
rightsizer_pods_resized_total
rightsizer_processing_duration_seconds

# Safety and validation
rightsizer_safety_threshold_violations_total
rightsizer_resource_validation_errors_total

# Performance and reliability
rightsizer_retry_attempts_total
rightsizer_circuit_breaker_state
```

This testing approach validates all enhanced features while providing clear visibility into the operator's behavior and performance.