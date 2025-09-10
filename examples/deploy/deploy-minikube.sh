#!/bin/bash

# Right Sizer - Complete Minikube Deployment Script
# This script deploys all Right Sizer components to a minikube cluster

set -e # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="right-sizer-cluster"
NAMESPACE="right-sizer"
KUBERNETES_VERSION="v1.33.1"
MEMORY="4096"
CPUS="2"
DRIVER="docker"
VERSION=$(cat VERSION)

# Function to print colored output
print_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
  echo ""
  echo -e "${BLUE}========================================${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}========================================${NC}"
  echo ""
}

# Function to check if command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Function to wait for deployment
wait_for_deployment() {
  local namespace=$1
  local deployment=$2
  local timeout=${3:-300}

  print_info "Waiting for deployment $deployment in namespace $namespace..."
  kubectl wait --for=condition=available --timeout=${timeout}s \
    deployment/$deployment -n $namespace
}

# Function to wait for pods
wait_for_pods() {
  local namespace=$1
  local label=$2
  local timeout=${3:-300}

  print_info "Waiting for pods with label $label in namespace $namespace..."
  kubectl wait --for=condition=ready --timeout=${timeout}s \
    pods -l $label -n $namespace
}

# Check prerequisites
check_prerequisites() {
  print_header "Checking Prerequisites"

  local missing_deps=()

  if ! command_exists minikube; then
    missing_deps+=("minikube")
  fi

  if ! command_exists kubectl; then
    missing_deps+=("kubectl")
  fi

  if ! command_exists docker; then
    missing_deps+=("docker")
  fi

  if ! command_exists helm; then
    missing_deps+=("helm")
  fi

  if [ ${#missing_deps[@]} -gt 0 ]; then
    print_error "Missing required dependencies: ${missing_deps[*]}"
    print_info "Please install the missing dependencies and try again."
    exit 1
  fi

  print_success "All prerequisites are installed"
}

# Start or configure minikube
setup_minikube() {
  print_header "Setting up Minikube"

  # Check if cluster exists
  if minikube profile list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    print_info "Minikube cluster '$CLUSTER_NAME' already exists"

    # Switch to the cluster
    minikube profile $CLUSTER_NAME

    # Check if it's running
    if ! minikube status -p $CLUSTER_NAME | grep -q "Running"; then
      print_info "Starting existing cluster..."
      minikube start -p $CLUSTER_NAME
    fi
  else
    print_info "Creating new minikube cluster '$CLUSTER_NAME'..."
    minikube start \
      -p $CLUSTER_NAME \
      --kubernetes-version=$KUBERNETES_VERSION \
      --memory=$MEMORY \
      --cpus=$CPUS \
      --driver=$DRIVER \
      --feature-gates=InPlacePodVerticalScaling=true \
      --extra-config=kubelet.feature-gates=InPlacePodVerticalScaling=true \
      --extra-config=apiserver.feature-gates=InPlacePodVerticalScaling=true \
      --extra-config=scheduler.feature-gates=InPlacePodVerticalScaling=true \
      --extra-config=controller-manager.feature-gates=InPlacePodVerticalScaling=true
  fi

  # Set kubectl context
  kubectl config use-context $CLUSTER_NAME

  print_success "Minikube cluster is ready"

  # Display cluster info
  print_info "Cluster Info:"
  kubectl cluster-info
}

# Install metrics server
install_metrics_server() {
  print_header "Installing Metrics Server"

  # Check if metrics-server is already installed
  if kubectl get deployment metrics-server -n kube-system >/dev/null 2>&1; then
    print_info "Metrics server is already installed"
  else
    print_info "Installing metrics server..."
    minikube addons enable metrics-server -p $CLUSTER_NAME

    # Wait for metrics server to be ready
    wait_for_deployment "kube-system" "metrics-server" 300
  fi

  print_success "Metrics server is ready"
}

# Build and load Docker images
build_and_load_images() {
  print_header "Building and Loading Docker Images"

  # Configure docker to use minikube's docker daemon
  print_info "Configuring Docker environment..."
  eval $(minikube -p $CLUSTER_NAME docker-env)

  # Build Right-Sizer operator image
  print_info "Building Right-Sizer operator image..."
  docker build \
    --build-arg VERSION=$VERSION \
    --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
    --build-arg GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
    -t right-sizer:$VERSION \
    -t right-sizer:latest \
    -f Dockerfile .

  print_success "Right-Sizer operator image built"

  # Build Dashboard image
  print_info "Building Right-Sizer dashboard image..."
  cd ../right-sizer-dashboard
  docker build \
    -t right-sizer-dashboard:$VERSION \
    -t right-sizer-dashboard:latest \
    -f Dockerfile .
  cd ../right-sizer

  print_success "Right-Sizer dashboard image built"

  # List images
  print_info "Available images:"
  docker images | grep -E "right-sizer|REPOSITORY"
}

# Create namespace
create_namespace() {
  print_header "Creating Namespace"

  if kubectl get namespace $NAMESPACE >/dev/null 2>&1; then
    print_info "Namespace '$NAMESPACE' already exists"
  else
    print_info "Creating namespace '$NAMESPACE'..."
    kubectl create namespace $NAMESPACE
  fi

  # Label namespace for monitoring
  kubectl label namespace $NAMESPACE \
    monitoring=enabled \
    right-sizer=enabled \
    --overwrite

  print_success "Namespace is ready"
}

# Deploy Right-Sizer operator
deploy_operator() {
  print_header "Deploying Right-Sizer Operator"

  # Add Helm repo (for CRDs)
  print_info "Adding Helm repository..."
  helm repo add right-sizer https://aavishay.github.io/right-sizer/charts || true
  helm repo update

  # Create values file for local deployment
  cat >/tmp/right-sizer-values.yaml <<EOF
image:
  repository: right-sizer
  tag: latest
  pullPolicy: IfNotPresent

config:
  enabled: true
  dryRun: false
  mode: balanced
  resizeInterval: "30s"
  logLevel: info

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

metrics:
  enabled: true
  port: 8081

health:
  enabled: true
  livenessProbe:
    initialDelaySeconds: 10
    periodSeconds: 10
  readinessProbe:
    initialDelaySeconds: 5
    periodSeconds: 5

serviceMonitor:
  enabled: false

nodeSelector: {}
tolerations: []
affinity: {}
EOF

  # Install or upgrade the operator
  print_info "Installing Right-Sizer operator..."
  helm upgrade --install right-sizer ./helm \
    --namespace $NAMESPACE \
    --values /tmp/right-sizer-values.yaml \
    --wait \
    --timeout 5m

  # Wait for operator to be ready
  wait_for_deployment $NAMESPACE "right-sizer" 300

  print_success "Right-Sizer operator deployed successfully"

  # Show operator status
  print_info "Operator status:"
  kubectl get all -n $NAMESPACE -l app=right-sizer
}

# Deploy Dashboard
deploy_dashboard() {
  print_header "Deploying Right-Sizer Dashboard"

  # Create dashboard deployment
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
  labels:
    app: right-sizer-dashboard
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-dashboard
  template:
    metadata:
      labels:
        app: right-sizer-dashboard
    spec:
      containers:
      - name: dashboard
        image: right-sizer-dashboard:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
          name: http
        env:
        - name: REACT_APP_METRICS_ENDPOINT
          value: "http://right-sizer:8081/metrics"
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
  labels:
    app: right-sizer-dashboard
spec:
  type: NodePort
  ports:
  - port: 80
    targetPort: 80
    nodePort: 30080
    name: http
  selector:
    app: right-sizer-dashboard
EOF

  # Wait for dashboard to be ready
  wait_for_deployment $NAMESPACE "right-sizer-dashboard" 300

  print_success "Dashboard deployed successfully"
}

# Deploy test workloads
deploy_test_workloads() {
  print_header "Deploying Test Workloads"

  print_info "Creating test workloads..."

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: test-workloads
  labels:
    right-sizer: enabled
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: over-provisioned-app
  namespace: test-workloads
  labels:
    app: over-provisioned
    right-sizer: enabled
spec:
  replicas: 2
  selector:
    matchLabels:
      app: over-provisioned
  template:
    metadata:
      labels:
        app: over-provisioned
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: under-provisioned-app
  namespace: test-workloads
  labels:
    app: under-provisioned
    right-sizer: enabled
spec:
  replicas: 3
  selector:
    matchLabels:
      app: under-provisioned
  template:
    metadata:
      labels:
        app: under-provisioned
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 10m
            memory: 32Mi
          limits:
            cpu: 50m
            memory: 64Mi
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: correctly-sized-app
  namespace: test-workloads
  labels:
    app: correctly-sized
    right-sizer: enabled
spec:
  replicas: 2
  selector:
    matchLabels:
      app: correctly-sized
  template:
    metadata:
      labels:
        app: correctly-sized
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
        ports:
        - containerPort: 80
---
apiVersion: batch/v1
kind: Job
metadata:
  name: load-generator
  namespace: test-workloads
spec:
  template:
    spec:
      containers:
      - name: load
        image: busybox
        command:
        - /bin/sh
        - -c
        - |
          echo "Generating load on test applications..."
          for i in \$(seq 1 100); do
            wget -q -O /dev/null http://over-provisioned-app.test-workloads.svc.cluster.local 2>/dev/null || true
            wget -q -O /dev/null http://under-provisioned-app.test-workloads.svc.cluster.local 2>/dev/null || true
            wget -q -O /dev/null http://correctly-sized-app.test-workloads.svc.cluster.local 2>/dev/null || true
            sleep 1
          done
          echo "Load generation complete"
      restartPolicy: Never
  backoffLimit: 1
EOF

  # Wait for deployments
  wait_for_deployment "test-workloads" "over-provisioned-app" 180
  wait_for_deployment "test-workloads" "under-provisioned-app" 180
  wait_for_deployment "test-workloads" "correctly-sized-app" 180

  print_success "Test workloads deployed"
}

# Configure port forwarding
setup_port_forwarding() {
  print_header "Setting up Port Forwarding"

  # Kill existing port-forward processes
  pkill -f "kubectl port-forward" || true

  # Port forward for dashboard
  print_info "Setting up port forwarding for dashboard..."
  kubectl port-forward -n $NAMESPACE service/right-sizer-dashboard 3000:80 >/dev/null 2>&1 &
  DASHBOARD_PF_PID=$!

  # Port forward for metrics
  print_info "Setting up port forwarding for metrics..."
  kubectl port-forward -n $NAMESPACE service/right-sizer 8081:8081 >/dev/null 2>&1 &
  METRICS_PF_PID=$!

  sleep 2

  print_success "Port forwarding established"
  print_info "Dashboard PID: $DASHBOARD_PF_PID"
  print_info "Metrics PID: $METRICS_PF_PID"
}

# Show access information
show_access_info() {
  print_header "Access Information"

  # Get minikube IP
  MINIKUBE_IP=$(minikube -p $CLUSTER_NAME ip)

  echo -e "${GREEN}Right-Sizer is successfully deployed!${NC}"
  echo ""
  echo "📊 Dashboard Access:"
  echo "   - Via port-forward: http://localhost:3000"
  echo "   - Via NodePort: http://$MINIKUBE_IP:30080"
  echo "   - Minikube service: minikube -p $CLUSTER_NAME service right-sizer-dashboard -n $NAMESPACE"
  echo ""
  echo "📈 Metrics Endpoint:"
  echo "   - Via port-forward: http://localhost:8081/metrics"
  echo ""
  echo "🔍 Useful Commands:"
  echo "   - View operator logs: kubectl logs -f deployment/right-sizer -n $NAMESPACE"
  echo "   - View dashboard logs: kubectl logs -f deployment/right-sizer-dashboard -n $NAMESPACE"
  echo "   - Check pod resources: kubectl top pods -A"
  echo "   - View optimization events: kubectl get events -n test-workloads | grep right-sizer"
  echo "   - Watch test workloads: kubectl get pods -n test-workloads -w"
  echo ""
  echo "🛠️ Management Commands:"
  echo "   - Stop cluster: minikube stop -p $CLUSTER_NAME"
  echo "   - Delete cluster: minikube delete -p $CLUSTER_NAME"
  echo "   - SSH to node: minikube ssh -p $CLUSTER_NAME"
  echo ""
  echo "⏱️ Wait 2-3 minutes for the operator to start optimizing resources"
  echo ""
}

# Verify deployment
verify_deployment() {
  print_header "Verifying Deployment"

  local all_good=true

  # Check operator
  if kubectl get deployment right-sizer -n $NAMESPACE >/dev/null 2>&1; then
    print_success "✅ Operator is deployed"
  else
    print_error "❌ Operator is not deployed"
    all_good=false
  fi

  # Check dashboard
  if kubectl get deployment right-sizer-dashboard -n $NAMESPACE >/dev/null 2>&1; then
    print_success "✅ Dashboard is deployed"
  else
    print_error "❌ Dashboard is not deployed"
    all_good=false
  fi

  # Check CRDs
  if kubectl get crd rightsizerpolicies.rightsizer.io >/dev/null 2>&1; then
    print_success "✅ CRDs are installed"
  else
    print_error "❌ CRDs are not installed"
    all_good=false
  fi

  # Check metrics server
  if kubectl top nodes >/dev/null 2>&1; then
    print_success "✅ Metrics server is working"
  else
    print_error "❌ Metrics server is not working"
    all_good=false
  fi

  # Check test workloads
  if kubectl get deployment -n test-workloads >/dev/null 2>&1; then
    print_success "✅ Test workloads are deployed"
  else
    print_error "❌ Test workloads are not deployed"
    all_good=false
  fi

  if [ "$all_good" = true ]; then
    print_success "All components are successfully deployed!"
    return 0
  else
    print_error "Some components failed to deploy"
    return 1
  fi
}

# Clean up function
cleanup() {
  print_header "Cleanup"

  read -p "Do you want to delete the minikube cluster? (y/N) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_info "Deleting minikube cluster..."
    minikube delete -p $CLUSTER_NAME
    print_success "Cluster deleted"
  else
    print_info "Keeping cluster. To delete later, run: minikube delete -p $CLUSTER_NAME"
  fi
}

# Main function
main() {
  print_header "Right-Sizer Minikube Deployment"
  echo "Version: $VERSION"
  echo "Cluster: $CLUSTER_NAME"
  echo ""

  # Run deployment steps
  check_prerequisites
  setup_minikube
  install_metrics_server
  build_and_load_images
  create_namespace
  deploy_operator
  deploy_dashboard
  deploy_test_workloads
  setup_port_forwarding
  verify_deployment
  show_access_info

  # Optional: Open dashboard in browser
  if command_exists open; then
    read -p "Open dashboard in browser? (Y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
      open http://localhost:3000
    fi
  elif command_exists xdg-open; then
    read -p "Open dashboard in browser? (Y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
      xdg-open http://localhost:3000
    fi
  fi

  # Keep script running to maintain port forwarding
  print_info "Press Ctrl+C to stop port forwarding and exit"
  wait $DASHBOARD_PF_PID $METRICS_PF_PID
}

# Trap Ctrl+C to cleanup
trap 'echo ""; print_warning "Interrupted. Cleaning up..."; pkill -f "kubectl port-forward" || true; exit 1' INT

# Run main function
main "$@"
