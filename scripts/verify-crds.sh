#!/bin/bash

# Script to verify Right Sizer CRDs are correctly installed
# This checks for proper field definitions and validates there are no field validation issues

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="${NAMESPACE:-right-sizer-system}"
VERBOSE="${VERBOSE:-false}"
FIX_ISSUES="${FIX_ISSUES:-false}"

# Counters
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# Function to print colored output
print_status() {
  local status=$1
  local message=$2

  case $status in
  "SUCCESS")
    echo -e "${GREEN}✓${NC} $message"
    ((PASSED_CHECKS++))
    ;;
  "ERROR")
    echo -e "${RED}✗${NC} $message"
    ((FAILED_CHECKS++))
    ;;
  "WARNING")
    echo -e "${YELLOW}⚠${NC} $message"
    ((WARNING_CHECKS++))
    ;;
  "INFO")
    echo -e "${BLUE}ℹ${NC} $message"
    ;;
  "CHECK")
    echo -e "${CYAN}◆${NC} $message"
    ;;
  esac
  ((TOTAL_CHECKS++))
}

# Function to check if kubectl is available
check_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    print_status "ERROR" "kubectl is not installed or not in PATH"
    exit 1
  fi
}

# Function to check cluster connectivity
check_cluster() {
  if ! kubectl cluster-info &>/dev/null; then
    print_status "ERROR" "Cannot connect to Kubernetes cluster"
    exit 1
  fi
}

# Function to check if CRDs exist
check_crds_exist() {
  print_status "CHECK" "Checking if CRDs are installed..."

  local config_crd_exists=false
  local policy_crd_exists=false

  if kubectl get crd rightsizerconfigs.rightsizer.io &>/dev/null; then
    config_crd_exists=true
    print_status "SUCCESS" "RightSizerConfig CRD is installed"
  else
    print_status "ERROR" "RightSizerConfig CRD is not installed"
  fi

  if kubectl get crd rightsizerpolicies.rightsizer.io &>/dev/null; then
    policy_crd_exists=true
    print_status "SUCCESS" "RightSizerPolicy CRD is installed"
  else
    print_status "ERROR" "RightSizerPolicy CRD is not installed"
  fi

  if [ "$config_crd_exists" = true ] && [ "$policy_crd_exists" = true ]; then
    return 0
  else
    return 1
  fi
}

# Function to check for simplified CRD schema
check_crd_schema() {
  print_status "CHECK" "Checking CRD schema definitions..."

  local has_issues=false

  # Check RightSizerConfig CRD
  if kubectl get crd rightsizerconfigs.rightsizer.io -o yaml 2>/dev/null | grep -q "x-kubernetes-preserve-unknown-fields: true"; then
    print_status "ERROR" "RightSizerConfig CRD uses simplified schema (causes field validation issues)"
    has_issues=true
  else
    print_status "SUCCESS" "RightSizerConfig CRD has proper field definitions"
  fi

  # Check RightSizerPolicy CRD
  if kubectl get crd rightsizerpolicies.rightsizer.io -o yaml 2>/dev/null | grep -q "x-kubernetes-preserve-unknown-fields: true"; then
    print_status "ERROR" "RightSizerPolicy CRD uses simplified schema (causes field validation issues)"
    has_issues=true
  else
    print_status "SUCCESS" "RightSizerPolicy CRD has proper field definitions"
  fi

  if [ "$has_issues" = true ]; then
    return 1
  else
    return 0
  fi
}

# Function to verify specific fields are accessible
check_field_definitions() {
  print_status "CHECK" "Verifying field definitions are accessible..."

  local all_fields_ok=true

  # List of important fields to check
  local fields=(
    "rightsizerconfig.spec.defaultResourceStrategy"
    "rightsizerconfig.spec.globalConstraints"
    "rightsizerconfig.spec.metricsConfig"
    "rightsizerconfig.spec.namespaceConfig"
    "rightsizerconfig.spec.notificationConfig"
    "rightsizerconfig.spec.observabilityConfig"
    "rightsizerconfig.spec.operatorConfig"
    "rightsizerconfig.spec.securityConfig"
    "rightsizerconfig.status.systemHealth"
    "rightsizerpolicy.spec.targetRef"
    "rightsizerpolicy.spec.resourceStrategy"
    "rightsizerpolicy.spec.constraints"
  )

  for field in "${fields[@]}"; do
    if kubectl explain "$field" &>/dev/null; then
      if [[ "$VERBOSE" == "true" ]]; then
        print_status "SUCCESS" "Field accessible: $field"
      fi
    else
      print_status "ERROR" "Field not accessible: $field"
      all_fields_ok=false
    fi
  done

  if [ "$all_fields_ok" = true ]; then
    print_status "SUCCESS" "All critical fields are properly defined"
    return 0
  else
    return 1
  fi
}

# Function to check CRD versions
check_crd_versions() {
  print_status "CHECK" "Checking CRD versions..."

  # Check controller-gen version
  local config_version=$(kubectl get crd rightsizerconfigs.rightsizer.io -o jsonpath='{.metadata.annotations.controller-gen\.kubebuilder\.io/version}' 2>/dev/null || echo "unknown")
  local policy_version=$(kubectl get crd rightsizerpolicies.rightsizer.io -o jsonpath='{.metadata.annotations.controller-gen\.kubebuilder\.io/version}' 2>/dev/null || echo "unknown")

  if [[ "$config_version" == "unknown" ]]; then
    print_status "WARNING" "RightSizerConfig CRD version unknown (might be manually created)"
  else
    print_status "SUCCESS" "RightSizerConfig CRD version: $config_version"
  fi

  if [[ "$policy_version" == "unknown" ]]; then
    print_status "WARNING" "RightSizerPolicy CRD version unknown (might be manually created)"
  else
    print_status "SUCCESS" "RightSizerPolicy CRD version: $policy_version"
  fi
}

# Function to check operator logs for field errors
check_operator_logs() {
  print_status "CHECK" "Checking operator logs for field validation errors..."

  local pod=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o name 2>/dev/null | head -1)

  if [[ -z "$pod" ]]; then
    print_status "WARNING" "Right-sizer operator not found in namespace $NAMESPACE"
    return 0
  fi

  # Check for unknown field errors in logs
  local error_count=$(kubectl logs "$pod" -n "$NAMESPACE" --tail=100 2>/dev/null | grep -c "unknown field" || true)

  if [[ "$error_count" -gt 0 ]]; then
    print_status "ERROR" "Found $error_count 'unknown field' errors in operator logs"

    if [[ "$VERBOSE" == "true" ]]; then
      echo "Recent errors:"
      kubectl logs "$pod" -n "$NAMESPACE" --tail=100 2>/dev/null | grep "unknown field" | head -5
    fi

    return 1
  else
    print_status "SUCCESS" "No field validation errors in operator logs"
    return 0
  fi
}

# Function to check CRD resources
check_crd_resources() {
  print_status "CHECK" "Checking for CRD resources..."

  local config_count=$(kubectl get rightsizerconfigs -A --no-headers 2>/dev/null | wc -l || echo "0")
  local policy_count=$(kubectl get rightsizerpolicies -A --no-headers 2>/dev/null | wc -l || echo "0")

  if [[ "$config_count" -gt 0 ]]; then
    print_status "SUCCESS" "Found $config_count RightSizerConfig resources"
  else
    print_status "INFO" "No RightSizerConfig resources found"
  fi

  if [[ "$policy_count" -gt 0 ]]; then
    print_status "SUCCESS" "Found $policy_count RightSizerPolicy resources"
  else
    print_status "INFO" "No RightSizerPolicy resources found"
  fi
}

# Function to suggest fixes
suggest_fixes() {
  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Suggested Fixes"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if [[ "$FAILED_CHECKS" -gt 0 ]]; then
    echo
    echo "To fix the issues found, you can:"
    echo
    echo "1. Run the automated fix script:"
    echo "   ./scripts/fix-crd-fields.sh"
    echo
    echo "2. Or manually reinstall CRDs:"
    echo "   kubectl delete crd rightsizerconfigs.rightsizer.io rightsizerpolicies.rightsizer.io"
    echo "   ./scripts/install-crds.sh"
    echo
    echo "3. Then restart the operator:"
    echo "   kubectl rollout restart deployment/right-sizer -n $NAMESPACE"

    if [[ "$FIX_ISSUES" == "true" ]]; then
      echo
      echo "Attempting to fix issues automatically..."
      if [[ -f "./scripts/fix-crd-fields.sh" ]]; then
        ./scripts/fix-crd-fields.sh
      else
        echo "Fix script not found. Please fix manually."
      fi
    fi
  else
    echo
    echo "✅ Your CRDs are correctly configured!"
    echo
    echo "No action required. The Right Sizer operator should work without field validation issues."
  fi
}

# Function to print summary
print_summary() {
  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Verification Summary"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo
  echo "Total checks performed: $TOTAL_CHECKS"
  echo -e "${GREEN}Passed:${NC} $PASSED_CHECKS"

  if [[ "$WARNING_CHECKS" -gt 0 ]]; then
    echo -e "${YELLOW}Warnings:${NC} $WARNING_CHECKS"
  fi

  if [[ "$FAILED_CHECKS" -gt 0 ]]; then
    echo -e "${RED}Failed:${NC} $FAILED_CHECKS"
  fi

  echo

  if [[ "$FAILED_CHECKS" -eq 0 ]]; then
    echo -e "${GREEN}✓ CRD verification passed!${NC}"
    return 0
  else
    echo -e "${RED}✗ CRD verification failed!${NC}"
    return 1
  fi
}

# Main function
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Right Sizer CRD Verification"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo

  # Check prerequisites
  check_kubectl
  check_cluster

  echo

  # Run verification checks
  check_crds_exist || true
  check_crd_schema || true
  check_field_definitions || true
  check_crd_versions || true
  check_operator_logs || true
  check_crd_resources || true

  # Print summary
  local exit_code=0
  print_summary || exit_code=$?

  # Suggest fixes if needed
  if [[ "$exit_code" -ne 0 ]]; then
    suggest_fixes
  fi

  exit $exit_code
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --namespace | -n)
    NAMESPACE="$2"
    shift 2
    ;;
  --verbose | -v)
    VERBOSE=true
    shift
    ;;
  --fix)
    FIX_ISSUES=true
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Verify Right Sizer CRDs are correctly installed and configured"
    echo
    echo "Options:"
    echo "  -n, --namespace NAME  Namespace where right-sizer is installed (default: right-sizer-system)"
    echo "  -v, --verbose         Show detailed output for all checks"
    echo "  --fix                 Attempt to fix issues automatically if found"
    echo "  -h, --help            Show this help message"
    echo
    echo "Examples:"
    echo "  # Basic verification"
    echo "  $0"
    echo
    echo "  # Verbose verification"
    echo "  $0 --verbose"
    echo
    echo "  # Check in custom namespace"
    echo "  $0 --namespace my-namespace"
    echo
    echo "  # Verify and fix if needed"
    echo "  $0 --fix"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use -h or --help for usage information"
    exit 1
    ;;
  esac
done

# Run main function
main
