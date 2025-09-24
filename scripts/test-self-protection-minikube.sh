#!/bin/bash

# Self-Protection Test Script for Right-Sizer on Minikube
# This script tests that the right-sizer operator does not attempt to resize itself

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
KUBERNETES_VERSION="v1.28.3"
TEST_TIMEOUT=300 # 5 minutes
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
    print_success "minikube: $(minikube version --short)"
  fi

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    missing_deps+=("kubectl")
  else
    print_success "kubectl: $(kubectl version --client --short)"
  fi

  # Check helm
  if ! command -v helm &>/dev/null; then
    missing_deps+=("helm")
  else
    print_success "helm: $(helm version --short)"
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

# Function to setup minikube
setup_minikube() {
  print_header "Setting up Minikube"

  # Check if minikube is running
  if minikube status | grep -q "Running"; then
    print_info "Minikube is already running"
    minikube status
  else
    print_info "Starting Minikube with Kubernetes $KUBERNETES_VERSION..."
    minikube start \
      --kubernetes-version=$KUBERNETES_VERSION \
      --memory=4096 \
      --cpus=2 \
      --driver=docker
    print_success "Minikube started successfully"
  fi

  # Set docker environment
  print_info "Setting up Docker environment..."
  eval $(minikube docker-env)
  print_success "Docker environment configured"

  # Verify cluster
  print_info "Verifying cluster readiness..."
  wait_for_condition "kubectl get nodes | grep -q Ready" "Cluster nodes ready" 60
  kubectl get nodes
}

# Function to build images
build_images() {
  print_header "Building Docker Images"

  # Set minikube docker environment
  eval $(minikube docker-env)

  # Build right-sizer image
  print_info "Building right-sizer operator image..."
  docker build -t right-sizer:test-latest -f Dockerfile .
  docker tag right-sizer:test-latest aavishay/right-sizer:test-latest
  print_success "Right-sizer operator image built"

  # List images to confirm
  print_info "Available images:"
  docker images | grep right-sizer
}

# Function to deploy right-sizer with helm
deploy_right_sizer() {
  print_header "Deploying Right-Sizer with Helm"

  # Create namespace
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
  print_success "Namespace '$NAMESPACE' ready"

  # Create custom values for testing
  cat >/tmp/right-sizer-test-values.yaml <<EOF
replicaCount: 1

image:
  repository: right-sizer
  tag: test-latest
  pullPolicy: IfNotPresent

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 64Mi

rightsizerConfig:
  enabled: true
  dryRun: false
  defaultMode: "balanced"
  resizeInterval: "30s"  # Faster for testing

  # Namespace configuration - should exclude right-sizer namespace
  namespaceConfig:
    includeNamespaces: []
    excludeNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"
      - "right-sizer"  # This should prevent self-resizing
    systemNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"

  # Logging for debugging
  logging:
    level: "debug"
    format: "json"
    enableAudit: true

  # Observability
  observabilityConfig:
    enableMetricsExport: true
    metricsPort: 9090

  # Operational settings
  operationalConfig:
    resizeInterval: "30s"
    retryAttempts: 3
    retryInterval: "5s"
    batchSize: 1
    delayBetweenBatches: "2s"

  # Default resource strategy
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 0.5  # Should try to reduce CPU
      limitMultiplier: 1.5
      minRequest: "10m"
      maxLimit: "1000m"
    memory:
      requestMultiplier: 0.8  # Should try to reduce memory
      limitMultiplier: 1.5
      minRequest: "32Mi"
      maxLimit: "1Gi"
EOF

  # Deploy with helm
  print_info "Installing Right-Sizer Helm chart..."
  helm upgrade --install $HELM_RELEASE_NAME ./helm \
    --namespace $NAMESPACE \
    --values /tmp/right-sizer-test-values.yaml \
    --wait \
    --timeout 5m

  print_success "Right-Sizer Helm chart installed"

  # Verify deployment
  print_info "Verifying deployment..."
  wait_for_condition "kubectl get pods -n $NAMESPACE | grep right-sizer | grep -q Running" "Right-sizer pod running" 120

  kubectl get pods -n $NAMESPACE -o wide
  kubectl get rightsizerconfig -n $NAMESPACE -o yaml
}

# Function to deploy test workloads
deploy_test_workloads() {
  print_header "Deploying Test Workloads"

  # Create test namespace
  kubectl create namespace test-workloads --dry-run=client -o yaml | kubectl apply -f -

  # Deploy nginx test pod with high resource requests
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  namespace: test-workloads
  labels:
    app: nginx-test
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
            cpu: 500m      # Intentionally high
            memory: 512Mi  # Intentionally high
          limits:
            cpu: 1000m
            memory: 1Gi
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-test
  namespace: test-workloads
spec:
  selector:
    app: nginx-test
  ports:
  - port: 80
    targetPort: 80
EOF

  print_success "Test workloads deployed"

  # Wait for test pods to be ready
  wait_for_condition "kubectl get pods -n test-workloads | grep nginx-test | grep -q Running" "Test workloads running" 60
  kubectl get pods -n test-workloads -o wide
}

# Function to monitor self-protection
monitor_self_protection() {
  print_header "Monitoring Self-Protection"

  local pod_name=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')
  print_info "Monitoring right-sizer pod: $pod_name"

  # Start log monitoring in background
  print_info "Starting log collection..."
  kubectl logs -n $NAMESPACE $pod_name -f >$LOG_FILE 2>&1 &
  local log_pid=$!

  # Monitor for specific log patterns
  print_section "Watching for Self-Protection Messages"

  local start_time=$(date +%s)
  local protection_detected=false
  local self_resize_attempt=false
  local config_protection=false

  while [ $(($(date +%s) - start_time)) -lt $TEST_TIMEOUT ]; do
    if [ -f "$LOG_FILE" ]; then
      # Check for configuration-level protection
      if grep -q "Added operator namespace.*to exclude list for self-protection" $LOG_FILE; then
        if [ "$config_protection" = false ]; then
          print_success "Configuration-level self-protection activated"
          config_protection=true
        fi
      fi

      # Check for pod-level protection
      if grep -q "Skipping self-pod.*to prevent self-modification" $LOG_FILE; then
        if [ "$protection_detected" = false ]; then
          print_success "Pod-level self-protection activated"
          protection_detected=true
        fi
      fi

      # Check for any resize attempts on right-sizer pods (should not happen)
      if grep -q "Resizing.*pod right-sizer/right-sizer" $LOG_FILE; then
        print_error "FAILURE: Right-sizer attempted to resize itself!"
        self_resize_attempt=true
      fi

      # Check for resize attempts on test workloads (should happen)
      if grep -q "Resizing.*pod test-workloads/nginx-test" $LOG_FILE; then
        print_success "Right-sizer is processing test workloads (expected behavior)"
      fi

      # Check for the specific error we're trying to fix
      if grep -q "CPU resize failed: the server could not find the requested resource" $LOG_FILE; then
        print_error "FAILURE: Still getting 'server could not find the requested resource' error"
        break
      fi
    fi

    sleep 5
  done

  # Stop log monitoring
  kill $log_pid 2>/dev/null || true

  return $protection_detected
}

# Function to analyze logs
analyze_logs() {
  print_header "Log Analysis"

  if [ ! -f "$LOG_FILE" ]; then
    print_error "Log file not found: $LOG_FILE"
    return 1
  fi

  print_section "Self-Protection Events"

  # Configuration-level protection
  local config_events=$(grep -c "Added operator namespace.*to exclude list for self-protection" $LOG_FILE 2>/dev/null || echo "0")
  print_info "Configuration protection events: $config_events"

  # Pod-level protection
  local pod_events=$(grep -c "Skipping self-pod.*to prevent self-modification" $LOG_FILE 2>/dev/null || echo "0")
  print_info "Pod-level protection events: $pod_events"

  # Self-resize attempts (should be 0)
  local self_resize=$(grep -c "Resizing.*pod right-sizer/right-sizer" $LOG_FILE 2>/dev/null || echo "0")
  print_info "Self-resize attempts: $self_resize"

  # Test workload resizes (should be > 0)
  local workload_resize=$(grep -c "Resizing.*pod test-workloads" $LOG_FILE 2>/dev/null || echo "0")
  print_info "Test workload resize attempts: $workload_resize"

  # Error occurrences
  local errors=$(grep -c "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null || echo "0")
  print_info "Resource not found errors: $errors"

  print_section "Test Results Summary"

  if [ $config_events -gt 0 ]; then
    print_success "✅ Configuration-level protection working"
  else
    print_warning "⚠️  No configuration protection events detected"
  fi

  if [ $pod_events -gt 0 ]; then
    print_success "✅ Pod-level protection working"
  else
    print_warning "⚠️  No pod-level protection events detected"
  fi

  if [ $self_resize -eq 0 ]; then
    print_success "✅ No self-resize attempts detected"
  else
    print_error "❌ FAILURE: $self_resize self-resize attempts detected!"
    return 1
  fi

  if [ $errors -eq 0 ]; then
    print_success "✅ No 'server could not find the requested resource' errors"
  else
    print_error "❌ FAILURE: $errors resource not found errors detected!"
    return 1
  fi

  if [ $workload_resize -gt 0 ]; then
    print_success "✅ Right-sizer is processing other workloads normally"
  else
    print_warning "⚠️  No test workload resize attempts detected (may need more time)"
  fi

  return 0
}

# Function to verify pod resources
verify_pod_resources() {
  print_header "Verifying Pod Resources"

  # Get right-sizer pod resources
  local pod_name=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')

  print_info "Right-sizer pod resources:"
  kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq .

  print_info "Checking if resources changed during test..."

  # Compare with expected values from deployment
  local cpu_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
  local memory_request=$(kubectl get pod $pod_name -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')

  print_info "Current resources - CPU: $cpu_request, Memory: $memory_request"

  # These should match our helm values (50m, 64Mi) and not be modified
  if [ "$cpu_request" = "50m" ] && [ "$memory_request" = "64Mi" ]; then
    print_success "Right-sizer pod resources unchanged (expected behavior)"
    return 0
  else
    print_warning "Right-sizer pod resources may have changed - investigating..."

    # Check if there are any resize operations in progress
    kubectl describe pod $pod_name -n $NAMESPACE | grep -A 10 -B 5 -i resize || true

    return 1
  fi
}

# Function to show detailed status
show_status() {
  print_header "Cluster Status"

  print_section "Nodes"
  kubectl get nodes -o wide

  print_section "Right-Sizer Pods"
  kubectl get pods -n $NAMESPACE -o wide

  print_section "Test Workload Pods"
  kubectl get pods -n test-workloads -o wide

  print_section "Right-Sizer Configuration"
  kubectl get rightsizerconfig -n $NAMESPACE -o yaml

  print_section "Right-Sizer Service"
  kubectl get svc -n $NAMESPACE

  print_section "Events"
  kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp
}

# Function to cleanup
cleanup() {
  print_header "Cleanup"

  print_info "Cleaning up test resources..."

  # Remove test workloads
  kubectl delete namespace test-workloads --ignore-not-found=true

  # Remove right-sizer
  helm uninstall $HELM_RELEASE_NAME -n $NAMESPACE || true
  kubectl delete namespace $NAMESPACE --ignore-not-found=true

  # Clean up temp files
  rm -f /tmp/right-sizer-test-values.yaml

  print_success "Cleanup completed"
}

# Function to run full test
run_full_test() {
  print_header "Right-Sizer Self-Protection Test"
  print_info "Testing that right-sizer does not attempt to resize itself"
  print_info "Log file: $LOG_FILE"

  # Initialize log file
  echo "=== Right-Sizer Self-Protection Test - $(date) ===" >$LOG_FILE

  local test_failed=false

  # Run test steps
  if ! check_dependencies; then test_failed=true; fi
  if ! setup_minikube; then test_failed=true; fi
  if ! build_images; then test_failed=true; fi
  if ! deploy_right_sizer; then test_failed=true; fi
  if ! deploy_test_workloads; then test_failed=true; fi

  # Monitor and analyze
  if ! monitor_self_protection; then test_failed=true; fi
  if ! analyze_logs; then test_failed=true; fi
  if ! verify_pod_resources; then test_failed=true; fi

  # Show final status
  show_status

  # Test results
  print_header "Test Results"

  if [ "$test_failed" = true ]; then
    print_error "❌ SELF-PROTECTION TEST FAILED"
    print_info "Check logs at: $LOG_FILE"
    return 1
  else
    print_success "✅ SELF-PROTECTION TEST PASSED"
    print_success "Right-sizer successfully prevented self-modification"
    return 0
  fi
}

# Main execution
main() {
  case "${1:-full}" in
  "setup")
    check_dependencies
    setup_minikube
    build_images
    ;;
  "deploy")
    deploy_right_sizer
    deploy_test_workloads
    ;;
  "monitor")
    monitor_self_protection
    ;;
  "analyze")
    analyze_logs
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
    echo "  setup    - Setup minikube and build images"
    echo "  deploy   - Deploy right-sizer and test workloads"
    echo "  monitor  - Monitor for self-protection events"
    echo "  analyze  - Analyze collected logs"
    echo "  status   - Show cluster and deployment status"
    echo "  cleanup  - Remove all test resources"
    echo "  full     - Run complete test (default)"
    echo "  help     - Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 full     # Run complete self-protection test"
    echo "  $0 setup    # Just setup environment"
    echo "  $0 cleanup  # Clean up after testing"
    ;;
  esac
}

# Trap for cleanup on exit
trap 'echo ""; print_warning "Script interrupted - cleaning up..."; cleanup; exit 1' INT TERM

# Run main function
main "$@"
