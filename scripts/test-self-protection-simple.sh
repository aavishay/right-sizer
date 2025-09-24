#!/bin/bash

# Simplified Self-Protection Test Script for Right-Sizer
# This script tests that the right-sizer operator does not attempt to resize itself
# Uses existing binary to avoid build issues

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
HELM_RELEASE_NAME="right-sizer"
TEST_TIMEOUT=180 # 3 minutes
LOG_FILE="/tmp/right-sizer-self-protection-test.log"

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

# Function to wait for condition with timeout
wait_for_condition() {
  local condition="$1"
  local description="$2"
  local timeout="${3:-60}"
  local interval="${4:-5}"

  print_info "Waiting for: $description (timeout: ${timeout}s)"

  local count=0
  while [ $count -lt $timeout ]; do
    if eval "$condition" >/dev/null 2>&1; then
      print_success "$description - completed"
      return 0
    fi
    sleep $interval
    count=$((count + interval))
    printf "."
  done

  echo ""
  print_error "$description - timed out after ${timeout}s"
  return 1
}

# Function to check dependencies
check_dependencies() {
  print_header "Checking Dependencies"

  local missing_deps=()

  # Check minikube
  if ! command -v minikube &>/dev/null; then
    missing_deps+=("minikube")
  else
    print_success "minikube: $(minikube version --short 2>/dev/null || echo 'installed')"
  fi

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    missing_deps+=("kubectl")
  else
    print_success "kubectl: $(kubectl version --client --short 2>/dev/null || echo 'installed')"
  fi

  # Check helm
  if ! command -v helm &>/dev/null; then
    missing_deps+=("helm")
  else
    print_success "helm: $(helm version --short 2>/dev/null || echo 'installed')"
  fi

  # Check docker
  if ! command -v docker &>/dev/null; then
    missing_deps+=("docker")
  else
    print_success "docker: $(docker --version)"
  fi

  if [ ${#missing_deps[@]} -ne 0 ]; then
    print_error "Missing required dependencies: ${missing_deps[*]}"
    echo ""
    echo "Installation instructions:"
    echo "  Minikube: https://minikube.sigs.k8s.io/docs/start/"
    echo "  kubectl:  https://kubernetes.io/docs/tasks/tools/"
    echo "  Helm:     https://helm.sh/docs/intro/install/"
    echo "  Docker:   https://docs.docker.com/get-docker/"
    exit 1
  fi
}

# Function to verify minikube
verify_minikube() {
  print_header "Verifying Minikube"

  # Check if minikube is running
  if ! minikube status | grep -q "Running" 2>/dev/null; then
    print_error "Minikube is not running. Please start it first:"
    echo "  minikube start --memory=4096 --cpus=2"
    exit 1
  fi

  print_success "Minikube is running"
  minikube status

  # Verify cluster access
  if ! kubectl get nodes >/dev/null 2>&1; then
    print_error "Cannot access Kubernetes cluster"
    exit 1
  fi

  print_success "Kubernetes cluster is accessible"
  kubectl get nodes
}

# Function to use existing binary
prepare_binary() {
  print_header "Preparing Right-Sizer Binary"

  # Find existing binary
  local binary=""
  for path in "./go/right-sizer" "./bin/right-sizer" "./build/right-sizer" "./dist/right-sizer"; do
    if [ -f "$path" ]; then
      binary="$path"
      break
    fi
  done

  if [ -z "$binary" ]; then
    print_error "No existing right-sizer binary found. Please build first:"
    echo "  cd go && go build -o right-sizer main.go"
    exit 1
  fi

  print_success "Using existing binary: $binary"

  # Copy binary to a standard location for Docker
  cp "$binary" "./right-sizer-binary"
  chmod +x "./right-sizer-binary"

  # Create simple Dockerfile for existing binary
  cat >Dockerfile.simple <<'EOF'
FROM gcr.io/distroless/static-debian12:nonroot
COPY right-sizer-binary /usr/local/bin/right-sizer
ENTRYPOINT ["/usr/local/bin/right-sizer"]
EOF

  print_success "Prepared binary and simple Dockerfile"
}

# Function to build simple image
build_simple_image() {
  print_header "Building Simple Docker Image"

  # Set minikube docker environment
  eval $(minikube docker-env)

  # Build simple image with existing binary
  print_info "Building right-sizer image with existing binary..."
  docker build -t right-sizer:self-protection-test -f Dockerfile.simple .
  print_success "Right-sizer image built successfully"

  # Clean up temp files
  rm -f ./right-sizer-binary Dockerfile.simple

  # Verify image
  docker images | grep right-sizer
}

# Function to deploy right-sizer with self-protection test config
deploy_right_sizer() {
  print_header "Deploying Right-Sizer for Self-Protection Test"

  # Create namespace
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
  print_success "Namespace '$NAMESPACE' ready"

  # Create test values with intentionally low resources to trigger resizing
  cat >/tmp/right-sizer-self-protection-values.yaml <<EOF
replicaCount: 1

image:
  repository: right-sizer
  tag: self-protection-test
  pullPolicy: IfNotPresent

# Intentionally high resources that should be candidates for downsizing
resources:
  limits:
    cpu: 500m      # High CPU limit
    memory: 512Mi  # High memory limit
  requests:
    cpu: 100m      # High CPU request
    memory: 128Mi  # High memory request

rightsizerConfig:
  enabled: true
  dryRun: false
  defaultMode: "adaptive"
  resizeInterval: "20s"  # Fast for testing

  # Namespace configuration - should exclude right-sizer namespace
  namespaceConfig:
    includeNamespaces: []
    excludeNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"
      - "right-sizer"  # Critical: exclude self
    systemNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"

  # Aggressive resource strategy to trigger resizing
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 0.3   # Try to reduce CPU significantly
      limitMultiplier: 0.5
      minRequest: "10m"
      maxLimit: "200m"
      scaleUpThreshold: 0.9
      scaleDownThreshold: 0.1
    memory:
      requestMultiplier: 0.5   # Try to reduce memory
      limitMultiplier: 0.8
      minRequest: "32Mi"
      maxLimit: "256Mi"
      scaleUpThreshold: 0.9
      scaleDownThreshold: 0.2
    historyWindow: "1m"        # Short window for quick results
    algorithm: "percentile"
    percentile: 50

  # Logging for debugging
  logging:
    level: "debug"
    format: "json"

  # Fast operational settings
  operationalConfig:
    resizeInterval: "20s"
    retryAttempts: 3
    retryInterval: "3s"
    batchSize: 1
    delayBetweenBatches: "1s"
    maxUpdatesPerRun: 10
EOF

  # Deploy with helm
  print_info "Installing Right-Sizer Helm chart..."
  helm upgrade --install $HELM_RELEASE_NAME ./helm \
    --namespace $NAMESPACE \
    --values /tmp/right-sizer-self-protection-values.yaml \
    --wait \
    --timeout 2m

  print_success "Right-Sizer Helm chart installed"

  # Verify deployment
  wait_for_condition "kubectl get pods -n $NAMESPACE | grep right-sizer | grep -q Running" "Right-sizer pod running" 60

  print_info "Right-sizer deployment status:"
  kubectl get pods -n $NAMESPACE -o wide
  kubectl get rightsizerconfig -n $NAMESPACE
}

# Function to deploy test workload
deploy_test_workload() {
  print_header "Deploying Test Workload"

  # Create test namespace
  kubectl create namespace test-workloads --dry-run=client -o yaml | kubectl apply -f -

  # Deploy nginx with intentionally high resources
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  namespace: test-workloads
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-test
  template:
    metadata:
      labels:
        app: nginx-test
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 200m      # Should be downsized
            memory: 256Mi  # Should be downsized
          limits:
            cpu: 400m
            memory: 512Mi
        ports:
        - containerPort: 80
EOF

  wait_for_condition "kubectl get pods -n test-workloads | grep nginx-test | grep -q Running" "Test workload running" 60

  print_success "Test workload deployed"
  kubectl get pods -n test-workloads -o wide
}

# Function to monitor self-protection behavior
monitor_self_protection() {
  print_header "Monitoring Self-Protection Behavior"

  local pod_name=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')
  print_info "Monitoring right-sizer pod: $pod_name"

  # Initialize results
  local config_protection=false
  local pod_protection=false
  local self_resize_attempt=false
  local test_workload_resize=false
  local resource_error=false

  # Start log monitoring
  print_info "Starting log collection..."
  kubectl logs -n $NAMESPACE $pod_name -f >$LOG_FILE 2>&1 &
  local log_pid=$!

  print_section "Monitoring for $(($TEST_TIMEOUT / 60)) minutes..."
  print_info "Watching for:"
  echo "  - Configuration-level self-protection"
  echo "  - Pod-level self-protection"
  echo "  - Self-resize attempts (should not happen)"
  echo "  - Test workload resize attempts (should happen)"
  echo "  - Resource not found errors (should not happen)"

  local start_time=$(date +%s)
  local last_check=0

  while [ $(($(date +%s) - start_time)) -lt $TEST_TIMEOUT ]; do
    if [ -f "$LOG_FILE" ] && [ $(($(date +%s) - last_check)) -ge 10 ]; then
      last_check=$(date +%s)

      # Check for configuration-level protection
      if grep -q "Added operator namespace.*to exclude list for self-protection" $LOG_FILE; then
        if [ "$config_protection" = false ]; then
          print_success "✅ Configuration self-protection detected"
          config_protection=true
        fi
      fi

      # Check for pod-level protection
      if grep -q "Skipping self-pod.*to prevent self-modification" $LOG_FILE; then
        if [ "$pod_protection" = false ]; then
          print_success "✅ Pod-level self-protection detected"
          pod_protection=true
        fi
      fi

      # Check for self-resize attempts (BAD)
      if grep -q "Resizing.*pod right-sizer/right-sizer" $LOG_FILE; then
        if [ "$self_resize_attempt" = false ]; then
          print_error "❌ FAILURE: Right-sizer attempted to resize itself!"
          self_resize_attempt=true
        fi
      fi

      # Check for test workload resize (GOOD)
      if grep -q "Resizing.*pod test-workloads/nginx-test" $LOG_FILE; then
        if [ "$test_workload_resize" = false ]; then
          print_success "✅ Right-sizer processing test workloads"
          test_workload_resize=true
        fi
      fi

      # Check for resource errors (BAD)
      if grep -q "CPU resize failed: the server could not find the requested resource" $LOG_FILE; then
        if [ "$resource_error" = false ]; then
          print_error "❌ FAILURE: Resource not found error detected!"
          resource_error=true
        fi
      fi

      # Show progress
      local elapsed=$(($(date +%s) - start_time))
      printf "\rElapsed: ${elapsed}s | Config: $([ "$config_protection" = true ] && echo "✅" || echo "⏳") | Pod: $([ "$pod_protection" = true ] && echo "✅" || echo "⏳") | Workload: $([ "$test_workload_resize" = true ] && echo "✅" || echo "⏳") | Self-resize: $([ "$self_resize_attempt" = true ] && echo "❌" || echo "✅") | Errors: $([ "$resource_error" = true ] && echo "❌" || echo "✅")"
    fi

    sleep 5
  done

  echo "" # New line after progress

  # Stop log monitoring
  kill $log_pid 2>/dev/null || true

  # Return results
  [ "$config_protection" = true ] && [ "$pod_protection" = true ] && [ "$self_resize_attempt" = false ] && [ "$resource_error" = false ]
}

# Function to analyze results
analyze_results() {
  print_header "Test Results Analysis"

  if [ ! -f "$LOG_FILE" ]; then
    print_error "Log file not found: $LOG_FILE"
    return 1
  fi

  print_section "Event Counts"

  local config_events=$(grep -c "Added operator namespace.*to exclude list for self-protection" $LOG_FILE 2>/dev/null || echo "0")
  local pod_events=$(grep -c "Skipping self-pod.*to prevent self-modification" $LOG_FILE 2>/dev/null || echo "0")
  local self_resize=$(grep -c "Resizing.*pod right-sizer/right-sizer" $LOG_FILE 2>/dev/null || echo "0")
  local workload_resize=$(grep -c "Resizing.*pod test-workloads" $LOG_FILE 2>/dev/null || echo "0")
  local errors=$(grep -c "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null || echo "0")

  print_info "Configuration protection events: $config_events"
  print_info "Pod-level protection events: $pod_events"
  print_info "Self-resize attempts: $self_resize"
  print_info "Test workload resizes: $workload_resize"
  print_info "Resource not found errors: $errors"

  print_section "Test Results Summary"

  local passed=0
  local total=5

  if [ $config_events -gt 0 ]; then
    print_success "✅ Configuration-level protection working"
    passed=$((passed + 1))
  else
    print_warning "⚠️  No configuration protection events detected"
  fi

  if [ $pod_events -gt 0 ]; then
    print_success "✅ Pod-level protection working"
    passed=$((passed + 1))
  else
    print_warning "⚠️  No pod-level protection events detected"
  fi

  if [ $self_resize -eq 0 ]; then
    print_success "✅ No self-resize attempts (expected)"
    passed=$((passed + 1))
  else
    print_error "❌ $self_resize self-resize attempts detected!"
  fi

  if [ $errors -eq 0 ]; then
    print_success "✅ No resource not found errors (expected)"
    passed=$((passed + 1))
  else
    print_error "❌ $errors resource not found errors!"
  fi

  if [ $workload_resize -gt 0 ]; then
    print_success "✅ Right-sizer processing other workloads normally"
    passed=$((passed + 1))
  else
    print_warning "⚠️  No test workload resizes detected (may need more time)"
  fi

  print_section "Final Score: $passed/$total tests passed"

  # Show some log samples
  print_section "Log Samples"
  if [ -f "$LOG_FILE" ]; then
    print_info "Self-protection messages:"
    grep -i "self-protection\|skipping self-pod" $LOG_FILE | head -3 || print_info "None found"

    print_info "Recent log entries:"
    tail -5 $LOG_FILE
  fi

  return $([ $passed -ge 3 ] && echo 0 || echo 1)
}

# Function to show current status
show_status() {
  print_header "Current Status"

  print_section "Right-Sizer Deployment"
  kubectl get pods -n $NAMESPACE -o wide

  print_section "Right-Sizer Pod Resources"
  local pod_name=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')
  kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq . 2>/dev/null || echo "Resources not available"

  print_section "Test Workloads"
  kubectl get pods -n test-workloads -o wide 2>/dev/null || print_info "No test workloads"

  print_section "Recent Events"
  kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp | tail -5

  print_section "Configuration"
  kubectl get rightsizerconfig -n $NAMESPACE -o yaml 2>/dev/null | grep -A 10 -B 5 excludeNamespaces || print_info "Config not available"
}

# Function to cleanup
cleanup() {
  print_header "Cleanup"

  print_info "Removing test resources..."

  # Remove test workloads
  kubectl delete namespace test-workloads --ignore-not-found=true --timeout=30s

  # Remove right-sizer
  helm uninstall $HELM_RELEASE_NAME -n $NAMESPACE --timeout=30s 2>/dev/null || true
  kubectl delete namespace $NAMESPACE --ignore-not-found=true --timeout=30s

  # Clean up temp files
  rm -f /tmp/right-sizer-self-protection-values.yaml

  print_success "Cleanup completed"
}

# Function to run full test
run_full_test() {
  print_header "Right-Sizer Self-Protection Test"
  print_info "Testing that right-sizer does NOT attempt to resize itself"
  print_info "This test validates the self-protection fix"
  print_info "Log file: $LOG_FILE"

  # Initialize log file
  echo "=== Right-Sizer Self-Protection Test - $(date) ===" >$LOG_FILE

  local test_failed=false

  # Pre-flight checks
  if ! check_dependencies; then exit 1; fi
  if ! verify_minikube; then exit 1; fi
  if ! prepare_binary; then exit 1; fi
  if ! build_simple_image; then exit 1; fi

  # Deploy and test
  if ! deploy_right_sizer; then test_failed=true; fi
  if ! deploy_test_workload; then test_failed=true; fi
  if ! monitor_self_protection; then test_failed=true; fi
  if ! analyze_results; then test_failed=true; fi

  # Show final status
  show_status

  # Final results
  print_header "Final Results"

  if [ "$test_failed" = true ]; then
    print_error "❌ SELF-PROTECTION TEST FAILED"
    print_info "The right-sizer may still be attempting to resize itself"
    print_info "Check detailed logs at: $LOG_FILE"
    return 1
  else
    print_success "✅ SELF-PROTECTION TEST PASSED"
    print_success "🛡️  Right-sizer successfully prevented self-modification"
    print_success "📋 The fix is working correctly!"
    return 0
  fi
}

# Main execution
main() {
  case "${1:-full}" in
  "check")
    check_dependencies
    verify_minikube
    ;;
  "deploy")
    prepare_binary
    build_simple_image
    deploy_right_sizer
    deploy_test_workload
    ;;
  "monitor")
    monitor_self_protection
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
  "full")
    run_full_test
    ;;
  "help" | *)
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  check    - Check dependencies and minikube status"
    echo "  deploy   - Deploy right-sizer and test workloads"
    echo "  monitor  - Monitor for self-protection events"
    echo "  analyze  - Analyze collected logs"
    echo "  status   - Show current cluster status"
    echo "  cleanup  - Remove all test resources"
    echo "  full     - Run complete self-protection test (default)"
    echo "  help     - Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 full     # Run complete test"
    echo "  $0 check    # Just verify environment"
    echo "  $0 cleanup  # Clean up after testing"
    echo ""
    echo "Requirements:"
    echo "  - Minikube running"
    echo "  - Right-sizer binary built (./go/right-sizer or ./bin/right-sizer)"
    ;;
  esac
}

# Trap for cleanup on exit
trap 'echo ""; print_warning "Test interrupted - run cleanup manually"; exit 1' INT TERM

# Run main function
main "$@"
