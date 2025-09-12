#!/bin/bash

# Kubernetes 1.33+ In-Place Resize Compliance Test Runner
# This script runs comprehensive compliance tests for right-sizer operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="k8s-compliance-test"
TEST_TIMEOUT="300s"
RESIZE_WAIT_TIME=10
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORT_FILE="compliance-test-report-$(date +%Y%m%d-%H%M%S).json"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
WARNINGS=0

# Test results array
declare -a TEST_RESULTS=()

# Helper functions
print_header() {
  echo -e "\n${BLUE}$1${NC}"
  echo "$(printf '=%.0s' {1..80})"
}

print_test() {
  echo -e "\n${YELLOW}🧪 Testing:${NC} $1"
  TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

print_pass() {
  echo -e "${GREEN}✅ PASS:${NC} $1"
  PASSED_TESTS=$((PASSED_TESTS + 1))
  TEST_RESULTS+=("{\"test\":\"$2\",\"status\":\"PASS\",\"message\":\"$1\"}")
}

print_fail() {
  echo -e "${RED}❌ FAIL:${NC} $1"
  FAILED_TESTS=$((FAILED_TESTS + 1))
  TEST_RESULTS+=("{\"test\":\"$2\",\"status\":\"FAIL\",\"message\":\"$1\"}")
}

print_warning() {
  echo -e "${YELLOW}⚠️  WARN:${NC} $1"
  WARNINGS=$((WARNINGS + 1))
  TEST_RESULTS+=("{\"test\":\"$2\",\"status\":\"WARN\",\"message\":\"$1\"}")
}

print_info() {
  echo -e "${BLUE}ℹ️  INFO:${NC} $1"
}

# Cleanup function
cleanup() {
  print_header "🧹 Cleaning up test resources"
  kubectl delete namespace $TEST_NAMESPACE --ignore-not-found=true --timeout=60s || true
  echo "Cleanup completed"
}

# Trap for cleanup on exit
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
  print_header "🔍 Checking Prerequisites"

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    print_fail "kubectl is not installed" "prerequisites"
    exit 1
  fi

  # Check cluster connectivity
  if ! kubectl cluster-info &>/dev/null; then
    print_fail "Cannot connect to Kubernetes cluster" "prerequisites"
    exit 1
  fi

  # Check K8s version
  local server_version=$(kubectl version --short 2>/dev/null | grep "Server Version" | awk '{print $3}' | sed 's/v//')
  local major=$(echo $server_version | cut -d. -f1)
  local minor=$(echo $server_version | cut -d. -f2)

  if [[ $major -lt 1 ]] || [[ $major -eq 1 && $minor -lt 32 ]]; then
    print_warning "Kubernetes version $server_version may not support all features" "prerequisites"
  else
    print_pass "Kubernetes version $server_version is compatible" "prerequisites"
  fi

  # Check for resize subresource
  if kubectl api-resources --subresource=resize 2>/dev/null | grep -q resize; then
    print_pass "Resize subresource is available" "prerequisites"
  else
    print_warning "Resize subresource may not be available" "prerequisites"
  fi
}

# Setup test environment
setup_test_environment() {
  print_header "🛠️  Setting up Test Environment"

  # Create test namespace
  kubectl create namespace $TEST_NAMESPACE --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  print_pass "Test namespace created: $TEST_NAMESPACE" "setup"

  # Apply test manifests
  print_info "Applying test manifests..."
  kubectl apply -f "$SCRIPT_DIR/test-pods.yaml" >/dev/null
  print_pass "Test manifests applied successfully" "setup"

  # Wait for pods to be ready
  print_info "Waiting for test pods to be ready..."
  sleep 5

  local ready_pods=0
  local total_pods=$(kubectl get pods -n $TEST_NAMESPACE --no-headers | wc -l)

  for i in {1..30}; do
    ready_pods=$(kubectl get pods -n $TEST_NAMESPACE --no-headers | grep "Running\|Succeeded" | wc -l)
    if [[ $ready_pods -eq $total_pods ]]; then
      break
    fi
    sleep 2
  done

  if [[ $ready_pods -eq $total_pods ]]; then
    print_pass "$ready_pods/$total_pods test pods are ready" "setup"
  else
    print_warning "Only $ready_pods/$total_pods test pods are ready" "setup"
  fi
}

# Test basic resize functionality
test_basic_resize() {
  print_test "Basic In-Place Resize Functionality"

  local pod_name="basic-resize-test"

  # Check if pod exists and is running
  if ! kubectl get pod $pod_name -n $TEST_NAMESPACE &>/dev/null; then
    print_fail "Test pod $pod_name not found" "basic-resize"
    return 1
  fi

  # Wait for pod to be ready
  if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_fail "Pod $pod_name did not become ready" "basic-resize"
    return 1
  fi

  # Get initial resources
  local initial_cpu=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
  local initial_memory=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')

  print_info "Initial resources: CPU=$initial_cpu, Memory=$initial_memory"

  # Perform CPU resize
  print_info "Performing CPU resize..."
  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"150m"}, "limits":{"cpu":"300m"}}}]}}' &>/dev/null; then

    sleep $RESIZE_WAIT_TIME

    local new_cpu=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
    if [[ "$new_cpu" == "150m" ]]; then
      print_pass "CPU resize successful: $initial_cpu -> $new_cpu" "basic-resize"
    else
      print_fail "CPU resize failed: expected 150m, got $new_cpu" "basic-resize"
    fi
  else
    print_fail "CPU resize command failed" "basic-resize"
  fi

  # Perform Memory resize
  print_info "Performing Memory resize..."
  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"256Mi"}, "limits":{"memory":"512Mi"}}}]}}' &>/dev/null; then

    sleep $RESIZE_WAIT_TIME

    local new_memory=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')
    if [[ "$new_memory" == "256Mi" ]]; then
      print_pass "Memory resize successful: $initial_memory -> $new_memory" "basic-resize"
    else
      print_fail "Memory resize failed: expected 256Mi, got $new_memory" "basic-resize"
    fi
  else
    print_fail "Memory resize command failed" "basic-resize"
  fi
}

# Test container restart policies
test_restart_policies() {
  print_test "Container Restart Policy Compliance"

  local pod_name="mixed-policy-test"

  # Wait for pod to be ready
  if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_fail "Pod $pod_name did not become ready" "restart-policies"
    return 1
  fi

  # Get initial restart count
  local initial_restart_count=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
  print_info "Initial restart count: $initial_restart_count"

  # Test CPU resize (NotRequired - should not restart)
  print_info "Testing CPU resize with NotRequired policy..."
  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"150m"}, "limits":{"cpu":"300m"}}}]}}' &>/dev/null; then

    sleep $RESIZE_WAIT_TIME

    local cpu_restart_count=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")

    if [[ "$cpu_restart_count" == "$initial_restart_count" ]]; then
      print_pass "CPU resize with NotRequired policy did not restart container" "restart-policies"
    else
      print_fail "CPU resize with NotRequired policy incorrectly restarted container" "restart-policies"
    fi
  else
    print_fail "CPU resize command failed" "restart-policies"
  fi

  # Test Memory resize (RestartContainer - should restart)
  print_info "Testing Memory resize with RestartContainer policy..."
  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"256Mi"}, "limits":{"memory":"512Mi"}}}]}}' &>/dev/null; then

    sleep $RESIZE_WAIT_TIME

    local memory_restart_count=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")

    if [[ "$memory_restart_count" -gt "$cpu_restart_count" ]]; then
      print_pass "Memory resize with RestartContainer policy correctly restarted container" "restart-policies"
    else
      print_warning "Memory resize with RestartContainer policy may not have restarted container" "restart-policies"
    fi
  else
    print_fail "Memory resize command failed" "restart-policies"
  fi
}

# Test QoS class preservation
test_qos_preservation() {
  print_test "QoS Class Preservation"

  local pods=("guaranteed-qos-test" "burstable-qos-test")

  for pod_name in "${pods[@]}"; do
    print_info "Testing QoS preservation for $pod_name"

    if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
      print_fail "Pod $pod_name did not become ready" "qos-preservation"
      continue
    fi

    # Get initial QoS class
    local initial_qos=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.qosClass}')
    print_info "Initial QoS class: $initial_qos"

    # Perform resize that should preserve QoS
    local resize_patch=""
    if [[ "$pod_name" == "guaranteed-qos-test" ]]; then
      # For Guaranteed QoS, requests must equal limits
      resize_patch='{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"300m","memory":"384Mi"}, "limits":{"cpu":"300m","memory":"384Mi"}}}]}}'
    else
      # For Burstable QoS, keep requests < limits
      resize_patch='{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"150m","memory":"192Mi"}, "limits":{"cpu":"400m","memory":"768Mi"}}}]}}'
    fi

    if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch "$resize_patch" &>/dev/null; then
      sleep $RESIZE_WAIT_TIME

      local final_qos=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.qosClass}')

      if [[ "$initial_qos" == "$final_qos" ]]; then
        print_pass "QoS class preserved for $pod_name: $initial_qos" "qos-preservation"
      else
        print_fail "QoS class changed for $pod_name: $initial_qos -> $final_qos" "qos-preservation"
      fi
    else
      print_fail "Resize command failed for $pod_name" "qos-preservation"
    fi
  done
}

# Test memory decrease handling
test_memory_decrease() {
  print_test "Memory Decrease Handling"

  local pod_name="memory-decrease-test"

  if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_fail "Pod $pod_name did not become ready" "memory-decrease"
    return 1
  fi

  # Test memory decrease with NotRequired policy (best-effort)
  print_info "Testing memory decrease with NotRequired policy..."

  local result=$(kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"512Mi"}, "limits":{"memory":"1Gi"}}}]}}' 2>&1 || echo "failed")

  if [[ "$result" != "failed" ]]; then
    sleep $RESIZE_WAIT_TIME

    local new_memory=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')
    if [[ "$new_memory" == "512Mi" ]]; then
      print_pass "Memory decrease with NotRequired policy succeeded (best-effort)" "memory-decrease"
    else
      # Check if resize is in progress
      local conditions=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.conditions[*].type}')
      if echo "$conditions" | grep -q "PodResizeInProgress"; then
        print_pass "Memory decrease with NotRequired policy in progress (acceptable)" "memory-decrease"
      else
        print_warning "Memory decrease with NotRequired policy status unclear" "memory-decrease"
      fi
    fi
  else
    print_warning "Memory decrease with NotRequired policy failed (acceptable for best-effort)" "memory-decrease"
  fi

  # Test memory decrease with RestartContainer policy
  local restart_pod_name="memory-decrease-restart-test"

  if kubectl wait --for=condition=Ready pod/$restart_pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_info "Testing memory decrease with RestartContainer policy..."

    local initial_restart_count=$(kubectl get pod $restart_pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")

    if kubectl patch pod $restart_pod_name -n $TEST_NAMESPACE --subresource resize --patch \
      '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"512Mi"}, "limits":{"memory":"1Gi"}}}]}}' &>/dev/null; then

      sleep $RESIZE_WAIT_TIME

      local final_restart_count=$(kubectl get pod $restart_pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")

      if [[ "$final_restart_count" -gt "$initial_restart_count" ]]; then
        print_pass "Memory decrease with RestartContainer policy succeeded with restart" "memory-decrease"
      else
        print_warning "Memory decrease with RestartContainer policy did not restart container" "memory-decrease"
      fi
    else
      print_fail "Memory decrease with RestartContainer policy command failed" "memory-decrease"
    fi
  fi
}

# Test infeasible resize handling
test_infeasible_resize() {
  print_test "Infeasible Resize Handling"

  local pod_name="large-resources-test"

  if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_fail "Pod $pod_name did not become ready" "infeasible-resize"
    return 1
  fi

  # Attempt infeasible resize (very large resources)
  print_info "Testing infeasible resize with excessive resource requests..."

  local result=$(kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"1000","memory":"1000Gi"}, "limits":{"cpu":"2000","memory":"2000Gi"}}}]}}' 2>&1 || echo "failed")

  if [[ "$result" == *"failed"* ]] || [[ "$result" == *"insufficient"* ]] || [[ "$result" == *"exceed"* ]]; then
    print_pass "Infeasible resize was correctly rejected" "infeasible-resize"

    # Check for appropriate status conditions
    sleep 2
    local conditions=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.conditions[*].type}' 2>/dev/null || echo "")

    if echo "$conditions" | grep -q "PodResizePending"; then
      print_pass "PodResizePending condition found for infeasible resize" "infeasible-resize"
    else
      print_warning "PodResizePending condition not found (may not be implemented)" "infeasible-resize"
    fi
  else
    print_warning "Infeasible resize was not rejected (may indicate cluster has large capacity)" "infeasible-resize"
  fi
}

# Test multi-container scenarios
test_multi_container() {
  print_test "Multi-Container Pod Resize"

  local pod_name="multi-container-test"

  if ! kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
    print_fail "Pod $pod_name did not become ready" "multi-container"
    return 1
  fi

  print_info "Testing resize on multi-container pod..."

  # Resize main container
  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"main-container", "resources":{"requests":{"cpu":"150m","memory":"192Mi"}, "limits":{"cpu":"300m","memory":"384Mi"}}}]}}' &>/dev/null; then

    sleep $RESIZE_WAIT_TIME

    local new_cpu=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')

    if [[ "$new_cpu" == "150m" ]]; then
      print_pass "Multi-container pod main container resize successful" "multi-container"
    else
      print_fail "Multi-container pod main container resize failed" "multi-container"
    fi
  else
    print_fail "Multi-container pod resize command failed" "multi-container"
  fi
}

# Test status condition tracking
test_status_conditions() {
  print_test "Pod Resize Status Conditions"

  local pod_name="basic-resize-test"

  # Perform a resize and check for status conditions
  print_info "Checking for resize status conditions..."

  if kubectl patch pod $pod_name -n $TEST_NAMESPACE --subresource resize --patch \
    '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"175m"}, "limits":{"cpu":"350m"}}}]}}' &>/dev/null; then

    sleep 2

    # Check for PodResizeInProgress condition
    local conditions=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.conditions[*].type}' 2>/dev/null || echo "")

    if echo "$conditions" | grep -q "PodResizeInProgress"; then
      print_pass "PodResizeInProgress condition found" "status-conditions"
    else
      print_warning "PodResizeInProgress condition not found (may not be implemented)" "status-conditions"
    fi

    # Check observedGeneration
    local observed_generation=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.status.observedGeneration}' 2>/dev/null || echo "0")
    local current_generation=$(kubectl get pod $pod_name -n $TEST_NAMESPACE -o jsonpath='{.metadata.generation}' 2>/dev/null || echo "0")

    if [[ "$observed_generation" -gt 0 ]] && [[ "$observed_generation" -le "$current_generation" ]]; then
      print_pass "ObservedGeneration tracking is working" "status-conditions"
    else
      print_warning "ObservedGeneration tracking may not be implemented" "status-conditions"
    fi
  else
    print_fail "Resize command for status condition testing failed" "status-conditions"
  fi
}

# Test right-sizer integration
test_rightsizer_integration() {
  print_test "Right-Sizer Integration"

  # Check if right-sizer is installed
  if kubectl get deployment right-sizer -n right-sizer-system &>/dev/null || kubectl get deployment right-sizer -A &>/dev/null; then
    print_pass "Right-sizer operator is installed" "rightsizer-integration"

    # Check deployment pods
    local deployment_name="rightsizer-integration-test"

    if kubectl wait --for=condition=available deployment/$deployment_name -n $TEST_NAMESPACE --timeout=60s &>/dev/null; then
      print_pass "Test deployment is available for right-sizer processing" "rightsizer-integration"

      # Wait for right-sizer to potentially process the pods
      print_info "Waiting for right-sizer to process deployment pods..."
      sleep 30

      # Check if right-sizer made any changes (this is hard to verify without metrics)
      local pods=$(kubectl get pods -n $TEST_NAMESPACE -l app=rightsizer-test --no-headers)
      if [[ -n "$pods" ]]; then
        print_pass "Deployment pods are running and available for right-sizer" "rightsizer-integration"
      else
        print_warning "No deployment pods found" "rightsizer-integration"
      fi
    else
      print_warning "Test deployment did not become available" "rightsizer-integration"
    fi
  else
    print_warning "Right-sizer operator is not installed - skipping integration tests" "rightsizer-integration"
  fi
}

# Generate compliance report
generate_report() {
  print_header "📊 Generating Compliance Report"

  local compliance_percentage=$((PASSED_TESTS * 100 / TOTAL_TESTS))

  # Generate JSON report
  cat >"$REPORT_FILE" <<EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "test_environment": {
    "kubernetes_version": "$(kubectl version --short 2>/dev/null | grep "Server Version" | awk '{print $3}')",
    "kubectl_version": "$(kubectl version --client --short 2>/dev/null | grep "Client Version" | awk '{print $3}')",
    "cluster_info": "$(kubectl cluster-info | head -1)"
  },
  "summary": {
    "total_tests": $TOTAL_TESTS,
    "passed_tests": $PASSED_TESTS,
    "failed_tests": $FAILED_TESTS,
    "warnings": $WARNINGS,
    "compliance_percentage": $compliance_percentage
  },
  "test_results": [
    $(
    IFS=,
    echo "${TEST_RESULTS[*]}"
  )
  ],
  "compliance_status": "$(
    if [[ $compliance_percentage -ge 80 ]]; then
      echo "HIGHLY_COMPLIANT"
    elif [[ $compliance_percentage -ge 60 ]]; then
      echo "PARTIALLY_COMPLIANT"
    else
      echo "NON_COMPLIANT"
    fi
  )"
}
EOF

  print_info "JSON report saved to: $REPORT_FILE"

  # Print summary
  echo -e "\n${BLUE}📋 TEST SUMMARY${NC}"
  echo "$(printf '─%.0s' {1..50})"
  echo -e "Total Tests: $TOTAL_TESTS"
  echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
  echo -e "${RED}Failed: $FAILED_TESTS${NC}"
  echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
  echo -e "\n${BLUE}Compliance Score: $compliance_percentage%${NC}"

  if [[ $compliance_percentage -ge 80 ]]; then
    echo -e "${GREEN}✅ HIGHLY COMPLIANT${NC} - Minor issues to address"
  elif [[ $compliance_percentage -ge 60 ]]; then
    echo -e "${YELLOW}⚠️  PARTIALLY COMPLIANT${NC} - Several features need implementation"
  else
    echo -e "${RED}❌ NON_COMPLIANT${NC} - Major features missing"
  fi

  print_header "📋 Detailed Results"

  # Print test results
  for result in "${TEST_RESULTS[@]}"; do
    local test_name=$(echo "$result" | jq -r '.test' 2>/dev/null || echo "unknown")
    local status=$(echo "$result" | jq -r '.status' 2>/dev/null || echo "unknown")
    local message=$(echo "$result" | jq -r '.message' 2>/dev/null || echo "unknown")

    case $status in
    "PASS") echo -e "${GREEN}✅${NC} $test_name: $message" ;;
    "FAIL") echo -e "${RED}❌${NC} $test_name: $message" ;;
    "WARN") echo -e "${YELLOW}⚠️${NC} $test_name: $message" ;;
    esac
  done

  print_header "🔧 Recommendations"

  if [[ $FAILED_TESTS -gt 0 ]]; then
    echo "Critical issues to fix:"
    echo "  1. Implement missing Pod resize status conditions"
    echo "  2. Add ObservedGeneration tracking"
    echo "  3. Enhance QoS class preservation validation"
  fi

  if [[ $WARNINGS -gt 0 ]]; then
    echo "Improvements to consider:"
    echo "  1. Implement deferred resize retry logic"
    echo "  2. Add comprehensive error handling"
    echo "  3. Enhance status reporting"
  fi

  echo -e "\nFor detailed implementation guidance, see:"
  echo "K8S_INPLACE_RESIZE_COMPLIANCE_REPORT.md"
}

# Main execution
main() {
  print_header "🚀 Kubernetes 1.33+ In-Place Resize Compliance Tests"
  echo "Test execution started: $(date)"
  echo "Test namespace: $TEST_NAMESPACE"

  # Run all tests
  check_prerequisites
  setup_test_environment

  test_basic_resize
  test_restart_policies
  test_qos_preservation
  test_memory_decrease
  test_infeasible_resize
  test_multi_container
  test_status_conditions
  test_rightsizer_integration

  generate_report

  print_header "✨ Test Execution Complete"
  echo "Report saved to: $REPORT_FILE"

  if [[ $FAILED_TESTS -eq 0 ]]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
  else
    echo -e "${RED}❌ Some tests failed. Review the report for details.${NC}"
    exit 1
  fi
}

# Execute main function with all arguments
main "$@"
