#!/bin/bash

# Right Sizer RBAC Comprehensive Test Suite
# This script performs extensive testing of all RBAC permissions required by the Right Sizer operator

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Test configuration
NAMESPACE="${NAMESPACE:-right-sizer-system}"
SERVICE_ACCOUNT="${SERVICE_ACCOUNT:-right-sizer}"
TEST_NAMESPACE="${TEST_NAMESPACE:-default}"
VERBOSE="${VERBOSE:-false}"
SKIP_CLEANUP="${SKIP_CLEANUP:-false}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-terminal}" # terminal, json, junit

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0
WARNING_TESTS=0

# Test results array for JSON/JUnit output
declare -a TEST_RESULTS=()

# Temporary resources created during testing
TEMP_RESOURCES=()

# Function to print test results
print_test() {
  local status=$1
  local category=$2
  local test_name=$3
  local details=${4:-""}

  ((TOTAL_TESTS++))

  local symbol=""
  local color=""

  case $status in
  PASS)
    symbol="✓"
    color=$GREEN
    ((PASSED_TESTS++))
    ;;
  FAIL)
    symbol="✗"
    color=$RED
    ((FAILED_TESTS++))
    ;;
  SKIP)
    symbol="○"
    color=$YELLOW
    ((SKIPPED_TESTS++))
    ;;
  WARN)
    symbol="⚠"
    color=$YELLOW
    ((WARNING_TESTS++))
    ;;
  esac

  if [[ "$OUTPUT_FORMAT" == "terminal" ]]; then
    printf "${color}%s${NC} [%-20s] %-40s" "$symbol" "$category" "$test_name"
    if [[ -n "$details" && "$VERBOSE" == "true" ]]; then
      printf " ${CYAN}%s${NC}" "$details"
    fi
    printf "\n"
  fi

  # Store result for other output formats
  TEST_RESULTS+=("{\"status\":\"$status\",\"category\":\"$category\",\"test\":\"$test_name\",\"details\":\"$details\"}")
}

# Function to print section header
print_section() {
  local section=$1
  if [[ "$OUTPUT_FORMAT" == "terminal" ]]; then
    echo
    echo -e "${BLUE}${BOLD}━━━ $section ━━━${NC}"
  fi
}

# Function to check if permission exists
test_permission() {
  local verb=$1
  local resource=$2
  local api_group=${3:-""}
  local namespace=${4:-""}

  local resource_display=$resource
  if [[ -n "$api_group" ]]; then
    resource_display="$resource.$api_group"
  fi

  local cmd="kubectl auth can-i $verb $resource_display"
  if [[ -n "$namespace" ]]; then
    cmd="$cmd -n $namespace"
  fi
  cmd="$cmd --as=system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT"

  if $cmd &>/dev/null; then
    return 0
  else
    return 1
  fi
}

# Function to test actual resource operations
test_operation() {
  local operation=$1
  local description=$2

  if eval "$operation" &>/dev/null; then
    return 0
  else
    return 1
  fi
}

# Function to check API availability
check_api() {
  local api_group=$1
  kubectl api-versions 2>/dev/null | grep -q "^$api_group" || return 1
}

# Function to create test resources
create_test_resource() {
  local resource_type=$1
  local resource_name=$2
  local namespace=${3:-$TEST_NAMESPACE}

  case $resource_type in
  configmap)
    kubectl create configmap "$resource_name" \
      --from-literal=test=data \
      -n "$namespace" \
      --dry-run=client -o yaml | kubectl apply -f - &>/dev/null
    ;;
  pod)
    kubectl run "$resource_name" \
      --image=busybox:latest \
      --command -- sleep 3600 \
      -n "$namespace" \
      --dry-run=client -o yaml | kubectl apply -f - &>/dev/null
    ;;
  esac

  TEMP_RESOURCES+=("$resource_type/$resource_name -n $namespace")
}

# Function to cleanup test resources
cleanup_test_resources() {
  if [[ "$SKIP_CLEANUP" == "false" ]]; then
    for resource in "${TEMP_RESOURCES[@]}"; do
      kubectl delete $resource --ignore-not-found=true &>/dev/null || true
    done
  fi
}

# Trap to ensure cleanup on exit
trap cleanup_test_resources EXIT

# Test Suite Functions

test_service_account() {
  print_section "Service Account Configuration"

  # Check service account exists
  if kubectl get sa "$SERVICE_ACCOUNT" -n "$NAMESPACE" &>/dev/null; then
    print_test "PASS" "ServiceAccount" "Service account exists"
  else
    print_test "FAIL" "ServiceAccount" "Service account exists"
    echo -e "${RED}CRITICAL: Service account not found. Cannot proceed with tests.${NC}"
    exit 1
  fi

  # Check for automountServiceAccountToken
  local automount=$(kubectl get sa "$SERVICE_ACCOUNT" -n "$NAMESPACE" -o jsonpath='{.automountServiceAccountToken}')
  if [[ "$automount" == "true" ]] || [[ -z "$automount" ]]; then
    print_test "PASS" "ServiceAccount" "Token auto-mount enabled"
  else
    print_test "WARN" "ServiceAccount" "Token auto-mount disabled"
  fi
}

test_core_resources() {
  print_section "Core Kubernetes Resources"

  # Pods
  local pod_verbs=("get" "list" "watch" "patch" "update")
  local pod_pass=true
  for verb in "${pod_verbs[@]}"; do
    if ! test_permission "$verb" "pods"; then
      pod_pass=false
      break
    fi
  done
  if $pod_pass; then
    print_test "PASS" "Pods" "All pod operations" "verbs: ${pod_verbs[*]}"
  else
    print_test "FAIL" "Pods" "Pod operations incomplete"
  fi

  # Pod status
  if test_permission "get" "pods/status" && test_permission "patch" "pods/status"; then
    print_test "PASS" "Pods" "Pod status operations"
  else
    print_test "FAIL" "Pods" "Pod status operations"
  fi

  # Pod resize (K8s 1.27+)
  if test_permission "patch" "pods/resize"; then
    print_test "PASS" "Pods" "In-place resize support"
  else
    print_test "WARN" "Pods" "In-place resize not available"
  fi

  # Nodes
  if test_permission "list" "nodes" && test_permission "get" "nodes/status"; then
    print_test "PASS" "Nodes" "Node information access"
  else
    print_test "FAIL" "Nodes" "Node information access"
  fi

  # Events
  if test_permission "create" "events" && test_permission "patch" "events"; then
    print_test "PASS" "Events" "Event creation/update"
  else
    print_test "FAIL" "Events" "Event creation/update"
  fi

  # Namespaces
  if test_permission "list" "namespaces"; then
    print_test "PASS" "Namespaces" "Namespace listing"
  else
    print_test "FAIL" "Namespaces" "Namespace listing"
  fi

  # ConfigMaps (cluster-wide read)
  if test_permission "list" "configmaps"; then
    print_test "PASS" "ConfigMaps" "ConfigMap read access"
  else
    print_test "FAIL" "ConfigMaps" "ConfigMap read access"
  fi

  # Secrets (cluster-wide read)
  if test_permission "list" "secrets"; then
    print_test "PASS" "Secrets" "Secret read access"
  else
    print_test "FAIL" "Secrets" "Secret read access"
  fi
}

test_metrics_apis() {
  print_section "Metrics APIs"

  # Check if metrics.k8s.io API is available
  if check_api "metrics.k8s.io"; then
    # Pod metrics
    if test_permission "list" "pods" "" "" "metrics.k8s.io"; then
      print_test "PASS" "Metrics" "Pod metrics access"
    else
      print_test "FAIL" "Metrics" "Pod metrics access"
    fi

    # Node metrics
    if test_permission "list" "nodes" "" "" "metrics.k8s.io"; then
      print_test "PASS" "Metrics" "Node metrics access"
    else
      print_test "FAIL" "Metrics" "Node metrics access"
    fi

    # PodMetrics resource
    if test_permission "list" "podmetrics" "" "" "metrics.k8s.io"; then
      print_test "PASS" "Metrics" "PodMetrics resource access"
    else
      print_test "FAIL" "Metrics" "PodMetrics resource access"
    fi
  else
    print_test "SKIP" "Metrics" "Metrics API not installed"
  fi

  # Custom metrics
  if check_api "custom.metrics.k8s.io"; then
    if test_permission "list" "*" "" "" "custom.metrics.k8s.io"; then
      print_test "PASS" "CustomMetrics" "Custom metrics access"
    else
      print_test "WARN" "CustomMetrics" "Custom metrics limited"
    fi
  else
    print_test "SKIP" "CustomMetrics" "Custom metrics API not installed"
  fi

  # External metrics
  if check_api "external.metrics.k8s.io"; then
    if test_permission "list" "*" "" "" "external.metrics.k8s.io"; then
      print_test "PASS" "ExternalMetrics" "External metrics access"
    else
      print_test "WARN" "ExternalMetrics" "External metrics limited"
    fi
  else
    print_test "SKIP" "ExternalMetrics" "External metrics API not installed"
  fi
}

test_workload_controllers() {
  print_section "Workload Controllers"

  local controllers=("deployments" "statefulsets" "daemonsets" "replicasets")

  for controller in "${controllers[@]}"; do
    local verbs=("get" "list" "watch" "patch" "update")
    local all_pass=true

    for verb in "${verbs[@]}"; do
      if ! test_permission "$verb" "$controller" "apps"; then
        all_pass=false
        break
      fi
    done

    if $all_pass; then
      print_test "PASS" "Workloads" "$controller management"
    else
      print_test "FAIL" "Workloads" "$controller management"
    fi

    # Test scale subresource
    if test_permission "patch" "$controller/scale" "apps"; then
      print_test "PASS" "Workloads" "$controller/scale access"
    else
      print_test "FAIL" "Workloads" "$controller/scale access"
    fi
  done

  # Batch jobs
  if test_permission "list" "jobs" "batch"; then
    print_test "PASS" "Batch" "Job read access"
  else
    print_test "FAIL" "Batch" "Job read access"
  fi

  if test_permission "list" "cronjobs" "batch"; then
    print_test "PASS" "Batch" "CronJob read access"
  else
    print_test "FAIL" "Batch" "CronJob read access"
  fi
}

test_autoscaling() {
  print_section "Autoscaling Resources"

  # HPA
  if test_permission "list" "horizontalpodautoscalers" "autoscaling"; then
    print_test "PASS" "Autoscaling" "HPA read access"
  else
    print_test "FAIL" "Autoscaling" "HPA read access"
  fi

  # VPA (if available)
  if check_api "autoscaling.k8s.io"; then
    if test_permission "list" "verticalpodautoscalers" "autoscaling.k8s.io"; then
      print_test "PASS" "Autoscaling" "VPA read access"
    else
      print_test "FAIL" "Autoscaling" "VPA read access"
    fi
  else
    print_test "SKIP" "Autoscaling" "VPA not installed"
  fi
}

test_resource_constraints() {
  print_section "Resource Constraints"

  # ResourceQuotas
  if test_permission "list" "resourcequotas"; then
    print_test "PASS" "Constraints" "ResourceQuota access"
  else
    print_test "FAIL" "Constraints" "ResourceQuota access"
  fi

  # LimitRanges
  if test_permission "list" "limitranges"; then
    print_test "PASS" "Constraints" "LimitRange access"
  else
    print_test "FAIL" "Constraints" "LimitRange access"
  fi

  # PodDisruptionBudgets
  if test_permission "list" "poddisruptionbudgets" "policy"; then
    print_test "PASS" "Constraints" "PodDisruptionBudget access"
  else
    print_test "FAIL" "Constraints" "PodDisruptionBudget access"
  fi
}

test_storage_resources() {
  print_section "Storage Resources"

  # PVCs
  if test_permission "list" "persistentvolumeclaims"; then
    print_test "PASS" "Storage" "PVC read access"
  else
    print_test "FAIL" "Storage" "PVC read access"
  fi

  # PVs
  if test_permission "list" "persistentvolumes"; then
    print_test "PASS" "Storage" "PV read access"
  else
    print_test "FAIL" "Storage" "PV read access"
  fi

  # StorageClasses
  if test_permission "list" "storageclasses" "storage.k8s.io"; then
    print_test "PASS" "Storage" "StorageClass read access"
  else
    print_test "FAIL" "Storage" "StorageClass read access"
  fi
}

test_networking() {
  print_section "Networking Resources"

  # Services
  if test_permission "list" "services" && test_permission "create" "services"; then
    print_test "PASS" "Networking" "Service management"
  else
    print_test "FAIL" "Networking" "Service management"
  fi

  # Endpoints
  if test_permission "list" "endpoints"; then
    print_test "PASS" "Networking" "Endpoint read access"
  else
    print_test "FAIL" "Networking" "Endpoint read access"
  fi

  # NetworkPolicies
  if test_permission "list" "networkpolicies" "networking.k8s.io"; then
    print_test "PASS" "Networking" "NetworkPolicy read access"
  else
    print_test "FAIL" "Networking" "NetworkPolicy read access"
  fi
}

test_admission_webhooks() {
  print_section "Admission Webhooks"

  # ValidatingWebhookConfiguration
  local webhook_verbs=("get" "list" "create" "update" "delete")
  local validating_pass=true

  for verb in "${webhook_verbs[@]}"; do
    if ! test_permission "$verb" "validatingwebhookconfigurations" "admissionregistration.k8s.io"; then
      validating_pass=false
      break
    fi
  done

  if $validating_pass; then
    print_test "PASS" "Webhooks" "ValidatingWebhook management"
  else
    print_test "FAIL" "Webhooks" "ValidatingWebhook management"
  fi

  # MutatingWebhookConfiguration
  local mutating_pass=true
  for verb in "${webhook_verbs[@]}"; do
    if ! test_permission "$verb" "mutatingwebhookconfigurations" "admissionregistration.k8s.io"; then
      mutating_pass=false
      break
    fi
  done

  if $mutating_pass; then
    print_test "PASS" "Webhooks" "MutatingWebhook management"
  else
    print_test "FAIL" "Webhooks" "MutatingWebhook management"
  fi
}

test_custom_resources() {
  print_section "Custom Resources"

  # Check if Right Sizer CRDs are installed
  if check_api "rightsizer.io"; then
    # RightSizerPolicies
    if test_permission "create" "rightsizerpolicies" "rightsizer.io" &&
      test_permission "update" "rightsizerpolicies/status" "rightsizer.io"; then
      print_test "PASS" "CRDs" "RightSizerPolicy management"
    else
      print_test "FAIL" "CRDs" "RightSizerPolicy management"
    fi

    # RightSizerConfigs
    if test_permission "create" "rightsizerconfigs" "rightsizer.io" &&
      test_permission "update" "rightsizerconfigs/status" "rightsizer.io"; then
      print_test "PASS" "CRDs" "RightSizerConfig management"
    else
      print_test "FAIL" "CRDs" "RightSizerConfig management"
    fi
  else
    print_test "SKIP" "CRDs" "Right Sizer CRDs not installed"
  fi
}

test_namespace_scoped() {
  print_section "Namespace-Scoped Permissions"

  # ConfigMaps in operator namespace
  if test_permission "create" "configmaps" "" "$NAMESPACE" &&
    test_permission "delete" "configmaps" "" "$NAMESPACE"; then
    print_test "PASS" "Namespace" "ConfigMap management in operator namespace"
  else
    print_test "FAIL" "Namespace" "ConfigMap management in operator namespace"
  fi

  # Secrets in operator namespace
  if test_permission "create" "secrets" "" "$NAMESPACE" &&
    test_permission "delete" "secrets" "" "$NAMESPACE"; then
    print_test "PASS" "Namespace" "Secret management in operator namespace"
  else
    print_test "FAIL" "Namespace" "Secret management in operator namespace"
  fi

  # Leases for leader election
  if test_permission "create" "leases" "coordination.k8s.io" "$NAMESPACE" &&
    test_permission "update" "leases" "coordination.k8s.io" "$NAMESPACE"; then
    print_test "PASS" "Namespace" "Lease management for leader election"
  else
    print_test "FAIL" "Namespace" "Lease management for leader election"
  fi
}

test_operational_scenarios() {
  print_section "Operational Scenarios"

  # Test 1: Can read pod and update its resources
  local test_pod="rbac-test-pod-$$"
  if kubectl run "$test_pod" --image=busybox:latest --command -- sleep 10 \
    -n "$TEST_NAMESPACE" --dry-run=client -o yaml |
    kubectl --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" apply -f - &>/dev/null; then
    print_test "PASS" "Operations" "Create test pod"
    kubectl delete pod "$test_pod" -n "$TEST_NAMESPACE" --ignore-not-found &>/dev/null
  else
    print_test "WARN" "Operations" "Cannot create test pod (expected)"
  fi

  # Test 2: Can list pods across namespaces
  if kubectl --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" \
    get pods --all-namespaces &>/dev/null; then
    print_test "PASS" "Operations" "List pods across all namespaces"
  else
    print_test "FAIL" "Operations" "List pods across all namespaces"
  fi

  # Test 3: Can get node information
  if kubectl --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" \
    get nodes &>/dev/null; then
    print_test "PASS" "Operations" "Get node information"
  else
    print_test "FAIL" "Operations" "Get node information"
  fi

  # Test 4: Can list deployments
  if kubectl --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" \
    get deployments --all-namespaces &>/dev/null; then
    print_test "PASS" "Operations" "List deployments"
  else
    print_test "FAIL" "Operations" "List deployments"
  fi
}

test_scheduling_resources() {
  print_section "Scheduling Resources"

  # PriorityClasses
  if test_permission "list" "priorityclasses" "scheduling.k8s.io"; then
    print_test "PASS" "Scheduling" "PriorityClass read access"
  else
    print_test "FAIL" "Scheduling" "PriorityClass read access"
  fi
}

generate_summary() {
  print_section "Test Summary"

  local total_critical=$((FAILED_TESTS))
  local success_rate=0
  if [[ $TOTAL_TESTS -gt 0 ]]; then
    success_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
  fi

  if [[ "$OUTPUT_FORMAT" == "terminal" ]]; then
    echo
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${BOLD}RBAC Test Suite Results${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo
    echo -e "Total Tests:    ${BOLD}$TOTAL_TESTS${NC}"
    echo -e "${GREEN}Passed:         $PASSED_TESTS${NC}"
    echo -e "${RED}Failed:         $FAILED_TESTS${NC}"
    echo -e "${YELLOW}Warnings:       $WARNING_TESTS${NC}"
    echo -e "${YELLOW}Skipped:        $SKIPPED_TESTS${NC}"
    echo
    echo -e "Success Rate:   ${BOLD}$success_rate%${NC}"
    echo

    if [[ $FAILED_TESTS -eq 0 ]]; then
      echo -e "${GREEN}${BOLD}✓ All critical RBAC permissions verified!${NC}"
      echo "The Right Sizer operator has all required permissions."
    elif [[ $FAILED_TESTS -le 3 ]]; then
      echo -e "${YELLOW}${BOLD}⚠ Some RBAC permissions are missing${NC}"
      echo "The operator may function with limited capabilities."
      echo "Run: ./scripts/rbac/apply-rbac-fix.sh to fix issues"
    else
      echo -e "${RED}${BOLD}✗ Critical RBAC permissions are missing${NC}"
      echo "The operator will not function correctly."
      echo "Run: ./scripts/rbac/apply-rbac-fix.sh --force to fix issues"
    fi
  elif [[ "$OUTPUT_FORMAT" == "json" ]]; then
    echo "{"
    echo "  \"total\": $TOTAL_TESTS,"
    echo "  \"passed\": $PASSED_TESTS,"
    echo "  \"failed\": $FAILED_TESTS,"
    echo "  \"warnings\": $WARNING_TESTS,"
    echo "  \"skipped\": $SKIPPED_TESTS,"
    echo "  \"success_rate\": $success_rate,"
    echo "  \"results\": ["
    printf '%s\n' "${TEST_RESULTS[@]}" | paste -sd ',' -
    echo "  ]"
    echo "}"
  fi

  # Exit with appropriate code
  if [[ $FAILED_TESTS -eq 0 ]]; then
    exit 0
  elif [[ $FAILED_TESTS -le 3 ]]; then
    exit 1
  else
    exit 2
  fi
}

# Main execution
main() {
  if [[ "$OUTPUT_FORMAT" == "terminal" ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${BOLD}Right Sizer RBAC Test Suite${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo
    echo -e "${CYAN}Configuration:${NC}"
    echo "  Namespace:       $NAMESPACE"
    echo "  Service Account: $SERVICE_ACCOUNT"
    echo "  Test Namespace:  $TEST_NAMESPACE"
    echo
  fi

  # Run all test suites
  test_service_account
  test_core_resources
  test_metrics_apis
  test_workload_controllers
  test_autoscaling
  test_resource_constraints
  test_storage_resources
  test_networking
  test_admission_webhooks
  test_custom_resources
  test_namespace_scoped
  test_scheduling_resources
  test_operational_scenarios

  # Generate summary
  generate_summary
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -n | --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  -s | --service-account)
    SERVICE_ACCOUNT="$2"
    shift 2
    ;;
  -t | --test-namespace)
    TEST_NAMESPACE="$2"
    shift 2
    ;;
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  --skip-cleanup)
    SKIP_CLEANUP=true
    shift
    ;;
  -o | --output)
    OUTPUT_FORMAT="$2"
    shift 2
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -n, --namespace NAME           Namespace where Right Sizer is installed"
    echo "  -s, --service-account NAME     Service account name"
    echo "  -t, --test-namespace NAME      Namespace for test resources"
    echo "  -v, --verbose                  Show detailed test output"
    echo "  --skip-cleanup                 Don't clean up test resources"
    echo "  -o, --output FORMAT            Output format (terminal, json, junit)"
    echo "  -h, --help                     Show this help message"
    echo
    echo "Examples:"
    echo "  # Run with defaults"
    echo "  $0"
    echo
    echo "  # Run with verbose output"
    echo "  $0 --verbose"
    echo
    echo "  # Output as JSON"
    echo "  $0 --output json > rbac-test-results.json"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use -h or --help for usage information"
    exit 1
    ;;
  esac
done

# Run main test suite
main
