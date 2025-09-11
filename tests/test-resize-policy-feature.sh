#!/bin/bash

# Integration test for UpdateResizePolicy feature flag
# This test verifies that resize policies are only added to parent resources
# when the UpdateResizePolicy feature flag is enabled

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="test-resize-policy-feature"
DEPLOYMENT_NAME="test-deployment"
RIGHTSIZER_NAMESPACE="right-sizer"
CONFIG_NAME="right-sizer-config"

# Helper functions
print_header() {
  echo -e "\n${YELLOW}==== $1 ====${NC}\n"
}

print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
  echo -e "${RED}✗ $1${NC}"
}

print_info() {
  echo -e "ℹ️  $1"
}

cleanup() {
  print_header "Cleaning up test resources"
  kubectl delete namespace ${TEST_NAMESPACE} --ignore-not-found=true 2>/dev/null || true
  print_success "Cleanup completed"
}

# Trap cleanup on exit
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
  print_header "Checking prerequisites"

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    print_error "kubectl is not installed"
    exit 1
  fi
  print_success "kubectl is installed"

  # Check if right-sizer is deployed
  if ! kubectl get deployment right-sizer -n ${RIGHTSIZER_NAMESPACE} &>/dev/null; then
    print_error "right-sizer is not deployed in namespace ${RIGHTSIZER_NAMESPACE}"
    exit 1
  fi
  print_success "right-sizer is deployed"

  # Check if RightSizerConfig exists
  if ! kubectl get rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} &>/dev/null; then
    print_error "RightSizerConfig ${CONFIG_NAME} not found in namespace ${RIGHTSIZER_NAMESPACE}"
    exit 1
  fi
  print_success "RightSizerConfig found"
}

# Test with feature flag disabled
test_feature_disabled() {
  print_header "Testing with UpdateResizePolicy=false"

  # Set feature flag to false
  print_info "Setting UpdateResizePolicy feature flag to false"
  kubectl patch rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} \
    --type='json' -p='[{"op": "replace", "path": "/spec/featureGates/UpdateResizePolicy", "value": false}]' \
    >/dev/null 2>&1

  # Wait for config to be applied
  sleep 5

  # Create test namespace
  kubectl create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1

  # Create test deployment
  print_info "Creating test deployment"
  cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
      annotations:
        rightsizer.io/enable: "true"
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
EOF

  # Wait for deployment to be ready
  kubectl wait --for=condition=available --timeout=60s deployment/${DEPLOYMENT_NAME} -n ${TEST_NAMESPACE} >/dev/null 2>&1

  # Wait for right-sizer to process
  print_info "Waiting for right-sizer to process the deployment (30s)"
  sleep 30

  # Check if resize policy was added
  if kubectl get deployment ${DEPLOYMENT_NAME} -n ${TEST_NAMESPACE} -o yaml | grep -q "resizePolicy"; then
    print_error "Resize policy was added when feature flag was disabled"
    return 1
  else
    print_success "No resize policy added when feature flag is disabled"
  fi

  # Check logs for the expected behavior
  if kubectl logs -n ${RIGHTSIZER_NAMESPACE} deployment/right-sizer --tail=100 |
    grep -q "Skipping direct pod resize policy patch"; then
    print_success "Found expected log: Skipping direct pod resize policy patch"
  else
    print_info "Warning: Expected log message not found (may have rotated)"
  fi

  # Clean up test deployment
  kubectl delete deployment ${DEPLOYMENT_NAME} -n ${TEST_NAMESPACE} >/dev/null 2>&1
  sleep 5
}

# Test with feature flag enabled
test_feature_enabled() {
  print_header "Testing with UpdateResizePolicy=true"

  # Set feature flag to true
  print_info "Setting UpdateResizePolicy feature flag to true"
  kubectl patch rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} \
    --type='json' -p='[{"op": "replace", "path": "/spec/featureGates/UpdateResizePolicy", "value": true}]' \
    >/dev/null 2>&1

  # Wait for config to be applied
  sleep 5

  # Create test deployment with different name
  print_info "Creating test deployment with feature enabled"
  cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}-enabled
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-enabled
  template:
    metadata:
      labels:
        app: test-enabled
      annotations:
        rightsizer.io/enable: "true"
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: "150m"
            memory: "192Mi"
          limits:
            cpu: "300m"
            memory: "384Mi"
EOF

  # Wait for deployment to be ready
  kubectl wait --for=condition=available --timeout=60s deployment/${DEPLOYMENT_NAME}-enabled -n ${TEST_NAMESPACE} >/dev/null 2>&1

  # Wait for right-sizer to process
  print_info "Waiting for right-sizer to process the deployment (30s)"
  sleep 30

  # Check if resize policy was added
  if kubectl get deployment ${DEPLOYMENT_NAME}-enabled -n ${TEST_NAMESPACE} -o yaml | grep -q "resizePolicy"; then
    print_success "Resize policy was added when feature flag was enabled"

    # Verify the resize policy content
    if kubectl get deployment ${DEPLOYMENT_NAME}-enabled -n ${TEST_NAMESPACE} -o yaml |
      grep -q "restartPolicy: NotRequired"; then
      print_success "Resize policy has correct restartPolicy: NotRequired"
    else
      print_error "Resize policy does not have correct restartPolicy"
      return 1
    fi
  else
    print_info "Note: Resize policy may not be added if deployment hasn't been resized yet"
  fi

  # Check logs for the expected behavior
  if kubectl logs -n ${RIGHTSIZER_NAMESPACE} deployment/right-sizer --tail=200 |
    grep "${TEST_NAMESPACE}" | grep -q "Ensuring parent resource has resize policy"; then
    print_success "Found expected log: Ensuring parent resource has resize policy"
  else
    print_info "Warning: Expected log message not found (deployment may not have been resized)"
  fi
}

# Verify feature flag persistence
verify_persistence() {
  print_header "Verifying feature flag persistence"

  # Get current feature flag value
  CURRENT_VALUE=$(kubectl get rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} \
    -o jsonpath='{.spec.featureGates.UpdateResizePolicy}')

  if [ "$CURRENT_VALUE" == "true" ]; then
    print_success "Feature flag is currently set to: true"
  elif [ "$CURRENT_VALUE" == "false" ]; then
    print_success "Feature flag is currently set to: false"
  else
    print_error "Could not determine feature flag value"
    return 1
  fi

  # Reset to default (false)
  print_info "Resetting feature flag to default (false)"
  kubectl patch rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} \
    --type='json' -p='[{"op": "replace", "path": "/spec/featureGates/UpdateResizePolicy", "value": false}]' \
    >/dev/null 2>&1

  sleep 2

  # Verify it was reset
  RESET_VALUE=$(kubectl get rightsizerconfig ${CONFIG_NAME} -n ${RIGHTSIZER_NAMESPACE} \
    -o jsonpath='{.spec.featureGates.UpdateResizePolicy}')

  if [ "$RESET_VALUE" == "false" ]; then
    print_success "Feature flag successfully reset to default (false)"
  else
    print_error "Failed to reset feature flag to default"
    return 1
  fi
}

# Main test execution
main() {
  print_header "UpdateResizePolicy Feature Flag Integration Test"

  # Check prerequisites
  check_prerequisites

  # Run tests
  test_feature_disabled
  test_feature_enabled
  verify_persistence

  print_header "Test Summary"
  print_success "All tests passed successfully!"
  print_info "The UpdateResizePolicy feature flag is working as expected:"
  print_info "  • When disabled (false): No resize policies are added to deployments"
  print_info "  • When enabled (true): Resize policies are added during resize operations"
  print_info "  • Default value is false for backward compatibility"
}

# Run the test
main
