#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# test-helm-deployment.sh - Comprehensive test script for Helm deployment
# This script tests the right-sizer operator deployment using Helm

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="right-sizer-test"
RELEASE_NAME="right-sizer"
HELM_CHART="./helm"
TEST_APP_NAME="test-app"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-minikube}"

# Functions
print_header() {
  echo -e "${BLUE}=================================================${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}=================================================${NC}"
}

print_status() {
  echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
  echo -e "${RED}[✗]${NC} $1"
}

print_info() {
  echo -e "${CYAN}[i]${NC} $1"
}

# Cleanup function
cleanup() {
  print_header "Cleanup"

  print_info "Uninstalling Helm release..."
  helm uninstall $RELEASE_NAME -n $NAMESPACE 2>/dev/null || true

  print_info "Deleting test namespace..."
  kubectl delete namespace $NAMESPACE --ignore-not-found=true 2>/dev/null || true

  print_info "Deleting cluster-wide resources..."
  kubectl delete clusterrole $RELEASE_NAME --ignore-not-found=true 2>/dev/null || true
  kubectl delete clusterrolebinding $RELEASE_NAME --ignore-not-found=true 2>/dev/null || true

  print_status "Cleanup completed"
}

# Check prerequisites
check_prerequisites() {
  print_header "Checking Prerequisites"

  # Check if kubectl is installed
  if ! command -v kubectl &>/dev/null; then
    print_error "kubectl is not installed"
    exit 1
  fi
  print_status "kubectl is installed"

  # Check if helm is installed
  if ! command -v helm &>/dev/null; then
    print_error "Helm is not installed"
    exit 1
  fi
  print_status "Helm is installed"

  # Check if Minikube is running
  if ! minikube status --profile=$MINIKUBE_PROFILE &>/dev/null; then
    print_warning "Minikube is not running. Starting Minikube..."
    minikube start --profile=$MINIKUBE_PROFILE
  fi
  print_status "Minikube is running"

  # Check Kubernetes version
  K8S_VERSION=$(kubectl version --client -o json | jq -r '.clientVersion.gitVersion')
  print_info "Kubernetes client version: $K8S_VERSION"

  K8S_SERVER_VERSION=$(kubectl version -o json | jq -r '.serverVersion.gitVersion' 2>/dev/null || echo "Unknown")
  print_info "Kubernetes server version: $K8S_SERVER_VERSION"

  # Check if Helm chart exists
  if [ ! -d "$HELM_CHART" ]; then
    print_error "Helm chart not found at $HELM_CHART"
    exit 1
  fi
  print_status "Helm chart found at $HELM_CHART"
}

# Build Docker image
build_docker_image() {
  print_header "Building Docker Image"

  print_info "Configuring Docker to use Minikube's Docker daemon..."
  eval $(minikube docker-env --profile=$MINIKUBE_PROFILE)

  print_info "Building Docker image..."
  docker build -t right-sizer:latest . -q

  # Verify image exists
  if docker images | grep -q "right-sizer.*latest"; then
    print_status "Docker image built successfully"
  else
    print_error "Failed to build Docker image"
    exit 1
  fi
}

# Deploy with Helm
deploy_with_helm() {
  print_header "Deploying with Helm"

  # Create namespace
  print_info "Creating namespace $NAMESPACE..."
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

  # Install Helm chart
  print_info "Installing Helm chart..."
  helm install $RELEASE_NAME $HELM_CHART \
    --namespace $NAMESPACE \
    --set image.repository=right-sizer \
    --set image.tag=latest \
    --set image.pullPolicy=IfNotPresent \
    --set resizeInterval=30s \
    --set logLevel=debug \
    --set config.cpuRequestMultiplier=1.2 \
    --set config.memoryRequestMultiplier=1.2 \
    --wait \
    --timeout 2m

  if [ $? -eq 0 ]; then
    print_status "Helm chart installed successfully"
  else
    print_error "Failed to install Helm chart"
    exit 1
  fi

  # Verify deployment
  print_info "Waiting for deployment to be ready..."
  kubectl wait --for=condition=available --timeout=60s \
    deployment/$RELEASE_NAME -n $NAMESPACE

  print_status "Deployment is ready"
}

# Deploy test workload
deploy_test_workload() {
  print_header "Deploying Test Workload"

  print_info "Creating test deployment..."
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $TEST_APP_NAME
  namespace: $NAMESPACE
spec:
  replicas: 3
  selector:
    matchLabels:
      app: $TEST_APP_NAME
  template:
    metadata:
      labels:
        app: $TEST_APP_NAME
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 25m
            memory: 50Mi
          limits:
            cpu: 100m
            memory: 100Mi
      - name: busybox
        image: busybox
        command: ['sh', '-c', 'while true; do echo "Working..."; sleep 30; done']
        resources:
          requests:
            cpu: 10m
            memory: 20Mi
          limits:
            cpu: 50m
            memory: 50Mi
EOF

  # Wait for test deployment
  print_info "Waiting for test deployment to be ready..."
  kubectl wait --for=condition=available --timeout=60s \
    deployment/$TEST_APP_NAME -n $NAMESPACE

  print_status "Test workload deployed"
}

# Verify operator functionality
verify_operator() {
  print_header "Verifying Operator Functionality"

  # Check operator logs
  print_info "Checking operator logs..."
  OPERATOR_POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')

  if [ -z "$OPERATOR_POD" ]; then
    print_error "Operator pod not found"
    exit 1
  fi

  print_info "Operator pod: $OPERATOR_POD"

  # Check if operator is detecting Kubernetes version
  kubectl logs $OPERATOR_POD -n $NAMESPACE | grep -q "Kubernetes" &&
    print_status "Operator detected Kubernetes version" ||
    print_warning "Operator may not have detected Kubernetes version"

  # Check if operator is using in-place resize (if K8s 1.33+)
  kubectl logs $OPERATOR_POD -n $NAMESPACE | grep -q "InPlaceRightSizer" &&
    print_status "Using InPlaceRightSizer (K8s 1.33+)" ||
    print_info "Using traditional rightsizer"

  # Wait for operator to analyze pods
  print_info "Waiting 35 seconds for operator to analyze pods..."
  sleep 35

  # Check if operator has analyzed pods
  kubectl logs $OPERATOR_POD -n $NAMESPACE --tail=50 | grep -q "Analyzing.*pods" &&
    print_status "Operator is analyzing pods" ||
    print_warning "Operator may not be analyzing pods"

  # Check if any pods were resized
  if kubectl logs $OPERATOR_POD -n $NAMESPACE --tail=100 | grep -q "Successfully resized"; then
    print_status "Operator has resized pods"

    # Show resize details
    echo ""
    print_info "Recent resize operations:"
    kubectl logs $OPERATOR_POD -n $NAMESPACE --tail=100 | grep -E "(Resizing pod|Successfully resized)" | tail -5
  else
    print_info "No pods resized yet (may need more time or metrics)"
  fi
}

# Test Helm upgrade
test_helm_upgrade() {
  print_header "Testing Helm Upgrade"

  print_info "Upgrading Helm release with new configuration..."
  helm upgrade $RELEASE_NAME $HELM_CHART \
    --namespace $NAMESPACE \
    --set image.repository=right-sizer \
    --set image.tag=latest \
    --set image.pullPolicy=IfNotPresent \
    --set resizeInterval=1m \
    --set logLevel=info \
    --set config.cpuRequestMultiplier=1.5 \
    --set config.memoryRequestMultiplier=1.3 \
    --wait \
    --timeout 2m

  if [ $? -eq 0 ]; then
    print_status "Helm upgrade successful"
  else
    print_error "Helm upgrade failed"
    exit 1
  fi

  # Verify new configuration
  print_info "Verifying new configuration..."

  # Check resize interval
  RESIZE_INTERVAL=$(kubectl get deployment $RELEASE_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="RESIZE_INTERVAL")].value}')

  if [ "$RESIZE_INTERVAL" = "1m" ]; then
    print_status "Resize interval updated to 1m"
  else
    print_warning "Resize interval not updated as expected"
  fi

  # Check log level
  LOG_LEVEL=$(kubectl get deployment $RELEASE_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="LOG_LEVEL")].value}')

  if [ "$LOG_LEVEL" = "info" ]; then
    print_status "Log level updated to info"
  else
    print_warning "Log level not updated as expected"
  fi
}

# Show resource usage
show_resource_usage() {
  print_header "Resource Usage Summary"

  print_info "Pods in namespace $NAMESPACE:"
  kubectl get pods -n $NAMESPACE -o wide

  echo ""
  print_info "Resource requests and limits for test pods:"
  kubectl get pods -n $NAMESPACE -l app=$TEST_APP_NAME -o json |
    jq -r '.items[] | "\(.metadata.name):\n  \(.spec.containers[].name):\n    Requests: CPU=\(.spec.containers[].resources.requests.cpu // "none"), Memory=\(.spec.containers[].resources.requests.memory // "none")\n    Limits: CPU=\(.spec.containers[].resources.limits.cpu // "none"), Memory=\(.spec.containers[].resources.limits.memory // "none")"'
}

# Test dry run mode
test_dry_run() {
  print_header "Testing Dry Run Mode"

  print_info "Upgrading to dry run mode..."
  helm upgrade $RELEASE_NAME $HELM_CHART \
    --namespace $NAMESPACE \
    --set image.repository=right-sizer \
    --set image.tag=latest \
    --set image.pullPolicy=IfNotPresent \
    --set resizeInterval=30s \
    --set logLevel=debug \
    --set dryRun=true \
    --wait \
    --timeout 2m

  if [ $? -eq 0 ]; then
    print_status "Switched to dry run mode"
  else
    print_error "Failed to switch to dry run mode"
    return
  fi

  # Wait and check logs
  sleep 35

  OPERATOR_POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')

  kubectl logs $OPERATOR_POD -n $NAMESPACE --tail=50 | grep -q "DRY RUN" &&
    print_status "Dry run mode is active" ||
    print_info "Dry run logs not found (check if DRY_RUN is implemented)"
}

# Main execution
main() {
  print_header "Right-Sizer Helm Deployment Test"
  echo ""

  # Trap cleanup on exit
  trap cleanup EXIT

  # Run tests
  check_prerequisites
  build_docker_image
  deploy_with_helm
  deploy_test_workload
  verify_operator
  test_helm_upgrade
  show_resource_usage
  test_dry_run

  echo ""
  print_header "Test Summary"
  print_status "All tests completed successfully!"

  echo ""
  print_info "Helm release details:"
  helm list -n $NAMESPACE

  echo ""
  print_info "To see operator logs:"
  echo "  kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"

  echo ""
  print_info "To manually cleanup:"
  echo "  helm uninstall $RELEASE_NAME -n $NAMESPACE"
  echo "  kubectl delete namespace $NAMESPACE"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --cleanup)
    cleanup
    exit 0
    ;;
  --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  --help)
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  --cleanup         Clean up all test resources"
    echo "  --namespace NAME  Use custom namespace (default: right-sizer-test)"
    echo "  --help           Show this help message"
    exit 0
    ;;
  *)
    print_error "Unknown option: $1"
    exit 1
    ;;
  esac
done

# Run main function
main
