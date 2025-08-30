#!/bin/bash

# Right Sizer RBAC Permissions Verification Script
# This script verifies that the Right Sizer operator has all required RBAC permissions

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="${NAMESPACE:-right-sizer-system}"
SERVICE_ACCOUNT="${SERVICE_ACCOUNT:-right-sizer}"
VERBOSE="${VERBOSE:-false}"
CHECK_METRICS="${CHECK_METRICS:-true}"
CHECK_CUSTOM_RESOURCES="${CHECK_CUSTOM_RESOURCES:-true}"

# Counters for summary
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# Function to print colored output
print_status() {
  local status=$1
  local message=$2

  case $status in
  "PASS")
    echo -e "${GREEN}✓${NC} $message"
    ((PASSED_CHECKS++))
    ;;
  "FAIL")
    echo -e "${RED}✗${NC} $message"
    ((FAILED_CHECKS++))
    ;;
  "WARN")
    echo -e "${YELLOW}⚠${NC} $message"
    ((WARNING_CHECKS++))
    ;;
  "INFO")
    echo -e "${BLUE}ℹ${NC} $message"
    ;;
  "HEADER")
    echo -e "\n${BLUE}═══ $message ═══${NC}"
    ;;
  esac
  ((TOTAL_CHECKS++)) || true
}

# Function to check if a permission exists
check_permission() {
  local verb=$1
  local resource=$2
  local api_group=${3:-""}
  local namespace_scope=${4:-""}

  local resource_display=$resource
  if [[ -n "$api_group" ]]; then
    resource_display="$resource.$api_group"
  fi

  local auth_check_cmd="kubectl auth can-i $verb $resource_display"

  # Add namespace flag if checking namespace-scoped permissions
  if [[ -n "$namespace_scope" ]]; then
    auth_check_cmd="$auth_check_cmd -n $namespace_scope"
  fi

  # Add service account context
  auth_check_cmd="$auth_check_cmd --as=system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT"

  if [[ "$VERBOSE" == "true" ]]; then
    echo "  Checking: $auth_check_cmd"
  fi

  if $auth_check_cmd &>/dev/null; then
    return 0
  else
    return 1
  fi
}

# Function to check a set of permissions
check_resource_permissions() {
  local resource=$1
  local api_group=$2
  shift 2
  local verbs=("$@")

  local resource_display=$resource
  if [[ -n "$api_group" ]]; then
    resource_display="$resource in API group '$api_group'"
  fi

  local all_passed=true
  local failed_verbs=()

  for verb in "${verbs[@]}"; do
    if ! check_permission "$verb" "$resource" "$api_group"; then
      all_passed=false
      failed_verbs+=("$verb")
    fi
  done

  if $all_passed; then
    print_status "PASS" "$resource_display: ${verbs[*]}"
  else
    print_status "FAIL" "$resource_display: Failed verbs: ${failed_verbs[*]}"
  fi
}

# Function to check API availability
check_api_availability() {
  local api_group=$1

  if kubectl api-versions | grep -q "^$api_group"; then
    return 0
  else
    return 1
  fi
}

# Main verification function
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Right Sizer RBAC Permissions Verification"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo
  print_status "INFO" "Namespace: $NAMESPACE"
  print_status "INFO" "Service Account: $SERVICE_ACCOUNT"
  echo

  # Check if service account exists
  if kubectl get serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" &>/dev/null; then
    print_status "PASS" "Service account exists"
  else
    print_status "FAIL" "Service account '$SERVICE_ACCOUNT' not found in namespace '$NAMESPACE'"
    echo
    echo "Please ensure the Right Sizer is properly installed."
    exit 1
  fi

  # Core Resources
  print_status "HEADER" "Core Resources"

  check_resource_permissions "pods" "" "get" "list" "watch" "patch" "update"
  check_resource_permissions "pods/status" "" "get" "list" "watch" "patch" "update"
  check_resource_permissions "pods/resize" "" "patch" "update"
  check_resource_permissions "nodes" "" "get" "list" "watch"
  check_resource_permissions "nodes/status" "" "get" "list" "watch"
  check_resource_permissions "events" "" "create" "patch" "update"
  check_resource_permissions "namespaces" "" "get" "list" "watch"
  check_resource_permissions "configmaps" "" "get" "list" "watch"
  check_resource_permissions "secrets" "" "get" "list" "watch"
  check_resource_permissions "services" "" "get" "list" "watch" "create" "update" "patch"
  check_resource_permissions "endpoints" "" "get" "list" "watch"

  # Resource Constraints
  print_status "HEADER" "Resource Constraints"

  check_resource_permissions "resourcequotas" "" "get" "list" "watch"
  check_resource_permissions "limitranges" "" "get" "list" "watch"
  check_resource_permissions "poddisruptionbudgets" "policy" "get" "list" "watch"

  # Workload Controllers
  print_status "HEADER" "Workload Controllers"

  check_resource_permissions "deployments" "apps" "get" "list" "watch" "patch" "update"
  check_resource_permissions "deployments/status" "apps" "get" "list" "watch"
  check_resource_permissions "deployments/scale" "apps" "get" "patch" "update"
  check_resource_permissions "statefulsets" "apps" "get" "list" "watch" "patch" "update"
  check_resource_permissions "statefulsets/status" "apps" "get" "list" "watch"
  check_resource_permissions "statefulsets/scale" "apps" "get" "patch" "update"
  check_resource_permissions "daemonsets" "apps" "get" "list" "watch" "patch" "update"
  check_resource_permissions "daemonsets/status" "apps" "get" "list" "watch"
  check_resource_permissions "replicasets" "apps" "get" "list" "watch" "patch" "update"
  check_resource_permissions "replicasets/status" "apps" "get" "list" "watch"
  check_resource_permissions "replicasets/scale" "apps" "get" "patch" "update"

  # Batch Jobs
  print_status "HEADER" "Batch Jobs"

  check_resource_permissions "jobs" "batch" "get" "list" "watch"
  check_resource_permissions "cronjobs" "batch" "get" "list" "watch"

  # Metrics APIs
  if [[ "$CHECK_METRICS" == "true" ]]; then
    print_status "HEADER" "Metrics APIs"

    if check_api_availability "metrics.k8s.io"; then
      check_resource_permissions "pods" "metrics.k8s.io" "get" "list" "watch"
      check_resource_permissions "nodes" "metrics.k8s.io" "get" "list" "watch"
      check_resource_permissions "podmetrics" "metrics.k8s.io" "get" "list" "watch"
      check_resource_permissions "nodemetrics" "metrics.k8s.io" "get" "list" "watch"
    else
      print_status "WARN" "Metrics API (metrics.k8s.io) not available in cluster"
    fi

    # Check custom metrics if available
    if check_api_availability "custom.metrics.k8s.io"; then
      print_status "INFO" "Custom Metrics API available"
      check_resource_permissions "*" "custom.metrics.k8s.io" "get" "list" "watch"
    fi

    if check_api_availability "external.metrics.k8s.io"; then
      print_status "INFO" "External Metrics API available"
      check_resource_permissions "*" "external.metrics.k8s.io" "get" "list" "watch"
    fi
  fi

  # Autoscaling Resources
  print_status "HEADER" "Autoscaling Resources"

  check_resource_permissions "horizontalpodautoscalers" "autoscaling" "get" "list" "watch"

  # Check VPA if available
  if check_api_availability "autoscaling.k8s.io"; then
    check_resource_permissions "verticalpodautoscalers" "autoscaling.k8s.io" "get" "list" "watch"
  else
    print_status "INFO" "VPA API (autoscaling.k8s.io) not installed"
  fi

  # Storage Resources
  print_status "HEADER" "Storage Resources"

  check_resource_permissions "persistentvolumeclaims" "" "get" "list" "watch"
  check_resource_permissions "persistentvolumes" "" "get" "list" "watch"
  check_resource_permissions "storageclasses" "storage.k8s.io" "get" "list" "watch"

  # Scheduling Resources
  print_status "HEADER" "Scheduling Resources"

  check_resource_permissions "priorityclasses" "scheduling.k8s.io" "get" "list" "watch"

  # Networking Resources
  print_status "HEADER" "Networking Resources"

  check_resource_permissions "networkpolicies" "networking.k8s.io" "get" "list" "watch"

  # Admission Webhooks
  print_status "HEADER" "Admission Webhooks"

  check_resource_permissions "validatingwebhookconfigurations" "admissionregistration.k8s.io" "get" "list" "watch" "create" "update" "patch" "delete"
  check_resource_permissions "mutatingwebhookconfigurations" "admissionregistration.k8s.io" "get" "list" "watch" "create" "update" "patch" "delete"

  # Custom Resources
  if [[ "$CHECK_CUSTOM_RESOURCES" == "true" ]]; then
    print_status "HEADER" "Custom Resources"

    if check_api_availability "rightsizer.io"; then
      check_resource_permissions "rightsizerpolicies" "rightsizer.io" "get" "list" "watch" "create" "update" "patch" "delete"
      check_resource_permissions "rightsizerpolicies/status" "rightsizer.io" "get" "update" "patch"
      check_resource_permissions "rightsizerconfigs" "rightsizer.io" "get" "list" "watch" "create" "update" "patch" "delete"
      check_resource_permissions "rightsizerconfigs/status" "rightsizer.io" "get" "update" "patch"
    else
      print_status "WARN" "Right Sizer CRDs (rightsizer.io) not installed"
    fi
  fi

  # Namespace-specific permissions
  print_status "HEADER" "Namespace-Specific Permissions (in $NAMESPACE)"

  # Check namespace-scoped permissions in the operator's namespace
  if check_permission "create" "configmaps" "" "$NAMESPACE" &&
    check_permission "update" "configmaps" "" "$NAMESPACE" &&
    check_permission "delete" "configmaps" "" "$NAMESPACE"; then
    print_status "PASS" "ConfigMaps management in operator namespace"
  else
    print_status "FAIL" "ConfigMaps management in operator namespace"
  fi

  if check_permission "create" "leases" "coordination.k8s.io" "$NAMESPACE" &&
    check_permission "update" "leases" "coordination.k8s.io" "$NAMESPACE" &&
    check_permission "delete" "leases" "coordination.k8s.io" "$NAMESPACE"; then
    print_status "PASS" "Leases for leader election"
  else
    print_status "FAIL" "Leases for leader election"
  fi

  if check_permission "create" "secrets" "" "$NAMESPACE" &&
    check_permission "update" "secrets" "" "$NAMESPACE" &&
    check_permission "delete" "secrets" "" "$NAMESPACE"; then
    print_status "PASS" "Secrets management in operator namespace"
  else
    print_status "FAIL" "Secrets management in operator namespace"
  fi

  # Summary
  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Verification Summary"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo
  echo -e "Total Checks: $TOTAL_CHECKS"
  echo -e "${GREEN}Passed: $PASSED_CHECKS${NC}"
  echo -e "${RED}Failed: $FAILED_CHECKS${NC}"
  echo -e "${YELLOW}Warnings: $WARNING_CHECKS${NC}"
  echo

  if [[ $FAILED_CHECKS -gt 0 ]]; then
    echo -e "${RED}⚠ RBAC verification failed!${NC}"
    echo
    echo "To fix RBAC issues, run:"
    echo "  ./scripts/rbac/apply-rbac-fix.sh"
    echo
    echo "Or manually apply the RBAC manifest:"
    echo "  kubectl apply -f helm/templates/rbac.yaml"
    exit 1
  elif [[ $WARNING_CHECKS -gt 0 ]]; then
    echo -e "${YELLOW}⚠ RBAC verification completed with warnings${NC}"
    echo "Some optional features may not be available."
    exit 0
  else
    echo -e "${GREEN}✓ All RBAC permissions verified successfully!${NC}"
    exit 0
  fi
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
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  --no-metrics)
    CHECK_METRICS=false
    shift
    ;;
  --no-custom-resources)
    CHECK_CUSTOM_RESOURCES=false
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -n, --namespace NAME           Namespace where Right Sizer is installed (default: right-sizer-system)"
    echo "  -s, --service-account NAME     Service account name (default: right-sizer)"
    echo "  -v, --verbose                  Show detailed permission checks"
    echo "  --no-metrics                   Skip metrics API checks"
    echo "  --no-custom-resources          Skip custom resource checks"
    echo "  -h, --help                     Show this help message"
    echo
    echo "Environment Variables:"
    echo "  NAMESPACE                      Alternative to -n flag"
    echo "  SERVICE_ACCOUNT                Alternative to -s flag"
    echo "  VERBOSE                        Alternative to -v flag (true/false)"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use -h or --help for usage information"
    exit 1
    ;;
  esac
done

# Run main verification
main
