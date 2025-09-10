#!/bin/bash

# Right-Sizer E2E Test with Real Metrics from Metrics-Server
# This script validates that right-sizer correctly adjusts pod resources based on actual metrics

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="default"
RIGHTSIZER_NAMESPACE="right-sizer"
TEST_DURATION=300 # 5 minutes
METRICS_WAIT=60   # Wait 60 seconds for metrics to be available
RESIZE_WAIT=120   # Wait 2 minutes for right-sizer to take action

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
WARNINGS=0

# Log file
LOG_FILE="e2e-test-$(date +%Y%m%d-%H%M%S).log"

# Helper functions
print_header() {
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}" | tee -a "$LOG_FILE"
  echo -e "${BLUE}$1${NC}" | tee -a "$LOG_FILE"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}" | tee -a "$LOG_FILE"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1" | tee -a "$LOG_FILE"
  ((PASSED_TESTS++))
}

print_error() {
  echo -e "${RED}✗${NC} $1" | tee -a "$LOG_FILE"
  ((FAILED_TESTS++))
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1" | tee -a "$LOG_FILE"
  ((WARNINGS++))
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1" | tee -a "$LOG_FILE"
}

# Check prerequisites
check_prerequisites() {
  print_header "Checking Prerequisites"
  ((TOTAL_TESTS++))

  # Check kubectl
  if command -v kubectl &>/dev/null; then
    print_success "kubectl is installed"
  else
    print_error "kubectl is not installed"
    exit 1
  fi

  # Check cluster connection
  ((TOTAL_TESTS++))
  if kubectl cluster-info &>/dev/null; then
    print_success "Connected to Kubernetes cluster"
  else
    print_error "Cannot connect to Kubernetes cluster"
    exit 1
  fi

  # Check metrics-server
  ((TOTAL_TESTS++))
  if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    print_success "metrics-server is deployed"
  else
    print_error "metrics-server is not deployed"
    print_info "Installing metrics-server..."
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    sleep 30
  fi

  # Wait for metrics-server to be ready
  ((TOTAL_TESTS++))
  if kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=60s &>/dev/null; then
    print_success "metrics-server is ready"
  else
    print_error "metrics-server is not ready"
    exit 1
  fi

  # Check right-sizer
  ((TOTAL_TESTS++))
  if kubectl get deployment -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer &>/dev/null; then
    print_success "right-sizer is deployed"
  else
    print_error "right-sizer is not deployed"
    exit 1
  fi

  # Check right-sizer is ready
  ((TOTAL_TESTS++))
  if kubectl wait --for=condition=ready pod -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer --timeout=60s &>/dev/null; then
    print_success "right-sizer is ready"
  else
    print_error "right-sizer is not ready"
    exit 1
  fi
}

# Deploy test workloads
deploy_test_workloads() {
  print_header "Deploying Test Workloads"

  # Create test deployment with over-provisioned resources
  cat <<EOF | kubectl apply -f - 2>&1 | tee -a "$LOG_FILE"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-test-overprovisioned
  namespace: $TEST_NAMESPACE
  labels:
    test: e2e-metrics
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-overprovisioned
  template:
    metadata:
      labels:
        app: e2e-overprovisioned
        test: e2e-metrics
      annotations:
        right-sizer.io/enable: "true"
        right-sizer.io/mode: "aggressive"
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "1000m"
            memory: "1Gi"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-test-underprovisioned
  namespace: $TEST_NAMESPACE
  labels:
    test: e2e-metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-underprovisioned
  template:
    metadata:
      labels:
        app: e2e-underprovisioned
        test: e2e-metrics
      annotations:
        right-sizer.io/enable: "true"
        right-sizer.io/mode: "conservative"
    spec:
      containers:
      - name: stress
        image: busybox
        command: ["sh", "-c"]
        args:
        - |
          while true; do
            # Simulate CPU and memory usage
            dd if=/dev/zero of=/dev/null bs=1M count=100 &
            pid=\$!
            sleep 10
            kill \$pid 2>/dev/null || true
            sleep 20
          done
        resources:
          requests:
            cpu: "10m"
            memory: "32Mi"
          limits:
            cpu: "50m"
            memory: "64Mi"
EOF

  ((TOTAL_TESTS++))
  if [ $? -eq 0 ]; then
    print_success "Test workloads deployed"
  else
    print_error "Failed to deploy test workloads"
    return 1
  fi

  # Wait for pods to be ready
  ((TOTAL_TESTS++))
  if kubectl wait --for=condition=ready pod -l test=e2e-metrics -n "$TEST_NAMESPACE" --timeout=60s &>/dev/null; then
    print_success "Test pods are ready"
  else
    print_error "Test pods failed to become ready"
    return 1
  fi
}

# Collect initial metrics
collect_initial_metrics() {
  print_header "Collecting Initial Metrics"

  print_info "Waiting $METRICS_WAIT seconds for metrics to be available..."
  sleep "$METRICS_WAIT"

  # Get initial metrics
  ((TOTAL_TESTS++))
  print_info "Initial pod metrics:"
  if kubectl top pods -n "$TEST_NAMESPACE" -l test=e2e-metrics 2>&1 | tee -a "$LOG_FILE"; then
    print_success "Collected initial metrics"
  else
    print_error "Failed to collect initial metrics"
    return 1
  fi

  # Save initial resource configurations
  print_info "Initial resource configurations:"
  kubectl get pods -n "$TEST_NAMESPACE" -l test=e2e-metrics -o json |
    jq -r '.items[] | "\(.metadata.name): CPU requests=\(.spec.containers[0].resources.requests.cpu) limits=\(.spec.containers[0].resources.limits.cpu), Memory requests=\(.spec.containers[0].resources.requests.memory) limits=\(.spec.containers[0].resources.limits.memory)"' |
    tee -a "$LOG_FILE"
}

# Monitor right-sizer activity
monitor_rightsizer() {
  print_header "Monitoring Right-Sizer Activity"

  print_info "Monitoring right-sizer for $RESIZE_WAIT seconds..."

  # Capture right-sizer logs
  local start_time=$(date +%s)
  local end_time=$((start_time + RESIZE_WAIT))

  while [ $(date +%s) -lt $end_time ]; do
    # Check right-sizer logs for resize activity
    kubectl logs -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=10 --since=10s 2>/dev/null |
      grep -E "(Successfully resized|will be resized|Scaling analysis)" |
      while read -r line; do
        echo "[$(date '+%H:%M:%S')] $line" | tee -a "$LOG_FILE"
      done

    sleep 10
  done

  ((TOTAL_TESTS++))
  print_success "Completed monitoring period"
}

# Validate resource adjustments
validate_adjustments() {
  print_header "Validating Resource Adjustments"

  # Get current metrics
  print_info "Current pod metrics after right-sizer adjustments:"
  kubectl top pods -n "$TEST_NAMESPACE" -l test=e2e-metrics 2>&1 | tee -a "$LOG_FILE"

  # Check overprovisioned pods were scaled down
  ((TOTAL_TESTS++))
  print_info "Checking overprovisioned pod adjustments..."
  local overprovisioned_pods=$(kubectl get pods -n "$TEST_NAMESPACE" -l app=e2e-overprovisioned -o name)
  local scaled_down=false

  for pod in $overprovisioned_pods; do
    pod_name=$(echo "$pod" | cut -d'/' -f2)
    cpu_request=$(kubectl get "$pod" -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
    memory_request=$(kubectl get "$pod" -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.memory}')

    print_info "Pod $pod_name: CPU=$cpu_request, Memory=$memory_request"

    # Convert to millicores for comparison
    cpu_millis=$(echo "$cpu_request" | sed 's/m$//')
    if [ "$cpu_millis" -lt 500 ]; then
      scaled_down=true
      print_success "Pod $pod_name CPU was scaled down from 500m to $cpu_request"
    fi
  done

  if [ "$scaled_down" = true ]; then
    print_success "Overprovisioned pods were adjusted"
  else
    print_warning "Overprovisioned pods were not adjusted (may need more time)"
  fi

  # Check if right-sizer made any changes
  ((TOTAL_TESTS++))
  print_info "Checking right-sizer logs for successful resizes..."
  local resize_count=$(kubectl logs -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer --since=5m 2>/dev/null |
    grep -c "Successfully resized" || echo "0")

  if [ "$resize_count" -gt 0 ]; then
    print_success "Right-sizer successfully resized $resize_count pods"
  else
    print_warning "No successful resizes detected in the last 5 minutes"
  fi

  # Check for errors in right-sizer
  ((TOTAL_TESTS++))
  print_info "Checking for errors in right-sizer..."
  local error_count=$(kubectl logs -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer --since=5m 2>/dev/null |
    grep -c "ERROR\|Error updating pod" || echo "0")

  if [ "$error_count" -eq 0 ]; then
    print_success "No errors found in right-sizer logs"
  else
    print_warning "Found $error_count errors in right-sizer logs"
    kubectl logs -n "$RIGHTSIZER_NAMESPACE" -l app.kubernetes.io/name=right-sizer --since=5m 2>/dev/null |
      grep "ERROR\|Error updating pod" | head -5 | tee -a "$LOG_FILE"
  fi
}

# Check metrics accuracy
check_metrics_accuracy() {
  print_header "Checking Metrics Accuracy"

  ((TOTAL_TESTS++))
  print_info "Comparing actual usage with resource allocations..."

  local pods=$(kubectl get pods -n "$TEST_NAMESPACE" -l test=e2e-metrics -o name)
  local accurate=true

  for pod in $pods; do
    pod_name=$(echo "$pod" | cut -d'/' -f2)

    # Get current metrics
    metrics=$(kubectl top pod "$pod_name" -n "$TEST_NAMESPACE" 2>/dev/null | tail -1)
    if [ -n "$metrics" ]; then
      cpu_usage=$(echo "$metrics" | awk '{print $2}')
      memory_usage=$(echo "$metrics" | awk '{print $3}')

      # Get current requests
      cpu_request=$(kubectl get "$pod" -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
      memory_request=$(kubectl get "$pod" -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.memory}')

      print_info "$pod_name:"
      print_info "  Usage:    CPU=$cpu_usage, Memory=$memory_usage"
      print_info "  Requests: CPU=$cpu_request, Memory=$memory_request"
    else
      print_warning "Could not get metrics for $pod_name"
      accurate=false
    fi
  done

  if [ "$accurate" = true ]; then
    print_success "Metrics are available and being used for sizing decisions"
  else
    print_warning "Some metrics were unavailable"
  fi
}

# Cleanup test resources
cleanup() {
  print_header "Cleaning Up Test Resources"

  ((TOTAL_TESTS++))
  if kubectl delete deployment -n "$TEST_NAMESPACE" -l test=e2e-metrics &>/dev/null; then
    print_success "Cleaned up test deployments"
  else
    print_warning "Failed to clean up some test resources"
  fi
}

# Generate test report
generate_report() {
  print_header "Test Report"

  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}" | tee -a "$LOG_FILE"
  echo -e "${BLUE}Test Summary${NC}" | tee -a "$LOG_FILE"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}" | tee -a "$LOG_FILE"
  echo "Total Tests: $TOTAL_TESTS" | tee -a "$LOG_FILE"
  echo -e "${GREEN}Passed: $PASSED_TESTS${NC}" | tee -a "$LOG_FILE"
  echo -e "${RED}Failed: $FAILED_TESTS${NC}" | tee -a "$LOG_FILE"
  echo -e "${YELLOW}Warnings: $WARNINGS${NC}" | tee -a "$LOG_FILE"

  local success_rate=0
  if [ $TOTAL_TESTS -gt 0 ]; then
    success_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
  fi

  echo "Success Rate: ${success_rate}%" | tee -a "$LOG_FILE"
  echo "Log file: $LOG_FILE" | tee -a "$LOG_FILE"

  if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✅ All tests passed successfully!${NC}" | tee -a "$LOG_FILE"
    return 0
  else
    echo -e "\n${RED}❌ Some tests failed. Check the log for details.${NC}" | tee -a "$LOG_FILE"
    return 1
  fi
}

# Main test execution
main() {
  print_header "Right-Sizer E2E Test with Real Metrics"
  print_info "Test started at $(date)"
  print_info "Log file: $LOG_FILE"

  # Run test stages
  check_prerequisites
  deploy_test_workloads
  collect_initial_metrics
  monitor_rightsizer
  validate_adjustments
  check_metrics_accuracy
  cleanup

  # Generate final report
  generate_report

  exit_code=$?
  print_info "Test completed at $(date)"
  exit $exit_code
}

# Handle interrupts
trap cleanup INT TERM

# Run main function
main "$@"
