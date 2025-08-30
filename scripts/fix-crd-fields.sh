#!/bin/bash

# Script to fix CRD field validation issues by replacing simplified CRDs with fully-defined ones
# This fixes "unknown field" errors in right-sizer pod logs

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CRD_DIR="${CRD_DIR:-$PROJECT_ROOT/helm/crds}"
NAMESPACE="${NAMESPACE:-right-sizer-system}"
DRY_RUN="${DRY_RUN:-false}"

# Function to print colored output
print_status() {
  local status=$1
  local message=$2

  case $status in
  "SUCCESS")
    echo -e "${GREEN}✓${NC} $message"
    ;;
  "ERROR")
    echo -e "${RED}✗${NC} $message"
    ;;
  "WARNING")
    echo -e "${YELLOW}⚠${NC} $message"
    ;;
  "INFO")
    echo -e "${BLUE}ℹ${NC} $message"
    ;;
  esac
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

# Function to backup existing CRDs
backup_crds() {
  print_status "INFO" "Backing up existing CRDs..."

  local backup_dir="/tmp/right-sizer-crd-backup-$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$backup_dir"

  # Backup RightSizerConfig CRD
  if kubectl get crd rightsizerconfigs.rightsizer.io &>/dev/null; then
    kubectl get crd rightsizerconfigs.rightsizer.io -o yaml >"$backup_dir/rightsizerconfigs.yaml"
    print_status "SUCCESS" "Backed up rightsizerconfigs.rightsizer.io to $backup_dir"
  fi

  # Backup RightSizerPolicy CRD
  if kubectl get crd rightsizerpolicies.rightsizer.io &>/dev/null; then
    kubectl get crd rightsizerpolicies.rightsizer.io -o yaml >"$backup_dir/rightsizerpolicies.yaml"
    print_status "SUCCESS" "Backed up rightsizerpolicies.rightsizer.io to $backup_dir"
  fi

  # Backup existing custom resources
  if kubectl get rightsizerconfigs -A &>/dev/null; then
    kubectl get rightsizerconfigs -A -o yaml >"$backup_dir/rightsizerconfigs-resources.yaml"
    print_status "SUCCESS" "Backed up RightSizerConfig resources to $backup_dir"
  fi

  if kubectl get rightsizerpolicies -A &>/dev/null; then
    kubectl get rightsizerpolicies -A -o yaml >"$backup_dir/rightsizerpolicies-resources.yaml"
    print_status "SUCCESS" "Backed up RightSizerPolicy resources to $backup_dir"
  fi

  echo "$backup_dir"
}

# Function to check current CRD version
check_crd_version() {
  print_status "INFO" "Checking current CRD definitions..."

  local has_issues=false

  # Check if CRDs use x-kubernetes-preserve-unknown-fields
  if kubectl get crd rightsizerconfigs.rightsizer.io -o yaml | grep -q "x-kubernetes-preserve-unknown-fields: true"; then
    print_status "WARNING" "RightSizerConfig CRD uses simplified schema (causes field validation issues)"
    has_issues=true
  fi

  if kubectl get crd rightsizerpolicies.rightsizer.io -o yaml | grep -q "x-kubernetes-preserve-unknown-fields: true"; then
    print_status "WARNING" "RightSizerPolicy CRD uses simplified schema (causes field validation issues)"
    has_issues=true
  fi

  if [ "$has_issues" = true ]; then
    return 1
  else
    print_status "SUCCESS" "CRDs appear to be using the correct schema"
    return 0
  fi
}

# Function to apply the correct CRDs
fix_crds() {
  print_status "INFO" "Applying correct CRD definitions..."

  # Check if files exist
  local config_crd="$CRD_DIR/rightsizer.io_rightsizerconfigs.yaml"
  local policy_crd="$CRD_DIR/rightsizer.io_rightsizerpolicies.yaml"

  if [[ ! -f "$config_crd" ]]; then
    print_status "ERROR" "CRD file not found: $config_crd"
    exit 1
  fi

  if [[ ! -f "$policy_crd" ]]; then
    print_status "ERROR" "CRD file not found: $policy_crd"
    exit 1
  fi

  # Apply the CRDs
  local cmd_prefix=""
  if [[ "$DRY_RUN" == "true" ]]; then
    cmd_prefix="echo [DRY-RUN] Would execute: "
    print_status "INFO" "Running in dry-run mode"
  fi

  print_status "INFO" "Replacing RightSizerConfig CRD..."
  if $cmd_prefix kubectl apply -f "$config_crd"; then
    print_status "SUCCESS" "RightSizerConfig CRD updated"
  else
    print_status "ERROR" "Failed to update RightSizerConfig CRD"
    exit 1
  fi

  print_status "INFO" "Replacing RightSizerPolicy CRD..."
  if $cmd_prefix kubectl apply -f "$policy_crd"; then
    print_status "SUCCESS" "RightSizerPolicy CRD updated"
  else
    print_status "ERROR" "Failed to update RightSizerPolicy CRD"
    exit 1
  fi

  # Wait for CRDs to be established
  if [[ "$DRY_RUN" != "true" ]]; then
    print_status "INFO" "Waiting for CRDs to be established..."
    kubectl wait --for=condition=Established --timeout=60s \
      crd/rightsizerconfigs.rightsizer.io \
      crd/rightsizerpolicies.rightsizer.io
    print_status "SUCCESS" "CRDs established successfully"
  fi
}

# Function to restart the operator
restart_operator() {
  if [[ "$DRY_RUN" == "true" ]]; then
    print_status "INFO" "[DRY-RUN] Would restart the right-sizer operator"
    return
  fi

  print_status "INFO" "Restarting right-sizer operator to apply changes..."

  # Find the deployment
  local deployment=$(kubectl get deployment -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o name 2>/dev/null | head -1)

  if [[ -z "$deployment" ]]; then
    print_status "WARNING" "Could not find right-sizer deployment in namespace $NAMESPACE"
    print_status "INFO" "Please manually restart the operator after CRD update"
    return
  fi

  # Restart the deployment
  kubectl rollout restart "$deployment" -n "$NAMESPACE"
  print_status "SUCCESS" "Operator restart initiated"

  # Wait for rollout to complete
  print_status "INFO" "Waiting for operator rollout to complete..."
  kubectl rollout status "$deployment" -n "$NAMESPACE" --timeout=5m
  print_status "SUCCESS" "Operator restarted successfully"
}

# Function to verify the fix
verify_fix() {
  if [[ "$DRY_RUN" == "true" ]]; then
    return
  fi

  print_status "INFO" "Verifying CRD fix..."

  # Check if operator logs still show unknown field errors
  sleep 5 # Give operator time to start

  local pod=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o name 2>/dev/null | head -1)

  if [[ -z "$pod" ]]; then
    print_status "WARNING" "Could not find right-sizer pod for verification"
    return
  fi

  # Check for unknown field errors in recent logs
  if kubectl logs "$pod" -n "$NAMESPACE" --tail=50 2>/dev/null | grep -q "unknown field"; then
    print_status "WARNING" "Still seeing 'unknown field' errors in operator logs"
    print_status "INFO" "This might be due to cached data. Wait a moment and check again."
  else
    print_status "SUCCESS" "No 'unknown field' errors found in recent operator logs"
  fi
}

# Main function
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Right Sizer CRD Field Validation Fix"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo
  print_status "INFO" "This script fixes 'unknown field' errors in right-sizer pod logs"
  echo

  # Check prerequisites
  check_kubectl
  check_cluster

  # Check current CRD version
  if check_crd_version; then
    print_status "INFO" "CRDs appear to be correct. If you're still seeing issues, force update with --force"
    if [[ "${FORCE:-false}" != "true" ]]; then
      exit 0
    fi
  fi

  # Backup existing CRDs
  if [[ "$DRY_RUN" != "true" ]]; then
    local backup_dir=$(backup_crds)
    print_status "INFO" "Backups saved to: $backup_dir"
  fi

  # Apply the correct CRDs
  fix_crds

  # Restart operator
  restart_operator

  # Verify the fix
  verify_fix

  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_status "INFO" "Dry-run completed. Run without --dry-run to apply fixes."
  else
    print_status "SUCCESS" "CRD field validation issues fixed!"
    echo
    echo "Next steps:"
    echo "1. Monitor operator logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"
    echo "2. Check CRD status: kubectl get crd | grep rightsizer"
    echo "3. Verify resources: kubectl get rightsizerconfigs,rightsizerpolicies -A"
  fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --namespace | -n)
    NAMESPACE="$2"
    shift 2
    ;;
  --crd-dir)
    CRD_DIR="$2"
    shift 2
    ;;
  --dry-run)
    DRY_RUN=true
    shift
    ;;
  --force)
    FORCE=true
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Fix Right Sizer CRD field validation issues"
    echo
    echo "Options:"
    echo "  -n, --namespace NAME  Namespace where right-sizer is installed (default: right-sizer-system)"
    echo "  --crd-dir PATH        Path to directory containing correct CRD files"
    echo "  --dry-run             Show what would be done without making changes"
    echo "  --force               Force update even if CRDs appear correct"
    echo "  -h, --help            Show this help message"
    echo
    echo "Examples:"
    echo "  # Fix CRDs with defaults"
    echo "  $0"
    echo
    echo "  # Dry run to see what would be done"
    echo "  $0 --dry-run"
    echo
    echo "  # Fix CRDs in custom namespace"
    echo "  $0 --namespace my-namespace"
    echo
    echo "  # Force update CRDs"
    echo "  $0 --force"
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
