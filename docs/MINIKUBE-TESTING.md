# Enhanced Right-Sizer Minikube Testing Guide

This comprehensive guide provides step-by-step instructions for testing the enhanced right-sizer operator with all new features including policy-based sizing, admission controllers, audit logging, and advanced observability in a Minikube environment.

## Table of Contents
- [Prerequisites](#prerequisites)  
- [Environment Setup](#environment-setup)
- [Basic Feature Testing](#basic-feature-testing)
- [Enhanced Features Testing](#enhanced-features-testing)
- [Policy Engine Testing](#policy-engine-testing)
- [Security & Admission Controller Testing](#security--admission-controller-testing)
- [Observability Testing](#observability-testing)
- [End-to-End Scenarios](#end-to-end-scenarios)
- [Performance Testing](#performance-testing)
- [Troubleshooting](#troubleshooting)
- [Cleanup](#cleanup)

## Prerequisites

### Required Tools
- **Minikube**: v1.32+ ([Installation Guide](https://minikube.sigs.k8s.io/docs/start/))
- **kubectl**: v1.33+ ([Installation Guide](https://kubernetes.io/docs/tasks/tools/))
- **Docker**: v24.0+ ([Installation Guide](https://docs.docker.com/get-docker/))
- **Go**: v1.24+ (for building from source)
- **Helm**: v3.12+ ([Installation Guide](https://helm.sh/docs/intro/install/))
- **cert-manager**: For admission controller testing

### System Requirements
- Minimum 8GB RAM available for Minikube (enhanced features require more resources)
- Minimum 4 CPU cores 
- 40GB free disk space
- Internet connection for pulling required images

### Feature Dependencies
- **Kubernetes 1.33+**: For in-place pod resizing feature
- **Prometheus**: For metrics testing (will be installed via addons)
- **cert-manager**: For admission webhook certificates

## Environment Setup

### 1. Start Enhanced Minikube Cluster

```bash
# Start Minikube with enhanced resources for all features
minikube start \
  --memory=8192 \
  --cpus=4 \
  --kubernetes-version=v1.33.1 \
  --extra-config=apiserver.enable-admission-plugins=ValidatingAdmissionWebhook,MutatingAdmissionWebhook \
  --addons=metrics-server,ingress

# Enable required addons
minikube addons enable metrics-server
minikube addons enable prometheus  # For metrics testing

# Use Minikube's Docker daemon
eval $(minikube docker-env)
```

### 2. Build Enhanced Right-Sizer Image

```bash
# Navigate to project root
cd right-sizer

# Build the enhanced image with all features
docker build -t right-sizer:enhanced-test .

# Verify image exists
docker images | grep right-sizer
```

### 3. Install Dependencies

```bash
# Install cert-manager for admission controller
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app=cert-manager -n cert-manager --timeout=300s

# Create right-sizer namespace
kubectl create namespace right-sizer-system
```

## Basic Feature Testing

### Test 1: Basic Configuration and Deployment

```bash
# Deploy basic configuration first
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: right-sizer-config
  namespace: right-sizer-system
data:
  CPU_REQUEST_MULTIPLIER: "1.3"
  MEMORY_REQUEST_MULTIPLIER: "1.2"
  CPU_LIMIT_MULTIPLIER: "2.0"
  MEMORY_LIMIT_MULTIPLIER: "2.0"
  LOG_LEVEL: "debug"
  RESIZE_INTERVAL: "30s"
  METRICS_ENABLED: "true"
  AUDIT_ENABLED: "true"
EOF

# Deploy using Helm with enhanced features disabled initially
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --set image.repository=right-sizer \
  --set image.tag=enhanced-test \
  --set image.pullPolicy=Never \
  --set config.policyBasedSizing=false \
  --set security.admissionWebhook.enabled=false \
  --set observability.metricsEnabled=true \
  --set observability.auditEnabled=true
```

### Test 2: Verify Basic Functionality

```bash
# Check operator startup
kubectl logs -n right-sizer-system deployment/right-sizer -f | head -50

# Deploy test workload
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-basic
  namespace: default
  labels:
    app: test-app-basic
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app-basic
  template:
    metadata:
      labels:
        app: test-app-basic
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

# Wait and check for resource adjustments
sleep 60
kubectl get deployment test-app-basic -o yaml | grep -A 10 resources:
```

## Enhanced Features Testing

### Test 3: Policy Engine Testing

```bash
# Create policy ConfigMap
kubectl apply -f examples/policy-rules-example.yaml

# Enable policy-based sizing
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set config.policyBasedSizing=true \
  --set policyEngine.enabled=true \
  --set policyEngine.configMapName=right-sizer-policies

# Deploy workloads that match different policy rules
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: high-priority-app
  namespace: default
  labels:
    app: high-priority-app
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
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database-app
  namespace: default
  labels:
    app.kubernetes.io/component: database
spec:
  replicas: 1
  selector:
    matchLabels:
      app: database-app
  template:
    metadata:
      labels:
        app: database-app
        app.kubernetes.io/component: database
    spec:
      containers:
      - name: db
        image: postgres:alpine
        env:
        - name: POSTGRES_DB
          value: test
        - name: POSTGRES_USER
          value: test
        - name: POSTGRES_PASSWORD
          value: test
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
EOF

# Check policy applications
sleep 120
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "policy.*applied"
```

### Test 4: Validation Engine Testing

```bash
# Test resource validation by creating pods that should trigger validation
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: invalid-resources-test
  namespace: default
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sleep", "3600"]
    resources:
      requests:
        cpu: "16"      # Exceeds typical node capacity
        memory: "64Gi"  # Exceeds typical node capacity
      limits:
        cpu: "32"
        memory: "128Gi"
EOF

# Check validation logs
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "validation\|error"
```

## Policy Engine Testing

### Test 5: Comprehensive Policy Rules

```bash
# Apply comprehensive policy configuration
kubectl apply -f examples/policy-rules-example.yaml

# Verify policies are loaded
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "policy.*rules.*loaded"

# Create workloads that match different policies
kubectl apply -f - <<EOF
# High priority production workload
apiVersion: apps/v1
kind: Deployment
metadata:
  name: critical-service
  namespace: default
  labels:
    priority: high
    environment: production
spec:
  replicas: 2
  selector:
    matchLabels:
      app: critical-service
  template:
    metadata:
      labels:
        app: critical-service
        priority: high
        environment: production
    spec:
      containers:
      - name: service
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
---
# Development workload (should get conservative resources)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dev-service
  namespace: default
  labels:
    environment: dev
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dev-service
  template:
    metadata:
      labels:
        app: dev-service
        environment: dev
    spec:
      containers:
      - name: service
        image: nginx:alpine
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
---
# Batch job (should get job-specific limits)  
apiVersion: batch/v1
kind: Job
metadata:
  name: batch-processor
  namespace: default
spec:
  template:
    metadata:
      labels:
        app: batch-processor
    spec:
      restartPolicy: Never
      containers:
      - name: processor
        image: busybox:latest
        command: ["sleep", "300"]
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
EOF

# Monitor policy applications
sleep 60
kubectl logs -n right-sizer-system deployment/right-sizer | grep -E "policy.*applied|rule.*matched" | tail -10
```

### Test 6: Time-Based Policy Rules

```bash
# Create time-based policy for business hours
kubectl patch configmap right-sizer-policies -n right-sizer-system --type merge -p='
{
  "data": {
    "rules.yaml": "
    - name: business-hours-boost
      priority: 100
      enabled: true
      selectors:
        labels:
          scaling-policy: business-hours
      schedule:
        timeRanges:
          - start: \"08:00\"
            end: \"18:00\"
        daysOfWeek:
          - Monday
          - Tuesday
          - Wednesday
          - Thursday
          - Friday
      actions:
        cpuMultiplier: 1.5
        memoryMultiplier: 1.3
    "
  }
}'

# Deploy workload with business hours policy
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: business-app
  namespace: default
  labels:
    scaling-policy: business-hours
spec:
  replicas: 1
  selector:
    matchLabels:
      app: business-app
  template:
    metadata:
      labels:
        app: business-app
        scaling-policy: business-hours
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
EOF

# Check if time-based policy is being evaluated
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "schedule\|time.*range\|business"
```

## Security & Admission Controller Testing

### Test 7: Enable Admission Controller

```bash
# Generate certificates for webhook
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: right-sizer-admission-certs
  namespace: right-sizer-system
spec:
  secretName: right-sizer-admission-certs
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  dnsNames:
  - right-sizer-admission-webhook.right-sizer-system.svc
  - right-sizer-admission-webhook.right-sizer-system.svc.cluster.local
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
EOF

# Enable admission controller
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set security.admissionWebhook.enabled=true \
  --set security.admissionWebhook.port=8443

# Wait for webhook to be ready
kubectl wait --for=condition=ready pod -l app=right-sizer -n right-sizer-system --timeout=300s
```

### Test 8: Test Admission Validation

```bash
# Try to create pod with invalid resources (should be blocked)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: invalid-resource-pod
  namespace: default
spec:
  containers:
  - name: invalid
    image: nginx:alpine
    resources:
      requests:
        cpu: "50"     # Exceeds max CPU limit
        memory: "100Gi" # Exceeds max memory limit
      limits:
        cpu: "100"
        memory: "200Gi"
EOF

# Check admission controller logs
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "admission\|webhook\|validation"
```

### Test 9: Test Admission Mutation

```bash
# Enable mutation webhook
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set security.admissionWebhook.enableMutation=true

# Create pod without resource specifications (should be auto-added)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: auto-resource-pod
  namespace: default
spec:
  containers:
  - name: app
    image: nginx:alpine
    # No resources specified - should be added by mutating webhook
EOF

# Check if resources were automatically added
kubectl get pod auto-resource-pod -o yaml | grep -A 10 resources:
```

## Observability Testing

### Test 10: Metrics and Monitoring

```bash
# Port forward to metrics endpoint
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &

# Wait for port forward to establish
sleep 5

# Check available metrics
curl -s http://localhost:9090/metrics | grep rightsizer_ | head -20

# Check specific metrics
echo "=== Pod Processing Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_pods_processed_total

echo "=== Resource Change Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_resource_change

echo "=== Safety Threshold Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_safety_threshold

echo "=== Circuit Breaker Metrics ==="
curl -s http://localhost:9090/metrics | grep rightsizer_retry

# Kill port forward
pkill -f "kubectl port-forward.*9090:9090"
```

### Test 11: Audit Logging

```bash
# Check audit logging is enabled
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "audit.*initialized"

# Generate some activity to create audit events
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: audit-test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: audit-test-app
  template:
    metadata:
      labels:
        app: audit-test-app
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 75m
            memory: 96Mi
EOF

# Wait for processing
sleep 120

# Check audit logs
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "audit.*event" | tail -10

# Check Kubernetes events created by auditing
kubectl get events --field-selector source=right-sizer --sort-by='.lastTimestamp' | tail -5
```

### Test 12: Health Checks and Circuit Breakers

```bash
# Test health endpoints
kubectl port-forward -n right-sizer-system deployment/right-sizer 8081:8081 &
sleep 5

echo "=== Health Check ==="
curl -f http://localhost:8081/healthz

echo "=== Ready Check ==="
curl -f http://localhost:8081/readyz

# Test circuit breaker by creating failing scenarios
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "circuit.*breaker\|retry.*attempt" | tail -5

pkill -f "kubectl port-forward.*8081:8081"
```

## End-to-End Scenarios

### Test 13: Complete Production Simulation

```bash
# Deploy production-like configuration
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --set config.policyBasedSizing=true \
  --set security.admissionWebhook.enabled=true \
  --set observability.metricsEnabled=true \
  --set observability.auditEnabled=true \
  --set config.safetyThreshold=0.3 \
  --set config.maxRetries=5

# Create diverse workload types
kubectl apply -f - <<EOF
# Production web service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-frontend
  namespace: default
  labels:
    app.kubernetes.io/component: frontend
    tier: web
    priority: high
    environment: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-frontend
  template:
    metadata:
      labels:
        app: web-frontend
        app.kubernetes.io/component: frontend
        tier: web
        priority: high
        environment: production
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
---
# Database service
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: database
  namespace: default
  labels:
    app.kubernetes.io/component: database
spec:
  serviceName: database
  replicas: 1
  selector:
    matchLabels:
      app: database
  template:
    metadata:
      labels:
        app: database
        app.kubernetes.io/component: database
    spec:
      containers:
      - name: postgres
        image: postgres:13-alpine
        env:
        - name: POSTGRES_DB
          value: webapp
        - name: POSTGRES_USER
          value: webapp
        - name: POSTGRES_PASSWORD
          value: webapp123
        resources:
          requests:
            cpu: 200m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 2Gi
---
# Background worker
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: default
  labels:
    app.kubernetes.io/component: worker
    queue-type: background
spec:
  replicas: 2
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
        app.kubernetes.io/component: worker
        queue-type: background
    spec:
      containers:
      - name: worker
        image: busybox:latest
        command: ["sh", "-c", "while true; do echo processing...; sleep 30; done"]
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
EOF

# Monitor comprehensive operations
sleep 180

echo "=== Policy Applications ==="
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i "policy.*applied" | tail -5

echo "=== Resource Changes ==="
kubectl get events --field-selector reason=ResourceChange --sort-by='.lastTimestamp' | tail -5

echo "=== Final Resource States ==="
for deploy in web-frontend worker; do
  echo "--- $deploy ---"
  kubectl get deployment $deploy -o yaml | grep -A 8 resources: | head -10
done

kubectl get statefulset database -o yaml | grep -A 8 resources: | head -10
```

## Performance Testing

### Test 14: Load and Scale Testing

```bash
# Create multiple workloads to test operator performance
for i in {1..10}; do
  kubectl create deployment load-test-$i --image=nginx:alpine --replicas=2
  kubectl set resources deployment load-test-$i --requests=cpu=${i}0m,memory=$((i*64))Mi --limits=cpu=$((i*200))m,memory=$((i*256))Mi
done

# Monitor processing performance
kubectl port-forward -n right-sizer-system service/right-sizer-operator 9090:9090 &
sleep 5

# Check processing duration metrics
echo "=== Processing Duration ==="
curl -s http://localhost:9090/metrics | grep rightsizer_processing_duration_seconds

# Check throughput metrics  
echo "=== Processing Throughput ==="
curl -s http://localhost:9090/metrics | grep rightsizer_pods_processed_total

pkill -f "kubectl port-forward.*9090:9090"

# Cleanup load test
for i in {1..10}; do
  kubectl delete deployment load-test-$i
done
```

### Test 15: Failure Recovery Testing

```bash
# Test operator resilience by simulating failures
echo "=== Testing Pod Restart Recovery ==="
kubectl delete pod -n right-sizer-system -l app=right-sizer

# Wait for pod to restart
kubectl wait --for=condition=ready pod -l app=right-sizer -n right-sizer-system --timeout=300s

# Verify operator recovers and continues processing
kubectl logs -n right-

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