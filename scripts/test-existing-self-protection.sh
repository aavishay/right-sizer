#!/bin/bash

# Test Self-Protection on Existing Right-Sizer Deployment
# This script monitors an existing right-sizer deployment to verify self-protection works

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="right-sizer"
TEST_TIMEOUT=300 # 5 minutes
LOG_FILE="/tmp/right-sizer-existing-test.log"

# Function to print colored output
print_color() {
  printf "${1}${2}${NC}\n"
}

print_header() {
  echo "========================================"
  print_color $CYAN "$1"
  echo "========================================"
}

print_section() {
  echo "----------------------------------------"
  print_color $BLUE "$1"
  echo "----------------------------------------"
}

print_success() {
  print_color $GREEN "✅ $1"
}

print_warning() {
  print_color $YELLOW "⚠️  $1"
}

print_error() {
  print_color $RED "❌ $1"
}

print_info() {
  print_color $PURPLE "ℹ️  $1"
}

# Function to check if right-sizer is deployed
check_deployment() {
  print_header "Checking Existing Right-Sizer Deployment"

  # Check if namespace exists
  if ! kubectl get namespace $NAMESPACE >/dev/null 2>&1; then
    print_error "Namespace '$NAMESPACE' not found"
    print_info "Please deploy right-sizer first: helm install right-sizer ./helm --namespace $NAMESPACE --create-namespace"
    exit 1
  fi

  # Check if pods are running
  local running_pods=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
  if [ "$running_pods" -eq 0 ]; then
    print_error "No running right-sizer pods found in namespace '$NAMESPACE'"
    kubectl get pods -n $NAMESPACE
    exit 1
  fi

  print_success "Found $running_pods running right-sizer pod(s)"
  kubectl get pods -n $NAMESPACE -o wide

  # Get the first running pod that's actually ready
  local pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[?(@.status.containerStatuses[0].ready==true)].metadata.name}' 2>/dev/null | head -1)
  if [ -z "$pod_name" ]; then
    # Fallback to any running pod
    pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  fi
  if [ -z "$pod_name" ]; then
    print_error "Could not get running pod name"
    exit 1
  fi

  print_info "Will monitor pod: $pod_name"
  echo "$pod_name"
}

# Function to analyze current pod resources
analyze_current_resources() {
  print_header "Analyzing Current Pod Resources"

  local pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[?(@.status.containerStatuses[0].ready==true)].metadata.name}' 2>/dev/null | head -1)
  if [ -z "$pod_name" ]; then
    pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}')
  fi

  print_info "Current right-sizer pod resources:"
  local cpu_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}' 2>/dev/null || echo "not set")
  local memory_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}' 2>/dev/null || echo "not set")
  local cpu_limit=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.cpu}' 2>/dev/null || echo "not set")
  local memory_limit=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.memory}' 2>/dev/null || echo "not set")

  echo "  CPU Request: $cpu_request"
  echo "  Memory Request: $memory_request"
  echo "  CPU Limit: $cpu_limit"
  echo "  Memory Limit: $memory_limit"

  # Store initial values for comparison later
  echo "$cpu_request|$memory_request|$cpu_limit|$memory_limit" >/tmp/initial_resources
}

# Function to check configuration
check_configuration() {
  print_header "Checking Self-Protection Configuration"

  # Check if RightSizerConfig exists
  if ! kubectl get rightsizerconfig -n $NAMESPACE >/dev/null 2>&1; then
    print_warning "No RightSizerConfig found"
    return 1
  fi

  # Check namespace exclusion
  local excluded_namespaces=$(kubectl get rightsizerconfig -n $NAMESPACE -o jsonpath='{.items[0].spec.namespaceConfig.excludeNamespaces}' 2>/dev/null || echo "[]")
  print_info "Excluded namespaces: $excluded_namespaces"

  if echo "$excluded_namespaces" | grep -q "right-sizer"; then
    print_success "✅ Right-sizer namespace is excluded in configuration"
  else
    print_warning "⚠️  Right-sizer namespace not found in excluded list"
    print_info "This test will verify if code-level protection works"
  fi
}

# Function to create test workload
create_test_workload() {
  print_header "Creating Test Workload"

  # Create test namespace
  kubectl create namespace test-self-protection --dry-run=client -o yaml | kubectl apply -f -

  # Deploy high-resource pod that should be resized
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: test-self-protection
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx:1.20
        resources:
          requests:
            cpu: 300m      # Intentionally high
            memory: 300Mi  # Intentionally high
          limits:
            cpu: 600m
            memory: 600Mi
        ports:
        - containerPort: 80
EOF

  # Wait for pod to be ready
  print_info "Waiting for test workload to be ready..."
  local count=0
  while [ $count -lt 60 ]; do
    if kubectl get pods -n test-self-protection --field-selector=status.phase=Running --no-headers 2>/dev/null | grep -q test-app; then
      print_success "Test workload is ready"
      kubectl get pods -n test-self-protection
      return 0
    fi
    sleep 2
    count=$((count + 2))
  done

  print_warning "Test workload did not become ready in time"
  kubectl get pods -n test-self-protection
}

# Function to monitor behavior
monitor_behavior() {
  print_header "Monitoring Self-Protection Behavior"

  local pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[?(@.status.containerStatuses[0].ready==true)].metadata.name}' 2>/dev/null | head -1)
  if [ -z "$pod_name" ]; then
    pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}')
  fi
  print_info "Monitoring pod: $pod_name"

  # Initialize tracking variables
  local config_protection=false
  local pod_protection=false
  local self_resize_attempt=false
  local test_workload_resize=false
  local resource_error=false

  # Start log collection
  print_info "Starting log collection for $TEST_TIMEOUT seconds..."
  kubectl logs -n $NAMESPACE $pod_name -f --since=10s >$LOG_FILE 2>&1 &
  local log_pid=$!

  local start_time=$(date +%s)
  local last_status=$(date +%s)

  while [ $(($(date +%s) - start_time)) -lt $TEST_TIMEOUT ]; do
    # Update status every 15 seconds
    if [ $(($(date +%s) - last_status)) -ge 15 ] && [ -f "$LOG_FILE" ]; then
      last_status=$(date +%s)

      # Check for various events in logs
      if grep -q "Added operator namespace.*to exclude list for self-protection\|Preserved operator namespace.*in exclude list" $LOG_FILE 2>/dev/null; then
        if [ "$config_protection" = false ]; then
          print_success "✅ Configuration-level self-protection detected"
          config_protection=true
        fi
      fi

      if grep -q "Skipping self-pod.*to prevent self-modification" $LOG_FILE 2>/dev/null; then
        if [ "$pod_protection" = false ]; then
          print_success "✅ Pod-level self-protection detected"
          pod_protection=true
        fi
      fi

      if grep -q "Resizing.*pod right-sizer/.*right-sizer" $LOG_FILE 2>/dev/null; then
        if [ "$self_resize_attempt" = false ]; then
          print_error "❌ Self-resize attempt detected!"
          self_resize_attempt=true
        fi
      fi

      if grep -q "Resizing.*pod test-self-protection/test-app" $LOG_FILE 2>/dev/null; then
        if [ "$test_workload_resize" = false ]; then
          print_success "✅ Right-sizer processing test workloads"
          test_workload_resize=true
        fi
      fi

      if grep -q "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null; then
        if [ "$resource_error" = false ]; then
          print_error "❌ Resource not found error detected!"
          resource_error=true
        fi
      fi

      # Show progress
      local elapsed=$(($(date +%s) - start_time))
      printf "\rElapsed: ${elapsed}s | Config: %s | Pod: %s | Test: %s | Self: %s | Error: %s" \
        "$([ "$config_protection" = true ] && echo "✅" || echo "⏳")" \
        "$([ "$pod_protection" = true ] && echo "✅" || echo "⏳")" \
        "$([ "$test_workload_resize" = true ] && echo "✅" || echo "⏳")" \
        "$([ "$self_resize_attempt" = true ] && echo "❌" || echo "✅")" \
        "$([ "$resource_error" = true ] && echo "❌" || echo "✅")"
    fi

    sleep 3
  done

  echo "" # New line after progress

  # Stop log monitoring
  kill $log_pid 2>/dev/null || true

  # Check if resources actually changed
  check_resource_changes

  # Return success if no self-resize attempts and no errors
  [ "$self_resize_attempt" = false ] && [ "$resource_error" = false ]
}

# Function to check if pod resources changed
check_resource_changes() {
  print_section "Checking Resource Changes"

  local pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[?(@.status.containerStatuses[0].ready==true)].metadata.name}' 2>/dev/null | head -1)
  if [ -z "$pod_name" ]; then
    pod_name=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}')
  fi

  local cpu_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}' 2>/dev/null || echo "not set")
  local memory_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}' 2>/dev/null || echo "not set")
  local cpu_limit=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.cpu}' 2>/dev/null || echo "not set")
  local memory_limit=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.memory}' 2>/dev/null || echo "not set")

  local current_resources="$cpu_request|$memory_request|$cpu_limit|$memory_limit"
  local initial_resources=$(cat /tmp/initial_resources 2>/dev/null || echo "unknown")

  print_info "Initial resources: $initial_resources"
  print_info "Current resources: $current_resources"

  if [ "$current_resources" = "$initial_resources" ]; then
    print_success "✅ Right-sizer pod resources unchanged (expected)"
  else
    print_warning "⚠️  Right-sizer pod resources may have changed"
    print_info "This could indicate self-modification occurred"
  fi
}

# Function to analyze results
analyze_results() {
  print_header "Test Results Analysis"

  if [ ! -f "$LOG_FILE" ]; then
    print_error "Log file not found: $LOG_FILE"
    return 1
  fi

  print_section "Event Summary"

  local config_events=$(grep -c "Added operator namespace.*to exclude list for self-protection\|Preserved operator namespace.*in exclude list" $LOG_FILE 2>/dev/null | head -1)
  local pod_events=$(grep -c "Skipping self-pod.*to prevent self-modification" $LOG_FILE 2>/dev/null | head -1)
  local self_resize_events=$(grep -c "Resizing.*pod right-sizer/.*right-sizer" $LOG_FILE 2>/dev/null | head -1)
  local test_resize_events=$(grep -c "Resizing.*pod test-self-protection" $LOG_FILE 2>/dev/null | head -1)
  local error_events=$(grep -c "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null | head -1)

  # Ensure variables are numeric
  config_events=${config_events:-0}
  pod_events=${pod_events:-0}
  self_resize_events=${self_resize_events:-0}
  test_resize_events=${test_resize_events:-0}
  error_events=${error_events:-0}

  print_info "Configuration protection events: $config_events"
  print_info "Pod-level protection events: $pod_events"
  print_info "Self-resize attempts: $self_resize_events"
  print_info "Test workload resizes: $test_resize_events"
  print_info "Resource errors: $error_events"

  print_section "Test Assessment"

  local success_count=0
  local total_tests=4

  if [ $config_events -gt 0 ]; then
    print_success "✅ Configuration protection active"
    success_count=$((success_count + 1))
  else
    print_info "ℹ️  Configuration protection not observed (may have occurred before monitoring)"
  fi

  if [ $pod_events -gt 0 ]; then
    print_success "✅ Pod-level protection active"
    success_count=$((success_count + 1))
  else
    print_info "ℹ️  Pod-level protection not observed (pods may not have been processed yet)"
  fi

  if [ $self_resize_events -eq 0 ]; then
    print_success "✅ No self-resize attempts (GOOD)"
    success_count=$((success_count + 1))
  else
    print_error "❌ Self-resize attempts detected (BAD)"
  fi

  if [ $error_events -eq 0 ]; then
    print_success "✅ No resource errors (GOOD)"
    success_count=$((success_count + 1))
  else
    print_error "❌ Resource errors detected (BAD)"
  fi

  print_section "Result: $success_count/$total_tests checks passed"

  # Show relevant log excerpts
  if [ -f "$LOG_FILE" ] && [ -s "$LOG_FILE" ]; then
    print_section "Key Log Entries"

    print_info "Self-protection messages:"
    grep -i "self-protection\|skipping self-pod" $LOG_FILE 2>/dev/null | head -3 || print_info "  None found"

    print_info "Recent activity (last 10 lines):"
    tail -10 $LOG_FILE | sed 's/^/  /'
  else
    print_warning "Log file is empty or unavailable"
  fi

  # Test passes if no self-resize attempts and no errors
  return $([ $self_resize_events -eq 0 ] && [ $error_events -eq 0 ] && echo 0 || echo 1)
}

# Function to cleanup
cleanup() {
  print_header "Cleanup"

  print_info "Removing test resources..."
  kubectl delete namespace test-self-protection --ignore-not-found=true --timeout=30s >/dev/null 2>&1

  rm -f /tmp/initial_resources

  print_success "Test cleanup completed"
}

# Function to show status
show_status() {
  print_header "Current Status"

  print_section "Right-Sizer Pods"
  kubectl get pods -n $NAMESPACE -o wide

  print_section "Right-Sizer Configuration"
  kubectl get rightsizerconfig -n $NAMESPACE -o yaml 2>/dev/null | grep -A 20 namespaceConfig || print_info "No config found"

  print_section "Test Workloads"
  kubectl get pods -n test-self-protection 2>/dev/null || print_info "No test workloads"

  print_section "Recent Events"
  kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp 2>/dev/null | tail -5
}

# Main test function
run_test() {
  print_header "Right-Sizer Self-Protection Test (Existing Deployment)"
  print_info "Testing existing right-sizer deployment for self-protection behavior"
  print_info "Log file: $LOG_FILE"

  echo "=== Right-Sizer Self-Protection Test - $(date) ===" >$LOG_FILE

  local pod_name=$(check_deployment)
  analyze_current_resources
  check_configuration
  create_test_workload

  if monitor_behavior && analyze_results; then
    print_header "✅ SELF-PROTECTION TEST PASSED"
    print_success "🛡️  Right-sizer is properly protected from self-modification"
    print_success "📋 The self-protection mechanisms are working correctly"
    return 0
  else
    print_header "❌ SELF-PROTECTION TEST FAILED"
    print_error "Issues detected with self-protection"
    print_info "Review the detailed logs at: $LOG_FILE"
    return 1
  fi
}

# Main execution
case "${1:-test}" in
"test" | "full")
  run_test
  ;;
"monitor")
  monitor_behavior
  ;;
"analyze")
  analyze_results
  ;;
"status")
  show_status
  ;;
"cleanup")
  cleanup
  ;;
"help" | *)
  echo "Usage: $0 [command]"
  echo ""
  echo "Commands:"
  echo "  test     - Run complete self-protection test (default)"
  echo "  monitor  - Just monitor existing deployment"
  echo "  analyze  - Analyze existing log file"
  echo "  status   - Show current deployment status"
  echo "  cleanup  - Remove test resources"
  echo "  help     - Show this help"
  echo ""
  echo "This script tests an existing right-sizer deployment"
  echo "to verify it does not attempt to resize itself."
  ;;
esac

# Cleanup on exit
trap cleanup EXIT
