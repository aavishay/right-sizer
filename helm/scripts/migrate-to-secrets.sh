#!/bin/bash

# migrate-to-secrets.sh - Migration script for upgrading Right-Sizer to use secure secret management
# This script helps migrate from plain text credentials in values.yaml to Kubernetes Secrets

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${NAMESPACE:-right-sizer}"
RELEASE_NAME="${RELEASE_NAME:-right-sizer}"
DRY_RUN="${DRY_RUN:-false}"
BACKUP_DIR="./migration-backup-$(date +%Y%m%d-%H%M%S)"

# Function to print colored messages
print_message() {
  local color=$1
  local message=$2
  echo -e "${color}${message}${NC}"
}

print_header() {
  echo ""
  print_message "$BLUE" "==========================================="
  print_message "$BLUE" "$1"
  print_message "$BLUE" "==========================================="
  echo ""
}

# Function to check prerequisites
check_prerequisites() {
  print_header "Checking Prerequisites"

  # Check for required tools
  for tool in kubectl helm jq; do
    if ! command -v $tool &>/dev/null; then
      print_message "$RED" "✗ $tool is not installed"
      exit 1
    else
      print_message "$GREEN" "✓ $tool is installed"
    fi
  done

  # Check if namespace exists
  if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    print_message "$GREEN" "✓ Namespace '$NAMESPACE' exists"
  else
    print_message "$RED" "✗ Namespace '$NAMESPACE' does not exist"
    exit 1
  fi

  # Check if release exists
  if helm get values "$RELEASE_NAME" -n "$NAMESPACE" &>/dev/null; then
    print_message "$GREEN" "✓ Helm release '$RELEASE_NAME' exists"
  else
    print_message "$RED" "✗ Helm release '$RELEASE_NAME' not found in namespace '$NAMESPACE'"
    exit 1
  fi
}

# Function to backup current configuration
backup_current_config() {
  print_header "Backing Up Current Configuration"

  mkdir -p "$BACKUP_DIR"

  # Backup Helm values
  print_message "$YELLOW" "Backing up Helm values..."
  helm get values "$RELEASE_NAME" -n "$NAMESPACE" >"$BACKUP_DIR/values.yaml"

  # Backup current deployment
  print_message "$YELLOW" "Backing up deployment manifest..."
  kubectl get deployment -n "$NAMESPACE" -l "app.kubernetes.io/name=right-sizer" -o yaml >"$BACKUP_DIR/deployment.yaml"

  # Save rollback instructions
  cat >"$BACKUP_DIR/rollback.sh" <<'EOF'
#!/bin/bash
# Rollback script - use if migration fails

echo "Rolling back to previous configuration..."
helm rollback $RELEASE_NAME -n $NAMESPACE
echo "Rollback complete. Please verify your deployment."
EOF
  chmod +x "$BACKUP_DIR/rollback.sh"

  print_message "$GREEN" "✓ Backup saved to $BACKUP_DIR"
}

# Function to extract current credentials
extract_credentials() {
  print_header "Extracting Current Credentials"

  # Get current values
  local values
  values=$(helm get values "$RELEASE_NAME" -n "$NAMESPACE" -o json)

  # Extract API token
  API_TOKEN=$(echo "$values" | jq -r '.config.apiToken // .rightsizerConfig.metricsBuffer.dashboard.apiToken // empty' || echo "")
  DASHBOARD_URL=$(echo "$values" | jq -r '.config.dashboardUrl // .rightsizerConfig.metricsBuffer.dashboard.url // empty' || echo "")
  CLUSTER_ID=$(echo "$values" | jq -r '.config.clusterId // .rightsizerConfig.multiTenancy.clusterId // empty' || echo "")
  CLUSTER_NAME=$(echo "$values" | jq -r '.config.clusterName // .rightsizerConfig.multiTenancy.clusterName // empty' || echo "")

  # Check if we found any credentials
  if [[ -z "$API_TOKEN" && -z "$CLUSTER_ID" ]]; then
    print_message "$YELLOW" "⚠ No existing credentials found in Helm values"
    print_message "$YELLOW" "  You may be using environment variables or external secrets already"

    # Try to get from running deployment
    print_message "$YELLOW" "Checking running deployment for credentials..."

    local pod
    pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=right-sizer" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [[ -n "$pod" ]]; then
      API_TOKEN=$(kubectl exec -n "$NAMESPACE" "$pod" -- printenv DASHBOARD_API_TOKEN 2>/dev/null || echo "")
      DASHBOARD_URL=$(kubectl exec -n "$NAMESPACE" "$pod" -- printenv DASHBOARD_URL 2>/dev/null || echo "")
      CLUSTER_ID=$(kubectl exec -n "$NAMESPACE" "$pod" -- printenv CLUSTER_ID 2>/dev/null || echo "")
      CLUSTER_NAME=$(kubectl exec -n "$NAMESPACE" "$pod" -- printenv CLUSTER_NAME 2>/dev/null || echo "")
    fi
  fi

  # Display found credentials (masked)
  if [[ -n "$API_TOKEN" ]]; then
    print_message "$GREEN" "✓ API Token found: ${API_TOKEN:0:8}..."
  else
    print_message "$YELLOW" "⚠ No API Token found"
  fi

  if [[ -n "$DASHBOARD_URL" ]]; then
    print_message "$GREEN" "✓ Dashboard URL: $DASHBOARD_URL"
  fi

  if [[ -n "$CLUSTER_ID" ]]; then
    print_message "$GREEN" "✓ Cluster ID: $CLUSTER_ID"
  fi

  if [[ -n "$CLUSTER_NAME" ]]; then
    print_message "$GREEN" "✓ Cluster Name: $CLUSTER_NAME"
  fi
}

# Function to create Kubernetes secrets
create_secrets() {
  print_header "Creating Kubernetes Secrets"

  local secret_created=false

  # Create dashboard API token secret if we have the token
  if [[ -n "$API_TOKEN" ]]; then
    print_message "$YELLOW" "Creating secret for dashboard API token..."

    if [[ "$DRY_RUN" == "true" ]]; then
      print_message "$BLUE" "[DRY RUN] Would create secret: ${RELEASE_NAME}-dashboard"
      echo "kubectl create secret generic ${RELEASE_NAME}-dashboard \\"
      echo "  --from-literal=api-token=<token> \\"
      echo "  -n $NAMESPACE"
    else
      kubectl create secret generic "${RELEASE_NAME}-dashboard" \
        --from-literal=api-token="$API_TOKEN" \
        -n "$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -

      print_message "$GREEN" "✓ Created secret: ${RELEASE_NAME}-dashboard"
      secret_created=true
    fi
  fi

  # Create cluster credentials secret if we have cluster info
  if [[ -n "$CLUSTER_ID" || -n "$CLUSTER_NAME" ]]; then
    print_message "$YELLOW" "Creating secret for cluster credentials..."

    if [[ "$DRY_RUN" == "true" ]]; then
      print_message "$BLUE" "[DRY RUN] Would create secret: ${RELEASE_NAME}-cluster"
      echo "kubectl create secret generic ${RELEASE_NAME}-cluster \\"
      [[ -n "$CLUSTER_ID" ]] && echo "  --from-literal=cluster-id=$CLUSTER_ID \\"
      [[ -n "$CLUSTER_NAME" ]] && echo "  --from-literal=cluster-name=$CLUSTER_NAME \\"
      echo "  -n $NAMESPACE"
    else
      local cmd="kubectl create secret generic ${RELEASE_NAME}-cluster"
      [[ -n "$CLUSTER_ID" ]] && cmd="$cmd --from-literal=cluster-id=\"$CLUSTER_ID\""
      [[ -n "$CLUSTER_NAME" ]] && cmd="$cmd --from-literal=cluster-name=\"$CLUSTER_NAME\""
      cmd="$cmd -n $NAMESPACE --dry-run=client -o yaml"

      eval "$cmd" | kubectl apply -f -

      print_message "$GREEN" "✓ Created secret: ${RELEASE_NAME}-cluster"
      secret_created=true
    fi
  fi

  if [[ "$secret_created" == "false" ]]; then
    print_message "$YELLOW" "⚠ No secrets were created (no credentials found to migrate)"
  fi
}

# Function to prepare new values file
prepare_new_values() {
  print_header "Preparing New Values Configuration"

  local new_values_file="$BACKUP_DIR/new-values.yaml"

  # Start with current values but remove deprecated fields
  helm get values "$RELEASE_NAME" -n "$NAMESPACE" >"$new_values_file"

  # Create migration values overlay
  cat >"$BACKUP_DIR/migration-values.yaml" <<EOF
# Migration to secure secrets
dashboard:
  url: "${DASHBOARD_URL:-}"
  apiToken:
    existingSecret: "${RELEASE_NAME}-dashboard"
    key: "api-token"
  cluster:
    existingSecret: "${RELEASE_NAME}-cluster"
    idKey: "cluster-id"
    nameKey: "cluster-name"

# Deprecated fields (to be removed)
config:
  apiToken: null
  dashboardUrl: null
  clusterId: null
  clusterName: null

rightsizerConfig:
  metricsBuffer:
    dashboard:
      apiToken: null
  multiTenancy:
    clusterId: null
    clusterName: null
EOF

  print_message "$GREEN" "✓ New values configuration prepared"
  print_message "$YELLOW" "  Review: $BACKUP_DIR/migration-values.yaml"
}

# Function to perform the upgrade
perform_upgrade() {
  print_header "Performing Helm Upgrade"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_message "$BLUE" "[DRY RUN] Would run:"
    echo "helm upgrade $RELEASE_NAME ./helm \\"
    echo "  -n $NAMESPACE \\"
    echo "  -f $BACKUP_DIR/new-values.yaml \\"
    echo "  -f $BACKUP_DIR/migration-values.yaml"
    echo ""
    print_message "$BLUE" "Run with DRY_RUN=false to perform actual migration"
  else
    print_message "$YELLOW" "Upgrading Helm release..."

    # Get the chart reference (could be a repo chart or local path)
    local chart_ref
    chart_ref=$(helm get metadata "$RELEASE_NAME" -n "$NAMESPACE" -o json | jq -r '.chart // "right-sizer/right-sizer"')

    helm upgrade "$RELEASE_NAME" "$chart_ref" \
      -n "$NAMESPACE" \
      -f "$BACKUP_DIR/new-values.yaml" \
      -f "$BACKUP_DIR/migration-values.yaml" \
      --wait \
      --timeout 5m

    print_message "$GREEN" "✓ Helm upgrade completed successfully"
  fi
}

# Function to verify the migration
verify_migration() {
  print_header "Verifying Migration"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_message "$BLUE" "[DRY RUN] Skipping verification"
    return
  fi

  # Check if secrets exist
  print_message "$YELLOW" "Checking secrets..."
  kubectl get secrets -n "$NAMESPACE" | grep -E "${RELEASE_NAME}-(dashboard|cluster)" || true

  # Check if deployment is running
  print_message "$YELLOW" "Checking deployment status..."
  kubectl rollout status deployment -n "$NAMESPACE" -l "app.kubernetes.io/name=right-sizer" --timeout=60s

  # Check environment variables in pod
  print_message "$YELLOW" "Checking pod environment variables..."
  local pod
  pod=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=right-sizer" -o jsonpath='{.items[0].metadata.name}')

  if [[ -n "$pod" ]]; then
    kubectl describe pod -n "$NAMESPACE" "$pod" | grep -E "DASHBOARD_API_TOKEN|CLUSTER_ID|CLUSTER_NAME" | head -5

    print_message "$GREEN" "✓ Migration verified successfully"
  else
    print_message "$RED" "✗ Could not verify pod - please check manually"
  fi
}

# Function to show post-migration instructions
show_instructions() {
  print_header "Post-Migration Instructions"

  cat <<EOF
${GREEN}✓ Migration completed successfully!${NC}

${YELLOW}Important Notes:${NC}

1. ${BLUE}Backup Location:${NC} $BACKUP_DIR
   - Original values: $BACKUP_DIR/values.yaml
   - Rollback script: $BACKUP_DIR/rollback.sh

2. ${BLUE}Secrets Created:${NC}
   - ${RELEASE_NAME}-dashboard (contains API token)
   - ${RELEASE_NAME}-cluster (contains cluster ID and name)

3. ${BLUE}Next Steps:${NC}
   a. Verify the application is working correctly
   b. Remove plain text credentials from any values files
   c. Update your CI/CD pipelines to use the new secret structure
   d. Consider implementing secret rotation

4. ${BLUE}Secret Management:${NC}
   To update tokens in the future:
   ${GREEN}kubectl create secret generic ${RELEASE_NAME}-dashboard \\
     --from-literal=api-token=\$NEW_TOKEN \\
     -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -${NC}

5. ${BLUE}If Issues Occur:${NC}
   Run the rollback script: ${GREEN}$BACKUP_DIR/rollback.sh${NC}

EOF

  if [[ "$DRY_RUN" == "true" ]]; then
    print_message "$YELLOW" "This was a DRY RUN. To perform the actual migration, run:"
    print_message "$GREEN" "DRY_RUN=false $0"
  fi
}

# Main execution
main() {
  print_header "Right-Sizer Secret Migration Tool"

  if [[ "$DRY_RUN" == "true" ]]; then
    print_message "$YELLOW" "Running in DRY RUN mode - no changes will be made"
  fi

  check_prerequisites
  backup_current_config
  extract_credentials
  create_secrets
  prepare_new_values
  perform_upgrade
  verify_migration
  show_instructions
}

# Show usage
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<EOF
Usage: $0 [OPTIONS]

Migrate Right-Sizer from plain text credentials to Kubernetes Secrets.

Environment Variables:
  NAMESPACE        Kubernetes namespace (default: right-sizer)
  RELEASE_NAME     Helm release name (default: right-sizer)
  DRY_RUN          Run without making changes (default: false)

Examples:
  # Dry run to see what would be done
  DRY_RUN=true $0

  # Perform actual migration
  DRY_RUN=false $0

  # Migrate a specific release
  NAMESPACE=production RELEASE_NAME=my-right-sizer $0

EOF
  exit 0
fi

# Run main function
main
