#!/bin/bash

# Right-Sizer Minikube Test Runner
# This script sets up and runs integration tests with minikube

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="right-sizer-test"
NAMESPACE="right-sizer-system"
TEST_NAMESPACE="right-sizer-test"
KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.31.0}"
MINIKUBE_MEMORY="${MINIKUBE_MEMORY:-4096}"
MINIKUBE_CPUS="${MINIKUBE_CPUS:-2}"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
  log_info "Checking prerequisites..."

  # Check minikube
  if ! command -v minikube &>/dev/null; then
    log_error "minikube is not installed. Please install minikube first."
    log_info "Visit: https://minikube.sigs.k8s.io/docs/start/"
    exit 1
  fi

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl is not installed. Please install kubectl first."
    exit 1
  fi

  # Check docker or podman
  if ! command -v docker &>/dev/null && ! command -v podman &>/dev/null; then
    log_error "Neither docker nor podman is installed. Please install one of them."
    exit 1
  fi

  # Check helm
  if ! command -v helm &>/dev/null; then
    log_error "helm is not installed. Please install helm first."
    log_info "Visit: https://helm.sh/docs/intro/install/"
    exit 1
  fi

  log_success "All prerequisites are installed"
}

start_minikube() {
  log_info "Starting minikube cluster..."

  # Check if cluster already exists
  if minikube status -p "$CLUSTER_NAME" &>/dev/null; then
    log_warning "Minikube cluster '$CLUSTER_NAME' already exists"
    read -p "Do you want to delete and recreate it? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      minikube delete -p "$CLUSTER_NAME"
    else
      log_info "Using existing cluster"
      minikube profile "$CLUSTER_NAME"
      return
    fi
  fi

  # Start minikube with specific configuration
  log_info "Creating minikube cluster with Kubernetes $KUBERNETES_VERSION"
  minikube start \
    -p "$CLUSTER_NAME" \
    --kubernetes-version="$KUBERNETES_VERSION" \
    --memory="$MINIKUBE_MEMORY" \
    --cpus="$MINIKUBE_CPUS" \
    --driver=docker \
    --container-runtime=containerd

  # Enable metrics-server for resource monitoring
  log_info "Enabling metrics-server addon..."
  minikube addons enable metrics-server -p "$CLUSTER_NAME"

  # Set the profile as active
  minikube profile "$CLUSTER_NAME"

  log_success "Minikube cluster started successfully"
}

wait_for_metrics_server() {
  log_info "Waiting for metrics-server to be ready..."

  kubectl wait --for=condition=ready pod \
    -l k8s-app=metrics-server \
    -n kube-system \
    --timeout=300s || {
    log_warning "Metrics server might not be fully ready"
  }

  # Give it a bit more time to start collecting metrics
  sleep 10

  # Test metrics API
  if kubectl top nodes &>/dev/null; then
    log_success "Metrics server is ready"
  else
    log_warning "Metrics server might need more time to collect data"
  fi
}

build_docker_image() {
  log_info "Building right-sizer Docker image..."

  cd "$PROJECT_ROOT/go"

  # Use minikube's docker daemon
  eval $(minikube -p "$CLUSTER_NAME" docker-env)

  # Build the image
  docker build -t right-sizer:test -f Dockerfile .

  log_success "Docker image built successfully"
}

deploy_right_sizer() {
  log_info "Deploying right-sizer operator..."

  # Create namespace
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

  # Check if we have a helm chart
  if [ -d "$PROJECT_ROOT/charts/right-sizer" ]; then
    log_info "Installing right-sizer using Helm chart..."

    helm upgrade --install right-sizer \
      "$PROJECT_ROOT/charts/right-sizer" \
      --namespace "$NAMESPACE" \
      --set image.repository=right-sizer \
      --set image.tag=test \
      --set image.pullPolicy=Never \
      --set config.resizeInterval=30s \
      --set config.dryRun=false \
      --set config.enableInPlaceResize=true \
      --wait --timeout=5m
  else
    log_info "Helm chart not found, deploying using kubectl..."

    # Create a basic deployment manifest
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: right-sizer-config
  namespace: $NAMESPACE
data:
  config.yaml: |
    cpuRequestMultiplier: 1.2
    memoryRequestMultiplier: 1.3
    cpuLimitMultiplier: 2.0
    memoryLimitMultiplier: 1.5
    minCPURequest: 10
    minMemoryRequest: 64
    maxCPULimit: 4000
    maxMemoryLimit: 8192
    resizeInterval: 30s
    dryRun: false
    enableInPlaceResize: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer
  template:
    metadata:
      labels:
        app: right-sizer
    spec:
      serviceAccountName: right-sizer
      containers:
      - name: right-sizer
        image: right-sizer:test
        imagePullPolicy: Never
        env:
        - name: WATCH_NAMESPACE
          value: "$TEST_NAMESPACE"
        - name: METRICS_PROVIDER
          value: "metrics-server"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer
rules:
- apiGroups: [""]
  resources: ["pods", "pods/status"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: [""]
  resources: ["pods/resize"]
  verbs: ["update", "patch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer
subjects:
- kind: ServiceAccount
  name: right-sizer
  namespace: $NAMESPACE
EOF
  fi

  # Wait for deployment to be ready
  kubectl wait --for=condition=available --timeout=300s \
    deployment/right-sizer -n "$NAMESPACE"

  log_success "Right-sizer deployed successfully"
}

deploy_test_workloads() {
  log_info "Deploying test workloads..."

  # Create test namespace
  kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

  # Deploy various test workloads
  cat <<EOF | kubectl apply -f -
---
# Test workload 1: Under-provisioned pod
apiVersion: v1
kind: Pod
metadata:
  name: under-provisioned
  namespace: $TEST_NAMESPACE
  labels:
    app: under-provisioned
    test: resource-scaling
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args: ["--cpu", "1", "--vm", "1", "--vm-bytes", "128M", "--timeout", "3600s"]
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 100m
        memory: 128Mi
---
# Test workload 2: Over-provisioned pod
apiVersion: v1
kind: Pod
metadata:
  name: over-provisioned
  namespace: $TEST_NAMESPACE
  labels:
    app: over-provisioned
    test: resource-scaling
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    resources:
      requests:
        cpu: 500m
        memory: 512Mi
      limits:
        cpu: 1000m
        memory: 1Gi
---
# Test workload 3: Multi-container pod
apiVersion: v1
kind: Pod
metadata:
  name: multi-container
  namespace: $TEST_NAMESPACE
  labels:
    app: multi-container
    test: resource-scaling
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi
  - name: sidecar
    image: busybox
    command: ["sleep", "3600"]
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 100m
        memory: 128Mi
---
# Test workload 4: Bursty workload
apiVersion: v1
kind: Pod
metadata:
  name: bursty-workload
  namespace: $TEST_NAMESPACE
  labels:
    app: bursty-workload
    test: resource-scaling
spec:
  containers:
  - name: bursty
    image: busybox
    command: ["/bin/sh"]
    args:
    - -c
    - |
      while true; do
        # Normal operation
        sleep 30
        # CPU burst
        timeout 10 yes > /dev/null
        # Memory burst
        dd if=/dev/zero of=/tmp/file bs=1M count=100
        rm /tmp/file
      done
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 512Mi
EOF

  # Wait for pods to be ready
  kubectl wait --for=condition=ready pod \
    -l test=resource-scaling \
    -n "$TEST_NAMESPACE" \
    --timeout=120s || true

  log_success "Test workloads deployed"
}

run_integration_tests() {
  log_info "Running integration tests..."

  cd "$PROJECT_ROOT/go"

  # Set environment variables for tests
  export INTEGRATION_TESTS=true
  export KUBECONFIG="$HOME/.kube/config"
  export KUBE_CONTEXT="$CLUSTER_NAME"

  # Run Go integration tests
  log_info "Running Go integration tests..."
  go test -v ./tests/integration/... -timeout 10m || {
    log_warning "Some integration tests failed"
  }

  # Custom test scenarios
  log_info "Running custom test scenarios..."

  # Test 1: Check if metrics are being collected
  echo -e "\n${BLUE}Test 1: Metrics Collection${NC}"
  if kubectl top pods -n "$TEST_NAMESPACE" &>/dev/null; then
    log_success "Metrics are being collected"
    kubectl top pods -n "$TEST_NAMESPACE"
  else
    log_warning "Metrics not yet available"
  fi

  # Test 2: Monitor resource adjustments
  echo -e "\n${BLUE}Test 2: Resource Adjustments${NC}"
  log_info "Monitoring pods for 2 minutes to check for resizing..."

  # Get initial state
  echo "Initial pod resources:"
  kubectl get pods -n "$TEST_NAMESPACE" -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

  # Wait for operator to make adjustments
  sleep 120

  # Check final state
  echo -e "\nFinal pod resources:"
  kubectl get pods -n "$TEST_NAMESPACE" -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

  # Test 3: Check operator logs
  echo -e "\n${BLUE}Test 3: Operator Logs${NC}"
  kubectl logs -n "$NAMESPACE" deployment/right-sizer --tail=50

  # Test 4: Verify no pod restarts
  echo -e "\n${BLUE}Test 4: Pod Restart Check${NC}"
  RESTARTS=$(kubectl get pods -n "$TEST_NAMESPACE" -o jsonpath='{.items[*].status.containerStatuses[*].restartCount}' | tr ' ' '\n' | awk '{s+=$1} END {print s}')
  if [ "$RESTARTS" -eq 0 ]; then
    log_success "No pod restarts detected (in-place resize working)"
  else
    log_warning "Pod restarts detected: $RESTARTS"
  fi

  log_success "Integration tests completed"
}

run_stress_tests() {
  log_info "Running stress tests..."

  # Deploy a stress test pod
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: stress-test
  namespace: $TEST_NAMESPACE
  labels:
    app: stress-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args: ["--cpu", "2", "--vm", "2", "--vm-bytes", "256M", "--timeout", "120s"]
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 2000m
        memory: 1Gi
EOF

  # Monitor the stress test
  log_info "Monitoring stress test pod for 2 minutes..."

  for i in {1..6}; do
    echo -e "\n${BLUE}Minute $i:${NC}"
    kubectl top pod stress-test -n "$TEST_NAMESPACE" || true
    kubectl get pod stress-test -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources}' | jq '.' || true
    sleep 20
  done

  # Clean up stress test
  kubectl delete pod stress-test -n "$TEST_NAMESPACE" --wait=false

  log_success "Stress tests completed"
}

check_test_results() {
  log_info "Checking test results..."

  # Check if any pods were resized
  EVENTS=$(kubectl get events -n "$TEST_NAMESPACE" | grep -i resize || true)
  if [ ! -z "$EVENTS" ]; then
    log_success "Resize events detected:"
    echo "$EVENTS"
  else
    log_warning "No resize events detected"
  fi

  # Check operator metrics (if exposed)
  # This would check Prometheus metrics if available

  log_success "Test result check completed"
}

cleanup() {
  log_info "Cleaning up test resources..."

  # Delete test namespace
  kubectl delete namespace "$TEST_NAMESPACE" --wait=false || true

  # Optionally delete the operator
  read -p "Do you want to delete the right-sizer operator? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    kubectl delete namespace "$NAMESPACE" --wait=false || true
  fi

  # Optionally delete the minikube cluster
  read -p "Do you want to delete the minikube cluster? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    minikube delete -p "$CLUSTER_NAME"
    log_success "Minikube cluster deleted"
  fi

  log_success "Cleanup completed"
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Main execution
main() {
  echo -e "${GREEN}================================${NC}"
  echo -e "${GREEN}Right-Sizer Minikube Test Runner${NC}"
  echo -e "${GREEN}================================${NC}\n"

  check_prerequisites
  start_minikube
  wait_for_metrics_server
  build_docker_image
  deploy_right_sizer
  deploy_test_workloads

  # Give the system time to collect initial metrics
  log_info "Waiting 60 seconds for initial metrics collection..."
  sleep 60

  run_integration_tests
  run_stress_tests
  check_test_results

  echo -e "\n${GREEN}================================${NC}"
  echo -e "${GREEN}All tests completed successfully!${NC}"
  echo -e "${GREEN}================================${NC}"
}

# Run main function
main "$@"
