#!/bin/bash

# Right-Sizer Monitoring Script
# This script provides real-time monitoring of the right-sizer operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Configuration
NAMESPACE="${NAMESPACE:-right-sizer-system}"
APP_LABEL="app.kubernetes.io/name=right-sizer"
REFRESH_INTERVAL="${REFRESH_INTERVAL:-10}"
WATCH_MODE="${WATCH_MODE:-false}"

# Function to print colored output
print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

# Function to print section headers
print_header() {
  echo ""
  print_color "$CYAN" "========================================="
  print_color "$CYAN" "$1"
  print_color "$CYAN" "========================================="
}

# Function to check if kubectl is available
check_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    print_color "$RED" "Error: kubectl is not installed or not in PATH"
    exit 1
  fi
}

# Function to check if namespace exists
check_namespace() {
  if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    print_color "$RED" "Error: Namespace '$NAMESPACE' does not exist"
    exit 1
  fi
}

# Function to get pod status
get_pod_status() {
  print_header "📦 Right-Sizer Pod Status"

  local pod_info=$(kubectl get pods -n "$NAMESPACE" -l "$APP_LABEL" -o wide 2>/dev/null)

  if [ -z "$pod_info" ]; then
    print_color "$RED" "No right-sizer pods found in namespace $NAMESPACE"
    return 1
  fi

  echo "$pod_info"

  # Get pod details
  local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "$APP_LABEL" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

  if [ -n "$pod_name" ]; then
    echo ""
    print_color "$BLUE" "Pod Details for: $pod_name"
    kubectl describe pod -n "$NAMESPACE" "$pod_name" | grep -E "Status:|Ready:|Restart Count:|Started:" || true
  fi
}

# Function to check recent logs
get_recent_logs() {
  print_header "📝 Recent Logs (Last 20 lines)"

  kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=20 2>/dev/null || {
    print_color "$RED" "Failed to fetch logs"
    return 1
  }
}

# Function to check for errors in logs
check_errors() {
  print_header "❌ Recent Errors and Warnings"

  local errors=$(kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=100 2>/dev/null | grep -E "ERROR|WARN|Failed|Error|error|failed" | tail -10)

  if [ -z "$errors" ]; then
    print_color "$GREEN" "✅ No recent errors or warnings found"
  else
    print_color "$YELLOW" "$errors"
  fi
}

# Function to show resizing activity
show_resizing_activity() {
  print_header "🔄 Resizing Activity"

  echo "Recent resizing operations:"
  kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=100 2>/dev/null | grep -E "resized|Resized|Right-sizing complete" | tail -10 || {
    print_color "$YELLOW" "No recent resizing activity found"
  }
}

# Function to show resource changes
show_resource_changes() {
  print_header "📊 Resource Change Events"

  # Check events in all namespaces for resize events
  echo "Recent resize events across all namespaces:"
  kubectl get events --all-namespaces --field-selector reason=Resized --sort-by='.lastTimestamp' 2>/dev/null | tail -10 || {
    print_color "$YELLOW" "No resize events found"
  }
}

# Function to show monitored pods
show_monitored_pods() {
  print_header "👁️  Monitored Pods"

  # Get the namespace filter from logs
  local include_ns=$(kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=100 2>/dev/null | grep "KUBE_NAMESPACE_INCLUDE" | tail -1 | cut -d':' -f2 | tr -d ' []')
  local exclude_ns=$(kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=100 2>/dev/null | grep "KUBE_NAMESPACE_EXCLUDE" | tail -1 | cut -d':' -f2 | tr -d ' []')

  if [ -n "$include_ns" ]; then
    print_color "$BLUE" "Monitoring namespace: $include_ns"
    echo ""
    kubectl get pods -n "$include_ns" -o wide | head -10
  else
    print_color "$YELLOW" "Could not determine monitored namespaces"
  fi

  if [ -n "$exclude_ns" ]; then
    echo ""
    print_color "$YELLOW" "Excluded namespace: $exclude_ns"
  fi
}

# Function to show operator configuration
show_configuration() {
  print_header "⚙️  Operator Configuration"

  kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=200 2>/dev/null | grep -A 20 "Configuration Loaded:" | head -25 || {
    print_color "$YELLOW" "Could not retrieve configuration"
  }
}

# Function to check metrics endpoint
check_metrics() {
  print_header "📈 Metrics Endpoint"

  local pod_name=$(kubectl get pods -n "$NAMESPACE" -l "$APP_LABEL" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

  if [ -n "$pod_name" ]; then
    # Check if metrics port is accessible
    kubectl get pod -n "$NAMESPACE" "$pod_name" -o jsonpath='{.spec.containers[0].ports[?(@.name=="metrics")].containerPort}' &>/dev/null && {
      print_color "$GREEN" "✅ Metrics endpoint is configured on port 8080"
      echo ""
      echo "To access metrics, run:"
      echo "  kubectl port-forward -n $NAMESPACE $pod_name 8080:8080"
      echo "  Then visit: http://localhost:8080/metrics"
    } || {
      print_color "$YELLOW" "Metrics endpoint not configured"
    }
  fi
}

# Function to show summary statistics
show_summary() {
  print_header "📊 Summary Statistics"

  local last_run=$(kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=50 2>/dev/null | grep "Right-sizing complete:" | tail -1)

  if [ -n "$last_run" ]; then
    print_color "$GREEN" "Last run: $last_run"
  else
    print_color "$YELLOW" "No recent right-sizing runs found"
  fi

  echo ""
  echo "Total pods in monitored namespaces:"
  local include_ns=$(kubectl logs -n "$NAMESPACE" -l "$APP_LABEL" --tail=100 2>/dev/null | grep "KUBE_NAMESPACE_INCLUDE" | tail -1 | cut -d':' -f2 | tr -d ' []')
  if [ -n "$include_ns" ]; then
    local pod_count=$(kubectl get pods -n "$include_ns" --no-headers 2>/dev/null | wc -l)
    print_color "$BLUE" "  $include_ns: $pod_count pods"
  fi
}

# Function to show help
show_help() {
  echo "Right-Sizer Monitoring Script"
  echo ""
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  -n, --namespace NAMESPACE    Specify the namespace (default: right-sizer-system)"
  echo "  -w, --watch                  Enable watch mode with auto-refresh"
  echo "  -i, --interval SECONDS       Refresh interval in watch mode (default: 10)"
  echo "  -s, --summary                Show summary only"
  echo "  -e, --errors                 Show errors only"
  echo "  -l, --logs                   Show logs only"
  echo "  -h, --help                   Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  NAMESPACE                    Override default namespace"
  echo "  REFRESH_INTERVAL            Set refresh interval for watch mode"
  echo ""
  echo "Examples:"
  echo "  $0                          # Run full monitoring check"
  echo "  $0 -w                       # Watch mode with auto-refresh"
  echo "  $0 -n my-namespace          # Monitor in specific namespace"
  echo "  $0 -s                       # Show summary only"
}

# Function to run full monitoring
run_full_monitoring() {
  clear
  print_color "$BOLD$GREEN" "🚀 Right-Sizer Operator Monitor"
  print_color "$BLUE" "Namespace: $NAMESPACE"
  print_color "$BLUE" "Timestamp: $(date)"

  get_pod_status
  show_configuration
  show_summary
  show_resizing_activity
  check_errors
  show_resource_changes
  show_monitored_pods
  check_metrics
  get_recent_logs
}

# Function to run in watch mode
run_watch_mode() {
  while true; do
    run_full_monitoring
    echo ""
    print_color "$YELLOW" "Refreshing in $REFRESH_INTERVAL seconds... (Press Ctrl+C to exit)"
    sleep "$REFRESH_INTERVAL"
  done
}

# Main execution
main() {
  check_kubectl

  # Parse command line arguments
  while [[ $# -gt 0 ]]; do
    case $1 in
    -n | --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    -w | --watch)
      WATCH_MODE="true"
      shift
      ;;
    -i | --interval)
      REFRESH_INTERVAL="$2"
      shift 2
      ;;
    -s | --summary)
      check_namespace
      show_summary
      exit 0
      ;;
    -e | --errors)
      check_namespace
      check_errors
      exit 0
      ;;
    -l | --logs)
      check_namespace
      get_recent_logs
      exit 0
      ;;
    -h | --help)
      show_help
      exit 0
      ;;
    *)
      print_color "$RED" "Unknown option: $1"
      show_help
      exit 1
      ;;
    esac
  done

  check_namespace

  if [ "$WATCH_MODE" = "true" ]; then
    run_watch_mode
  else
    run_full_monitoring
  fi
}

# Run main function
main "$@"
