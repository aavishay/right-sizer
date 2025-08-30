#!/bin/bash

# Right Sizer CRDs Installation Script
# This script installs the Custom Resource Definitions required by the Right Sizer operator

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
CRD_PATH="${CRD_PATH:-$PROJECT_ROOT/helm/crds/rightsizer-crds.yaml}"
DRY_RUN="${DRY_RUN:-false}"
WAIT="${WAIT:-true}"
TIMEOUT="${TIMEOUT:-60s}"

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

# Function to check if CRDs already exist
check_existing_crds() {
  local crds=("rightsizerpolicies.rightsizer.io" "rightsizerconfigs.rightsizer.io")
  local existing=()

  for crd in "${crds[@]}"; do
    if kubectl get crd "$crd" &>/dev/null; then
      existing+=("$crd")
    fi
  done

  if [ ${#existing[@]} -gt 0 ]; then
    print_status "WARNING" "Found existing CRDs: ${existing[*]}"
    return 0
  else
    print_status "INFO" "No existing Right Sizer CRDs found"
    return 1
  fi
}

# Function to install CRDs
install_crds() {
  print_status "INFO" "Installing Right Sizer CRDs from: $CRD_PATH"

  if [[ ! -f "$CRD_PATH" ]]; then
    print_status "ERROR" "CRD file not found at: $CRD_PATH"
    exit 1
  fi

  local cmd="kubectl apply -f $CRD_PATH"

  if [[ "$DRY_RUN" == "true" ]]; then
    cmd="$cmd --dry-run=client"
    print_status "INFO" "Running in dry-run mode"
  fi

  if $cmd; then
    if [[ "$DRY_RUN" != "true" ]]; then
      print_status "SUCCESS" "CRDs installed successfully"
    else
      print_status "INFO" "Dry-run completed successfully"
    fi
  else
    print_status "ERROR" "Failed to install CRDs"
    exit 1
  fi
}

# Function to wait for CRDs to be established
wait_for_crds() {
  if [[ "$DRY_RUN" == "true" ]] || [[ "$WAIT" != "true" ]]; then
    return 0
  fi

  print_status "INFO" "Waiting for CRDs to be established..."

  local crds=("rightsizerpolicies.rightsizer.io" "rightsizerconfigs.rightsizer.io")

  for crd in "${crds[@]}"; do
    if kubectl wait --for=condition=Established --timeout="$TIMEOUT" "crd/$crd" &>/dev/null; then
      print_status "SUCCESS" "CRD established: $crd"
    else
      print_status "ERROR" "Timeout waiting for CRD: $crd"
      exit 1
    fi
  done
}

# Function to verify CRDs
verify_crds() {
  if [[ "$DRY_RUN" == "true" ]]; then
    return 0
  fi

  print_status "INFO" "Verifying CRD installation..."

  # Check RightSizerPolicy CRD
  if kubectl get crd rightsizerpolicies.rightsizer.io &>/dev/null; then
    print_status "SUCCESS" "RightSizerPolicy CRD verified"

    # Show short names
    local shortnames=$(kubectl get crd rightsizerpolicies.rightsizer.io -o jsonpath='{.spec.names.shortNames[*]}')
    print_status "INFO" "  Short names: $shortnames"

    # Show API versions
    local versions=$(kubectl get crd rightsizerpolicies.rightsizer.io -o jsonpath='{.spec.versions[*].name}')
    print_status "INFO" "  API versions: $versions"
  else
    print_status "ERROR" "RightSizerPolicy CRD not found"
    exit 1
  fi

  # Check RightSizerConfig CRD
  if kubectl get crd rightsizerconfigs.rightsizer.io &>/dev/null; then
    print_status "SUCCESS" "RightSizerConfig CRD verified"

    # Show short names
    local shortnames=$(kubectl get crd rightsizerconfigs.rightsizer.io -o jsonpath='{.spec.names.shortNames[*]}')
    print_status "INFO" "  Short names: $shortnames"

    # Show API versions
    local versions=$(kubectl get crd rightsizerconfigs.rightsizer.io -o jsonpath='{.spec.versions[*].name}')
    print_status "INFO" "  API versions: $versions"
  else
    print_status "ERROR" "RightSizerConfig CRD not found"
    exit 1
  fi
}

# Function to show usage examples
show_usage_examples() {
  echo
  echo "CRDs installed successfully! You can now:"
  echo
  echo "1. Create a global configuration:"
  echo "   kubectl apply -f examples/policy-rules-example.yaml"
  echo
  echo "2. List policies:"
  echo "   kubectl get rightsizerpolicies"
  echo "   kubectl get rsp  # Using short name"
  echo
  echo "3. View configuration:"
  echo "   kubectl get rightsizerconfig"
  echo "   kubectl get rsc  # Using short name"
  echo
  echo "4. Deploy the Right Sizer operator:"
  echo "   helm install right-sizer ./helm"
}

# Main function
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Right Sizer CRDs Installation"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo

  # Check prerequisites
  check_kubectl
  check_cluster

  # Check for existing CRDs
  check_existing_crds || true

  # Install CRDs
  install_crds

  # Wait for CRDs to be established
  wait_for_crds

  # Verify installation
  verify_crds

  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "Dry-run completed. Run without --dry-run to install CRDs."
  else
    show_usage_examples
  fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --crd-path)
    CRD_PATH="$2"
    shift 2
    ;;
  --dry-run)
    DRY_RUN=true
    shift
    ;;
  --no-wait)
    WAIT=false
    shift
    ;;
  --timeout)
    TIMEOUT="$2"
    shift 2
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --crd-path PATH    Path to CRD definitions file"
    echo "  --dry-run          Simulate installation without applying changes"
    echo "  --no-wait          Don't wait for CRDs to be established"
    echo "  --timeout DURATION Timeout for waiting (default: 60s)"
    echo "  -h, --help         Show this help message"
    echo
    echo "Environment Variables:"
    echo "  CRD_PATH           Alternative to --crd-path flag"
    echo "  DRY_RUN            Alternative to --dry-run flag (true/false)"
    echo "  WAIT               Alternative to --no-wait flag (true/false)"
    echo "  TIMEOUT            Alternative to --timeout flag"
    echo
    echo "Examples:"
    echo "  # Install CRDs with defaults"
    echo "  $0"
    echo
    echo "  # Dry run to see what would be installed"
    echo "  $0 --dry-run"
    echo
    echo "  # Install from custom path"
    echo "  $0 --crd-path /path/to/crds.yaml"
    echo
    echo "  # Install without waiting"
    echo "  $0 --no-wait"
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
