#!/bin/bash

# Right-Sizer Minikube Deployment Script
# This script deploys the Right-Sizer Helm chart to a Minikube cluster

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROFILE_NAME="${MINIKUBE_PROFILE:-right-sizer}"
NAMESPACE="${NAMESPACE:-right-sizer}"
RELEASE_NAME="${RELEASE_NAME:-right-sizer}"
K8S_VERSION="${K8S_VERSION:-stable}"
CPUS="${CPUS:-4}"
MEMORY="${MEMORY:-6144}"
DRIVER="${DRIVER:-docker}"
HELM_TIMEOUT="${HELM_TIMEOUT:-5m}"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Functions
print_header() {
  echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
}

print_error() {
  echo -e "${RED}✗${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

check_command() {
  if ! command -v $1 &>/dev/null; then
    print_error "$1 is not installed. Please install it first."
    exit 1
  fi
}

cleanup() {
  print_warning "Cleaning up..."
  if [ "$1" == "error" ]; then
    print_error "Deployment failed. Check logs above for details."
  fi
}

trap 'cleanup error' ERR

# Parse command line arguments
ACTION="deploy"
SKIP_BUILD=false
SKIP_CLUSTER=false
USE_LOCAL_IMAGE=true
ENABLE_DASHBOARD=false
ENABLE_MONITORING=false
DRY_RUN=false
VALUES_FILE=""

while [[ $# -gt 0 ]]; do
  case $1 in
  --undeploy | --delete)
    ACTION="undeploy"
    shift
    ;;
  --upgrade)
    ACTION="upgrade"
    shift
    ;;
  --status)
    ACTION="status"
    shift
    ;;
  --skip-build)
    SKIP_BUILD=true
    shift
    ;;
  --skip-cluster)
    SKIP_CLUSTER=true
    shift
    ;;
  --use-registry)
    USE_LOCAL_IMAGE=false
    shift
    ;;
  --enable-dashboard)
    ENABLE_DASHBOARD=true
    shift
    ;;
  --enable-monitoring)
    ENABLE_MONITORING=true
    shift
    ;;
  --dry-run)
    DRY_RUN=true
    shift
    ;;
  --values)
    VALUES_FILE="$2"
    shift 2
    ;;
  --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  --profile)
    PROFILE_NAME="$2"
    shift 2
    ;;
  -h | --help)
    cat <<EOF
Usage: $0 [OPTIONS]

Deploy Right-Sizer to Minikube cluster.

OPTIONS:
    --undeploy, --delete    Uninstall Right-Sizer and cleanup
    --upgrade              Upgrade existing deployment
    --status               Show deployment status
    --skip-build           Skip building local Docker image
    --skip-cluster         Skip Minikube cluster setup
    --use-registry         Use registry image instead of local build
    --enable-dashboard     Enable Kubernetes dashboard
    --enable-monitoring    Enable Prometheus and Grafana
    --dry-run             Show what would be deployed without deploying
    --values FILE          Custom values file for Helm
    --namespace NAME       Kubernetes namespace (default: right-sizer)
    --profile NAME         Minikube profile name (default: right-sizer)
    -h, --help            Show this help message

EXAMPLES:
    $0                                    # Deploy with defaults
    $0 --enable-monitoring                # Deploy with monitoring stack
    $0 --values custom-values.yaml        # Deploy with custom values
    $0 --undeploy                        # Remove deployment
    $0 --status                          # Check deployment status

EOF
    exit 0
    ;;
  *)
    print_error "Unknown option: $1"
    exit 1
    ;;
  esac
done

# Check prerequisites
print_header "Checking Prerequisites"

check_command minikube
check_command kubectl
check_command helm
check_command docker

print_success "All required tools are installed"

# Show current configuration
print_header "Configuration"
echo "Profile Name: $PROFILE_NAME"
echo "Namespace: $NAMESPACE"
echo "Release Name: $RELEASE_NAME"
echo "Kubernetes Version: $K8S_VERSION"
echo "CPUs: $CPUS"
echo "Memory: ${MEMORY}MB"
echo "Driver: $DRIVER"
echo "Use Local Image: $USE_LOCAL_IMAGE"

# Handle different actions
case $ACTION in
undeploy)
  print_header "Undeploying Right-Sizer"

  # Check if release exists
  if helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
    print_info "Uninstalling Helm release..."
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE"
    print_success "Helm release uninstalled"
  else
    print_warning "Release $RELEASE_NAME not found in namespace $NAMESPACE"
  fi

  # Delete namespace if empty
  if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    if [ -z "$(kubectl get all -n "$NAMESPACE" -o name 2>/dev/null)" ]; then
      print_info "Deleting empty namespace..."
      kubectl delete namespace "$NAMESPACE"
      print_success "Namespace deleted"
    else
      print_warning "Namespace $NAMESPACE is not empty, keeping it"
    fi
  fi

  # Optionally delete Minikube profile
  read -p "Delete Minikube profile '$PROFILE_NAME'? (y/N) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    minikube delete -p "$PROFILE_NAME"
    print_success "Minikube profile deleted"
  fi

  exit 0
  ;;

status)
  print_header "Deployment Status"

  # Check Minikube status
  echo -e "\n${CYAN}Minikube Status:${NC}"
  minikube status -p "$PROFILE_NAME" || {
    print_error "Minikube profile '$PROFILE_NAME' not found"
    exit 1
  }

  # Check Helm release
  echo -e "\n${CYAN}Helm Release:${NC}"
  helm list -n "$NAMESPACE" | grep "$RELEASE_NAME" || {
    print_warning "Release '$RELEASE_NAME' not found"
  }

  # Check pods
  echo -e "\n${CYAN}Pods:${NC}"
  kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer

  # Check deployment
  echo -e "\n${CYAN}Deployment:${NC}"
  kubectl get deployment -n "$NAMESPACE" "$RELEASE_NAME"

  # Check service
  echo -e "\n${CYAN}Service:${NC}"
  kubectl get service -n "$NAMESPACE" "$RELEASE_NAME"

  # Check ConfigMap
  echo -e "\n${CYAN}ConfigMap:${NC}"
  kubectl get configmap -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer

  # Recent logs
  echo -e "\n${CYAN}Recent Logs:${NC}"
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=20

  exit 0
  ;;

upgrade)
  print_header "Upgrading Right-Sizer"
  HELM_ACTION="upgrade"
  ;;

*)
  print_header "Deploying Right-Sizer"
  HELM_ACTION="upgrade --install"
  ;;
esac

# Setup Minikube cluster
if [ "$SKIP_CLUSTER" = false ]; then
  print_header "Setting up Minikube Cluster"

  # Check if profile exists
  if minikube status -p "$PROFILE_NAME" &>/dev/null; then
    print_info "Minikube profile '$PROFILE_NAME' already exists"
    print_info "Starting cluster if stopped..."
    minikube start -p "$PROFILE_NAME"
  else
    print_info "Creating new Minikube profile '$PROFILE_NAME'..."
    minikube start \
      -p "$PROFILE_NAME" \
      --kubernetes-version="$K8S_VERSION" \
      --driver="$DRIVER" \
      --cpus="$CPUS" \
      --memory="${MEMORY}" \
      --addons=metrics-server
  fi

  print_success "Minikube cluster ready"

  # Set kubectl context
  kubectl config use-context "$PROFILE_NAME"
  print_success "kubectl context set to '$PROFILE_NAME'"

  # Enable addons
  print_info "Enabling required addons..."
  minikube -p "$PROFILE_NAME" addons enable metrics-server
  print_success "metrics-server addon enabled"

  if [ "$ENABLE_DASHBOARD" = true ]; then
    minikube -p "$PROFILE_NAME" addons enable dashboard
    minikube -p "$PROFILE_NAME" addons enable ingress
    print_success "Dashboard and ingress addons enabled"
  fi

  # Wait for metrics-server to be ready
  print_info "Waiting for metrics-server to be ready..."
  kubectl wait --for=condition=Ready pods -n kube-system -l k8s-app=metrics-server --timeout=120s || {
    print_warning "metrics-server took too long to start, continuing anyway..."
  }
else
  print_warning "Skipping cluster setup (--skip-cluster flag set)"
fi

# Build Docker image
if [ "$USE_LOCAL_IMAGE" = true ] && [ "$SKIP_BUILD" = false ]; then
  print_header "Building Docker Image"

  print_info "Configuring Docker to use Minikube's daemon..."
  eval $(minikube -p "$PROFILE_NAME" docker-env)

  print_info "Building multi-platform right-sizer:local image (linux/amd64, linux/arm64)..."
  cd "$ROOT_DIR"

  # Get version info
  VERSION=$(cat VERSION 2>/dev/null || echo "0.0.0-dev")
  GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
  BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  docker buildx create --use --name minikube-builder --driver docker-container 2>/dev/null || true
  docker buildx build \
    --platform linux/amd64,linux/arm64 \
    -t right-sizer:local \
    --build-arg VERSION="$VERSION" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    -f Dockerfile \
    --load \
    .

  print_success "Multi-platform Docker image built successfully"

  # List the image
  docker images | grep right-sizer
  print_info "Image supports platforms: linux/amd64, linux/arm64"
else
  if [ "$SKIP_BUILD" = true ]; then
    print_warning "Skipping image build (--skip-build flag set)"
  else
    print_info "Using registry image (--use-registry flag set)"
  fi
fi

# Create namespace
print_header "Preparing Namespace"

if kubectl get namespace "$NAMESPACE" &>/dev/null; then
  print_info "Namespace '$NAMESPACE' already exists"
else
  print_info "Creating namespace '$NAMESPACE'..."
  kubectl create namespace "$NAMESPACE"
  print_success "Namespace created"
fi

# Prepare Helm values
print_header "Preparing Helm Values"

HELM_VALUES=""

# Use custom values file if provided
if [ -n "$VALUES_FILE" ]; then
  if [ -f "$VALUES_FILE" ]; then
    HELM_VALUES="$HELM_VALUES -f $VALUES_FILE"
    print_info "Using custom values from: $VALUES_FILE"
  else
    print_error "Values file not found: $VALUES_FILE"
    exit 1
  fi
fi

# Set image values based on build choice
if [ "$USE_LOCAL_IMAGE" = true ]; then
  HELM_VALUES="$HELM_VALUES --set image.repository=right-sizer"
  HELM_VALUES="$HELM_VALUES --set image.tag=local"
  HELM_VALUES="$HELM_VALUES --set image.pullPolicy=IfNotPresent"
fi

# Add monitoring configuration if enabled
if [ "$ENABLE_MONITORING" = true ]; then
  HELM_VALUES="$HELM_VALUES --set serviceMonitor.enabled=true"
  HELM_VALUES="$HELM_VALUES --set rightsizerConfig.observability.enableMetricsExport=true"

  # Deploy Prometheus if not exists
  if ! helm list -n monitoring | grep -q prometheus; then
    print_info "Installing Prometheus operator..."
    kubectl create namespace monitoring 2>/dev/null || true
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    helm install prometheus prometheus-community/kube-prometheus-stack \
      -n monitoring \
      --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
  fi
fi

# Deploy/Upgrade Right-Sizer
print_header "Deploying Right-Sizer with Helm"

if [ "$DRY_RUN" = true ]; then
  print_warning "DRY RUN MODE - Not actually deploying"
  echo "Would run: helm $HELM_ACTION $RELEASE_NAME ./helm -n $NAMESPACE $HELM_VALUES"

  # Show generated manifests
  print_info "Generated manifests:"
  helm template "$RELEASE_NAME" "$ROOT_DIR/helm" -n "$NAMESPACE" $HELM_VALUES
  exit 0
fi

cd "$ROOT_DIR"

print_info "Running Helm $HELM_ACTION..."
helm $HELM_ACTION "$RELEASE_NAME" ./helm \
  -n "$NAMESPACE" \
  --create-namespace \
  --timeout "$HELM_TIMEOUT" \
  --wait \
  $HELM_VALUES

print_success "Helm deployment completed"

# Wait for deployment to be ready
print_header "Waiting for Deployment"

print_info "Waiting for deployment to be available..."
kubectl wait --for=condition=available \
  deployment/"$RELEASE_NAME" \
  -n "$NAMESPACE" \
  --timeout=120s

print_success "Deployment is ready"

# Show deployment status
print_header "Deployment Summary"

# Get pod status
echo -e "\n${CYAN}Pod Status:${NC}"
kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer

# Get service info
echo -e "\n${CYAN}Service Info:${NC}"
kubectl get service -n "$NAMESPACE" "$RELEASE_NAME"

# Show recent logs
echo -e "\n${CYAN}Recent Logs:${NC}"
kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=20

# Create sample workload
print_header "Creating Sample Workload (Optional)"

read -p "Deploy sample workload for testing? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  print_info "Creating sample deployment..."
  kubectl create namespace demo-workload 2>/dev/null || true

  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-demo
  namespace: demo-workload
  labels:
    app: nginx-demo
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx-demo
  template:
    metadata:
      labels:
        app: nginx-demo
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
EOF

  print_success "Sample workload deployed to demo-workload namespace"
  echo "Watch it with: kubectl get pods -n demo-workload -w"
fi

# Port forwarding options
print_header "Access Options"

echo -e "${CYAN}1. Port Forward to Access Metrics:${NC}"
echo "   kubectl port-forward -n $NAMESPACE svc/$RELEASE_NAME 9090:9090"
echo "   Then access: http://localhost:9090/metrics"

echo -e "\n${CYAN}2. View Logs:${NC}"
echo "   kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"

echo -e "\n${CYAN}3. Access Kubernetes Dashboard (if enabled):${NC}"
if [ "$ENABLE_DASHBOARD" = true ]; then
  echo "   minikube -p $PROFILE_NAME dashboard"
else
  echo "   Enable with: minikube -p $PROFILE_NAME addons enable dashboard"
  echo "   Then run: minikube -p $PROFILE_NAME dashboard"
fi

echo -e "\n${CYAN}4. Execute Shell in Pod:${NC}"
POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD_NAME" ]; then
  echo "   kubectl exec -it -n $NAMESPACE $POD_NAME -- /bin/sh"
else
  echo "   kubectl exec -it -n $NAMESPACE <pod-name> -- /bin/sh"
fi

# Health check
print_header "Health Check"

print_info "Checking operator health..."
POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -n "$POD_NAME" ]; then
  # Check readiness probe
  if kubectl exec -n "$NAMESPACE" "$POD_NAME" -- wget -q -O- http://localhost:8081/readyz 2>/dev/null; then
    print_success "Readiness check passed"
  else
    print_warning "Readiness check failed or not accessible"
  fi

  # Check liveness probe
  if kubectl exec -n "$NAMESPACE" "$POD_NAME" -- wget -q -O- http://localhost:8081/healthz 2>/dev/null; then
    print_success "Liveness check passed"
  else
    print_warning "Liveness check failed or not accessible"
  fi
fi

# Save deployment info
print_header "Saving Deployment Information"

DEPLOYMENT_INFO="$HOME/.right-sizer-deployment-info"
cat >"$DEPLOYMENT_INFO" <<EOF
# Right-Sizer Deployment Information
# Generated: $(date)

PROFILE_NAME=$PROFILE_NAME
NAMESPACE=$NAMESPACE
RELEASE_NAME=$RELEASE_NAME
DEPLOYMENT_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Quick Commands:
# Status: minikube -p $PROFILE_NAME status
# Logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f
# Port-forward: kubectl port-forward -n $NAMESPACE svc/$RELEASE_NAME 9090:9090
# Uninstall: helm uninstall $RELEASE_NAME -n $NAMESPACE
# Delete cluster: minikube delete -p $PROFILE_NAME
EOF

print_success "Deployment info saved to: $DEPLOYMENT_INFO"

print_header "Deployment Complete!"
print_success "Right-Sizer has been successfully deployed to Minikube!"
echo -e "\n${GREEN}Profile:${NC} $PROFILE_NAME"
echo -e "${GREEN}Namespace:${NC} $NAMESPACE"
echo -e "${GREEN}Release:${NC} $RELEASE_NAME"

echo -e "\n${CYAN}Next Steps:${NC}"
echo "1. Monitor the operator: make mk-logs"
echo "2. Check metrics: kubectl port-forward -n $NAMESPACE svc/$RELEASE_NAME 9090:9090"
echo "3. Deploy workloads to test resizing"
echo "4. View the dashboard: minikube -p $PROFILE_NAME dashboard"

echo -e "\n${YELLOW}To remove the deployment:${NC}"
echo "   $0 --undeploy"

exit 0
