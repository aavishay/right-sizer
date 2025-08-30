#!/bin/bash

# Script to easily change the log level of the Right Sizer operator
# This updates the RightSizerConfig CRD to set the desired log level

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
CONFIG_NAME="${CONFIG_NAME:-default}"
NAMESPACE="${NAMESPACE:-}"
RESTART="${RESTART:-false}"
FOLLOW_LOGS="${FOLLOW_LOGS:-false}"
LOG_LEVEL=""

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

# Function to validate log level
validate_log_level() {
  local level=$1
  case $level in
  debug | info | warn | error)
    return 0
    ;;
  *)
    print_status "ERROR" "Invalid log level: $level"
    print_status "INFO" "Valid levels are: debug, info, warn, error"
    exit 1
    ;;
  esac
}

# Function to get current log level
get_current_log_level() {
  local current=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.spec.observabilityConfig.logLevel}' 2>/dev/null || echo "unknown")
  echo "$current"
}

# Function to set log level
set_log_level() {
  local level=$1

  print_status "INFO" "Setting log level to: $level"

  # Check if config exists
  if ! kubectl get rightsizerconfig "$CONFIG_NAME" &>/dev/null; then
    print_status "ERROR" "RightSizerConfig '$CONFIG_NAME' not found"
    print_status "INFO" "Available configs:"
    kubectl get rightsizerconfigs -A
    exit 1
  fi

  # Get current level
  local current_level=$(get_current_log_level)
  print_status "INFO" "Current log level: $current_level"

  if [[ "$current_level" == "$level" ]]; then
    print_status "WARNING" "Log level is already set to $level"
    if [[ "$RESTART" != "true" ]]; then
      exit 0
    fi
  fi

  # Patch the config
  if kubectl patch rightsizerconfig "$CONFIG_NAME" --type='merge' -p "{\"spec\":{\"observabilityConfig\":{\"logLevel\":\"$level\"}}}" &>/dev/null; then
    print_status "SUCCESS" "Log level updated to $level"
  else
    print_status "ERROR" "Failed to update log level"
    exit 1
  fi
}

# Function to find and restart the operator
restart_operator() {
  print_status "INFO" "Looking for right-sizer deployment..."

  # Try to find the deployment
  local deployment=""
  local ns=""

  # First try the specified namespace
  if [[ -n "$NAMESPACE" ]]; then
    if kubectl get deployment -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer &>/dev/null; then
      deployment=$(kubectl get deployment -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o name | head -1)
      ns="$NAMESPACE"
    fi
  fi

  # If not found, search all namespaces
  if [[ -z "$deployment" ]]; then
    local result=$(kubectl get deployment -A -l app.kubernetes.io/name=right-sizer -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers 2>/dev/null | head -1)
    if [[ -n "$result" ]]; then
      ns=$(echo "$result" | awk '{print $1}')
      deployment="deployment/$(echo "$result" | awk '{print $2}')"
    fi
  fi

  # If still not found, try common names
  if [[ -z "$deployment" ]]; then
    for common_ns in "right-sizer-system" "right-sizer" "default"; do
      if kubectl get deployment right-sizer -n "$common_ns" &>/dev/null; then
        deployment="deployment/right-sizer"
        ns="$common_ns"
        break
      fi
    done
  fi

  if [[ -z "$deployment" ]]; then
    print_status "WARNING" "Could not find right-sizer deployment"
    print_status "INFO" "The configuration has been updated, but you may need to restart the operator manually"
    return
  fi

  print_status "INFO" "Restarting $deployment in namespace $ns..."
  kubectl rollout restart "$deployment" -n "$ns"
  print_status "SUCCESS" "Restart initiated"

  # Wait for rollout to complete
  print_status "INFO" "Waiting for rollout to complete..."
  if kubectl rollout status "$deployment" -n "$ns" --timeout=60s &>/dev/null; then
    print_status "SUCCESS" "Operator restarted successfully"

    # Store namespace for log following
    NAMESPACE="$ns"
  else
    print_status "WARNING" "Rollout is taking longer than expected. Check status with:"
    echo "kubectl rollout status $deployment -n $ns"
  fi
}

# Function to follow logs
follow_logs() {
  if [[ -z "$NAMESPACE" ]]; then
    # Try to find the namespace
    local ns=$(kubectl get deployment -A -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
    if [[ -n "$ns" ]]; then
      NAMESPACE="$ns"
    else
      print_status "WARNING" "Could not find right-sizer deployment to follow logs"
      return
    fi
  fi

  print_status "INFO" "Following logs from namespace $NAMESPACE..."
  print_status "INFO" "Press Ctrl+C to stop following logs"
  echo

  kubectl logs -f -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=50
}

# Function to show current status
show_status() {
  print_status "INFO" "Checking Right Sizer log configuration..."

  # Get current log level
  local current_level=$(get_current_log_level)
  if [[ "$current_level" != "unknown" ]]; then
    print_status "SUCCESS" "Current log level: $current_level"
  else
    print_status "WARNING" "Could not determine current log level"
  fi

  # Find deployment
  local deployments=$(kubectl get deployment -A -l app.kubernetes.io/name=right-sizer -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,READY:.status.readyReplicas,REPLICAS:.spec.replicas --no-headers 2>/dev/null)
  if [[ -n "$deployments" ]]; then
    echo
    echo "Right Sizer Deployments:"
    echo "$deployments"
  else
    print_status "WARNING" "No right-sizer deployments found"
  fi

  # Show recent log summary
  echo
  print_status "INFO" "Recent log level distribution:"
  local ns=$(kubectl get deployment -A -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
  if [[ -n "$ns" ]]; then
    kubectl logs -n "$ns" -l app.kubernetes.io/name=right-sizer --tail=100 2>/dev/null |
      grep -oE '\[(DEBUG|INFO|WARN|ERROR)\]' | sort | uniq -c || echo "  No logs found"
  fi
}

# Function to show usage
usage() {
  echo "Usage: $0 [OPTIONS] <log-level>"
  echo
  echo "Set the log level for the Right Sizer operator"
  echo
  echo "Log Levels:"
  echo "  debug    - Detailed debugging information"
  echo "  info     - General informational messages (default)"
  echo "  warn     - Warning messages and errors"
  echo "  error    - Only error messages"
  echo
  echo "Options:"
  echo "  -c, --config NAME     Name of RightSizerConfig to update (default: default)"
  echo "  -n, --namespace NAME  Namespace of the operator deployment"
  echo "  -r, --restart         Restart the operator after changing log level"
  echo "  -f, --follow          Follow logs after changing log level"
  echo "  -s, --status          Show current log level and status (no changes)"
  echo "  -h, --help            Show this help message"
  echo
  echo "Examples:"
  echo "  # Set log level to warn"
  echo "  $0 warn"
  echo
  echo "  # Set to debug and restart operator"
  echo "  $0 --restart debug"
  echo
  echo "  # Set to error and follow logs"
  echo "  $0 --follow error"
  echo
  echo "  # Check current status"
  echo "  $0 --status"
  echo
  echo "Environment Variables:"
  echo "  CONFIG_NAME    Name of RightSizerConfig (default: default)"
  echo "  NAMESPACE      Namespace of the operator"
  echo "  RESTART        Auto-restart operator (true/false, default: false)"
  echo "  FOLLOW_LOGS    Follow logs after change (true/false, default: false)"
}

# Main function
main() {
  # Check prerequisites
  check_kubectl
  check_cluster

  # Validate and apply log level
  validate_log_level "$LOG_LEVEL"
  set_log_level "$LOG_LEVEL"

  # Restart if requested
  if [[ "$RESTART" == "true" ]]; then
    restart_operator
  else
    print_status "INFO" "Note: Changes will be applied on the next reconciliation (typically within 30s)"
    print_status "INFO" "Use --restart flag to apply immediately"
  fi

  # Follow logs if requested
  if [[ "$FOLLOW_LOGS" == "true" ]]; then
    echo
    follow_logs
  fi
}

# Parse command line arguments
SHOW_STATUS=false

while [[ $# -gt 0 ]]; do
  case $1 in
  -c | --config)
    CONFIG_NAME="$2"
    shift 2
    ;;
  -n | --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  -r | --restart)
    RESTART=true
    shift
    ;;
  -f | --follow)
    FOLLOW_LOGS=true
    shift
    ;;
  -s | --status)
    SHOW_STATUS=true
    shift
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  -*)
    echo "Unknown option: $1"
    usage
    exit 1
    ;;
  *)
    LOG_LEVEL="$1"
    shift
    ;;
  esac
done

# If status flag is set, just show status and exit
if [[ "$SHOW_STATUS" == "true" ]]; then
  check_kubectl
  check_cluster
  show_status
  exit 0
fi

# Check if log level was provided
if [[ -z "$LOG_LEVEL" ]]; then
  print_status "ERROR" "Log level is required"
  echo
  usage
  exit 1
fi

# Run main function
main
