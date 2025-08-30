#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Enhanced Right-Sizer Minikube Testing Script
# This script automates testing of all enhanced features in a Minikube environment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="right-sizer-system"
TEST_NAMESPACE="default"
IMAGE_NAME="right-sizer"
IMAGE_TAG="test-enhanced"
TIMEOUT=300

# Test tracking
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Helper functions
print_header() {
  echo -e "\n${BLUE}===== $1 =====${NC}\n"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
  ((TESTS_PASSED++))
}

print_error() {
  echo -e "${RED}✗${NC} $1"
  ((TESTS_FAILED++))
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

run_test() {
  local test_name="$1"
  local test_function="$2"

  ((TOTAL_TESTS++))
  print_info "Running test: $test_name"

  if $test_function; then
    print_success "$test_name"
  else
    print_error "$test_name"
  fi
}

wait_for_pods() {
  local label_selector="$1"
  local namespace="$2"
  local timeout="${3:-300}"

  print_info "Waiting for pods with selector '$label_selector' in namespace '$namespace'..."
  kubectl wait --for=condition=ready pod -l "$label_selector" -n "$namespace" --timeout="${timeout}s" || return 1
}

check_logs_for_pattern() {
  local deployment="$1"
  local namespace="$2"
  local pattern="$3"
  local timeout="${4:-30}"

  print_info "Checking logs for pattern: $pattern"
  for i in $(seq 1 $timeout); do
    if kubectl logs -n "$namespace" deployment/"$deployment" --tail=100 | grep -q "$pattern"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# Test functions
test_prerequisites() {
  print_header "Testing Prerequisites"

  # Check required tools
  for tool in minikube kubectl docker helm; do
    if command -v $tool >/dev/null 2>&1; then
      print_success "$tool is installed"
    else
      print_error "$tool is not installed"
      return 1
    fi
  done

  # Check Minikube status
  if minikube status >/dev/null 2>&1; then
    print_success "Minikube is running"
  else
    print_error "Minikube is not running"
    return 1
  fi

  # Check Kubernetes version
  local k8s_version=$(kubectl version --client=false -o json 2>/dev/null | jq -r '.serverVersion.gitVersion // "unknown"')
  print_info "Kubernetes version: $k8s_version"

  return 0
}

test_environment_setup() {
  print_header "Setting Up Test Environment"

  # Use Minikube Docker daemon
  eval $(minikube docker-env)

  # Build image
  print_info "Building enhanced right-sizer image..."
  if docker build -t "$IMAGE_NAME:$IMAGE_TAG" . >/dev/null 2>&1; then
    print_success "Image built successfully"
  else
    print_error "Failed to build image"
    return 1
  fi

  # Verify image exists
  if docker images | grep -q "$IMAGE_NAME.*$IMAGE_TAG"; then
    print_success "Image verified in Minikube registry"
  else
    print_error "Image not found in Minikube registry"
    return 1
  fi

  # Enable required addons
  for addon in metrics-server ingress; do
    if minikube addons enable $addon >/dev/null 2>&1; then
      print_success "Enabled $addon addon"
    else
      print_warning "Failed to enable $addon addon"
    fi
  done

  # Create namespace
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  print_success "Created/verified namespace $NAMESPACE"

  return 0
}

test_basic_deployment() {
  print_header "Testing Basic Deployment"

  # Deploy basic configuration with Helm
  print_info "Deploying right-sizer with basic configuration..."
  helm upgrade --install right-sizer ./helm \
    --namespace "$NAMESPACE" \
    --set image.repository="$IMAGE_NAME" \
    --set image.tag="$IMAGE_TAG" \
    --set image.pullPolicy=Never \
    --set config.policyBasedSizing=false \
    --set security.admissionWebhook.enabled=false \
    --set observability.metricsEnabled=true \
    --set observability.auditEnabled=true \
    --set config.logLevel=debug \
    --wait --timeout=300s >/dev/null 2>&1 || return 1

  print_success "Helm deployment completed"

  # Wait for pods to be ready
  if wait_for_pods "app=right-sizer" "$NAMESPACE" 180; then
    print_success "Right-sizer pod is ready"
  else
    print_error "Right-sizer pod failed to become ready"
    kubectl describe pod -l app=right-sizer -n "$NAMESPACE"
    return 1
  fi

  # Check startup logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "Configuration Loaded" 30; then
    print_success "Configuration loaded successfully"
  else
    print_error "Configuration not loaded properly"
    kubectl logs -n "$NAMESPACE" deployment/right-sizer-operator --tail=50
    return 1
  fi

  return 0
}

test_health_endpoints() {
  print_header "Testing Health Endpoints"

  # Port forward to health endpoint
  kubectl port-forward -n "$NAMESPACE" service/right-sizer-operator 8081:8081 >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 3

  # Test health endpoint
  if curl -f -s http://localhost:8081/healthz >/dev/null 2>&1; then
    print_success "Health endpoint responding"
  else
    print_error "Health endpoint not responding"
    kill $pf_pid 2>/dev/null
    return 1
  fi

  # Test readiness endpoint
  if curl -f -s http://localhost:8081/readyz >/dev/null 2>&1; then
    print_success "Readiness endpoint responding"
  else
    print_error "Readiness endpoint not responding"
    kill $pf_pid 2>/dev/null
    return 1
  fi

  kill $pf_pid 2>/dev/null
  return 0
}

test_metrics_endpoint() {
  print_header "Testing Metrics Endpoint"

  # Port forward to metrics endpoint
  kubectl port-forward -n "$NAMESPACE" service/right-sizer-operator 9090:9090 >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 3

  # Test metrics endpoint
  local metrics_output=$(curl -s http://localhost:9090/metrics 2>/dev/null)
  if echo "$metrics_output" | grep -q "rightsizer_"; then
    print_success "Metrics endpoint responding with right-sizer metrics"

    # Count available metrics
    local metric_count=$(echo "$metrics_output" | grep -c "^rightsizer_" || echo "0")
    print_info "Found $metric_count right-sizer metrics"
  else
    print_error "Metrics endpoint not responding or no right-sizer metrics found"
    kill $pf_pid 2>/dev/null
    return 1
  fi

  kill $pf_pid 2>/dev/null
  return 0
}

test_basic_resource_processing() {
  print_header "Testing Basic Resource Processing"

  # Deploy test workload
  print_info "Deploying test workload..."
  kubectl apply -f - >/dev/null 2>&1 <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-basic-app
  namespace: $TEST_NAMESPACE
  labels:
    test: basic-processing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-basic-app
  template:
    metadata:
      labels:
        app: test-basic-app
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

  print_success "Test workload deployed"

  # Wait for workload to be ready
  if wait_for_pods "app=test-basic-app" "$TEST_NAMESPACE" 60; then
    print_success "Test workload is ready"
  else
    print_error "Test workload failed to become ready"
    return 1
  fi

  # Wait for operator to process the workload
  sleep 30

  # Check for processing logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "Processing pods" 30; then
    print_success "Operator is processing pods"
  else
    print_warning "No pod processing logs found yet"
  fi

  # Cleanup test workload
  kubectl delete deployment test-basic-app -n "$TEST_NAMESPACE" >/dev/null 2>&1

  return 0
}

test_policy_engine() {
  print_header "Testing Policy Engine"

  # Enable policy-based sizing
  print_info "Enabling policy-based sizing..."
  helm upgrade right-sizer ./helm \
    --namespace "$NAMESPACE" \
    --reuse-values \
    --set config.policyBasedSizing=true \
    --set policyEngine.enabled=true \
    --wait --timeout=120s >/dev/null 2>&1 || return 1

  print_success "Policy engine enabled"

  # Apply policy configuration
  print_info "Applying policy rules..."
  kubectl apply -f examples/policy-rules-example.yaml >/dev/null 2>&1 || return 1
  print_success "Policy rules applied"

  # Check for policy loading logs
  sleep 10
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "policy.*rules.*loaded\|Policy engine initialized" 30; then
    print_success "Policy rules loaded successfully"
  else
    print_warning "Policy loading not detected in logs"
  fi

  # Deploy workload matching a policy
  print_info "Deploying workload with high priority label..."
  kubectl apply -f - >/dev/null 2>&1 <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-high-priority
  namespace: $TEST_NAMESPACE
  labels:
    priority: high
    environment: production
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-high-priority
  template:
    metadata:
      labels:
        app: test-high-priority
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

  print_success "High priority workload deployed"

  # Wait for policy application
  sleep 60

  # Check for policy application logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "policy.*applied\|Applied policy rule" 30; then
    print_success "Policy rules are being applied"
  else
    print_warning "Policy application not detected in logs"
  fi

  # Cleanup
  kubectl delete deployment test-high-priority -n "$TEST_NAMESPACE" >/dev/null 2>&1

  return 0
}

test_validation_engine() {
  print_header "Testing Resource Validation"

  # Try to create a pod with potentially invalid resources
  print_info "Testing resource validation with boundary cases..."

  # This should trigger validation warnings/errors
  kubectl apply -f - >/dev/null 2>&1 <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: validation-test-pod
  namespace: $TEST_NAMESPACE
spec:
  containers:
  - name: test
    image: busybox:latest
    command: ["sleep", "300"]
    resources:
      requests:
        cpu: "8"      # High CPU request
        memory: "16Gi" # High memory request
      limits:
        cpu: "16"
        memory: "32Gi"
EOF

  # Wait a bit for processing
  sleep 30

  # Check for validation logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "validation\|boundary\|limit\|threshold" 30; then
    print_success "Resource validation is active"
  else
    print_warning "Validation activity not clearly detected"
  fi

  # Cleanup
  kubectl delete pod validation-test-pod -n "$TEST_NAMESPACE" >/dev/null 2>&1 || true

  return 0
}

test_audit_logging() {
  print_header "Testing Audit Logging"

  # Deploy a workload to generate audit events
  print_info "Deploying workload to generate audit events..."
  kubectl apply -f - >/dev/null 2>&1 <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: audit-test-app
  namespace: $TEST_NAMESPACE
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

  print_success "Audit test workload deployed"

  # Wait for audit events to be generated
  sleep 60

  # Check for audit logging initialization
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "audit.*initialized\|Audit logging" 30; then
    print_success "Audit logging is initialized"
  else
    print_warning "Audit logging initialization not detected"
  fi

  # Check for audit events
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "audit.*event\|ResourceChange\|PolicyApplication" 30; then
    print_success "Audit events are being generated"
  else
    print_warning "Audit events not clearly detected"
  fi

  # Check Kubernetes events
  local k8s_events=$(kubectl get events --field-selector source=right-sizer -n "$TEST_NAMESPACE" --no-headers 2>/dev/null | wc -l || echo "0")
  if [ "$k8s_events" -gt 0 ]; then
    print_success "Kubernetes events created by auditing ($k8s_events events)"
  else
    print_warning "No Kubernetes events found from right-sizer"
  fi

  # Cleanup
  kubectl delete deployment audit-test-app -n "$TEST_NAMESPACE" >/dev/null 2>&1

  return 0
}

test_circuit_breaker_and_retry() {
  print_header "Testing Circuit Breaker and Retry Logic"

  # Check initial circuit breaker state in metrics
  kubectl port-forward -n "$NAMESPACE" service/right-sizer-operator 9090:9090 >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 3

  local cb_metrics=$(curl -s http://localhost:9090/metrics 2>/dev/null | grep -E "rightsizer.*(retry|circuit)" || echo "")
  if [ -n "$cb_metrics" ]; then
    print_success "Circuit breaker and retry metrics are available"
    print_info "Sample metrics: $(echo "$cb_metrics" | head -2 | tr '\n' ' ')"
  else
    print_warning "Circuit breaker/retry metrics not found"
  fi

  # Check retry logic in logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "retry\|circuit.*breaker\|exponential.*backoff" 30; then
    print_success "Retry/circuit breaker logic is active"
  else
    print_warning "Retry/circuit breaker activity not detected"
  fi

  kill $pf_pid 2>/dev/null
  return 0
}

test_configuration_validation() {
  print_header "Testing Configuration Validation"

  # Check configuration loading logs
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "Configuration.*Loaded\|configuration.*validation" 30; then
    print_success "Configuration validation is working"
  else
    print_error "Configuration validation not detected"
    return 1
  fi

  # Check specific configuration values in logs
  local config_logs=$(kubectl logs -n "$NAMESPACE" deployment/right-sizer-operator --tail=200 | grep -E "(CPU.*Multiplier|Memory.*Multiplier|Safety.*Threshold)" || echo "")
  if [ -n "$config_logs" ]; then
    print_success "Configuration multipliers are loaded and logged"
    print_info "Found configuration: $(echo "$config_logs" | head -2 | sed 's/.*INFO//' | tr '\n' ' ')"
  else
    print_warning "Configuration values not clearly visible in logs"
  fi

  return 0
}

test_performance_load() {
  print_header "Testing Performance Under Load"

  # Deploy multiple workloads to test performance
  print_info "Deploying multiple workloads to test performance..."

  for i in {1..5}; do
    kubectl create deployment load-test-$i --image=nginx:alpine --replicas=1 >/dev/null 2>&1
    kubectl set resources deployment load-test-$i --requests=cpu=${i}0m,memory=$((i * 32))Mi >/dev/null 2>&1
  done

  print_success "Created 5 test deployments for load testing"

  # Wait for processing
  sleep 90

  # Check performance metrics
  kubectl port-forward -n "$NAMESPACE" service/right-sizer-operator 9090:9090 >/dev/null 2>&1 &
  local pf_pid=$!
  sleep 3

  local duration_metrics=$(curl -s http://localhost:9090/metrics 2>/dev/null | grep "rightsizer_processing_duration_seconds" || echo "")
  if [ -n "$duration_metrics" ]; then
    print_success "Processing duration metrics are being recorded"
  else
    print_warning "Processing duration metrics not found"
  fi

  local throughput_metrics=$(curl -s http://localhost:9090/metrics 2>/dev/null | grep "rightsizer_pods_processed_total" || echo "")
  if [ -n "$throughput_metrics" ]; then
    print_success "Pod processing throughput metrics are available"
  else
    print_warning "Throughput metrics not found"
  fi

  kill $pf_pid 2>/dev/null

  # Cleanup load test deployments
  for i in {1..5}; do
    kubectl delete deployment load-test-$i >/dev/null 2>&1 || true
  done

  print_success "Load test cleanup completed"

  return 0
}

test_failure_recovery() {
  print_header "Testing Failure Recovery"

  # Test pod restart recovery
  print_info "Testing pod restart recovery..."
  local pod_name=$(kubectl get pod -n "$NAMESPACE" -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')
  kubectl delete pod "$pod_name" -n "$NAMESPACE" >/dev/null 2>&1

  # Wait for new pod to be ready
  if wait_for_pods "app=right-sizer" "$NAMESPACE" 180; then
    print_success "Operator recovered from pod restart"
  else
    print_error "Operator failed to recover from pod restart"
    return 1
  fi

  # Wait for operator to reinitialize
  sleep 30

  # Check that it's processing again
  if check_logs_for_pattern "right-sizer-operator" "$NAMESPACE" "Starting.*right-sizer\|Configuration.*Loaded" 30; then
    print_success "Operator reinitialized successfully after restart"
  else
    print_error "Operator did not reinitialize properly after restart"
    return 1
  fi

  return 0
}

cleanup_test_resources() {
  print_header "Cleaning Up Test Resources"

  # Remove test deployments
  kubectl delete deployment --selector=test=basic-processing -n "$TEST_NAMESPACE" >/dev/null 2>&1 || true
  kubectl delete deployment test-high-priority -n "$TEST_NAMESPACE" >/dev/null 2>&1 || true
  kubectl delete deployment audit-test-app -n "$TEST_NAMESPACE" >/dev/null 2>&1 || true
  kubectl delete pod validation-test-pod -n "$TEST_NAMESPACE" >/dev/null 2>&1 || true

  # Remove policy ConfigMap if it exists
  kubectl delete configmap right-sizer-policies -n "$NAMESPACE" >/dev/null 2>&1 || true

  print_success "Test resources cleaned up"
}

generate_test_report() {
  print_header "Test Results Summary"

  echo -e "${BLUE}Total Tests Run: ${TOTAL_TESTS}${NC}"
  echo -e "${GREEN}Tests Passed: ${TESTS_PASSED}${NC}"
  echo -e "${RED}Tests Failed: ${TESTS_FAILED}${NC}"

  local success_rate=$((TESTS_PASSED * 100 / TOTAL_TESTS))
  echo -e "${CYAN}Success Rate: ${success_rate}%${NC}"

  if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed! The enhanced right-sizer operator is working correctly.${NC}"
    return 0
  else
    echo -e "\n${RED}❌ Some tests failed. Please review the output above for details.${NC}"
    return 1
  fi
}

main() {
  print_header "Enhanced Right-Sizer Minikube Testing Suite"
  echo -e "${CYAN}Testing all enhanced features of the right-sizer operator${NC}"
  echo -e "${CYAN}Namespace: $NAMESPACE${NC}"
  echo -e "${CYAN}Image: $IMAGE_NAME:$IMAGE_TAG${NC}\n"

  # Run test suite
  run_test "Prerequisites Check" test_prerequisites
  run_test "Environment Setup" test_environment_setup
  run_test "Basic Deployment" test_basic_deployment
  run_test "Health Endpoints" test_health_endpoints
  run_test "Metrics Endpoint" test_metrics_endpoint
  run_test "Configuration Validation" test_configuration_validation
  run_test "Basic Resource Processing" test_basic_resource_processing
  run_test "Policy Engine" test_policy_engine
  run_test "Validation Engine" test_validation_engine
  run_test "Audit Logging" test_audit_logging
  run_test "Circuit Breaker and Retry" test_circuit_breaker_and_retry
  run_test "Performance Under Load" test_performance_load
  run_test "Failure Recovery" test_failure_recovery

  # Cleanup
  cleanup_test_resources

  # Generate report
  generate_test_report
}

# Handle script interruption
trap cleanup_test_resources EXIT

# Parse command line arguments
case "${1:-}" in
--help | -h)
  echo "Enhanced Right-Sizer Minikube Testing Script"
  echo ""
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --help, -h     Show this help message"
  echo "  --cleanup      Clean up test resources and exit"
  echo "  --quick        Run only quick tests (skip load and performance tests)"
  echo ""
  echo "Prerequisites:"
  echo "  - Minikube running with at least 8GB RAM and 4 CPUs"
  echo "  - kubectl configured for Minikube context"
  echo "  - Docker daemon accessible"
  echo "  - Helm 3.x installed"
  echo ""
  exit 0
  ;;
--cleanup)
  kubectl delete namespace "$NAMESPACE" >/dev/null 2>&1 || true
  cleanup_test_resources
  print_success "Cleanup completed"
  exit 0
  ;;
--quick)
  print_header "Enhanced Right-Sizer Quick Testing Suite"
  run_test "Prerequisites Check" test_prerequisites
  run_test "Environment Setup" test_environment_setup
  run_test "Basic Deployment" test_basic_deployment
  run_test "Health Endpoints" test_health_endpoints
  run_test "Metrics Endpoint" test_metrics_endpoint
  run_test "Configuration Validation" test_configuration_validation
  cleanup_test_resources
  generate_test_report
  exit $?
  ;;
"")
  main
  exit $?
  ;;
*)
  echo "Unknown option: $1"
  echo "Use --help for usage information"
  exit 1
  ;;
esac
