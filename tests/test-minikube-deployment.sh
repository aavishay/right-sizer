#!/bin/bash

# Right-Sizer Minikube Sanity Test
# This script deploys and tests the right-sizer operator on Minikube

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-default}
DEPLOYMENT_NAME=${DEPLOYMENT_NAME:-right-sizer}
TEST_TIMEOUT=${TEST_TIMEOUT:-300}
MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-right-sizer}

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

print_header() {
  echo ""
  print_color $BLUE "========================================="
  print_color $BLUE "$1"
  print_color $BLUE "========================================="
}

check_command() {
  local cmd=$1
  if ! command -v $cmd &>/dev/null; then
    print_color $RED "Error: $cmd is not installed"
    return 1
  fi
  return 0
}

test_step() {
  local test_name=$1
  local test_command=$2

  echo -n "Testing: $test_name... "
  if eval $test_command &>/dev/null; then
    print_color $GREEN "PASSED"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    return 0
  else
    print_color $RED "FAILED"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    return 1
  fi
}

wait_for_condition() {
  local condition=$1
  local timeout=$2
  local message=$3

  echo -n "$message"
  local elapsed=0
  while [ $elapsed -lt $timeout ]; do
    if eval $condition &>/dev/null; then
      print_color $GREEN " Done!"
      return 0
    fi
    echo -n "."
    sleep 2
    elapsed=$((elapsed + 2))
  done
  print_color $RED " Timeout!"
  return 1
}

cleanup() {
  print_header "Cleaning Up"

  # Delete test resources
  kubectl delete deployment test-nginx --ignore-not-found=true -n $NAMESPACE &>/dev/null || true
  kubectl delete rightsizerconfig test-config --ignore-not-found=true -n $NAMESPACE &>/dev/null || true
  kubectl delete rightsizerpolicy test-policy --ignore-not-found=true -n $NAMESPACE &>/dev/null || true

  # Uninstall operator
  if [ -f "helm/Chart.yaml" ]; then
    helm uninstall $DEPLOYMENT_NAME -n $NAMESPACE &>/dev/null || true
  else
    kubectl delete deployment $DEPLOYMENT_NAME -n $NAMESPACE --ignore-not-found=true &>/dev/null || true
  fi

  # Stop Minikube if we started it
  if [ "$STOP_MINIKUBE" = "true" ]; then
    print_color $YELLOW "Stopping Minikube profile: $MINIKUBE_PROFILE"
    minikube stop -p $MINIKUBE_PROFILE || true
    minikube delete -p $MINIKUBE_PROFILE || true
  fi
}

# Trap cleanup on exit
trap cleanup EXIT

print_header "Right-Sizer Minikube Sanity Test"

# Step 1: Check prerequisites
print_header "Checking Prerequisites"

if ! check_command minikube; then
  exit 1
fi

if ! check_command kubectl; then
  exit 1
fi

if ! check_command helm; then
  print_color $YELLOW "Warning: helm not found, will use kubectl for deployment"
fi

# Step 2: Setup Minikube
print_header "Setting Up Minikube"

# Check if Minikube is already running
if minikube status -p $MINIKUBE_PROFILE &>/dev/null; then
  print_color $GREEN "Using existing Minikube profile: $MINIKUBE_PROFILE"
  STOP_MINIKUBE=false
else
  print_color $YELLOW "Starting new Minikube profile: $MINIKUBE_PROFILE"
  minikube start -p $MINIKUBE_PROFILE \
    --memory=4096 \
    --cpus=2 \
    --kubernetes-version=v1.28.0 \
    --addons=metrics-server
  STOP_MINIKUBE=true
fi

# Set kubectl context
kubectl config use-context $MINIKUBE_PROFILE

# Wait for cluster to be ready
wait_for_condition "kubectl get nodes | grep -q Ready" 60 "Waiting for cluster to be ready"

# Step 3: Build and Load Docker Image
print_header "Building and Loading Docker Image"

# Build the operator image
print_color $YELLOW "Building Docker image..."
docker build -t right-sizer:test .

# Load image into Minikube
print_color $YELLOW "Loading image into Minikube..."
minikube image load right-sizer:test -p $MINIKUBE_PROFILE

# Step 4: Deploy CRDs
print_header "Deploying CRDs"

kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml

# Wait for CRDs to be established
wait_for_condition "kubectl get crd rightsizerconfigs.rightsizer.io" 30 "Waiting for CRDs"

# Step 5: Deploy Operator
print_header "Deploying Right-Sizer Operator"

# Create deployment manifest for testing
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer
rules:
- apiGroups: [""]
  resources: ["pods", "pods/status"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs", "rightsizerpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs/status", "rightsizerpolicies/status"]
  verbs: ["get", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer
subjects:
- kind: ServiceAccount
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $DEPLOYMENT_NAME
  namespace: $NAMESPACE
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
        ports:
        - containerPort: 8081
          name: health
        - containerPort: 8080
          name: metrics
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            memory: "256Mi"
            cpu: "500m"
          requests:
            memory: "128Mi"
            cpu: "100m"
EOF

# Wait for deployment to be ready
wait_for_condition "kubectl get deployment $DEPLOYMENT_NAME -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1" 120 "Waiting for operator deployment"

# Get pod name
POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')
print_color $GREEN "Operator pod: $POD_NAME"

# Step 6: Test Health Endpoints
print_header "Testing Health Endpoints"

# Port-forward for testing
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8081:8081 &
PF_PID=$!
sleep 3

# Test liveness endpoint
test_step "Liveness probe (/healthz)" "curl -f http://localhost:8081/healthz"

# Test readiness endpoint
test_step "Readiness probe (/readyz)" "curl -f http://localhost:8081/readyz"

# Kill port-forward
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

# Step 7: Deploy Test Configuration
print_header "Testing CRD Configuration"

# Create a test configuration
cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: test-config
  namespace: $NAMESPACE
spec:
  enabled: true
  dryRun: true
  defaultMode: balanced
  resizeInterval: "30s"
  globalConstraints:
    cpu:
      min: "10m"
      max: "2000m"
    memory:
      min: "10Mi"
      max: "2Gi"
  metricsConfig:
    provider: metrics-server
  observabilityConfig:
    enableMetricsExport: true
    metricsPort: 9090
EOF

# Wait for config to be processed
sleep 5

# Check if config was processed
test_step "RightSizerConfig created" "kubectl get rightsizerconfig test-config -n $NAMESPACE"
test_step "RightSizerConfig has status" "kubectl get rightsizerconfig test-config -n $NAMESPACE -o jsonpath='{.status.phase}' | grep -E 'Active|Ready'"

# Step 8: Deploy Test Policy
print_header "Testing Policy Configuration"

cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: test-policy
  namespace: $NAMESPACE
spec:
  enabled: true
  namespaceSelector:
    include:
    - $NAMESPACE
  workloadSelector:
    types:
    - Deployment
    labels:
      test: "true"
  scalingPolicy:
    cpu:
      targetUtilization: 70
      scaleUpThreshold: 80
      scaleDownThreshold: 50
    memory:
      targetUtilization: 75
      scaleUpThreshold: 85
      scaleDownThreshold: 60
EOF

# Wait for policy to be processed
sleep 5

# Check if policy was processed
test_step "RightSizerPolicy created" "kubectl get rightsizerpolicy test-policy -n $NAMESPACE"
test_step "RightSizerPolicy has status" "kubectl get rightsizerpolicy test-policy -n $NAMESPACE -o jsonpath='{.status.conditions}' | grep -q type"

# Step 9: Test with Sample Workload
print_header "Testing with Sample Workload"

# Deploy a test workload
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-nginx
  namespace: $NAMESPACE
  labels:
    test: "true"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-nginx
  template:
    metadata:
      labels:
        app: test-nginx
        test: "true"
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            memory: "100Mi"
            cpu: "100m"
          limits:
            memory: "200Mi"
            cpu: "200m"
EOF

# Wait for test deployment
wait_for_condition "kubectl get deployment test-nginx -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1" 60 "Waiting for test deployment"

# Check operator logs for processing
print_color $YELLOW "Checking operator logs for workload processing..."
kubectl logs $POD_NAME -n $NAMESPACE --tail=20 | grep -i "test-nginx" && print_color $GREEN "Workload detected by operator" || print_color $YELLOW "Workload not yet processed"

# Step 10: Check Metrics
print_header "Testing Metrics Endpoint"

# Port-forward for metrics
kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8080:8080 &
PF_PID=$!
sleep 3

# Test metrics endpoint
test_step "Metrics endpoint (/metrics)" "curl -f http://localhost:8080/metrics | grep -q rightsizer"

# Kill port-forward
kill $PF_PID 2>/dev/null || true
wait $PF_PID 2>/dev/null || true

# Step 11: Test Operator Restart Recovery
print_header "Testing Operator Recovery"

# Delete pod to test recovery
kubectl delete pod $POD_NAME -n $NAMESPACE
wait_for_condition "kubectl get deployment $DEPLOYMENT_NAME -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1" 120 "Waiting for operator recovery"

# Get new pod name
NEW_POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')
test_step "Operator recovered after restart" "[ '$NEW_POD_NAME' != '$POD_NAME' ]"

# Final Summary
print_header "Test Summary"

TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
echo "Total Tests: $TOTAL_TESTS"
print_color $GREEN "Passed: $TESTS_PASSED"
print_color $RED "Failed: $TESTS_FAILED"

if [ $TESTS_FAILED -eq 0 ]; then
  print_color $GREEN "\n✅ All sanity tests passed!"
  exit 0
else
  print_color $RED "\n❌ Some tests failed"
  exit 1
fi
