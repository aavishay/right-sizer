#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Test script for right-sizer operator with configurable multipliers in Minikube
# This script deploys the operator with custom configuration and tests the functionality

set -e

echo "================================================"
echo "Right-Sizer Operator - Minikube Configuration Test"
echo "================================================"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
  echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
  echo -e "${RED}[✗]${NC} $1"
}

# Check if minikube is installed
if ! command -v minikube &>/dev/null; then
  print_error "Minikube is not installed. Please install minikube first."
  exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &>/dev/null; then
  print_error "kubectl is not installed. Please install kubectl first."
  exit 1
fi

# Configuration options
NAMESPACE="right-sizer-test"
DEPLOYMENT_NAME="right-sizer"
TEST_APP_NAME="nginx-test"

# Custom configuration values for testing
export CPU_REQUEST_MULTIPLIER="1.5"
export MEMORY_REQUEST_MULTIPLIER="1.3"
export CPU_LIMIT_MULTIPLIER="2.5"
export MEMORY_LIMIT_MULTIPLIER="2.0"
export MAX_CPU_LIMIT="6000"
export MAX_MEMORY_LIMIT="12288"
export MIN_CPU_REQUEST="20"
export MIN_MEMORY_REQUEST="128"

echo "Test Configuration:"
echo "  CPU_REQUEST_MULTIPLIER: $CPU_REQUEST_MULTIPLIER"
echo "  MEMORY_REQUEST_MULTIPLIER: $MEMORY_REQUEST_MULTIPLIER"
echo "  CPU_LIMIT_MULTIPLIER: $CPU_LIMIT_MULTIPLIER"
echo "  MEMORY_LIMIT_MULTIPLIER: $MEMORY_LIMIT_MULTIPLIER"
echo "  MAX_CPU_LIMIT: $MAX_CPU_LIMIT"
echo "  MAX_MEMORY_LIMIT: $MAX_MEMORY_LIMIT"
echo "  MIN_CPU_REQUEST: $MIN_CPU_REQUEST"
echo "  MIN_MEMORY_REQUEST: $MIN_MEMORY_REQUEST"
echo ""

# Step 1: Start Minikube if not running
echo "Step 1: Checking Minikube status..."
if minikube status | grep -q "Running"; then
  print_status "Minikube is already running"
else
  print_warning "Starting Minikube with Kubernetes v1.31..."
  minikube start --kubernetes-version=v1.31.0 --memory=4096 --cpus=2
  print_status "Minikube started"
fi

# Step 2: Use Minikube's Docker daemon
echo ""
echo "Step 2: Configuring Docker environment..."
eval $(minikube docker-env)
print_status "Using Minikube's Docker daemon"

# Step 3: Build the Docker image
echo ""
echo "Step 3: Building right-sizer Docker image..."
docker build -t right-sizer:config-test .
print_status "Docker image built: right-sizer:config-test"

# Step 4: Create namespace
echo ""
echo "Step 4: Creating test namespace..."
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
print_status "Namespace '$NAMESPACE' ready"

# Step 5: Apply RBAC
echo ""
echo "Step 5: Setting up RBAC..."
cat <<EOF | kubectl apply -n $NAMESPACE -f -
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
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["get", "create"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
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
print_status "RBAC configured"

# Step 6: Deploy right-sizer with custom configuration
echo ""
echo "Step 6: Deploying right-sizer operator with custom configuration..."
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $DEPLOYMENT_NAME
  namespace: $NAMESPACE
  labels:
    app: right-sizer
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
          image: right-sizer:config-test
          imagePullPolicy: Never
          env:
            - name: OPERATOR_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: METRICS_PROVIDER
              value: "kubernetes"
            - name: ENABLE_INPLACE_RESIZE
              value: "false"
            - name: DRY_RUN
              value: "false"
            # Custom configuration values
            - name: CPU_REQUEST_MULTIPLIER
              value: "$CPU_REQUEST_MULTIPLIER"
            - name: MEMORY_REQUEST_MULTIPLIER
              value: "$MEMORY_REQUEST_MULTIPLIER"
            - name: CPU_LIMIT_MULTIPLIER
              value: "$CPU_LIMIT_MULTIPLIER"
            - name: MEMORY_LIMIT_MULTIPLIER
              value: "$MEMORY_LIMIT_MULTIPLIER"
            - name: MAX_CPU_LIMIT
              value: "$MAX_CPU_LIMIT"
            - name: MAX_MEMORY_LIMIT
              value: "$MAX_MEMORY_LIMIT"
            - name: MIN_CPU_REQUEST
              value: "$MIN_CPU_REQUEST"
            - name: MIN_MEMORY_REQUEST
              value: "$MIN_MEMORY_REQUEST"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          ports:
            - containerPort: 8080
              name: metrics
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
EOF
print_status "Right-sizer operator deployed"

# Step 7: Deploy test application
echo ""
echo "Step 7: Deploying test application (nginx)..."
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $TEST_APP_NAME
  namespace: $NAMESPACE
  labels:
    app: nginx
    rightsizer: enabled
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
        rightsizer: enabled
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 100m
              memory: 128Mi
          ports:
            - containerPort: 80
EOF
print_status "Test application deployed"

# Step 8: Wait for deployments to be ready
echo ""
echo "Step 8: Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/$DEPLOYMENT_NAME -n $NAMESPACE
kubectl wait --for=condition=available --timeout=60s deployment/$TEST_APP_NAME -n $NAMESPACE
print_status "All deployments are ready"

# Step 9: Check operator logs for configuration
echo ""
echo "Step 9: Verifying configuration loaded correctly..."
echo "----------------------------------------"
echo "Operator Configuration Logs:"
kubectl logs -n $NAMESPACE deployment/$DEPLOYMENT_NAME --tail=50 | grep -E "Configuration|Multiplier|Limit|Request" || true
echo "----------------------------------------"

# Step 10: Monitor test application
echo ""
echo "Step 10: Monitoring test application resources..."
echo "Initial resource allocation:"
kubectl get pods -n $NAMESPACE -l app=nginx -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

# Step 11: Generate some load on test app (optional)
echo ""
echo "Step 11: Generating test load..."
TEST_POD=$(kubectl get pods -n $NAMESPACE -l app=nginx -o jsonpath='{.items[0].metadata.name}')
if [ ! -z "$TEST_POD" ]; then
  print_status "Using pod: $TEST_POD for load generation"
  # Run a simple CPU stress test in the background
  kubectl exec -n $NAMESPACE $TEST_POD -- sh -c "dd if=/dev/zero of=/dev/null bs=1M count=1000 2>/dev/null" &
  LOAD_PID=$!
  sleep 5
  kill $LOAD_PID 2>/dev/null || true
else
  print_warning "No test pod found for load generation"
fi

# Step 12: Wait for right-sizer to process
echo ""
echo "Step 12: Waiting for right-sizer to process (30 seconds)..."
sleep 30

# Step 13: Check if resources were adjusted
echo ""
echo "Step 13: Checking resource adjustments..."
echo "Current resource allocation:"
kubectl get pods -n $NAMESPACE -l app=nginx -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

# Step 14: Show operator logs
echo ""
echo "Step 14: Recent operator logs:"
echo "----------------------------------------"
kubectl logs -n $NAMESPACE deployment/$DEPLOYMENT_NAME --tail=30
echo "----------------------------------------"

# Step 15: Cleanup option
echo ""
echo "================================================"
echo "Test Complete!"
echo "================================================"
echo ""
echo "Test Summary:"
print_status "Right-sizer operator deployed with custom configuration"
print_status "Test application deployed and monitored"
print_status "Configuration values were loaded successfully"
echo ""
echo "Useful commands:"
echo "  View operator logs:    kubectl logs -n $NAMESPACE deployment/$DEPLOYMENT_NAME -f"
echo "  View test app pods:    kubectl get pods -n $NAMESPACE -l app=nginx"
echo "  Describe a pod:        kubectl describe pod -n $NAMESPACE <pod-name>"
echo "  View deployments:      kubectl get deployments -n $NAMESPACE"
echo ""
read -p "Do you want to cleanup the test resources? (y/n): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo "Cleaning up..."
  kubectl delete namespace $NAMESPACE
  print_status "Cleanup complete"
else
  print_warning "Resources left running in namespace: $NAMESPACE"
  echo "To cleanup later, run: kubectl delete namespace $NAMESPACE"
fi

echo ""
echo "Test script finished!"
