#!/bin/bash

# Right Sizer RBAC Application and Fix Script
# This script applies or fixes RBAC permissions for the Right Sizer operator

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="${NAMESPACE:-right-sizer-system}"
SERVICE_ACCOUNT="${SERVICE_ACCOUNT:-right-sizer}"
RELEASE_NAME="${RELEASE_NAME:-right-sizer}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"
VERIFY_AFTER="${VERIFY_AFTER:-true}"
USE_HELM="${USE_HELM:-false}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Function to print colored output
print_status() {
  local status=$1
  local message=$2

  case $status in
  "SUCCESS")
    echo -e "${GREEN}✓${NC} $message"
    ;;
  "ERROR")
    echo -e "${RED}✗${NC} $message"
    ;;
  "WARNING")
    echo -e "${YELLOW}⚠${NC} $message"
    ;;
  "INFO")
    echo -e "${BLUE}ℹ${NC} $message"
    ;;
  "HEADER")
    echo -e "\n${BLUE}═══ $message ═══${NC}"
    ;;
  esac
}

# Function to check if kubectl is available
check_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    print_status "ERROR" "kubectl is not installed or not in PATH"
    exit 1
  fi
}

# Function to check if helm is available
check_helm() {
  if ! command -v helm &>/dev/null; then
    print_status "ERROR" "helm is not installed or not in PATH"
    return 1
  fi
  return 0
}

# Function to check cluster connectivity
check_cluster() {
  if ! kubectl cluster-info &>/dev/null; then
    print_status "ERROR" "Cannot connect to Kubernetes cluster"
    exit 1
  fi
}

# Function to create namespace if it doesn't exist
create_namespace() {
  if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    print_status "INFO" "Namespace '$NAMESPACE' already exists"
  else
    print_status "INFO" "Creating namespace '$NAMESPACE'"
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "DRY RUN: Would create namespace $NAMESPACE"
    else
      kubectl create namespace "$NAMESPACE"
      print_status "SUCCESS" "Namespace created"
    fi
  fi
}

# Function to generate RBAC manifest
generate_rbac_manifest() {
  cat <<EOF
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $SERVICE_ACCOUNT
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: right-sizer
    app.kubernetes.io/instance: $RELEASE_NAME
automountServiceAccountToken: true

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: $RELEASE_NAME
  labels:
    app.kubernetes.io/name: right-sizer
    app.kubernetes.io/instance: $RELEASE_NAME
rules:
  # Custom Resource Definitions
  - apiGroups: ["rightsizer.io"]
    resources: ["rightsizerpolicies", "rightsizerconfigs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["rightsizer.io"]
    resources: ["rightsizerpolicies/status", "rightsizerconfigs/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["rightsizer.io"]
    resources: ["rightsizerpolicies/finalizers", "rightsizerconfigs/finalizers"]
    verbs: ["update"]

  # Core pod operations
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods/resize"]
    verbs: ["patch", "update"]

  # Node information
  - apiGroups: [""]
    resources: ["nodes", "nodes/status"]
    verbs: ["get", "list", "watch"]

  # Metrics APIs
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes", "podmetrics", "nodemetrics"]
    verbs: ["get", "list", "watch"]

  # Custom and external metrics
  - apiGroups: ["custom.metrics.k8s.io", "external.metrics.k8s.io"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]

  # Workload controllers
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: ["apps"]
    resources: ["deployments/status", "statefulsets/status", "daemonsets/status", "replicasets/status"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments/scale", "statefulsets/scale", "replicasets/scale"]
    verbs: ["get", "patch", "update"]

  # Batch jobs
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]

  # Events
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]

  # Secrets for reading sensitive configuration
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]

  # Namespaces
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]

  # Autoscaling
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["autoscaling.k8s.io"]
    resources: ["verticalpodautoscalers"]
    verbs: ["get", "list", "watch"]

  # Policy
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch"]

  # Resource constraints
  - apiGroups: [""]
    resources: ["resourcequotas", "limitranges"]
    verbs: ["get", "list", "watch"]

  # Storage
  - apiGroups: [""]
    resources: ["persistentvolumeclaims", "persistentvolumes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list", "watch"]

  # Scheduling
  - apiGroups: ["scheduling.k8s.io"]
    resources: ["priorityclasses"]
    verbs: ["get", "list", "watch"]

  # Networking
  - apiGroups: ["networking.k8s.io"]
    resources: ["networkpolicies"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["create", "update", "patch"]

  # Admission webhooks
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $RELEASE_NAME
  labels:
    app.kubernetes.io/name: right-sizer
    app.kubernetes.io/instance: $RELEASE_NAME
roleRef:
  kind: ClusterRole
  name: $RELEASE_NAME
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: $SERVICE_ACCOUNT
    namespace: $NAMESPACE

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $RELEASE_NAME-namespace
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: right-sizer
    app.kubernetes.io/instance: $RELEASE_NAME
rules:
  # ConfigMaps for leader election only (policies use CRDs)
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Leases for leader election
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Secrets for TLS certificates
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Events in operator namespace
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch", "update"]

  # Pods for self-monitoring
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $RELEASE_NAME-namespace
  namespace: $NAMESPACE
  labels:
    app.kubernetes.io/name: right-sizer
    app.kubernetes.io/instance: $RELEASE_NAME
roleRef:
  kind: Role
  name: $RELEASE_NAME-namespace
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: $SERVICE_ACCOUNT
    namespace: $NAMESPACE
EOF
}

# Function to apply RBAC using kubectl
apply_rbac_kubectl() {
  print_status "INFO" "Applying RBAC using kubectl..."

  local temp_file="/tmp/right-sizer-rbac-$$.yaml"
  generate_rbac_manifest >"$temp_file"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_status "INFO" "DRY RUN mode - showing what would be applied:"
    echo "---"
    cat "$temp_file"
    echo "---"
  else
    if kubectl apply -f "$temp_file"; then
      print_status "SUCCESS" "RBAC resources applied successfully"
    else
      print_status "ERROR" "Failed to apply RBAC resources"
      rm -f "$temp_file"
      exit 1
    fi
  fi

  rm -f "$temp_file"
}

# Function to apply RBAC using Helm
apply_rbac_helm() {
  print_status "INFO" "Applying RBAC using Helm..."

  local helm_dir="$PROJECT_ROOT/helm"

  if [[ ! -d "$helm_dir" ]]; then
    print_status "ERROR" "Helm chart directory not found at $helm_dir"
    exit 1
  fi

  local helm_cmd="helm"

  # Check if release exists
  if helm list -n "$NAMESPACE" | grep -q "^$RELEASE_NAME"; then
    helm_cmd="$helm_cmd upgrade $RELEASE_NAME"
  else
    helm_cmd="$helm_cmd install $RELEASE_NAME"
  fi

  helm_cmd="$helm_cmd $helm_dir --namespace $NAMESPACE --create-namespace"

  if [[ "$DRY_RUN" == "true" ]]; then
    helm_cmd="$helm_cmd --dry-run --debug"
  fi

  print_status "INFO" "Running: $helm_cmd"

  if $helm_cmd; then
    print_status "SUCCESS" "Helm deployment successful"
  else
    print_status "ERROR" "Helm deployment failed"
    exit 1
  fi
}

# Function to clean up existing RBAC resources
cleanup_rbac() {
  print_status "WARNING" "Cleaning up existing RBAC resources..."

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "DRY RUN: Would delete the following resources:"
    kubectl get clusterrole "$RELEASE_NAME" 2>/dev/null || true
    kubectl get clusterrolebinding "$RELEASE_NAME" 2>/dev/null || true
    kubectl get role "$RELEASE_NAME-namespace" -n "$NAMESPACE" 2>/dev/null || true
    kubectl get rolebinding "$RELEASE_NAME-namespace" -n "$NAMESPACE" 2>/dev/null || true
    kubectl get serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" 2>/dev/null || true
  else
    kubectl delete clusterrole "$RELEASE_NAME" --ignore-not-found=true
    kubectl delete clusterrolebinding "$RELEASE_NAME" --ignore-not-found=true
    kubectl delete role "$RELEASE_NAME-namespace" -n "$NAMESPACE" --ignore-not-found=true
    kubectl delete rolebinding "$RELEASE_NAME-namespace" -n "$NAMESPACE" --ignore-not-found=true
    kubectl delete serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" --ignore-not-found=true

    print_status "SUCCESS" "Existing RBAC resources cleaned up"
  fi
}

# Function to verify RBAC after application
verify_rbac() {
  local verify_script="$SCRIPT_DIR/verify-permissions.sh"

  if [[ -f "$verify_script" ]]; then
    print_status "INFO" "Verifying RBAC permissions..."

    # Wait a moment for resources to propagate
    sleep 2

    if NAMESPACE="$NAMESPACE" SERVICE_ACCOUNT="$SERVICE_ACCOUNT" "$verify_script"; then
      print_status "SUCCESS" "RBAC verification passed"
    else
      print_status "WARNING" "RBAC verification failed - please check the output above"
    fi
  else
    print_status "WARNING" "Verification script not found at $verify_script"
  fi
}

# Main function
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Right Sizer RBAC Application and Fix Script"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo

  # Check prerequisites
  check_kubectl
  check_cluster

  print_status "INFO" "Namespace: $NAMESPACE"
  print_status "INFO" "Service Account: $SERVICE_ACCOUNT"
  print_status "INFO" "Release Name: $RELEASE_NAME"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_status "WARNING" "Running in DRY RUN mode - no changes will be made"
  fi

  # Create namespace if needed
  create_namespace

  # Clean up if forced
  if [[ "$FORCE" == "true" ]]; then
    cleanup_rbac
  fi

  # Apply RBAC
  if [[ "$USE_HELM" == "true" ]]; then
    if check_helm; then
      apply_rbac_helm
    else
      print_status "WARNING" "Helm not available, falling back to kubectl"
      apply_rbac_kubectl
    fi
  else
    apply_rbac_kubectl
  fi

  # Verify if not in dry run mode
  if [[ "$DRY_RUN" != "true" && "$VERIFY_AFTER" == "true" ]]; then
    verify_rbac
  fi

  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "DRY RUN completed. Run without --dry-run to apply changes."
  else
    echo "RBAC application completed!"
    echo
    echo "Next steps:"
    echo "1. Verify permissions: ./scripts/rbac/verify-permissions.sh"
    echo "2. Deploy the operator: kubectl apply -f deployment.yaml"
    echo "3. Check operator logs: kubectl logs -n $NAMESPACE -l app=right-sizer"
  fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -n | --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  -s | --service-account)
    SERVICE_ACCOUNT="$2"
    shift 2
    ;;
  -r | --release-name)
    RELEASE_NAME="$2"
    shift 2
    ;;
  --dry-run)
    DRY_RUN=true
    shift
    ;;
  -f | --force)
    FORCE=true
    shift
    ;;
  --no-verify)
    VERIFY_AFTER=false
    shift
    ;;
  --use-helm)
    USE_HELM=true
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -n, --namespace NAME           Namespace to install Right Sizer (default: right-sizer-system)"
    echo "  -s, --service-account NAME     Service account name (default: right-sizer)"
    echo "  -r, --release-name NAME        Helm release name (default: right-sizer)"
    echo "  --dry-run                      Show what would be done without making changes"
    echo "  -f, --force                    Force cleanup and reapply RBAC resources"
    echo "  --no-verify                    Skip verification after applying RBAC"
    echo "  --use-helm                     Use Helm to apply the entire chart"
    echo "  -h, --help                     Show this help message"
    echo
    echo "Environment Variables:"
    echo "  NAMESPACE                      Alternative to -n flag"
    echo "  SERVICE_ACCOUNT                Alternative to -s flag"
    echo "  RELEASE_NAME                   Alternative to -r flag"
    echo "  DRY_RUN                        Alternative to --dry-run flag (true/false)"
    echo "  FORCE                          Alternative to -f flag (true/false)"
    echo "  VERIFY_AFTER                   Alternative to --no-verify flag (true/false)"
    echo "  USE_HELM                       Alternative to --use-helm flag (true/false)"
    echo
    echo "Examples:"
    echo "  # Apply RBAC with defaults"
    echo "  $0"
    echo
    echo "  # Apply RBAC in custom namespace"
    echo "  $0 -n my-namespace"
    echo
    echo "  # Dry run to see what would be applied"
    echo "  $0 --dry-run"
    echo
    echo "  # Force reapply all RBAC resources"
    echo "  $0 --force"
    echo
    echo "  # Use Helm to deploy the entire chart"
    echo "  $0 --use-helm"
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use -h or --help for usage information"
    exit 1
    ;;
  esac
done

# Run main function
main
