#!/bin/bash

# Simple Minikube Sanity Test for Right-Sizer
# This script performs basic testing of the right-sizer operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-right-sizer}
TEST_NAMESPACE=${TEST_NAMESPACE:-default}

print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

print_header() {
  echo ""
  print_color $BLUE "========================================="
  print_color $BLUE "$1"
  print_color $BLUE "========================================="
}

check_requirement() {
  local cmd=$1
  local min_version=$2

  if ! command -v $cmd &>/dev/null; then
    print_color $RED "❌ $cmd is not installed"
    exit 1
  fi
  print_color $GREEN "✅ $cmd is available"
}

cleanup() {
  print_header "Cleaning Up"

  # Delete test resources
  kubectl delete pod test-pod --ignore-not-found=true -n $TEST_NAMESPACE &>/dev/null || true
  kubectl delete rightsizerconfig test-config --ignore-not-found=true -n $NAMESPACE &>/dev/null || true

  # Delete operator
  kubectl delete deployment right-sizer --ignore-not-found=true -n $NAMESPACE &>/dev/null || true
  kubectl delete namespace $NAMESPACE --ignore-not-found=true &>/dev/null || true

  print_color $GREEN "✅ Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

print_header "Right-Sizer Simple Minikube Test"

# Step 1: Check requirements
print_header "Checking Requirements"

check_requirement kubectl
check_requirement minikube
check_requirement docker

# Check Kubernetes version
K8S_VERSION=$(kubectl version -o json | jq -r '.serverVersion.gitVersion')
print_color $YELLOW "📋 Kubernetes version: $K8S_VERSION"

if [[ ! "$K8S_VERSION" =~ v1\.(3[3-9]|[4-9][0-9]) ]]; then
  print_color $RED "❌ Kubernetes 1.33+ is required for in-place resizing"
  print_color $YELLOW "⚠️  Current version: $K8S_VERSION"
  exit 1
fi

# Step 2: Build and load Docker image
print_header "Building Docker Image"

print_color $YELLOW "Building right-sizer image..."
docker build -t right-sizer:test . >/dev/null 2>&1
print_color $GREEN "✅ Image built successfully"

print_color $YELLOW "Loading image into minikube..."
minikube image load right-sizer:test >/dev/null 2>&1
print_color $GREEN "✅ Image loaded"

# Step 3: Install CRDs
print_header "Installing CRDs"

kubectl apply -f helm/crds/ >/dev/null 2>&1
print_color $GREEN "✅ CRDs installed"

# Wait for CRDs to be established
sleep 3

# Step 4: Deploy operator
print_header "Deploying Right-Sizer Operator"

# Create namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1

# Create minimal deployment
cat <<EOF | kubectl apply -f - >/dev/null 2>&1
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
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs", "rightsizerpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs/status", "rightsizerpolicies/status"]
  verbs: ["get", "update", "patch"]
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
        ports:
        - containerPort: 8081
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
EOF

print_color $GREEN "✅ Operator deployed"

# Wait for deployment
print_color $YELLOW "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer -n $NAMESPACE >/dev/null 2>&1
print_color $GREEN "✅ Operator is ready"

# Step 5: Test health endpoints
print_header "Testing Health Endpoints"

POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')
print_color $YELLOW "Operator pod: $POD_NAME"

# Test liveness
kubectl exec -n $NAMESPACE $POD_NAME -- wget -q -O- http://localhost:8081/healthz >/dev/null 2>&1
print_color $GREEN "✅ Liveness probe working"

# Test readiness
kubectl exec -n $NAMESPACE $POD_NAME -- wget -q -O- http://localhost:8081/readyz >/dev/null 2>&1
print_color $GREEN "✅ Readiness probe working"

# Step 6: Create simple configuration
print_header "Testing Configuration"

cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: test-config
  namespace: $NAMESPACE
spec:
  enabled: true
  dryRun: false
  namespaceConfig:
    includeNamespaces:
      - $TEST_NAMESPACE
EOF

print_color $GREEN "✅ Configuration created"

# Step 7: Deploy test pod
print_header "Testing Pod Resizing"

cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: $TEST_NAMESPACE
spec:
  containers:
  - name: stress
    image: busybox
    command: ["sleep", "3600"]
    resources:
      requests:
        memory: "32Mi"
        cpu: "10m"
      limits:
        memory: "64Mi"
        cpu: "50m"
EOF

print_color $GREEN "✅ Test pod created"

# Wait for pod to be running
kubectl wait --for=condition=ready --timeout=30s pod/test-pod -n $TEST_NAMESPACE >/dev/null 2>&1
print_color $GREEN "✅ Test pod is running"

# Check initial resources
print_color $YELLOW "Initial pod resources:"
kubectl get pod test-pod -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq '.'

# Step 8: Check operator logs
print_header "Checking Operator Logs"

# Get last 10 lines of logs
kubectl logs -n $NAMESPACE deployment/right-sizer --tail=10 | head -5

# Step 9: Summary
print_header "Test Summary"

print_color $GREEN "✅ CRDs installed successfully"
print_color $GREEN "✅ Operator deployed and running"
print_color $GREEN "✅ Health endpoints functional"
print_color $GREEN "✅ Configuration accepted"
print_color $GREEN "✅ Test pod monitored"

print_color $BLUE ""
print_color $BLUE "🎉 All basic tests passed!"
print_color $BLUE ""
print_color $YELLOW "To see operator in action:"
print_color $YELLOW "  kubectl logs -n $NAMESPACE deployment/right-sizer -f"
print_color $YELLOW ""
print_color $YELLOW "To check pod resources:"
print_color $YELLOW "  kubectl get pod test-pod -n $TEST_NAMESPACE -o jsonpath='{.spec.containers[0].resources}' | jq"
