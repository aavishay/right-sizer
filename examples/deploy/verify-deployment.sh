#!/bin/bash

# Right-Sizer Deployment Verification Script
# This script verifies that all Right-Sizer components are operating correctly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${NAMESPACE:-right-sizer}"
CLUSTER_NAME="${CLUSTER_NAME:-right-sizer-cluster}"
MAX_WAIT_TIME=300
CHECK_INTERVAL=5

# Function to print colored output
print_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
  echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[⚠]${NC} $1"
}

print_error() {
  echo -e "${RED}[✗]${NC} $1"
}

print_header() {
  echo ""
  echo -e "${BLUE}═══════════════════════════════════════${NC}"
  echo -e "${BLUE}  $1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════${NC}"
  echo ""
}

# Check if kubectl is configured correctly
check_kubectl_context() {
  print_header "Checking Kubectl Context"

  local current_context=$(kubectl config current-context)
  if [[ "$current_context" == *"$CLUSTER_NAME"* ]]; then
    print_success "Kubectl context is set to: $current_context"
  else
    print_warning "Current context: $current_context"
    print_warning "Expected context to contain: $CLUSTER_NAME"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
    fi
  fi
}

# Check Kubernetes version
check_kubernetes_version() {
  print_header "Checking Kubernetes Version"

  local k8s_version=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}')
  print_info "Kubernetes version: $k8s_version"

  # Check if version supports in-place resizing (1.33+)
  local major_version=$(echo $k8s_version | cut -d. -f1 | sed 's/v//')
  local minor_version=$(echo $k8s_version | cut -d. -f2)

  if [[ $major_version -ge 1 && $minor_version -ge 33 ]]; then
    print_success "Kubernetes version supports in-place pod resizing"
  else
    print_warning "Kubernetes version may not support in-place pod resizing (requires 1.33+)"
  fi
}

# Check if feature gates are enabled
check_feature_gates() {
  print_header "Checking Feature Gates"

  if kubectl get nodes -o json | grep -q "InPlacePodVerticalScaling"; then
    print_success "InPlacePodVerticalScaling feature gate is enabled"
  else
    print_warning "Cannot verify InPlacePodVerticalScaling feature gate status"
  fi
}

# Check namespace exists
check_namespace() {
  print_header "Checking Namespace"

  if kubectl get namespace $NAMESPACE &>/dev/null; then
    print_success "Namespace '$NAMESPACE' exists"

    # Check labels
    local labels=$(kubectl get namespace $NAMESPACE -o jsonpath='{.metadata.labels}')
    if [[ $labels == *"right-sizer"* ]]; then
      print_success "Namespace has right-sizer labels"
    else
      print_warning "Namespace missing right-sizer labels"
    fi
  else
    print_error "Namespace '$NAMESPACE' does not exist"
    return 1
  fi
}

# Check CRDs
check_crds() {
  print_header "Checking Custom Resource Definitions"

  local crds=("rightsizerpolicies.rightsizer.io" "rightsizerconfigs.rightsizer.io")
  local all_crds_found=true

  for crd in "${crds[@]}"; do
    if kubectl get crd $crd &>/dev/null; then
      print_success "CRD '$crd' is installed"
    else
      print_error "CRD '$crd' is not installed"
      all_crds_found=false
    fi
  done

  if [ "$all_crds_found" = false ]; then
    return 1
  fi
}

# Check operator deployment
check_operator() {
  print_header "Checking Right-Sizer Operator"

  # Check deployment exists
  if ! kubectl get deployment right-sizer -n $NAMESPACE &>/dev/null; then
    print_error "Right-Sizer operator deployment not found"
    return 1
  fi

  # Check deployment status
  local ready_replicas=$(kubectl get deployment right-sizer -n $NAMESPACE -o jsonpath='{.status.readyReplicas}')
  local desired_replicas=$(kubectl get deployment right-sizer -n $NAMESPACE -o jsonpath='{.spec.replicas}')

  if [[ "$ready_replicas" == "$desired_replicas" ]] && [[ "$ready_replicas" -gt 0 ]]; then
    print_success "Operator deployment is ready ($ready_replicas/$desired_replicas replicas)"
  else
    print_error "Operator deployment not ready ($ready_replicas/$desired_replicas replicas)"
    return 1
  fi

  # Check pod status
  local pod_status=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].status.phase}')
  if [[ "$pod_status" == "Running" ]]; then
    print_success "Operator pod is running"
  else
    print_error "Operator pod status: $pod_status"
    return 1
  fi

  # Check recent logs for errors
  print_info "Checking operator logs for errors..."
  local error_count=$(kubectl logs deployment/right-sizer -n $NAMESPACE --tail=50 2>/dev/null | grep -iE "error|fatal|panic" | wc -l)
  if [[ $error_count -eq 0 ]]; then
    print_success "No recent errors in operator logs"
  else
    print_warning "Found $error_count error messages in recent logs"
  fi
}

# Check dashboard deployment
check_dashboard() {
  print_header "Checking Right-Sizer Dashboard"

  # Check deployment exists
  if ! kubectl get deployment right-sizer-dashboard -n $NAMESPACE &>/dev/null; then
    print_warning "Dashboard deployment not found (optional component)"
    return 0
  fi

  # Check deployment status
  local ready_replicas=$(kubectl get deployment right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.status.readyReplicas}')
  local desired_replicas=$(kubectl get deployment right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.spec.replicas}')

  if [[ "$ready_replicas" == "$desired_replicas" ]] && [[ "$ready_replicas" -gt 0 ]]; then
    print_success "Dashboard deployment is ready ($ready_replicas/$desired_replicas replicas)"
  else
    print_warning "Dashboard deployment not ready ($ready_replicas/$desired_replicas replicas)"
  fi

  # Check service
  if kubectl get service right-sizer-dashboard -n $NAMESPACE &>/dev/null; then
    local service_type=$(kubectl get service right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.spec.type}')
    print_success "Dashboard service exists (type: $service_type)"

    if [[ "$service_type" == "NodePort" ]]; then
      local node_port=$(kubectl get service right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.spec.ports[0].nodePort}')
      local minikube_ip=$(minikube ip -p $CLUSTER_NAME 2>/dev/null || echo "unknown")
      print_info "Dashboard accessible at: http://$minikube_ip:$node_port"
    fi
  else
    print_warning "Dashboard service not found"
  fi
}

# Check metrics server
check_metrics_server() {
  print_header "Checking Metrics Server"

  # Check deployment
  if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    print_success "Metrics server is deployed"

    # Check if metrics are available
    if kubectl top nodes &>/dev/null; then
      print_success "Metrics server is providing node metrics"

      # Show current node metrics
      print_info "Current node resource usage:"
      kubectl top nodes | head -5
    else
      print_warning "Metrics server deployed but not providing metrics yet"
    fi

    # Check pod metrics
    if kubectl top pods -n $NAMESPACE &>/dev/null; then
      print_success "Pod metrics are available"
    else
      print_warning "Pod metrics not available yet"
    fi
  else
    print_error "Metrics server is not deployed"
    return 1
  fi
}

# Check operator metrics endpoint
check_operator_metrics() {
  print_header "Checking Operator Metrics Endpoint"

  # Get operator pod
  local pod_name=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

  if [[ -z "$pod_name" ]]; then
    print_error "No operator pod found"
    return 1
  fi

  # Check metrics endpoint
  print_info "Testing metrics endpoint..."
  local metrics=$(kubectl exec -n $NAMESPACE $pod_name -- wget -q -O - http://localhost:8081/metrics 2>/dev/null | head -20)

  if [[ -n "$metrics" ]]; then
    print_success "Metrics endpoint is responding"

    # Check for specific metrics
    if echo "$metrics" | grep -q "rightsizer_pods_processed_total"; then
      print_success "Found rightsizer_pods_processed_total metric"
    fi

    if echo "$metrics" | grep -q "rightsizer_pods_resized_total"; then
      print_success "Found rightsizer_pods_resized_total metric"
    fi
  else
    print_warning "Could not access metrics endpoint"
  fi
}

# Check health endpoints
check_health_endpoints() {
  print_header "Checking Health Endpoints"

  local pod_name=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

  if [[ -z "$pod_name" ]]; then
    print_error "No operator pod found"
    return 1
  fi

  # Check liveness endpoint
  local liveness=$(kubectl exec -n $NAMESPACE $pod_name -- wget -q -O - http://localhost:8081/healthz 2>/dev/null)
  if [[ $? -eq 0 ]]; then
    print_success "Liveness endpoint (/healthz) is responding"
  else
    print_warning "Liveness endpoint not responding"
  fi

  # Check readiness endpoint
  local readiness=$(kubectl exec -n $NAMESPACE $pod_name -- wget -q -O - http://localhost:8081/readyz 2>/dev/null)
  if [[ $? -eq 0 ]]; then
    print_success "Readiness endpoint (/readyz) is responding"
  else
    print_warning "Readiness endpoint not responding"
  fi
}

# Check for test workloads
check_test_workloads() {
  print_header "Checking Test Workloads"

  if kubectl get namespace test-workloads &>/dev/null; then
    print_success "Test workloads namespace exists"

    # Count deployments
    local deployment_count=$(kubectl get deployments -n test-workloads --no-headers 2>/dev/null | wc -l)
    if [[ $deployment_count -gt 0 ]]; then
      print_success "Found $deployment_count test deployments"

      # Show deployments
      print_info "Test deployments:"
      kubectl get deployments -n test-workloads
    else
      print_warning "No test deployments found"
    fi
  else
    print_warning "Test workloads namespace not found (optional)"
  fi
}

# Check optimization activity
check_optimization_activity() {
  print_header "Checking Optimization Activity"

  # Check for right-sizer events
  print_info "Looking for optimization events..."
  local event_count=$(kubectl get events -A --field-selector reason=ResourcesOptimized 2>/dev/null | wc -l)

  if [[ $event_count -gt 1 ]]; then
    print_success "Found $(($event_count - 1)) optimization events"

    # Show recent events
    print_info "Recent optimization events:"
    kubectl get events -A --field-selector reason=ResourcesOptimized --sort-by='.lastTimestamp' | tail -5
  else
    print_warning "No optimization events found yet (operator may need more time)"
  fi

  # Check operator logs for optimization activity
  local optimization_count=$(kubectl logs deployment/right-sizer -n $NAMESPACE --tail=100 2>/dev/null | grep -i "optimiz" | wc -l)
  if [[ $optimization_count -gt 0 ]]; then
    print_success "Found $optimization_count optimization log entries"
  else
    print_warning "No optimization activity in recent logs"
  fi
}

# Generate summary report
generate_summary() {
  print_header "Verification Summary"

  echo "═══════════════════════════════════════"
  echo "Component Status:"
  echo "───────────────────────────────────────"

  # Quick status checks
  kubectl get deployment right-sizer -n $NAMESPACE &>/dev/null &&
    echo "✓ Operator:        Running" || echo "✗ Operator:        Not Found"

  kubectl get deployment right-sizer-dashboard -n $NAMESPACE &>/dev/null &&
    echo "✓ Dashboard:       Running" || echo "○ Dashboard:       Not Deployed"

  kubectl top nodes &>/dev/null &&
    echo "✓ Metrics Server:  Active" || echo "✗ Metrics Server:  Not Working"

  kubectl get crd rightsizerpolicies.rightsizer.io &>/dev/null &&
    echo "✓ CRDs:           Installed" || echo "✗ CRDs:           Missing"

  echo "═══════════════════════════════════════"
  echo ""

  # Access information
  print_info "Access Information:"
  echo "───────────────────────────────────────"

  # Get minikube IP if available
  if command -v minikube &>/dev/null; then
    local minikube_ip=$(minikube ip -p $CLUSTER_NAME 2>/dev/null || echo "N/A")
    echo "Minikube IP: $minikube_ip"
  fi

  # Get service endpoints
  if kubectl get service right-sizer-dashboard -n $NAMESPACE &>/dev/null; then
    local node_port=$(kubectl get service right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.spec.ports[0].nodePort}')
    echo "Dashboard NodePort: $node_port"
  fi

  echo ""
  print_info "Useful commands:"
  echo "  • View logs:     kubectl logs -f deployment/right-sizer -n $NAMESPACE"
  echo "  • Access dashboard: kubectl port-forward -n $NAMESPACE service/right-sizer-dashboard 3000:80"
  echo "  • View metrics:  kubectl port-forward -n $NAMESPACE service/right-sizer 8081:8081"
  echo "  • Watch pods:    kubectl get pods -A -w"
}

# Main verification flow
main() {
  print_header "Right-Sizer Deployment Verification"
  echo "Starting verification process..."
  echo ""

  local failed_checks=0

  # Run all checks
  check_kubectl_context || ((failed_checks++))
  check_kubernetes_version || ((failed_checks++))
  check_feature_gates || ((failed_checks++))
  check_namespace || ((failed_checks++))
  check_crds || ((failed_checks++))
  check_operator || ((failed_checks++))
  check_dashboard || ((failed_checks++))
  check_metrics_server || ((failed_checks++))
  check_operator_metrics || ((failed_checks++))
  check_health_endpoints || ((failed_checks++))
  check_test_workloads || ((failed_checks++))
  check_optimization_activity || ((failed_checks++))

  # Generate summary
  generate_summary

  # Final status
  if [[ $failed_checks -eq 0 ]]; then
    print_success "All verification checks passed!"
    exit 0
  else
    print_warning "Verification completed with $failed_checks warnings/errors"
    print_info "Some components may still be initializing. Run again in a few minutes."
    exit 1
  fi
}

# Run main function
main "$@"
