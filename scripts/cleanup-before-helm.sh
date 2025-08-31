#!/bin/bash

# Cleanup script to remove existing right-sizer resources before Helm installation
# This script removes resources that may have been created manually or by previous installations

set -e

echo "================================================"
echo "    Right-Sizer Pre-Helm Cleanup Script"
echo "================================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to safely delete a resource
safe_delete() {
  local resource_type=$1
  local resource_name=$2
  local namespace=$3

  if [ -z "$namespace" ]; then
    # Cluster-scoped resource
    if kubectl get $resource_type $resource_name >/dev/null 2>&1; then
      echo "🗑️  Deleting $resource_type: $resource_name"
      kubectl delete $resource_type $resource_name --ignore-not-found=true
    else
      echo "✓  $resource_type $resource_name not found (already clean)"
    fi
  else
    # Namespaced resource
    if kubectl get $resource_type $resource_name -n $namespace >/dev/null 2>&1; then
      echo "🗑️  Deleting $resource_type: $resource_name in namespace $namespace"
      kubectl delete $resource_type $resource_name -n $namespace --ignore-not-found=true
    else
      echo "✓  $resource_type $resource_name in namespace $namespace not found (already clean)"
    fi
  fi
}

echo -e "${YELLOW}⚠️  WARNING: This will delete all existing right-sizer resources${NC}"
echo "This includes resources in the following namespaces:"
echo "  - default"
echo "  - right-sizer"
echo "  - kube-system (if any right-sizer resources exist there)"
echo ""
read -p "Do you want to continue? (y/N): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Cleanup cancelled."
  exit 0
fi

echo ""
echo "Starting cleanup..."
echo "=================="

# 1. Delete Deployments
echo ""
echo "📦 Cleaning up Deployments..."
safe_delete "deployment" "right-sizer" "default"
safe_delete "deployment" "right-sizer" "right-sizer"
safe_delete "deployment" "right-sizer" "kube-system"

# 2. Delete Services
echo ""
echo "🌐 Cleaning up Services..."
safe_delete "service" "right-sizer" "default"
safe_delete "service" "right-sizer" "right-sizer"
safe_delete "service" "right-sizer-webhook" "default"
safe_delete "service" "right-sizer-webhook" "right-sizer"
safe_delete "service" "right-sizer-metrics" "default"
safe_delete "service" "right-sizer-metrics" "right-sizer"

# 3. Delete ConfigMaps
echo ""
echo "📋 Cleaning up ConfigMaps..."
safe_delete "configmap" "right-sizer-config" "default"
safe_delete "configmap" "right-sizer-config" "right-sizer"
safe_delete "configmap" "right-sizer" "default"
safe_delete "configmap" "right-sizer" "right-sizer"

# 4. Delete ServiceAccounts
echo ""
echo "👤 Cleaning up ServiceAccounts..."
safe_delete "serviceaccount" "right-sizer" "default"
safe_delete "serviceaccount" "right-sizer" "right-sizer"
safe_delete "serviceaccount" "right-sizer" "kube-system"

# 5. Delete ClusterRoles
echo ""
echo "🔐 Cleaning up ClusterRoles..."
safe_delete "clusterrole" "right-sizer" ""
safe_delete "clusterrole" "right-sizer-view" ""
safe_delete "clusterrole" "right-sizer-edit" ""

# 6. Delete ClusterRoleBindings
echo ""
echo "🔗 Cleaning up ClusterRoleBindings..."
safe_delete "clusterrolebinding" "right-sizer" ""
safe_delete "clusterrolebinding" "right-sizer-view" ""
safe_delete "clusterrolebinding" "right-sizer-edit" ""

# 7. Delete Roles (in specific namespaces)
echo ""
echo "🔑 Cleaning up Roles..."
safe_delete "role" "right-sizer" "default"
safe_delete "role" "right-sizer" "right-sizer"

# 8. Delete RoleBindings
echo ""
echo "🔗 Cleaning up RoleBindings..."
safe_delete "rolebinding" "right-sizer" "default"
safe_delete "rolebinding" "right-sizer" "right-sizer"

# 9. Delete PersistentVolumeClaims
echo ""
echo "💾 Cleaning up PersistentVolumeClaims..."
safe_delete "pvc" "right-sizer-data" "default"
safe_delete "pvc" "right-sizer-data" "right-sizer"

# 10. Delete Secrets
echo ""
echo "🔒 Cleaning up Secrets..."
safe_delete "secret" "right-sizer-webhook-tls" "default"
safe_delete "secret" "right-sizer-webhook-tls" "right-sizer"

# 11. Delete NetworkPolicies
echo ""
echo "🔧 Cleaning up NetworkPolicies..."
safe_delete "networkpolicy" "right-sizer" "default"
safe_delete "networkpolicy" "right-sizer" "right-sizer"

# 12. Delete PodDisruptionBudgets
echo ""
echo "⚡ Cleaning up PodDisruptionBudgets..."
safe_delete "pdb" "right-sizer" "default"
safe_delete "pdb" "right-sizer" "right-sizer"

# 13. Delete CustomResourceDefinitions (if any)
echo ""
echo "📑 Cleaning up CustomResourceDefinitions..."
safe_delete "crd" "rightsizerconfigs.rightsizer.io" ""
safe_delete "crd" "rightsizerpolicies.rightsizer.io" ""

# 14. Clean up any remaining pods
echo ""
echo "🗑️  Cleaning up any remaining pods..."
kubectl delete pods -l app=right-sizer --all-namespaces --ignore-not-found=true 2>/dev/null || true
kubectl delete pods -l app.kubernetes.io/name=right-sizer --all-namespaces --ignore-not-found=true 2>/dev/null || true

# 15. Optional: Delete namespace (if empty)
echo ""
echo "📁 Checking namespace..."
if kubectl get namespace right-sizer >/dev/null 2>&1; then
  # Check if namespace has any remaining resources
  RESOURCE_COUNT=$(kubectl get all -n right-sizer --no-headers 2>/dev/null | wc -l)
  if [ "$RESOURCE_COUNT" -eq "0" ]; then
    echo -e "${YELLOW}The 'right-sizer' namespace is empty.${NC}"
    read -p "Do you want to delete the namespace? (y/N): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      kubectl delete namespace right-sizer --ignore-not-found=true
      echo "✅ Namespace deleted"
    else
      echo "⏭️  Keeping namespace"
    fi
  else
    echo "ℹ️  Namespace 'right-sizer' contains $RESOURCE_COUNT resources - keeping namespace"
  fi
else
  echo "✓  Namespace 'right-sizer' not found (already clean)"
fi

echo ""
echo "================================================"
echo -e "${GREEN}✅ Cleanup Complete!${NC}"
echo "================================================"
echo ""
echo "You can now install right-sizer with Helm:"
echo ""
echo "  helm install right-sizer ./helm/right-sizer \\"
echo "    --namespace right-sizer \\"
echo "    --create-namespace"
echo ""
echo "Or with custom values:"
echo ""
echo "  helm install right-sizer ./helm/right-sizer \\"
echo "    --namespace right-sizer \\"
echo "    --create-namespace \\"
echo "    --values custom-values.yaml"
echo ""
