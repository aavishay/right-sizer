#!/bin/bash

# Right-Sizer Deployment Monitor
# This script monitors the current Right-Sizer deployment and provides real-time status

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_header() {
  echo -e "${CYAN}========================================${NC}"
  echo -e "${CYAN}$1${NC}"
  echo -e "${CYAN}========================================${NC}"
}

NAMESPACE="right-sizer"
TEST_NAMESPACE="test-workloads"
CLUSTER_NAME="right-sizer-cluster"

# Function to check cluster connection
check_cluster() {
  print_header "Cluster Status"

  if ! kubectl cluster-info >/dev/null 2>&1; then
    print_error "Cannot connect to Kubernetes cluster"
    exit 1
  fi

  local context=$(kubectl config current-context)
  print_success "Connected to cluster: $context"

  local version=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}')
  print_info "Kubernetes version: $version"

  # Check if feature gates are enabled
  if kubectl get --raw="/api/v1" | grep -q "resize"; then
    print_success "✅ Pod resize API available"
  else
    print_warning "⚠️  Pod resize API status unclear"
  fi
}

# Function to show namespace status
show_namespace_status() {
  print_header "Namespace Status"

  if kubectl get namespace $NAMESPACE >/dev/null 2>&1; then
    print_success "✅ Namespace '$NAMESPACE' exists"
  else
    print_error "❌ Namespace '$NAMESPACE' missing"
    return 1
  fi

  if kubectl get namespace $TEST_NAMESPACE >/dev/null 2>&1; then
    print_success "✅ Test namespace '$TEST_NAMESPACE' exists"
  else
    print_warning "⚠️  Test namespace '$TEST_NAMESPACE' missing"
  fi
}

# Function to show deployment status
show_deployment_status() {
  print_header "Right-Sizer Components"

  # Check operator
  if kubectl get deployment right-sizer -n $NAMESPACE >/dev/null 2>&1; then
    local status=$(kubectl get deployment right-sizer -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
    if [[ "$status" == "True" ]]; then
      print_success "✅ Operator: Running"
    else
      print_error "❌ Operator: Not ready"
    fi
  else
    print_error "❌ Operator: Not deployed"
  fi

  # Check dashboard
  if kubectl get deployment right-sizer-dashboard -n $NAMESPACE >/dev/null 2>&1; then
    local status=$(kubectl get deployment right-sizer-dashboard -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
    if [[ "$status" == "True" ]]; then
      print_success "✅ Dashboard: Running"
    else
      print_warning "⚠️  Dashboard: Not ready"
    fi
  else
    print_warning "⚠️  Dashboard: Not deployed"
  fi

  # Show all pods in namespace
  echo ""
  print_info "All pods in $NAMESPACE namespace:"
  kubectl get pods -n $NAMESPACE -o wide 2>/dev/null || echo "No pods found"
}

# Function to show RBAC status
show_rbac_status() {
  print_header "RBAC Status"

  if kubectl get clusterrole right-sizer >/dev/null 2>&1; then
    print_success "✅ ClusterRole 'right-sizer' exists"

    # Check for pods/resize permission
    if kubectl get clusterrole right-sizer -o yaml | grep -q "pods/resize"; then
      print_success "✅ Has pods/resize permission"
    else
      print_warning "⚠️  Missing pods/resize permission"
    fi

    # Check for other key permissions
    if kubectl get clusterrole right-sizer -o yaml | grep -q "metrics.k8s.io"; then
      print_success "✅ Has metrics API permission"
    else
      print_warning "⚠️  Missing metrics API permission"
    fi
  else
    print_error "❌ ClusterRole 'right-sizer' missing"
  fi

  if kubectl get clusterrolebinding right-sizer >/dev/null 2>&1; then
    print_success "✅ ClusterRoleBinding exists"
  else
    print_error "❌ ClusterRoleBinding missing"
  fi
}

# Function to show test workloads
show_test_workloads() {
  print_header "Test Workloads"

  if kubectl get namespace $TEST_NAMESPACE >/dev/null 2>&1; then
    local pod_count=$(kubectl get pods -n $TEST_NAMESPACE --no-headers 2>/dev/null | wc -l)
    print_info "Found $pod_count test pods"

    echo ""
    kubectl get pods -n $TEST_NAMESPACE -o wide 2>/dev/null

    # Show resource usage if metrics-server is available
    echo ""
    print_info "Resource usage (if available):"
    kubectl top pods -n $TEST_NAMESPACE 2>/dev/null || print_warning "Metrics not available"
  else
    print_warning "No test workloads namespace found"
  fi
}

# Function to show metrics-server status
show_metrics_server() {
  print_header "Metrics Server"

  if kubectl get deployment metrics-server -n kube-system >/dev/null 2>&1; then
    local status=$(kubectl get deployment metrics-server -n kube-system -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
    if [[ "$status" == "True" ]]; then
      print_success "✅ Metrics-server: Running"

      # Test metrics API
      if kubectl top nodes >/dev/null 2>&1; then
        print_success "✅ Metrics API: Working"
      else
        print_warning "⚠️  Metrics API: Not ready yet"
      fi
    else
      print_warning "⚠️  Metrics-server: Not ready"
    fi
  else
    print_error "❌ Metrics-server: Not installed"
  fi
}

# Function to setup port forwarding
setup_port_forwarding() {
  print_header "Setting Up Access"

  # Kill existing port forwards
  pkill -f "kubectl port-forward" 2>/dev/null || true
  sleep 1

  # Dashboard port forward
  if kubectl get service right-sizer-dashboard -n $NAMESPACE >/dev/null 2>&1; then
    kubectl port-forward -n $NAMESPACE svc/right-sizer-dashboard 3000:80 >/dev/null 2>&1 &
    DASHBOARD_PID=$!
    sleep 2
    print_success "✅ Dashboard: http://localhost:3000"
  fi

  # Operator metrics (if running)
  if kubectl get service right-sizer -n $NAMESPACE >/dev/null 2>&1; then
    kubectl port-forward -n $NAMESPACE svc/right-sizer 8081:8081 >/dev/null 2>&1 &
    METRICS_PID=$!
    sleep 1
    print_success "✅ Operator metrics: http://localhost:8081/metrics"
  fi

  # NodePort access
  local minikube_ip=$(minikube -p $CLUSTER_NAME ip 2>/dev/null || echo "N/A")
  if [[ "$minikube_ip" != "N/A" ]]; then
    print_info "🔗 NodePort access: http://$minikube_ip:30080"
  fi
}

# Function to monitor logs in real-time
monitor_logs() {
  print_header "Log Monitoring"

  echo "Available components for log monitoring:"
  echo "1. Dashboard logs"
  echo "2. Operator logs (if running)"
  echo "3. Test workload logs"
  echo "4. Exit monitoring"
  echo ""

  while true; do
    read -p "Select component to monitor (1-4): " choice

    case $choice in
    1)
      if kubectl get deployment right-sizer-dashboard -n $NAMESPACE >/dev/null 2>&1; then
        print_info "Monitoring dashboard logs (Ctrl+C to stop)..."
        kubectl logs -f deployment/right-sizer-dashboard -n $NAMESPACE
      else
        print_error "Dashboard not found"
      fi
      ;;
    2)
      if kubectl get deployment right-sizer -n $NAMESPACE >/dev/null 2>&1; then
        print_info "Monitoring operator logs (Ctrl+C to stop)..."
        kubectl logs -f deployment/right-sizer -n $NAMESPACE
      else
        print_error "Operator not found"
      fi
      ;;
    3)
      print_info "Test workload pods:"
      kubectl get pods -n $TEST_NAMESPACE --no-headers 2>/dev/null | nl
      read -p "Enter pod number to monitor: " pod_num
      local pod_name=$(kubectl get pods -n $TEST_NAMESPACE --no-headers 2>/dev/null | sed -n "${pod_num}p" | awk '{print $1}')
      if [[ -n "$pod_name" ]]; then
        print_info "Monitoring logs for $pod_name (Ctrl+C to stop)..."
        kubectl logs -f $pod_name -n $TEST_NAMESPACE
      else
        print_error "Invalid pod selection"
      fi
      ;;
    4)
      break
      ;;
    *)
      print_error "Invalid choice"
      ;;
    esac
    echo ""
  done
}

# Function to show helpful commands
show_commands() {
  print_header "Useful Commands"

  echo "# View all Right-Sizer resources"
  echo "kubectl get all -n $NAMESPACE"
  echo ""
  echo "# Check test workloads"
  echo "kubectl get pods -n $TEST_NAMESPACE"
  echo "kubectl top pods -n $TEST_NAMESPACE"
  echo ""
  echo "# Check RBAC"
  echo "kubectl get clusterrole right-sizer -o yaml"
  echo "kubectl auth can-i resize pods --as=system:serviceaccount:$NAMESPACE:right-sizer"
  echo ""
  echo "# Dashboard access"
  echo "minikube service right-sizer-dashboard -n $NAMESPACE --url"
  echo ""
  echo "# Force pod restart"
  echo "kubectl rollout restart deployment/right-sizer-dashboard -n $NAMESPACE"
  echo ""
  echo "# Clean up"
  echo "kubectl delete namespace $NAMESPACE"
  echo "kubectl delete namespace $TEST_NAMESPACE"
}

# Function to create test load
create_test_load() {
  print_header "Creating Test Load"

  print_info "Creating over-provisioned test deployment..."

  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-load-$(date +%s)
  namespace: $TEST_NAMESPACE
  labels:
    created-by: monitor-script
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-load
  template:
    metadata:
      labels:
        app: test-load
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 600m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        ports:
        - containerPort: 80
EOF

  print_success "Test load created in $TEST_NAMESPACE namespace"
}

# Cleanup function
cleanup() {
  echo ""
  print_warning "Cleaning up port forwards..."
  pkill -f "kubectl port-forward" 2>/dev/null || true
  exit 0
}

# Main function
main() {
  echo ""
  print_header "Right-Sizer Deployment Monitor"
  echo "Monitoring Right-Sizer deployment on Minikube"
  echo ""

  # Run all checks
  check_cluster
  echo ""
  show_namespace_status
  echo ""
  show_deployment_status
  echo ""
  show_rbac_status
  echo ""
  show_metrics_server
  echo ""
  show_test_workloads
  echo ""

  # Setup access
  setup_port_forwarding
  echo ""

  # Interactive menu
  while true; do
    print_header "Monitoring Options"
    echo "1. Refresh status"
    echo "2. Monitor logs"
    echo "3. Show useful commands"
    echo "4. Create test load"
    echo "5. Access dashboard"
    echo "6. Exit"
    echo ""

    read -p "Select option (1-6): " choice

    case $choice in
    1)
      main
      ;;
    2)
      monitor_logs
      ;;
    3)
      show_commands
      ;;
    4)
      create_test_load
      ;;
    5)
      if command -v open >/dev/null; then
        open http://localhost:3000
      elif command -v xdg-open >/dev/null; then
        xdg-open http://localhost:3000
      else
        print_info "Open http://localhost:3000 in your browser"
      fi
      ;;
    6)
      cleanup
      ;;
    *)
      print_error "Invalid choice"
      ;;
    esac
    echo ""
  done
}

# Trap Ctrl+C
trap cleanup INT

# Run main function
main "$@"
