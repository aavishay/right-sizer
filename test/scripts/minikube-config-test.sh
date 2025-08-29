#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Comprehensive Minikube test for right-sizer with configurable multipliers
# This script ensures we're in a Minikube environment before proceeding

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
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
  echo -e "${BLUE}[i]${NC} $1"
}

# Banner
echo ""
echo "================================================="
echo "   Right-Sizer Configuration Test for Minikube"
echo "================================================="
echo ""

# Step 1: Environment Check
echo "Step 1: Checking environment..."
echo "--------------------------------"

# Check if minikube is installed
if ! command -v minikube &>/dev/null; then
  print_error "Minikube is not installed"
  echo "Please install Minikube from: https://minikube.sigs.k8s.io/docs/start/"
  exit 1
fi
print_status "Minikube is installed"

# Check if kubectl is installed
if ! command -v kubectl &>/dev/null; then
  print_error "kubectl is not installed"
  echo "Please install kubectl from: https://kubernetes.io/docs/tasks/tools/"
  exit 1
fi
print_status "kubectl is installed"

# Check if Docker is installed
if ! command -v docker &>/dev/null; then
  print_error "Docker is not installed"
  echo "Please install Docker from: https://docs.docker.com/get-docker/"
  exit 1
fi
print_status "Docker is installed"

# Step 2: Minikube Setup
echo ""
echo "Step 2: Setting up Minikube..."
echo "--------------------------------"

# Check if Minikube is running
if minikube status 2>/dev/null | grep -q "host: Running"; then
  print_status "Minikube is already running"
  MINIKUBE_IP=$(minikube ip)
  print_info "Minikube IP: $MINIKUBE_IP"
else
  print_warning "Minikube is not running. Starting Minikube..."
  minikube start --kubernetes-version=v1.31.0 --memory=4096 --cpus=2
  if [ $? -eq 0 ]; then
    print_status "Minikube started successfully"
    MINIKUBE_IP=$(minikube ip)
    print_info "Minikube IP: $MINIKUBE_IP"
  else
    print_error "Failed to start Minikube"
    exit 1
  fi
fi

# Verify we're using minikube context
CURRENT_CONTEXT=$(kubectl config current-context)
if [[ "$CURRENT_CONTEXT" != *"minikube"* ]]; then
  print_warning "Current context is not minikube: $CURRENT_CONTEXT"
  print_info "Switching to minikube context..."
  kubectl config use-context minikube
  if [ $? -eq 0 ]; then
    print_status "Switched to minikube context"
  else
    print_error "Failed to switch to minikube context"
    exit 1
  fi
else
  print_status "Using minikube context: $CURRENT_CONTEXT"
fi

# Step 3: Configure Docker to use Minikube's daemon
echo ""
echo "Step 3: Configuring Docker..."
echo "--------------------------------"
eval $(minikube docker-env)
print_status "Docker configured to use Minikube's daemon"

# Step 4: Build the right-sizer image
echo ""
echo "Step 4: Building right-sizer image..."
echo "--------------------------------"

if [ -f "Dockerfile" ]; then
  print_info "Building Docker image: right-sizer:config-test"
  docker build -t right-sizer:config-test . -q
  if [ $? -eq 0 ]; then
    print_status "Docker image built successfully"
    docker images | grep right-sizer | head -1
  else
    print_error "Failed to build Docker image"
    exit 1
  fi
else
  print_error "Dockerfile not found in current directory"
  exit 1
fi

# Step 5: Create test namespace
echo ""
echo "Step 5: Setting up Kubernetes resources..."
echo "--------------------------------"

NAMESPACE="rightsizer-config-test"
print_info "Creating namespace: $NAMESPACE"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
print_status "Namespace created/updated"

# Step 6: Apply RBAC
print_info "Applying RBAC permissions..."
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer-$NAMESPACE
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
    resources: ["events"]
    verbs: ["create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer-$NAMESPACE
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer-$NAMESPACE
subjects:
  - kind: ServiceAccount
    name: right-sizer
    namespace: $NAMESPACE
EOF
print_status "RBAC configured"

# Step 7: Deploy right-sizer with custom configuration
echo ""
echo "Step 7: Deploying right-sizer with custom configuration..."
echo "--------------------------------"

# Configuration values for testing
CPU_REQUEST_MULTIPLIER="1.6"
MEMORY_REQUEST_MULTIPLIER="1.4"
CPU_LIMIT_MULTIPLIER="2.5"
MEMORY_LIMIT_MULTIPLIER="2.2"
MAX_CPU_LIMIT="10000"
MAX_MEMORY_LIMIT="16384"
MIN_CPU_REQUEST="25"
MIN_MEMORY_REQUEST="128"

print_info "Configuration values:"
echo "  CPU_REQUEST_MULTIPLIER: $CPU_REQUEST_MULTIPLIER"
echo "  MEMORY_REQUEST_MULTIPLIER: $MEMORY_REQUEST_MULTIPLIER"
echo "  CPU_LIMIT_MULTIPLIER: $CPU_LIMIT_MULTIPLIER"
echo "  MEMORY_LIMIT_MULTIPLIER: $MEMORY_LIMIT_MULTIPLIER"
echo "  MAX_CPU_LIMIT: $MAX_CPU_LIMIT millicores"
echo "  MAX_MEMORY_LIMIT: $MAX_MEMORY_LIMIT MB"
echo "  MIN_CPU_REQUEST: $MIN_CPU_REQUEST millicores"
echo "  MIN_MEMORY_REQUEST: $MIN_MEMORY_REQUEST MB"

cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
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
            # Custom configuration
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
print_status "Right-sizer deployed"

# Step 8: Deploy test workloads
echo ""
echo "Step 8: Deploying test workloads..."
echo "--------------------------------"

print_info "Deploying nginx test application..."
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  labels:
    app: nginx
    test: config-validation
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
          image: nginx:alpine
          resources:
            requests:
              cpu: 30m
              memory: 50Mi
            limits:
              cpu: 60m
              memory: 100Mi
          ports:
            - containerPort: 80
EOF
print_status "Test workload deployed"

# Step 9: Wait for deployments
echo ""
echo "Step 9: Waiting for deployments to be ready..."
echo "--------------------------------"

print_info "Waiting for right-sizer deployment..."
kubectl wait --for=condition=available --timeout=120s deployment/right-sizer -n $NAMESPACE 2>/dev/null
if [ $? -eq 0 ]; then
  print_status "Right-sizer is ready"
else
  print_warning "Right-sizer deployment is taking longer than expected"
fi

print_info "Waiting for test workload..."
kubectl wait --for=condition=available --timeout=60s deployment/nginx-test -n $NAMESPACE 2>/dev/null
if [ $? -eq 0 ]; then
  print_status "Test workload is ready"
else
  print_warning "Test workload is taking longer than expected"
fi

# Step 10: Verify configuration
echo ""
echo "Step 10: Verifying configuration..."
echo "--------------------------------"

# Wait a moment for logs to be available
sleep 5

print_info "Checking operator logs for configuration..."
echo ""
echo "Configuration from operator logs:"
echo "---------------------------------"
kubectl logs -n $NAMESPACE deployment/right-sizer 2>/dev/null | grep -E "Configuration Loaded|CPU_REQUEST_MULTIPLIER|MEMORY_REQUEST_MULTIPLIER|CPU_LIMIT_MULTIPLIER|MEMORY_LIMIT_MULTIPLIER|MAX_CPU_LIMIT|MAX_MEMORY_LIMIT|MIN_CPU_REQUEST|MIN_MEMORY_REQUEST" | head -20 || {
  print_warning "Configuration logs not yet available, waiting..."
  sleep 5
  kubectl logs -n $NAMESPACE deployment/right-sizer 2>/dev/null | grep -E "Configuration|Multiplier|Limit|Request" | head -20
}

# Step 11: Show current state
echo ""
echo "Step 11: Current state..."
echo "--------------------------------"

print_info "Pods in namespace $NAMESPACE:"
kubectl get pods -n $NAMESPACE -o wide

echo ""
print_info "Test workload resources:"
kubectl get pods -n $NAMESPACE -l app=nginx -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

# Step 12: Calculation examples
echo ""
echo "Step 12: Expected calculations with current config..."
echo "--------------------------------"

echo "For a pod using 100m CPU and 100Mi memory:"
echo "  CPU Request: $(echo "100 * $CPU_REQUEST_MULTIPLIER" | bc)m (100 × $CPU_REQUEST_MULTIPLIER)"
echo "  Memory Request: $(echo "100 * $MEMORY_REQUEST_MULTIPLIER" | bc)Mi (100 × $MEMORY_REQUEST_MULTIPLIER)"
echo "  CPU Limit: $(echo "100 * $CPU_REQUEST_MULTIPLIER * $CPU_LIMIT_MULTIPLIER" | bc)m"
echo "  Memory Limit: $(echo "100 * $MEMORY_REQUEST_MULTIPLIER * $MEMORY_LIMIT_MULTIPLIER" | bc)Mi"

# Step 13: Provide useful commands
echo ""
echo "================================================="
echo -e "${GREEN}Test Setup Complete!${NC}"
echo "================================================="
echo ""
echo "Useful commands:"
echo "  Watch operator logs:"
echo "    kubectl logs -n $NAMESPACE deployment/right-sizer -f"
echo ""
echo "  Check pods:"
echo "    kubectl get pods -n $NAMESPACE -w"
echo ""
echo "  Check events:"
echo "    kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
echo ""
echo "  Describe right-sizer pod:"
echo "    kubectl describe pod -n $NAMESPACE -l app=right-sizer"
echo ""
echo "  Check test workload adjustments:"
echo "    kubectl get deployment nginx-test -n $NAMESPACE -o yaml | grep -A5 resources:"
echo ""
echo "  Generate load on test pod:"
echo "    POD=\$(kubectl get pod -n $NAMESPACE -l app=nginx -o jsonpath='{.items[0].metadata.name}')"
echo "    kubectl exec -n $NAMESPACE \$POD -- sh -c 'while true; do echo test > /dev/null; done'"
echo ""
echo "  Cleanup:"
echo "    kubectl delete namespace $NAMESPACE"
echo ""
echo "  Stop Minikube:"
echo "    minikube stop"
echo ""
echo "Note: The right-sizer will periodically check and adjust resources."
echo "      Default interval is 5-10 minutes. Watch the logs to see it in action."
echo ""
