#!/bin/bash

# Test script for Right-Sizer health endpoints
# This script checks the health and readiness endpoints of the operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
HEALTH_PORT=${HEALTH_PORT:-8081}
NAMESPACE=${NAMESPACE:-default}
DEPLOYMENT_NAME=${DEPLOYMENT_NAME:-right-sizer}

echo "========================================="
echo "Right-Sizer Health Check Test"
echo "========================================="
echo ""

# Function to check if the deployment exists
check_deployment() {
  echo "Checking if deployment exists..."
  if kubectl get deployment $DEPLOYMENT_NAME -n $NAMESPACE &>/dev/null; then
    echo -e "${GREEN}✓${NC} Deployment '$DEPLOYMENT_NAME' found in namespace '$NAMESPACE'"
    return 0
  else
    echo -e "${RED}✗${NC} Deployment '$DEPLOYMENT_NAME' not found in namespace '$NAMESPACE'"
    return 1
  fi
}

# Function to get pod name
get_pod_name() {
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [ -z "$POD_NAME" ]; then
    echo -e "${RED}✗${NC} No right-sizer pod found"
    return 1
  fi
  echo -e "${GREEN}✓${NC} Found pod: $POD_NAME"
  return 0
}

# Function to check pod status
check_pod_status() {
  echo ""
  echo "Checking pod status..."
  kubectl get pod $POD_NAME -n $NAMESPACE
  echo ""
}

# Function to test health endpoint
test_health_endpoint() {
  local endpoint=$1
  local description=$2

  echo "Testing $description ($endpoint)..."

  # Port-forward to access the health endpoint
  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8081:$HEALTH_PORT &>/dev/null &
  PF_PID=$!

  # Wait for port-forward to establish
  sleep 2

  # Test the endpoint
  if curl -s -f http://localhost:8081/$endpoint >/dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} $description is healthy"
    RESULT=0
  else
    echo -e "${RED}✗${NC} $description check failed"
    RESULT=1
  fi

  # Kill port-forward
  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true

  return $RESULT
}

# Function to get detailed health status
get_detailed_health() {
  echo ""
  echo "Getting detailed health status..."

  # Port-forward to access the health endpoint
  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8081:$HEALTH_PORT &>/dev/null &
  PF_PID=$!

  # Wait for port-forward to establish
  sleep 2

  # Get detailed health
  echo "Detailed health check (/readyz/detailed):"
  curl -s http://localhost:8081/readyz/detailed 2>/dev/null || echo "Endpoint not available"
  echo ""

  # Kill port-forward
  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true
}

# Function to check container logs for health-related messages
check_health_logs() {
  echo ""
  echo "Recent health-related log entries:"
  kubectl logs $POD_NAME -n $NAMESPACE --tail=100 | grep -i "health\|probe\|liveness\|readiness" | tail -10 || echo "No health-related logs found"
}

# Function to describe pod events
check_pod_events() {
  echo ""
  echo "Recent pod events:"
  kubectl get events -n $NAMESPACE --field-selector involvedObject.name=$POD_NAME --sort-by='.lastTimestamp' | tail -5
}

# Main execution
main() {
  echo "Configuration:"
  echo "  Namespace: $NAMESPACE"
  echo "  Deployment: $DEPLOYMENT_NAME"
  echo "  Health Port: $HEALTH_PORT"
  echo ""

  # Check if kubectl is available
  if ! command -v kubectl &>/dev/null; then
    echo -e "${RED}✗${NC} kubectl command not found. Please install kubectl."
    exit 1
  fi

  # Check if curl is available
  if ! command -v curl &>/dev/null; then
    echo -e "${RED}✗${NC} curl command not found. Please install curl."
    exit 1
  fi

  # Check deployment
  if ! check_deployment; then
    echo ""
    echo "Deployment not found. Please deploy right-sizer first."
    echo "You can deploy it using: make deploy"
    exit 1
  fi

  # Get pod name
  if ! get_pod_name; then
    exit 1
  fi

  # Check pod status
  check_pod_status

  # Test liveness probe
  echo "----------------------------------------"
  test_health_endpoint "healthz" "Liveness probe"
  LIVENESS_RESULT=$?

  # Test readiness probe
  test_health_endpoint "readyz" "Readiness probe"
  READINESS_RESULT=$?

  # Get detailed health
  get_detailed_health

  # Check logs
  echo "----------------------------------------"
  check_health_logs

  # Check events
  echo "----------------------------------------"
  check_pod_events

  # Summary
  echo ""
  echo "========================================="
  echo "Health Check Summary:"
  echo "========================================="

  if [ $LIVENESS_RESULT -eq 0 ]; then
    echo -e "Liveness:  ${GREEN}✓ HEALTHY${NC}"
  else
    echo -e "Liveness:  ${RED}✗ UNHEALTHY${NC}"
  fi

  if [ $READINESS_RESULT -eq 0 ]; then
    echo -e "Readiness: ${GREEN}✓ READY${NC}"
  else
    echo -e "Readiness: ${RED}✗ NOT READY${NC}"
  fi

  echo "========================================="

  # Exit with appropriate code
  if [ $LIVENESS_RESULT -eq 0 ] && [ $READINESS_RESULT -eq 0 ]; then
    echo -e "\n${GREEN}All health checks passed!${NC}"
    exit 0
  else
    echo -e "\n${YELLOW}Some health checks failed. Check the logs above for details.${NC}"
    exit 1
  fi
}

# Handle cleanup on exit
cleanup() {
  # Kill any remaining port-forward processes
  pkill -f "kubectl port-forward.*$POD_NAME" 2>/dev/null || true
}

trap cleanup EXIT

# Run main function
main "$@"
