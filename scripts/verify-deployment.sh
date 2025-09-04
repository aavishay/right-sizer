#!/bin/bash

# Right-Sizer Deployment Verification Script
# This script verifies the health and status of the Right-Sizer deployment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${NAMESPACE:-right-sizer}"
DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-right-sizer}"
CONFIG_NAME="${CONFIG_NAME:-default}"

echo "=========================================="
echo "   Right-Sizer Deployment Verification"
echo "=========================================="
echo ""

# Function to print status
print_status() {
  local status=$1
  local message=$2
  if [ "$status" = "OK" ]; then
    echo -e "${GREEN}✓${NC} $message"
  elif [ "$status" = "WARNING" ]; then
    echo -e "${YELLOW}⚠${NC} $message"
  else
    echo -e "${RED}✗${NC} $message"
  fi
}

# Function to check command success
check_command() {
  if "$@" &>/dev/null; then
    return 0
  else
    return 1
  fi
}

echo -e "${BLUE}Checking Deployment Status...${NC}"
echo "----------------------------------------"

# Check if namespace exists
if kubectl get namespace "$NAMESPACE" &>/dev/null; then
  print_status "OK" "Namespace '$NAMESPACE' exists"
else
  print_status "ERROR" "Namespace '$NAMESPACE' not found"
  exit 1
fi

# Check deployment
DEPLOYMENT_STATUS=$(kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "NotFound")
if [ "$DEPLOYMENT_STATUS" = "True" ]; then
  REPLICAS=$(kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.replicas}')
  READY_REPLICAS=$(kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
  print_status "OK" "Deployment is available ($READY_REPLICAS/$REPLICAS replicas ready)"

  if [ "$REPLICAS" -gt 1 ]; then
    # Check if leader election is enabled
    LEADER_ELECTION=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.spec.operatorConfig.leaderElection}' 2>/dev/null || echo "false")
    if [ "$LEADER_ELECTION" = "true" ]; then
      print_status "OK" "Leader election is enabled for $REPLICAS replicas"
    else
      print_status "WARNING" "Leader election is disabled but running $REPLICAS replicas (may cause conflicts)"
    fi
  fi
elif [ "$DEPLOYMENT_STATUS" = "NotFound" ]; then
  print_status "ERROR" "Deployment '$DEPLOYMENT_NAME' not found in namespace '$NAMESPACE'"
  exit 1
else
  print_status "ERROR" "Deployment is not available"
fi

echo ""
echo -e "${BLUE}Checking Pod Status...${NC}"
echo "----------------------------------------"

# Check pods
POD_COUNT=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name="$DEPLOYMENT_NAME" --no-headers 2>/dev/null | wc -l)
if [ "$POD_COUNT" -gt 0 ]; then
  print_status "OK" "Found $POD_COUNT pod(s)"

  # Check each pod's status
  kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name="$DEPLOYMENT_NAME" --no-headers | while read -r line; do
    POD_NAME=$(echo "$line" | awk '{print $1}')
    POD_STATUS=$(echo "$line" | awk '{print $3}')
    POD_READY=$(echo "$line" | awk '{print $2}')
    POD_RESTARTS=$(echo "$line" | awk '{print $4}')

    if [ "$POD_STATUS" = "Running" ]; then
      if [ "$POD_RESTARTS" -gt 5 ]; then
        print_status "WARNING" "Pod $POD_NAME is running but has $POD_RESTARTS restarts"
      else
        print_status "OK" "Pod $POD_NAME is $POD_STATUS ($POD_READY, Restarts: $POD_RESTARTS)"
      fi
    else
      print_status "ERROR" "Pod $POD_NAME is $POD_STATUS"
    fi
  done
else
  print_status "ERROR" "No pods found"
fi

echo ""
echo -e "${BLUE}Checking Service Endpoints...${NC}"
echo "----------------------------------------"

# Check service
if kubectl get service "$DEPLOYMENT_NAME" -n "$NAMESPACE" &>/dev/null; then
  SERVICE_IP=$(kubectl get service "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')
  print_status "OK" "Service exists (ClusterIP: $SERVICE_IP)"

  # Check endpoints
  ENDPOINTS=$(kubectl get endpoints "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)
  if [ -n "$ENDPOINTS" ]; then
    ENDPOINT_COUNT=$(echo "$ENDPOINTS" | wc -w)
    print_status "OK" "Service has $ENDPOINT_COUNT endpoint(s)"
  else
    print_status "WARNING" "Service has no endpoints"
  fi
else
  print_status "WARNING" "Service not found"
fi

echo ""
echo -e "${BLUE}Checking Configuration...${NC}"
echo "----------------------------------------"

# Check RightSizerConfig
CONFIG_STATUS=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
if [ "$CONFIG_STATUS" = "Active" ]; then
  print_status "OK" "RightSizerConfig '$CONFIG_NAME' is Active"

  # Check if enabled
  ENABLED=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.spec.enabled}' 2>/dev/null)
  if [ "$ENABLED" = "true" ]; then
    print_status "OK" "Right-sizing is enabled"
  else
    print_status "WARNING" "Right-sizing is disabled"
  fi

  # Check mode
  MODE=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.spec.defaultMode}' 2>/dev/null)
  print_status "OK" "Operating in '$MODE' mode"

  # Check dry-run
  DRY_RUN=$(kubectl get rightsizerconfig "$CONFIG_NAME" -o jsonpath='{.spec.dryRun}' 2>/dev/null)
  if [ "$DRY_RUN" = "true" ]; then
    print_status "WARNING" "Dry-run mode is enabled (no actual resizing will occur)"
  else
    print_status "OK" "Dry-run mode is disabled (actual resizing enabled)"
  fi
elif [ "$CONFIG_STATUS" = "NotFound" ]; then
  print_status "ERROR" "RightSizerConfig '$CONFIG_NAME' not found"
else
  print_status "WARNING" "RightSizerConfig status is '$CONFIG_STATUS'"
fi

echo ""
echo -e "${BLUE}Checking for Errors in Logs...${NC}"
echo "----------------------------------------"

# Get a representative pod
POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name="$DEPLOYMENT_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -n "$POD_NAME" ]; then
  # Check for errors in the last 5 minutes
  ERROR_COUNT=$(kubectl logs "$POD_NAME" -n "$NAMESPACE" --since=5m 2>/dev/null | grep -iE "ERROR|error|panic|fatal" | wc -l)

  if [ "$ERROR_COUNT" -eq 0 ]; then
    print_status "OK" "No errors found in logs (last 5 minutes)"
  else
    print_status "WARNING" "Found $ERROR_COUNT error(s) in logs (last 5 minutes)"
    echo "  Recent errors:"
    kubectl logs "$POD_NAME" -n "$NAMESPACE" --since=5m 2>/dev/null | grep -iE "ERROR|error" | tail -3 | sed 's/^/  /'
  fi

  # Check for reconciliation conflicts
  CONFLICT_COUNT=$(kubectl logs "$POD_NAME" -n "$NAMESPACE" --since=5m 2>/dev/null | grep -i "object has been modified" | wc -l)
  if [ "$CONFLICT_COUNT" -gt 0 ]; then
    print_status "ERROR" "Found $CONFLICT_COUNT reconciliation conflict(s) - leader election may not be working"
  fi
else
  print_status "WARNING" "Could not check logs (no pods available)"
fi

echo ""
echo -e "${BLUE}Checking RBAC Permissions...${NC}"
echo "----------------------------------------"

# Check service account
SA_NAME="${DEPLOYMENT_NAME}"
if kubectl get serviceaccount "$SA_NAME" -n "$NAMESPACE" &>/dev/null; then
  print_status "OK" "ServiceAccount '$SA_NAME' exists"

  # Check critical permissions
  if kubectl auth can-i get deployments --all-namespaces --as="system:serviceaccount:${NAMESPACE}:${SA_NAME}" &>/dev/null; then
    print_status "OK" "Can read deployments"
  else
    print_status "ERROR" "Cannot read deployments"
  fi

  if kubectl auth can-i update deployments --all-namespaces --as="system:serviceaccount:${NAMESPACE}:${SA_NAME}" &>/dev/null; then
    print_status "OK" "Can update deployments"
  else
    print_status "ERROR" "Cannot update deployments"
  fi

  # Check leader election permissions
  if kubectl auth can-i create configmaps -n "$NAMESPACE" --as="system:serviceaccount:${NAMESPACE}:${SA_NAME}" &>/dev/null; then
    print_status "OK" "Can create ConfigMaps (leader election)"
  else
    print_status "WARNING" "Cannot create ConfigMaps (needed for leader election)"
  fi
else
  print_status "ERROR" "ServiceAccount '$SA_NAME' not found"
fi

echo ""
echo -e "${BLUE}Checking Health Endpoints...${NC}"
echo "----------------------------------------"

# Test health endpoint if possible
if [ -n "$POD_NAME" ]; then
  # Try to check readiness probe
  READY=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
  if [ "$READY" = "True" ]; then
    print_status "OK" "Pod readiness probe is passing"
  else
    print_status "WARNING" "Pod readiness probe is not passing"
  fi

  # Check if liveness probe is configured
  LIVENESS_PROBE=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.containers[0].livenessProbe}' 2>/dev/null)
  if [ -n "$LIVENESS_PROBE" ]; then
    print_status "OK" "Liveness probe is configured"
  else
    print_status "WARNING" "No liveness probe configured"
  fi
fi

echo ""
echo -e "${BLUE}Checking Monitored Resources...${NC}"
echo "----------------------------------------"

# Count deployments in non-system namespaces
MONITORED_DEPLOYMENTS=$(kubectl get deployments --all-namespaces 2>/dev/null | grep -v -E "kube-system|kube-public|kube-node-lease|right-sizer" | tail -n +2 | wc -l)
if [ "$MONITORED_DEPLOYMENTS" -gt 0 ]; then
  print_status "OK" "Found $MONITORED_DEPLOYMENTS potential deployment(s) to monitor"
else
  print_status "WARNING" "No deployments found to monitor"
fi

# Check for policies
POLICY_COUNT=$(kubectl get rightsizerpolicy --all-namespaces 2>/dev/null | tail -n +2 | wc -l)
if [ "$POLICY_COUNT" -gt 0 ]; then
  print_status "OK" "Found $POLICY_COUNT RightSizerPolicy(ies)"
else
  print_status "OK" "No RightSizerPolicies defined (using default configuration)"
fi

echo ""
echo "=========================================="
echo -e "${BLUE}Verification Summary${NC}"
echo "=========================================="

# Generate summary
ISSUES=0

# Check for critical issues
if [ "$DEPLOYMENT_STATUS" != "True" ]; then
  ((ISSUES++))
  echo -e "${RED}✗${NC} Deployment is not healthy"
fi

if [ "$CONFIG_STATUS" != "Active" ] && [ "$CONFIG_STATUS" != "NotFound" ]; then
  ((ISSUES++))
  echo -e "${RED}✗${NC} Configuration is not active"
fi

if [ "$CONFLICT_COUNT" -gt 0 ] 2>/dev/null; then
  ((ISSUES++))
  echo -e "${RED}✗${NC} Reconciliation conflicts detected"
fi

if [ "$ERROR_COUNT" -gt 10 ] 2>/dev/null; then
  ((ISSUES++))
  echo -e "${RED}✗${NC} High number of errors in logs"
fi

if [ "$ISSUES" -eq 0 ]; then
  echo -e "${GREEN}✓ All checks passed! Right-Sizer is operating normally.${NC}"
  EXIT_CODE=0
else
  echo -e "${YELLOW}⚠ Found $ISSUES issue(s) that need attention.${NC}"
  EXIT_CODE=1
fi

echo ""
echo "Run this script periodically to monitor Right-Sizer health."
echo "For detailed logs, run: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=$DEPLOYMENT_NAME"

exit $EXIT_CODE
