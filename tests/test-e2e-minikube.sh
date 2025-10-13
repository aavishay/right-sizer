#!/bin/bash

# End-to-End Test Script for Right-Sizer on Minikube
# This script performs comprehensive testing of the right-sizer operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-right-sizer}
TEST_NAMESPACE=${TEST_NAMESPACE:-test-rightsizer}
TIMEOUT=${TIMEOUT:-300}
TEST_NAME="right-sizer-e2e-$(date +%s)"

# Test results tracking
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

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

print_test_result() {
  local test_name=$1
  local result=$2

  TESTS_RUN=$((TESTS_RUN + 1))

  if [ $result -eq 0 ]; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
    print_color $GREEN "✅ PASS: $test_name"
  else
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("$test_name")
    print_color $RED "❌ FAIL: $test_name"
  fi
}

check_requirement() {
  local cmd=$1
  if ! command -v $cmd &>/dev/null; then
    print_color $RED "❌ $cmd is not installed"
    exit 1
  fi
}

wait_for_condition() {
  local condition=$1
  local resource=$2
  local namespace=$3
  local timeout=${4:-60}

  if kubectl wait --for=$condition --timeout=${timeout}s $resource -n $namespace 2>/dev/null; then
    return 0
  else
    return 1
  fi
}

cleanup() {
  print_header "Cleaning Up Test Resources"

  # Delete test deployments
  kubectl delete deployment oversized-app stress-test low-usage-app high-cpu-app --ignore-not-found=true -n $TEST_NAMESPACE 2>/dev/null || true
  kubectl delete service oversized-app stress-test low-usage-app high-cpu-app --ignore-not-found=true -n $TEST_NAMESPACE 2>/dev/null || true
  kubectl delete pod test-pod-* --ignore-not-found=true -n $TEST_NAMESPACE 2>/dev/null || true

  print_color $GREEN "✅ Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

print_header "Right-Sizer End-to-End Test Suite"
print_color $CYAN "Test ID: $TEST_NAME"

# Step 1: Verify Prerequisites
print_header "1. Verifying Prerequisites"

check_requirement kubectl
check_requirement minikube
check_requirement jq

# Check Minikube status
if ! minikube status -p right-sizer &>/dev/null; then
  print_color $RED "❌ Minikube cluster 'right-sizer' is not running"
  exit 1
fi

print_color $GREEN "✅ Minikube cluster 'right-sizer' is running"

# Check Kubernetes version
K8S_VERSION=$(kubectl version -o json 2>/dev/null | jq -r '.serverVersion.gitVersion')
print_color $YELLOW "📋 Kubernetes version: $K8S_VERSION"

# Step 2: Verify Right-Sizer Installation
print_header "2. Verifying Right-Sizer Installation"

# Check namespace exists
if kubectl get namespace $NAMESPACE &>/dev/null; then
  print_color $GREEN "✅ Namespace '$NAMESPACE' exists"
else
  print_color $RED "❌ Namespace '$NAMESPACE' not found"
  exit 1
fi

# Check deployment
if kubectl get deployment -n $NAMESPACE -l app.kubernetes.io/name=rightsizer &>/dev/null; then
  print_color $GREEN "✅ Right-sizer deployment found"
else
  print_color $RED "❌ Right-sizer deployment not found"
  exit 1
fi

# Wait for deployment to be ready
if wait_for_condition "condition=available" "deployment -l app.kubernetes.io/name=rightsizer" "$NAMESPACE"; then
  print_color $GREEN "✅ Right-sizer deployment is ready"
else
  print_color $RED "❌ Right-sizer deployment is not ready"
  exit 1
fi

# Check CRDs
for crd in rightsizerpolicies.rightsizer.io rightsizerconfigs.rightsizer.io; do
  if kubectl get crd $crd &>/dev/null; then
    print_color $GREEN "✅ CRD '$crd' exists"
  else
    print_color $RED "❌ CRD '$crd' not found"
    exit 1
  fi
done

# Step 3: Test Health Endpoints
print_header "3. Testing Health Endpoints"

POD_NAME=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=rightsizer -o jsonpath='{.items[0].metadata.name}')
print_color $CYAN "Testing pod: $POD_NAME"

# Test liveness probe
if kubectl exec -n $NAMESPACE $POD_NAME -- wget -q -O- http://localhost:8081/healthz &>/dev/null; then
  print_test_result "Liveness probe" 0
else
  print_test_result "Liveness probe" 1
fi

# Test readiness probe
if kubectl exec -n $NAMESPACE $POD_NAME -- wget -q -O- http://localhost:8081/readyz &>/dev/null; then
  print_test_result "Readiness probe" 0
else
  print_test_result "Readiness probe" 1
fi

# Step 4: Test Namespace Creation and Policy
print_header "4. Testing Namespace and Policy Configuration"

# Create test namespace if it doesn't exist
if ! kubectl get namespace $TEST_NAMESPACE &>/dev/null; then
  kubectl create namespace $TEST_NAMESPACE
  print_color $GREEN "✅ Created test namespace '$TEST_NAMESPACE'"
else
  print_color $CYAN "ℹ️  Test namespace '$TEST_NAMESPACE' already exists"
fi

# Create a test policy
cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: e2e-test-policy
  namespace: $NAMESPACE
spec:
  enabled: true
  targetNamespace: $TEST_NAMESPACE
  updateMode: "auto"
  resourceDefaults:
    cpu:
      min: "1m"
      max: "1000m"
    memory:
      min: "4Mi"
      max: "2Gi"
EOF

if [ $? -eq 0 ]; then
  print_test_result "Create RightSizerPolicy" 0
else
  print_test_result "Create RightSizerPolicy" 1
fi

# Step 5: Test Oversized Pod Detection
print_header "5. Testing Oversized Pod Detection and Resizing"

# Deploy oversized application
cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oversized-app
  namespace: $TEST_NAMESPACE
  labels:
    test: e2e-oversized
spec:
  replicas: 1
  selector:
    matchLabels:
      app: oversized-app
  template:
    metadata:
      labels:
        app: oversized-app
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "200m"
EOF

if wait_for_condition "condition=available" "deployment/oversized-app" "$TEST_NAMESPACE" 60; then
  print_test_result "Deploy oversized app" 0
else
  print_test_result "Deploy oversized app" 1
fi

# Wait for metrics to be available
print_color $YELLOW "⏳ Waiting 45s for metrics to be collected..."
sleep 45

# Check if pod was resized
POD_NAME=$(kubectl get pods -n $TEST_NAMESPACE -l app=oversized-app -o jsonpath='{.items[0].metadata.name}')
if [ -n "$POD_NAME" ]; then
  CPU_REQUEST=$(kubectl get pod $POD_NAME -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
  CPU_REQUEST_MILLI=$(echo $CPU_REQUEST | sed 's/m$//')

  if [ "$CPU_REQUEST_MILLI" -lt "100" ]; then
    print_test_result "Oversized pod CPU reduction" 0
    print_color $CYAN "  CPU reduced from 100m to $CPU_REQUEST"
  else
    print_test_result "Oversized pod CPU reduction" 1
    print_color $YELLOW "  CPU remains at $CPU_REQUEST"
  fi
fi

# Step 6: Test Low Usage Pod Detection
print_header "6. Testing Low Usage Pod Detection"

cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: low-usage-app
  namespace: $TEST_NAMESPACE
  labels:
    test: e2e-low-usage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: low-usage-app
  template:
    metadata:
      labels:
        app: low-usage-app
    spec:
      containers:
      - name: alpine
        image: alpine:latest
        command: ["sleep", "3600"]
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
EOF

if wait_for_condition "condition=available" "deployment/low-usage-app" "$TEST_NAMESPACE" 60; then
  print_test_result "Deploy low-usage app" 0
else
  print_test_result "Deploy low-usage app" 1
fi

# Step 7: Test High CPU Usage Pod
print_header "7. Testing High CPU Usage Pod"

cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: high-cpu-app
  namespace: $TEST_NAMESPACE
  labels:
    test: e2e-high-cpu
spec:
  replicas: 1
  selector:
    matchLabels:
      app: high-cpu-app
  template:
    metadata:
      labels:
        app: high-cpu-app
    spec:
      containers:
      - name: stress
        image: alpine:latest
        command:
        - sh
        - -c
        - |
          while true; do
            echo "Computing..." > /dev/null
            dd if=/dev/zero of=/dev/null bs=1M count=100 2>/dev/null
            sleep 5
          done
        resources:
          requests:
            memory: "32Mi"
            cpu: "10m"
          limits:
            memory: "64Mi"
            cpu: "50m"
EOF

if wait_for_condition "condition=available" "deployment/high-cpu-app" "$TEST_NAMESPACE" 60; then
  print_test_result "Deploy high-CPU app" 0
else
  print_test_result "Deploy high-CPU app" 1
fi

# Step 8: Test Metrics Collection
print_header "8. Testing Metrics Collection and API"

# Wait for another rightsizing cycle
print_color $YELLOW "⏳ Waiting 30s for rightsizing cycle..."
sleep 30

# Check operator logs for activity
RECENT_LOGS=$(kubectl logs -n $NAMESPACE deployment/right-sizer-rightsizer --tail=20 2>/dev/null)

if echo "$RECENT_LOGS" | grep -q "Rightsizing run completed"; then
  print_test_result "Rightsizing cycle execution" 0
else
  print_test_result "Rightsizing cycle execution" 1
fi

if echo "$RECENT_LOGS" | grep -q "Found.*resources needing adjustment"; then
  print_test_result "Resource adjustment detection" 0
else
  print_test_result "Resource adjustment detection" 1
fi

# Step 9: Test Skip Annotation
print_header "9. Testing Skip Annotation"

cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-skip
  namespace: $TEST_NAMESPACE
  annotations:
    rightsizer.io/disable: "true"
  labels:
    test: skip-annotation
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "200m"
EOF

if kubectl get pod test-pod-skip -n $TEST_NAMESPACE &>/dev/null; then
  print_test_result "Create pod with skip annotation" 0

  # Wait and check if pod was NOT resized
  sleep 30
  CPU_REQUEST=$(kubectl get pod test-pod-skip -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}' 2>/dev/null)

  if [ "$CPU_REQUEST" == "100m" ]; then
    print_test_result "Skip annotation respected" 0
  else
    print_test_result "Skip annotation respected" 1
    print_color $YELLOW "  CPU changed to $CPU_REQUEST (should remain 100m)"
  fi
else
  print_test_result "Create pod with skip annotation" 1
fi

# Step 10: Test Resource Limits
print_header "10. Testing Resource Limits Configuration"

# Check if limits are being respected
POD_NAME=$(kubectl get pods -n $TEST_NAMESPACE -l app=low-usage-app -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD_NAME" ]; then
  MEM_REQUEST=$(kubectl get pod $POD_NAME -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}' 2>/dev/null)

  # Convert memory to Mi for comparison
  MEM_VALUE=$(echo $MEM_REQUEST | sed 's/Mi$//')

  if [ -n "$MEM_VALUE" ] && [ "$MEM_VALUE" -ge "4" ]; then
    print_test_result "Minimum memory limit respected" 0
  else
    print_test_result "Minimum memory limit respected" 1
    print_color $YELLOW "  Memory is $MEM_REQUEST (should be >= 4Mi)"
  fi
fi

# Step 11: Verify Audit Logging
print_header "11. Testing Audit Logging"

# Check for audit events in logs
if kubectl logs -n $NAMESPACE deployment/right-sizer-rightsizer --tail=100 2>/dev/null | grep -q "Resized.*for container"; then
  print_test_result "Audit logging active" 0
else
  print_test_result "Audit logging active" 1
fi

# Step 12: Test Concurrent Updates
print_header "12. Testing Concurrent Pod Updates"

# Create multiple pods at once
for i in {1..3}; do
  cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-concurrent-$i
  namespace: $TEST_NAMESPACE
  labels:
    test: concurrent
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    resources:
      requests:
        memory: "128Mi"
        cpu: "50m"
      limits:
        memory: "256Mi"
        cpu: "100m"
EOF
done

sleep 5

CONCURRENT_PODS=$(kubectl get pods -n $TEST_NAMESPACE -l test=concurrent --no-headers | wc -l)
if [ "$CONCURRENT_PODS" -eq "3" ]; then
  print_test_result "Create concurrent pods" 0
else
  print_test_result "Create concurrent pods" 1
fi

# Wait for processing
sleep 30

# Check if batch processing occurred
if kubectl logs -n $NAMESPACE deployment/right-sizer-rightsizer --tail=50 2>/dev/null | grep -q "Processing.*pod updates in.*batches"; then
  print_test_result "Batch processing" 0
else
  print_test_result "Batch processing" 1
fi

# Final Summary
print_header "Test Summary"

print_color $CYAN "Tests Run: $TESTS_RUN"
print_color $GREEN "Tests Passed: $TESTS_PASSED"
print_color $RED "Tests Failed: $TESTS_FAILED"

if [ $TESTS_FAILED -gt 0 ]; then
  print_color $RED "\nFailed Tests:"
  for test in "${FAILED_TESTS[@]}"; do
    print_color $RED "  - $test"
  done
fi

echo ""
if [ $TESTS_FAILED -eq 0 ]; then
  print_color $GREEN "🎉 All tests passed successfully!"
  exit 0
else
  print_color $RED "❌ Some tests failed. Please review the results above."
  exit 1
fi
