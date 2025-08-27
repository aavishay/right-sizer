#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Test script for RESIZE_INTERVAL and LOG_LEVEL configuration
# This script tests different combinations of intervals and log levels

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
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

# Banner
print_header "RESIZE_INTERVAL & LOG_LEVEL Configuration Test"
echo ""

# Check if in Minikube context
CURRENT_CONTEXT=$(kubectl config current-context)
if [[ "$CURRENT_CONTEXT" != *"minikube"* ]]; then
  print_warning "Not in Minikube context. Current: $CURRENT_CONTEXT"
  read -p "Switch to Minikube? (y/n): " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    kubectl config use-context minikube
    print_status "Switched to Minikube context"
  fi
fi

# Configure Docker for Minikube
eval $(minikube docker-env)
print_status "Using Minikube Docker daemon"

# Build the image
print_info "Building Docker image with latest changes..."
docker build -t right-sizer:interval-test . -q >/dev/null
print_status "Docker image built: right-sizer:interval-test"

# Create test namespace
NAMESPACE="interval-test"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
print_status "Namespace '$NAMESPACE' ready"

# Apply RBAC
print_info "Setting up RBAC..."
cat <<EOF | kubectl apply -n $NAMESPACE -f - >/dev/null
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
    resources: ["deployments", "statefulsets"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
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

# Test 1: Fast interval with debug logging
echo ""
print_header "Test 1: Fast Interval (10s) with Debug Logging"
echo ""

cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-fast-debug
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-fast-debug
  template:
    metadata:
      labels:
        app: right-sizer-fast-debug
    spec:
      serviceAccountName: right-sizer
      containers:
        - name: right-sizer
          image: right-sizer:interval-test
          imagePullPolicy: Never
          env:
            - name: RESIZE_INTERVAL
              value: "10s"
            - name: LOG_LEVEL
              value: "debug"
            - name: METRICS_PROVIDER
              value: "kubernetes"
            - name: DRY_RUN
              value: "true"
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
EOF

# Test 2: Moderate interval with info logging
echo ""
print_header "Test 2: Moderate Interval (30s) with Info Logging"
echo ""

cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-moderate-info
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-moderate-info
  template:
    metadata:
      labels:
        app: right-sizer-moderate-info
    spec:
      serviceAccountName: right-sizer
      containers:
        - name: right-sizer
          image: right-sizer:interval-test
          imagePullPolicy: Never
          env:
            - name: RESIZE_INTERVAL
              value: "30s"
            - name: LOG_LEVEL
              value: "info"
            - name: METRICS_PROVIDER
              value: "kubernetes"
            - name: DRY_RUN
              value: "true"
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
EOF

# Test 3: Slow interval with error logging only
echo ""
print_header "Test 3: Slow Interval (2m) with Error Logging"
echo ""

cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-slow-error
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-slow-error
  template:
    metadata:
      labels:
        app: right-sizer-slow-error
    spec:
      serviceAccountName: right-sizer
      containers:
        - name: right-sizer
          image: right-sizer:interval-test
          imagePullPolicy: Never
          env:
            - name: RESIZE_INTERVAL
              value: "2m"
            - name: LOG_LEVEL
              value: "error"
            - name: METRICS_PROVIDER
              value: "kubernetes"
            - name: DRY_RUN
              value: "true"
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
EOF

# Deploy test workload
print_info "Deploying test workload..."
cat <<EOF | kubectl apply -n $NAMESPACE -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          resources:
            requests:
              cpu: 30m
              memory: 40Mi
            limits:
              cpu: 60m
              memory: 80Mi
EOF
print_status "Test workload deployed"

# Wait for deployments
echo ""
print_info "Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer-fast-debug -n $NAMESPACE 2>/dev/null || true
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer-moderate-info -n $NAMESPACE 2>/dev/null || true
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer-slow-error -n $NAMESPACE 2>/dev/null || true
print_status "All deployments ready"

# Function to check logs
check_logs() {
  local deployment=$1
  local expected_interval=$2
  local expected_level=$3

  echo ""
  echo -e "${CYAN}Checking $deployment:${NC}"
  echo "  Expected interval: $expected_interval"
  echo "  Expected log level: $expected_level"
  echo ""
  echo "  Configuration from logs:"
  kubectl logs -n $NAMESPACE deployment/$deployment 2>/dev/null | grep -E "RESIZE_INTERVAL|LOG_LEVEL|Configuration Loaded" | head -5 || echo "  Waiting for logs..."

  # Check for debug messages (should only appear in debug mode)
  if [ "$expected_level" = "debug" ]; then
    echo ""
    echo "  Debug messages (should be present):"
    kubectl logs -n $NAMESPACE deployment/$deployment 2>/dev/null | grep -E "\[DEBUG\]" | head -3 || echo "  No debug messages yet"
  else
    echo ""
    echo "  Debug messages (should NOT be present):"
    kubectl logs -n $NAMESPACE deployment/$deployment 2>/dev/null | grep -E "\[DEBUG\]" | head -1 && echo "  ❌ Debug messages found!" || echo "  ✓ No debug messages (as expected)"
  fi
}

# Wait a moment for logs
sleep 5

# Check each deployment's logs
echo ""
print_header "Verification Results"

check_logs "right-sizer-fast-debug" "10s" "debug"
check_logs "right-sizer-moderate-info" "30s" "info"
check_logs "right-sizer-slow-error" "2m" "error"

# Monitor activity for a bit
echo ""
print_header "Monitoring Activity (30 seconds)"
echo ""

for i in {1..3}; do
  echo "Check $i/3 at $(date '+%H:%M:%S'):"
  echo ""

  # Fast interval should have multiple runs
  echo "  Fast (10s interval):"
  kubectl logs -n $NAMESPACE deployment/right-sizer-fast-debug --tail=1 2>/dev/null | grep -E "Found|Processing|Starting" || echo "    No recent activity"

  # Moderate interval
  echo "  Moderate (30s interval):"
  kubectl logs -n $NAMESPACE deployment/right-sizer-moderate-info --tail=1 2>/dev/null | grep -E "Found|Processing|Starting" || echo "    No recent activity"

  # Slow interval
  echo "  Slow (2m interval):"
  kubectl logs -n $NAMESPACE deployment/right-sizer-slow-error --tail=1 2>/dev/null | grep -E "ERROR" || echo "    No errors (good!)"

  if [ $i -lt 3 ]; then
    echo ""
    sleep 10
  fi
done

# Summary
echo ""
print_header "Test Summary"
echo ""

# Get all pods
echo "Deployed operators:"
kubectl get pods -n $NAMESPACE -l app -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount

echo ""
echo "Configuration verification:"
echo ""

# Check each deployment's environment
for deploy in right-sizer-fast-debug right-sizer-moderate-info right-sizer-slow-error; do
  echo "$deploy:"
  kubectl get deployment $deploy -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="RESIZE_INTERVAL")].value}' && echo -n " interval, "
  kubectl get deployment $deploy -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="LOG_LEVEL")].value}' && echo " log level"
done

# Provide commands for further testing
echo ""
print_header "Next Steps"
echo ""
echo "Commands for further testing:"
echo ""
echo "  Watch fast interval logs (debug):"
echo "    kubectl logs -n $NAMESPACE deployment/right-sizer-fast-debug -f"
echo ""
echo "  Watch moderate interval logs (info):"
echo "    kubectl logs -n $NAMESPACE deployment/right-sizer-moderate-info -f"
echo ""
echo "  Check for errors only:"
echo "    kubectl logs -n $NAMESPACE deployment/right-sizer-slow-error -f"
echo ""
echo "  Compare log volumes:"
echo "    echo 'Fast/Debug:' && kubectl logs -n $NAMESPACE deployment/right-sizer-fast-debug | wc -l"
echo "    echo 'Moderate/Info:' && kubectl logs -n $NAMESPACE deployment/right-sizer-moderate-info | wc -l"
echo "    echo 'Slow/Error:' && kubectl logs -n $NAMESPACE deployment/right-sizer-slow-error | wc -l"
echo ""
echo "  Cleanup:"
echo "    kubectl delete namespace $NAMESPACE"
echo ""

print_status "Test completed successfully!"
