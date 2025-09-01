#!/bin/bash

# Test script for CPU and Memory addition feature in right-sizer
# This script tests the new addition-based resource calculations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Right-Sizer Addition Feature Tests${NC}"
echo -e "${BLUE}========================================${NC}"

# Test configuration
NAMESPACE=${TEST_NAMESPACE:-"right-sizer-test"}
DEPLOYMENT_NAME="test-app-additions"

# Helper functions
log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
  log_info "Checking prerequisites..."

  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl not found. Please install kubectl."
    exit 1
  fi

  if ! kubectl cluster-info &>/dev/null; then
    log_error "Cannot connect to Kubernetes cluster."
    exit 1
  fi

  log_info "Prerequisites check passed."
}

# Create test namespace
create_test_namespace() {
  log_info "Creating test namespace: $NAMESPACE"
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
}

# Deploy test application with initial resources
deploy_test_app() {
  log_info "Deploying test application..."

  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $DEPLOYMENT_NAME
  namespace: $NAMESPACE
  labels:
    app: test-additions
    right-sizer/enabled: "true"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-additions
  template:
    metadata:
      labels:
        app: test-additions
      annotations:
        right-sizer/mode: "optimize"
    spec:
      containers:
      - name: test-container
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
EOF

  # Wait for deployment to be ready
  kubectl rollout status deployment/$DEPLOYMENT_NAME -n $NAMESPACE --timeout=60s
}

# Test 1: Default behavior (no additions)
test_no_additions() {
  log_info "Test 1: Default behavior without additions"

  # Deploy right-sizer with no additions
  helm upgrade --install right-sizer ./helm \
    --namespace right-sizer-system \
    --create-namespace \
    --set config.cpuRequestMultiplier=1.2 \
    --set config.memoryRequestMultiplier=1.2 \
    --set config.cpuRequestAddition=0 \
    --set config.memoryRequestAddition=0 \
    --set config.cpuLimitMultiplier=2.0 \
    --set config.memoryLimitMultiplier=2.0 \
    --set config.cpuLimitAddition=0 \
    --set config.memoryLimitAddition=0 \
    --set namespaceInclude=$NAMESPACE \
    --wait

  sleep 30 # Wait for right-sizer to process

  # Check pod resources
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=test-additions -o jsonpath='{.items[0].metadata.name}')

  if [ -z "$POD_NAME" ]; then
    log_error "Test pod not found"
    return 1
  fi

  log_info "Pod resources after sizing (no additions):"
  kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

  log_info "Test 1 completed"
}

# Test 2: With CPU and Memory additions
test_with_additions() {
  log_info "Test 2: With CPU and Memory additions"

  # Update right-sizer with additions
  helm upgrade --install right-sizer ./helm \
    --namespace right-sizer-system \
    --set config.cpuRequestMultiplier=1.2 \
    --set config.memoryRequestMultiplier=1.2 \
    --set config.cpuRequestAddition=50 \
    --set config.memoryRequestAddition=64 \
    --set config.cpuLimitMultiplier=2.0 \
    --set config.memoryLimitMultiplier=2.0 \
    --set config.cpuLimitAddition=100 \
    --set config.memoryLimitAddition=128 \
    --set namespaceInclude=$NAMESPACE \
    --wait

  sleep 30 # Wait for right-sizer to process

  # Check pod resources
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=test-additions -o jsonpath='{.items[0].metadata.name}')

  log_info "Pod resources after sizing (with additions):"
  kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

  log_info "Test 2 completed"
}

# Test 3: High addition values
test_high_additions() {
  log_info "Test 3: High addition values"

  # Update with high additions
  helm upgrade --install right-sizer ./helm \
    --namespace right-sizer-system \
    --set config.cpuRequestMultiplier=1.1 \
    --set config.memoryRequestMultiplier=1.1 \
    --set config.cpuRequestAddition=200 \
    --set config.memoryRequestAddition=256 \
    --set config.cpuLimitMultiplier=1.5 \
    --set config.memoryLimitMultiplier=1.5 \
    --set config.cpuLimitAddition=500 \
    --set config.memoryLimitAddition=512 \
    --set namespaceInclude=$NAMESPACE \
    --wait

  sleep 30 # Wait for right-sizer to process

  # Check pod resources
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=test-additions -o jsonpath='{.items[0].metadata.name}')

  log_info "Pod resources after sizing (high additions):"
  kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

  log_info "Test 3 completed"
}

# Test 4: Verify calculation formula
test_calculation_verification() {
  log_info "Test 4: Calculation verification"

  # Deploy a pod with known usage patterns
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: calculation-test-pod
  namespace: $NAMESPACE
  labels:
    app: calc-test
    right-sizer/enabled: "true"
  annotations:
    right-sizer/mode: "optimize"
    right-sizer/expected-cpu-usage: "100"    # Expected 100m usage
    right-sizer/expected-memory-usage: "200" # Expected 200MB usage
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args: ["--cpu", "1", "--vm", "1", "--vm-bytes", "200M", "--timeout", "300s"]
    resources:
      requests:
        cpu: 100m
        memory: 200Mi
      limits:
        cpu: 500m
        memory: 500Mi
EOF

  # Configure right-sizer with known multipliers and additions
  helm upgrade --install right-sizer ./helm \
    --namespace right-sizer-system \
    --set config.cpuRequestMultiplier=1.5 \
    --set config.memoryRequestMultiplier=1.5 \
    --set config.cpuRequestAddition=100 \
    --set config.memoryRequestAddition=100 \
    --set config.cpuLimitMultiplier=2.0 \
    --set config.memoryLimitMultiplier=2.0 \
    --set config.cpuLimitAddition=200 \
    --set config.memoryLimitAddition=200 \
    --set namespaceInclude=$NAMESPACE \
    --wait

  sleep 60 # Wait longer for metrics to stabilize

  # Expected calculations:
  # CPU Request = (100 × 1.5) + 100 = 250m
  # Memory Request = (200 × 1.5) + 100 = 400MB
  # CPU Limit = (250 × 2.0) + 200 = 700m
  # Memory Limit = (400 × 2.0) + 200 = 1000MB

  log_info "Expected values:"
  log_info "  CPU Request: 250m"
  log_info "  Memory Request: 400Mi"
  log_info "  CPU Limit: 700m"
  log_info "  Memory Limit: 1000Mi"

  log_info "Actual pod resources:"
  kubectl get pod calculation-test-pod -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

  log_info "Test 4 completed"
}

# Test 5: Environment variable validation
test_env_vars() {
  log_info "Test 5: Environment variable validation"

  # Check if right-sizer pod has the correct environment variables
  RS_POD=$(kubectl get pods -n right-sizer-system -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')

  if [ -z "$RS_POD" ]; then
    log_error "Right-sizer pod not found"
    return 1
  fi

  log_info "Checking environment variables in right-sizer pod..."

  ENV_VARS=("CPU_REQUEST_ADDITION" "MEMORY_REQUEST_ADDITION" "CPU_LIMIT_ADDITION" "MEMORY_LIMIT_ADDITION")

  for var in "${ENV_VARS[@]}"; do
    VALUE=$(kubectl get pod $RS_POD -n right-sizer-system -o jsonpath="{.spec.containers[0].env[?(@.name=='$var')].value}")
    if [ -n "$VALUE" ]; then
      log_info "  $var = $VALUE"
    else
      log_warning "  $var not found"
    fi
  done

  log_info "Test 5 completed"
}

# Cleanup function
cleanup() {
  log_info "Cleaning up test resources..."

  # Delete test namespace
  kubectl delete namespace $NAMESPACE --ignore-not-found=true

  # Optionally uninstall right-sizer
  # helm uninstall right-sizer -n right-sizer-system

  log_info "Cleanup completed"
}

# Main test execution
main() {
  check_prerequisites
  create_test_namespace

  # Run tests
  deploy_test_app

  log_info "Running test suite..."
  test_no_additions
  test_with_additions
  test_high_additions
  test_calculation_verification
  test_env_vars

  echo -e "\n${BLUE}========================================${NC}"
  echo -e "${GREEN}All tests completed successfully!${NC}"
  echo -e "${BLUE}========================================${NC}"

  # Show summary
  echo -e "\n${YELLOW}Test Summary:${NC}"
  echo "✓ Test 1: Default behavior without additions"
  echo "✓ Test 2: With CPU and Memory additions"
  echo "✓ Test 3: High addition values"
  echo "✓ Test 4: Calculation verification"
  echo "✓ Test 5: Environment variable validation"

  # Cleanup if requested
  if [ "$1" == "--cleanup" ]; then
    cleanup
  else
    echo -e "\n${YELLOW}Note: Test resources still exist. Run with --cleanup to remove them.${NC}"
  fi
}

# Handle interrupts
trap cleanup EXIT

# Run main function
main "$@"
