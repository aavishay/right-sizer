#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Quick test script for right-sizer configuration in Minikube
# This script quickly deploys and validates the configuration functionality

set -e

echo "========================================="
echo "Right-Sizer Quick Configuration Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check prerequisites
if ! command -v minikube &>/dev/null; then
  echo "Error: Minikube is not installed"
  exit 1
fi

if ! command -v kubectl &>/dev/null; then
  echo "Error: kubectl is not installed"
  exit 1
fi

# Start Minikube if needed
echo "Checking Minikube status..."
if ! minikube status | grep -q "Running" 2>/dev/null; then
  echo "Starting Minikube..."
  minikube start --memory=4096 --cpus=2
else
  echo -e "${GREEN}✓${NC} Minikube is running"
fi

# Use Minikube Docker
echo ""
echo "Configuring Docker environment..."
eval $(minikube docker-env)

# Build the image
echo ""
echo "Building Docker image..."
docker build -t right-sizer:test-config . -q
echo -e "${GREEN}✓${NC} Image built: right-sizer:test-config"

# Create test namespace
NAMESPACE="config-test"
echo ""
echo "Creating namespace: $NAMESPACE"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Apply RBAC
echo "Applying RBAC..."
kubectl apply -n $NAMESPACE -f - <<EOF
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
echo -e "${GREEN}✓${NC} RBAC configured"

# Deploy with custom configuration
echo ""
echo "Deploying right-sizer with custom configuration..."
echo -e "${YELLOW}Configuration:${NC}"
echo "  CPU_REQUEST_MULTIPLIER: 1.5"
echo "  MEMORY_REQUEST_MULTIPLIER: 1.3"
echo "  CPU_LIMIT_MULTIPLIER: 2.5"
echo "  MAX_CPU_LIMIT: 8000"

kubectl apply -n $NAMESPACE -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
spec:
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
          image: right-sizer:test-config
          imagePullPolicy: Never
          env:
            - name: CPU_REQUEST_MULTIPLIER
              value: "1.5"
            - name: MEMORY_REQUEST_MULTIPLIER
              value: "1.3"
            - name: CPU_LIMIT_MULTIPLIER
              value: "2.5"
            - name: MEMORY_LIMIT_MULTIPLIER
              value: "2.0"
            - name: MAX_CPU_LIMIT
              value: "8000"
            - name: MAX_MEMORY_LIMIT
              value: "16384"
            - name: METRICS_PROVIDER
              value: "kubernetes"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
EOF

# Deploy test application
echo ""
echo "Deploying test application..."
kubectl apply -n $NAMESPACE -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
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
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 100m
              memory: 128Mi
EOF

# Wait for deployment
echo ""
echo "Waiting for deployments..."
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer -n $NAMESPACE 2>/dev/null || true
kubectl wait --for=condition=available --timeout=60s deployment/nginx-test -n $NAMESPACE 2>/dev/null || true

# Check configuration in logs
echo ""
echo "========================================="
echo "Verifying Configuration"
echo "========================================="
echo ""
echo "Operator logs showing configuration:"
kubectl logs -n $NAMESPACE deployment/right-sizer 2>/dev/null | grep -E "Configuration Loaded|CPU_REQUEST_MULTIPLIER|MEMORY_REQUEST_MULTIPLIER|CPU_LIMIT_MULTIPLIER|MAX_CPU_LIMIT" | head -15 || echo "Waiting for logs..."

# Wait a bit and check again if needed
if [ $? -ne 0 ]; then
  sleep 5
  kubectl logs -n $NAMESPACE deployment/right-sizer 2>/dev/null | grep -E "Configuration|Multiplier|Limit" | head -15
fi

# Show test application resources
echo ""
echo "Test application resources:"
kubectl get pods -n $NAMESPACE -l app=nginx -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_LIM:.spec.containers[0].resources.limits.memory

# Provide commands for further testing
echo ""
echo "========================================="
echo -e "${GREEN}Test deployment successful!${NC}"
echo "========================================="
echo ""
echo "Useful commands:"
echo "  Watch logs:     kubectl logs -n $NAMESPACE deployment/right-sizer -f"
echo "  Check pods:     kubectl get pods -n $NAMESPACE"
echo "  Check events:   kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp'"
echo "  Cleanup:        kubectl delete namespace $NAMESPACE"
echo ""
echo "Expected behavior with test configuration:"
echo "  For 100m CPU usage → Request: 150m (100 × 1.5), Limit: 375m (150 × 2.5)"
echo "  For 100Mi memory  → Request: 130Mi (100 × 1.3), Limit: 260Mi (130 × 2.0)"
echo ""
