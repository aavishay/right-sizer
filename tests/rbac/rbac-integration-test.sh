#!/bin/bash

# Right Sizer RBAC Integration Test Script
# This script tests RBAC permissions in a real Kubernetes environment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="rbac-test-$(date +%s)"
SERVICE_ACCOUNT="right-sizer-test"
CLUSTER_ROLE="right-sizer-test"
HELM_RELEASE="right-sizer-test"
CLEANUP_ON_EXIT=${CLEANUP_ON_EXIT:-true}
VERBOSE=${VERBOSE:-false}

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Logging functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[PASS]${NC} $1"
  ((TESTS_PASSED++))
}

log_error() {
  echo -e "${RED}[FAIL]${NC} $1"
  ((TESTS_FAILED++))
}

log_warning() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_skip() {
  echo -e "${CYAN}[SKIP]${NC} $1"
  ((TESTS_SKIPPED++))
}

log_section() {
  echo ""
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${CYAN}▶ $1${NC}"
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Cleanup function
cleanup() {
  if [ "$CLEANUP_ON_EXIT" = "true" ]; then
    log_section "Cleanup"

    # Delete test namespace (this removes all resources in it)
    if kubectl get namespace "$TEST_NAMESPACE" &>/dev/null; then
      log_info "Deleting test namespace $TEST_NAMESPACE..."
      kubectl delete namespace "$TEST_NAMESPACE" --wait=false
    fi

    # Delete cluster-wide resources
    kubectl delete clusterrole "$CLUSTER_ROLE" --ignore-not-found=true
    kubectl delete clusterrolebinding "$CLUSTER_ROLE" --ignore-not-found=true

    log_info "Cleanup completed"
  else
    log_warning "Cleanup skipped. Test resources remain in namespace: $TEST_NAMESPACE"
  fi
}

# Set up trap for cleanup on exit
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
  log_section "Prerequisites Check"

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl is not installed"
    exit 1
  fi
  log_success "kubectl is available"

  # Check cluster connection
  if ! kubectl cluster-info &>/dev/null; then
    log_error "Cannot connect to Kubernetes cluster"
    exit 1
  fi
  log_success "Connected to Kubernetes cluster"

  # Check Kubernetes version
  K8S_VERSION=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}')
  log_info "Kubernetes version: $K8S_VERSION"

  # Check if metrics-server is available
  if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    log_success "metrics-server is deployed"
    METRICS_AVAILABLE=true
  else
    log_warning "metrics-server not found - some tests will be skipped"
    METRICS_AVAILABLE=false
  fi
}

# Setup test environment
setup_test_environment() {
  log_section "Setting Up Test Environment"

  # Create test namespace
  log_info "Creating test namespace: $TEST_NAMESPACE"
  kubectl create namespace "$TEST_NAMESPACE"
  log_success "Test namespace created"

  # Apply RBAC configuration from helm templates
  log_info "Applying RBAC configuration..."

  # Create service account
  kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $SERVICE_ACCOUNT
  namespace: $TEST_NAMESPACE
automountServiceAccountToken: true
EOF

  # Apply the complete RBAC from our fixed configuration
  kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: $CLUSTER_ROLE
rules:
  # Core pod operations
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods/resize"]
    verbs: ["patch", "update"]

  # Node operations
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["nodes/status"]
    verbs: ["get", "list", "watch"]

  # Metrics API
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes", "podmetrics", "nodemetrics"]
    verbs: ["get", "list", "watch"]

  # Custom metrics
  - apiGroups: ["custom.metrics.k8s.io", "external.metrics.k8s.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]

  # Workload controllers
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: ["apps"]
    resources: ["deployments/status", "statefulsets/status", "daemonsets/status", "replicasets/status"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments/scale", "statefulsets/scale", "replicasets/scale"]
    verbs: ["get", "patch", "update"]

  # Batch
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]

  # Events
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]

  # Configuration
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch"]

  # Namespaces
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]

  # Autoscaling
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["autoscaling.k8s.io"]
    resources: ["verticalpodautoscalers"]
    verbs: ["get", "list", "watch"]

  # Policy
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]

  # Resource constraints
  - apiGroups: [""]
    resources: ["resourcequotas", "limitranges"]
    verbs: ["get", "list", "watch"]

  # Storage
  - apiGroups: [""]
    resources: ["persistentvolumeclaims", "persistentvolumes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list", "watch"]

  # Scheduling
  - apiGroups: ["scheduling.k8s.io"]
    resources: ["priorityclasses"]
    verbs: ["get", "list", "watch"]

  # Networking
  - apiGroups: ["networking.k8s.io"]
    resources: ["networkpolicies"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["create", "update", "patch"]

  # Admission webhooks
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Custom Resources
  - apiGroups: ["rightsizer.io"]
    resources: ["rightsizerpolicies", "rightsizerconfigs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["rightsizer.io"]
    resources: ["rightsizerpolicies/status", "rightsizerconfigs/status"]
    verbs: ["get", "update", "patch"]
EOF

  # Create ClusterRoleBinding
  kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $CLUSTER_ROLE
roleRef:
  kind: ClusterRole
  name: $CLUSTER_ROLE
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: $SERVICE_ACCOUNT
    namespace: $TEST_NAMESPACE
EOF

  # Create namespace-scoped Role
  kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $SERVICE_ACCOUNT-namespace
  namespace: $TEST_NAMESPACE
rules:
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
EOF

  # Create RoleBinding
  kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $SERVICE_ACCOUNT-namespace
  namespace: $TEST_NAMESPACE
roleRef:
  kind: Role
  name: $SERVICE_ACCOUNT-namespace
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: $SERVICE_ACCOUNT
    namespace: $TEST_NAMESPACE
EOF

  log_success "RBAC configuration applied"
}

# Test permission function
test_permission() {
  local verb=$1
  local resource=$2
  local test_name=$3
  local namespace=${4:-""}

  local cmd="kubectl auth can-i $verb $resource --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT"

  if [ -n "$namespace" ]; then
    cmd="$cmd -n $namespace"
  fi

  if [ "$VERBOSE" = "true" ]; then
    log_info "Testing: $cmd"
  fi

  if $cmd &>/dev/null; then
    log_success "$test_name"
    return 0
  else
    log_error "$test_name"
    return 1
  fi
}

# Test core permissions
test_core_permissions() {
  log_section "Testing Core Permissions"

  # Pod operations
  test_permission "get" "pods" "Can get pods"
  test_permission "list" "pods" "Can list pods"
  test_permission "watch" "pods" "Can watch pods"
  test_permission "patch" "pods" "Can patch pods"
  test_permission "update" "pods" "Can update pods"

  # Pod subresources
  test_permission "get" "pods/status" "Can get pod status"
  test_permission "patch" "pods/status" "Can patch pod status"

  # Check for resize subresource (may not exist in older K8s)
  if kubectl api-resources | grep -q "pods/resize"; then
    test_permission "patch" "pods/resize" "Can patch pod resize"
  else
    log_skip "Pod resize subresource not available in this K8s version"
  fi

  # Node operations
  test_permission "get" "nodes" "Can get nodes"
  test_permission "list" "nodes" "Can list nodes"
  test_permission "watch" "nodes" "Can watch nodes"
  test_permission "get" "nodes/status" "Can get node status"

  # Events
  test_permission "create" "events" "Can create events"
  test_permission "patch" "events" "Can patch events"

  # Namespaces
  test_permission "get" "namespaces" "Can get namespaces"
  test_permission "list" "namespaces" "Can list namespaces"
}

# Test metrics permissions
test_metrics_permissions() {
  log_section "Testing Metrics Permissions"

  if [ "$METRICS_AVAILABLE" = "true" ]; then
    test_permission "get" "pods.metrics.k8s.io" "Can get pod metrics"
    test_permission "list" "pods.metrics.k8s.io" "Can list pod metrics"
    test_permission "get" "nodes.metrics.k8s.io" "Can get node metrics"
    test_permission "list" "nodes.metrics.k8s.io" "Can list node metrics"
  else
    log_skip "Metrics API tests skipped - metrics-server not available"
  fi
}

# Test workload controller permissions
test_workload_permissions() {
  log_section "Testing Workload Controller Permissions"

  # Deployments
  test_permission "get" "deployments" "Can get deployments"
  test_permission "list" "deployments" "Can list deployments"
  test_permission "patch" "deployments" "Can patch deployments"
  test_permission "get" "deployments/status" "Can get deployment status"
  test_permission "get" "deployments/scale" "Can get deployment scale"
  test_permission "patch" "deployments/scale" "Can patch deployment scale"

  # StatefulSets
  test_permission "get" "statefulsets" "Can get statefulsets"
  test_permission "list" "statefulsets" "Can list statefulsets"
  test_permission "patch" "statefulsets" "Can patch statefulsets"

  # DaemonSets
  test_permission "get" "daemonsets" "Can get daemonsets"
  test_permission "list" "daemonsets" "Can list daemonsets"

  # ReplicaSets
  test_permission "get" "replicasets" "Can get replicasets"
  test_permission "list" "replicasets" "Can list replicasets"
}

# Test autoscaling permissions
test_autoscaling_permissions() {
  log_section "Testing Autoscaling Permissions"

  test_permission "get" "horizontalpodautoscalers" "Can get HPA"
  test_permission "list" "horizontalpodautoscalers" "Can list HPA"

  # VPA might not be installed
  if kubectl api-versions | grep -q "autoscaling.k8s.io"; then
    test_permission "get" "verticalpodautoscalers.autoscaling.k8s.io" "Can get VPA"
    test_permission "list" "verticalpodautoscalers.autoscaling.k8s.io" "Can list VPA"
  else
    log_skip "VPA tests skipped - VPA not installed"
  fi
}

# Test resource constraint permissions
test_constraint_permissions() {
  log_section "Testing Resource Constraint Permissions"

  test_permission "get" "resourcequotas" "Can get resource quotas"
  test_permission "list" "resourcequotas" "Can list resource quotas"
  test_permission "get" "limitranges" "Can get limit ranges"
  test_permission "list" "limitranges" "Can list limit ranges"
  test_permission "get" "poddisruptionbudgets" "Can get PDB"
  test_permission "list" "poddisruptionbudgets" "Can list PDB"
}

# Test storage permissions
test_storage_permissions() {
  log_section "Testing Storage Permissions"

  test_permission "get" "persistentvolumeclaims" "Can get PVCs"
  test_permission "list" "persistentvolumeclaims" "Can list PVCs"
  test_permission "get" "persistentvolumes" "Can get PVs"
  test_permission "list" "persistentvolumes" "Can list PVs"
  test_permission "get" "storageclasses" "Can get storage classes"
  test_permission "list" "storageclasses" "Can list storage classes"
}

# Test networking permissions
test_networking_permissions() {
  log_section "Testing Networking Permissions"

  test_permission "get" "services" "Can get services"
  test_permission "list" "services" "Can list services"
  test_permission "create" "services" "Can create services"
  test_permission "get" "endpoints" "Can get endpoints"
  test_permission "list" "endpoints" "Can list endpoints"
  test_permission "get" "networkpolicies" "Can get network policies"
  test_permission "list" "networkpolicies" "Can list network policies"
}

# Test namespace-scoped permissions
test_namespace_permissions() {
  log_section "Testing Namespace-Scoped Permissions"

  test_permission "get" "configmaps" "Can get configmaps in test namespace" "$TEST_NAMESPACE"
  test_permission "create" "configmaps" "Can create configmaps in test namespace" "$TEST_NAMESPACE"
  test_permission "update" "configmaps" "Can update configmaps in test namespace" "$TEST_NAMESPACE"
  test_permission "get" "secrets" "Can get secrets in test namespace" "$TEST_NAMESPACE"
  test_permission "create" "secrets" "Can create secrets in test namespace" "$TEST_NAMESPACE"
  test_permission "get" "leases" "Can get leases in test namespace" "$TEST_NAMESPACE"
  test_permission "create" "leases" "Can create leases in test namespace" "$TEST_NAMESPACE"
}

# Test negative permissions (things we should NOT have)
test_negative_permissions() {
  log_section "Testing Negative Permissions (Should Fail)"

  # We should NOT be able to delete namespaces
  if kubectl auth can-i delete namespaces --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_error "Should NOT be able to delete namespaces"
  else
    log_success "Cannot delete namespaces (as expected)"
  fi

  # We should NOT be able to delete nodes
  if kubectl auth can-i delete nodes --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_error "Should NOT be able to delete nodes"
  else
    log_success "Cannot delete nodes (as expected)"
  fi

  # We should NOT be able to modify RBAC
  if kubectl auth can-i create clusterroles --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_error "Should NOT be able to create clusterroles"
  else
    log_success "Cannot create clusterroles (as expected)"
  fi
}

# Test practical scenarios
test_practical_scenarios() {
  log_section "Testing Practical Scenarios"

  # Create a test deployment
  log_info "Creating test deployment..."
  kubectl create deployment test-app --image=nginx:alpine -n "$TEST_NAMESPACE" --replicas=1

  # Wait for deployment to be ready
  kubectl wait --for=condition=available --timeout=60s deployment/test-app -n "$TEST_NAMESPACE"

  # Test if we can get the deployment
  if kubectl get deployment test-app -n "$TEST_NAMESPACE" \
    --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_success "Can get test deployment"
  else
    log_error "Cannot get test deployment"
  fi

  # Test if we can list pods
  if kubectl get pods -n "$TEST_NAMESPACE" \
    --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_success "Can list pods in test namespace"
  else
    log_error "Cannot list pods in test namespace"
  fi

  # Test if we can patch the deployment
  if kubectl patch deployment test-app -n "$TEST_NAMESPACE" \
    --type='json' -p='[{"op": "add", "path": "/metadata/annotations/test", "value": "test"}]' \
    --as=system:serviceaccount:$TEST_NAMESPACE:$SERVICE_ACCOUNT &>/dev/null; then
    log_success "Can patch test deployment"
  else
    log_error "Cannot patch test deployment"
  fi
}

# Generate summary report
generate_report() {
  log_section "Test Summary Report"

  local total=$((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "                      TEST RESULTS                              "
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo -e "  Total Tests:    $total"
  echo -e "  ${GREEN}Passed:         $TESTS_PASSED${NC}"
  echo -e "  ${RED}Failed:         $TESTS_FAILED${NC}"
  echo -e "  ${CYAN}Skipped:        $TESTS_SKIPPED${NC}"
  echo ""

  if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "  Result: ${GREEN}✅ ALL TESTS PASSED${NC}"
    echo ""
    echo "  The Right Sizer RBAC configuration is correctly set up!"
  else
    echo -e "  Result: ${RED}❌ SOME TESTS FAILED${NC}"
    echo ""
    echo "  Please review the failed tests and update the RBAC configuration."
    echo "  You can apply fixes using: ./scripts/rbac/apply-rbac-fix.sh"
  fi

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
}

# Main execution
main() {
  echo "================================================"
  echo "Right Sizer RBAC Integration Test"
  echo "================================================"
  echo ""
  echo "Test Configuration:"
  echo "  Test Namespace: $TEST_NAMESPACE"
  echo "  Service Account: $SERVICE_ACCOUNT"
  echo "  Cleanup on Exit: $CLEANUP_ON_EXIT"
  echo "  Verbose Mode: $VERBOSE"
  echo ""

  # Run tests
  check_prerequisites
  setup_test_environment

  # Core tests
  test_core_permissions
  test_metrics_permissions
  test_workload_permissions
  test_autoscaling_permissions
  test_constraint_permissions
  test_storage_permissions
  test_networking_permissions
  test_namespace_permissions
  test_negative_permissions
  test_practical_scenarios

  # Generate report
  generate_report

  # Exit with appropriate code
  if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
  else
    exit 0
  fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --no-cleanup)
    CLEANUP_ON_EXIT=false
    shift
    ;;
  --verbose | -v)
    VERBOSE=true
    shift
    ;;
  --namespace)
    TEST_NAMESPACE="$2"
    shift 2
    ;;
  --help | -h)
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --no-cleanup      Don't clean up test resources on exit"
    echo "  --verbose, -v     Enable verbose output"
    echo "  --namespace NAME  Use specific test namespace"
    echo "  --help, -h        Show this help message"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use --help for usage information"
    exit 1
    ;;
  esac
done

# Run main function
main
