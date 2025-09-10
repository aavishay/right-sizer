#!/bin/bash

# Quick RBAC Deployment for Right-Sizer
# This script deploys only the RBAC resources needed for Right-Sizer

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

NAMESPACE="right-sizer"

print_info "🔐 Deploying Right-Sizer RBAC..."

# Ensure namespace exists
kubectl create namespace $NAMESPACE 2>/dev/null || true

# Deploy RBAC
cat <<EOF | kubectl apply -f -
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
  # Core pod operations (resize, status, eviction)
  - apiGroups: [""]
    resources: ["pods", "pods/status", "pods/eviction", "pods/log"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Pod resize subresource (Kubernetes >= 1.27)
  - apiGroups: [""]
    resources: ["pods/resize"]
    verbs: ["get", "patch", "update"]

  # Cluster info
  - apiGroups: [""]
    resources: ["nodes", "namespaces"]
    verbs: ["get", "list", "watch"]

  # Events for notifications
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]

  # Metrics API
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list", "watch"]

  # Workload controllers
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch", "update", "patch"]

  # Batch workloads
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch", "update", "patch"]

  # Autoscaling
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]

  - apiGroups: ["autoscaling.k8s.io"]
    resources: ["verticalpodautoscalers"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Policy
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]

  # Leader election (if needed)
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Config storage
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
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

print_success "✅ RBAC deployed successfully"

# Verify RBAC
print_info "Verifying RBAC permissions..."

if kubectl get clusterrole right-sizer >/dev/null 2>&1; then
  print_success "✅ ClusterRole created"

  # Check for key permissions
  if kubectl get clusterrole right-sizer -o yaml | grep -q "pods/resize"; then
    print_success "✅ Has pods/resize permission"
  else
    print_warning "⚠️  Missing pods/resize permission"
  fi

  if kubectl get clusterrole right-sizer -o yaml | grep -q "metrics.k8s.io"; then
    print_success "✅ Has metrics API permission"
  else
    print_warning "⚠️  Missing metrics API permission"
  fi
else
  print_error "❌ ClusterRole creation failed"
fi

if kubectl get clusterrolebinding right-sizer >/dev/null 2>&1; then
  print_success "✅ ClusterRoleBinding created"
else
  print_error "❌ ClusterRoleBinding creation failed"
fi

# Test permissions
print_info "Testing permissions..."
if kubectl auth can-i resize pods --as=system:serviceaccount:$NAMESPACE:right-sizer >/dev/null 2>&1; then
  print_success "✅ Can resize pods"
else
  print_warning "⚠️  Cannot resize pods (may not be supported in this cluster)"
fi

if kubectl auth can-i get pods --as=system:serviceaccount:$NAMESPACE:right-sizer >/dev/null 2>&1; then
  print_success "✅ Can get pods"
else
  print_error "❌ Cannot get pods"
fi

print_success "🔐 RBAC deployment complete!"
echo ""
echo "Next steps:"
echo "1. Deploy Right-Sizer operator with this ServiceAccount"
echo "2. Verify operator can access required resources"
echo "3. Monitor logs for any permission issues"
