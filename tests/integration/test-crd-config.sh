#!/bin/bash

# Test script for Right Sizer CRD-based configuration
# This script tests the operator with CRD-only configuration (no environment variables)

set -e

NAMESPACE="right-sizer"
RELEASE_NAME="right-sizer"
TEST_DEPLOYMENT="test-nginx"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
  log_info "Checking prerequisites..."

  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl is not installed"
    exit 1
  fi

  if ! command -v helm &>/dev/null; then
    log_error "helm is not installed"
    exit 1
  fi

  log_info "Prerequisites check passed"
}

# Create test namespace
create_namespace() {
  log_info "Creating test namespace: $NAMESPACE"
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
}

# Build and push the operator image (optional - for local testing)
build_image() {
  log_info "Building operator image..."
  docker build -t right-sizer:test -f Dockerfile .

  # For kind/minikube, load the image
  if command -v kind &>/dev/null; then
    log_info "Loading image into kind cluster..."
    kind load docker-image right-sizer:test
  elif command -v minikube &>/dev/null; then
    log_info "Loading image into minikube..."
    minikube image load right-sizer:test
  fi
}

# Deploy CRDs
deploy_crds() {
  log_info "Deploying CRDs..."
  kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
  kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml

  # Wait for CRDs to be established
  kubectl wait --for condition=established --timeout=60s \
    crd/rightsizerconfigs.rightsizer.io \
    crd/rightsizerpolicies.rightsizer.io
}

# Deploy the operator using Helm
deploy_operator() {
  log_info "Deploying Right Sizer operator with Helm..."

  # Create values override file for testing
  cat >/tmp/test-values.yaml <<EOF
image:
  repository: right-sizer
  tag: test
  pullPolicy: IfNotPresent

createDefaultConfig: true

defaultConfig:
  name: "test-config"
  enabled: true
  mode: "balanced"
  resizeInterval: "30s"
  dryRun: false

  resourceStrategy:
    cpu:
      requestMultiplier: 1.5
      requestAddition: 50
      limitMultiplier: 2.0
      limitAddition: 100
      minRequest: 50
      maxLimit: 2000
    memory:
      requestMultiplier: 1.3
      requestAddition: 128
      limitMultiplier: 1.8
      limitAddition: 256
      minRequest: 128
      maxLimit: 4096

  observability:
    logLevel: "debug"
    enableMetricsExport: true
    metricsPort: 9090
    enableAuditLog: true

  namespaces:
    include: ["$NAMESPACE"]
    exclude: []

createExamplePolicies: true

examplePolicies:
  - name: "test-policy"
    enabled: true
    priority: 100
    mode: "aggressive"
    targetRef:
      kind: "Deployment"
      namespaces: ["$NAMESPACE"]
      labelSelector:
        matchLabels:
          app: nginx
    resourceStrategy:
      cpu:
        requestMultiplier: 1.2
        limitMultiplier: 1.5
      memory:
        requestMultiplier: 1.2
        limitMultiplier: 1.5
    schedule:
      interval: "1m"
EOF

  helm upgrade --install $RELEASE_NAME ./helm \
    --namespace $NAMESPACE \
    --values /tmp/test-values.yaml \
    --wait \
    --timeout 2m
}

# Create test deployment
create_test_deployment() {
  log_info "Creating test deployment..."

  cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $TEST_DEPLOYMENT
  labels:
    app: nginx
    test: right-sizer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
EOF

  kubectl wait --for=condition=available --timeout=60s \
    deployment/$TEST_DEPLOYMENT -n $NAMESPACE
}

# Verify operator is running
verify_operator() {
  log_info "Verifying operator is running..."

  # Check operator pod
  kubectl wait --for=condition=ready --timeout=120s \
    pod -l app.kubernetes.io/name=right-sizer -n $NAMESPACE

  # Check operator logs
  log_info "Operator logs:"
  kubectl logs -l app.kubernetes.io/name=right-sizer -n $NAMESPACE --tail=20
}

# Verify CRD configuration
verify_crd_config() {
  log_info "Verifying CRD configuration..."

  # Check RightSizerConfig
  log_info "RightSizerConfig status:"
  kubectl get rightsizerconfig test-config -n $NAMESPACE -o yaml | grep -A 10 "status:"

  # Check RightSizerPolicy
  log_info "RightSizerPolicy status:"
  kubectl get rightsizerpolicy test-policy -n $NAMESPACE -o yaml | grep -A 10 "status:"
}

# Test configuration update
test_config_update() {
  log_info "Testing configuration update via CRD..."

  # Update the RightSizerConfig
  kubectl patch rightsizerconfig test-config -n $NAMESPACE --type='merge' -p '
    {
      "spec": {
        "dryRun": true,
        "observabilityConfig": {
          "logLevel": "info"
        },
        "defaultResourceStrategy": {
          "cpu": {
            "requestMultiplier": 1.8
          }
        }
      }
    }'

  # Wait for reconciliation
  sleep 10

  # Check operator logs to verify configuration was updated
  log_info "Checking operator logs for configuration update..."
  kubectl logs -l app.kubernetes.io/name=right-sizer -n $NAMESPACE --tail=10 | grep -i "config"
}

# Test policy creation
test_policy_creation() {
  log_info "Testing policy creation..."

  cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: custom-policy
spec:
  enabled: true
  priority: 200
  mode: "conservative"
  targetRef:
    kind: "Deployment"
    namespaces: ["$NAMESPACE"]
    labelSelector:
      matchLabels:
        test: right-sizer
  resourceStrategy:
    cpu:
      requestMultiplier: 2.0
      minRequest: 100
      maxLimit: 1000
    memory:
      requestMultiplier: 1.5
      minRequest: 256
      maxLimit: 2048
  schedule:
    interval: "2m"
  constraints:
    maxChangePercentage: 25
    cooldownPeriod: "5m"
    respectPDB: true
EOF

  # Verify policy was created
  kubectl wait --for=condition=ready --timeout=30s \
    rightsizerpolicy/custom-policy -n $NAMESPACE || true

  log_info "Custom policy status:"
  kubectl get rightsizerpolicy custom-policy -n $NAMESPACE -o yaml | grep -A 5 "status:"
}

# Monitor resource changes
monitor_resources() {
  log_info "Monitoring resource changes for 2 minutes..."

  end_time=$(($(date +%s) + 120))

  while [ $(date +%s) -lt $end_time ]; do
    echo ""
    log_info "Current deployment resources:"
    kubectl get deployment $TEST_DEPLOYMENT -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].resources}' | jq .

    echo ""
    log_info "Pod resources:"
    kubectl get pods -n $NAMESPACE -l app=nginx -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.containers[0].resources}{"\n"}{end}'

    sleep 30
  done
}

# Test metrics endpoint
test_metrics() {
  log_info "Testing metrics endpoint..."

  # Port-forward to the operator pod
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')

  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 9090:9090 &
  PF_PID=$!

  sleep 5

  # Check metrics
  if curl -s http://localhost:9090/metrics | grep -q "rightsizer_"; then
    log_info "Metrics endpoint is working"
  else
    log_warn "Metrics endpoint may not be working correctly"
  fi

  kill $PF_PID 2>/dev/null || true
}

# Clean up
cleanup() {
  log_info "Cleaning up..."

  # Delete test resources
  kubectl delete deployment $TEST_DEPLOYMENT -n $NAMESPACE --ignore-not-found=true

  # Uninstall Helm release
  helm uninstall $RELEASE_NAME -n $NAMESPACE || true

  # Delete CRDs
  kubectl delete crd rightsizerconfigs.rightsizer.io rightsizerpolicies.rightsizer.io --ignore-not-found=true

  # Delete namespace
  kubectl delete namespace $NAMESPACE --ignore-not-found=true

  # Clean up temp files
  rm -f /tmp/test-values.yaml
}

# Main test flow
main() {
  log_info "Starting Right Sizer CRD configuration test..."

  check_prerequisites

  # Set up trap for cleanup on exit
  trap cleanup EXIT

  create_namespace

  # Optional: build image for local testing
  # build_image

  deploy_crds
  deploy_operator
  verify_operator

  create_test_deployment

  # Allow time for initial reconciliation
  sleep 20

  verify_crd_config
  test_config_update
  test_policy_creation

  # Monitor for resource changes
  monitor_resources

  test_metrics

  log_info "Test completed successfully!"
}

# Run main function
main "$@"
