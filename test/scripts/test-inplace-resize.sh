#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Test script for verifying Kubernetes 1.33+ in-place pod resize functionality
# This script creates a test deployment, monitors it, and verifies resize operations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Kubernetes In-Place Pod Resize Test ===${NC}"

# Check kubectl version
echo -e "\n${YELLOW}Checking kubectl version...${NC}"
kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion' || kubectl version --client

# Check if resize subresource is available
echo -e "\n${YELLOW}Checking for resize subresource support...${NC}"
# Use kubectl 1.33+ if available
if [ -x "/opt/homebrew/Cellar/kubernetes-cli/1.33.4/bin/kubectl" ]; then
  KUBECTL="/opt/homebrew/Cellar/kubernetes-cli/1.33.4/bin/kubectl"
  echo -e "${GREEN}✓ Using kubectl 1.33.4 for resize operations${NC}"
else
  KUBECTL="kubectl"
  echo -e "${YELLOW}⚠ Using default kubectl - resize may not work${NC}"
fi

# Create test namespace
NAMESPACE="resize-test-$(date +%s)"
echo -e "\n${YELLOW}Creating test namespace: ${NAMESPACE}${NC}"
kubectl create namespace $NAMESPACE

# Create a simple nginx deployment
echo -e "\n${YELLOW}Creating test deployment...${NC}"
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-test
  template:
    metadata:
      labels:
        app: nginx-test
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
        ports:
        - containerPort: 80
EOF

# Wait for pod to be ready
echo -e "\n${YELLOW}Waiting for pod to be ready...${NC}"
kubectl wait --for=condition=ready pod -l app=nginx-test -n $NAMESPACE --timeout=60s

# Get pod name
POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=nginx-test -o jsonpath='{.items[0].metadata.name}')
echo -e "${GREEN}Pod ready: ${POD_NAME}${NC}"

# Get initial state
echo -e "\n${YELLOW}Initial pod state:${NC}"
echo "Restart count: $(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')"
echo "Current resources:"
kubectl get pod $POD_NAME -n $NAMESPACE -o json | jq '.spec.containers[0].resources'

# Store initial restart count and creation time
INITIAL_RESTART_COUNT=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')
INITIAL_START_TIME=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].state.running.startedAt}')

# Perform in-place resize
echo -e "\n${YELLOW}Performing in-place resize...${NC}"
$KUBECTL patch pod $POD_NAME -n $NAMESPACE --subresource resize --patch \
  '{"spec": {"containers": [{"name": "nginx", "resources": {"requests": {"cpu": "150m", "memory": "192Mi"}, "limits": {"cpu": "300m", "memory": "384Mi"}}}]}}'

# Wait a moment for the resize to take effect
sleep 3

# Get post-resize state
echo -e "\n${YELLOW}Post-resize pod state:${NC}"
POST_RESTART_COUNT=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].restartCount}')
POST_START_TIME=$(kubectl get pod $POD_NAME -n $NAMESPACE -o jsonpath='{.status.containerStatuses[0].state.running.startedAt}')

echo "Restart count: $POST_RESTART_COUNT"
echo "Updated resources:"
kubectl get pod $POD_NAME -n $NAMESPACE -o json | jq '.spec.containers[0].resources'

# Verify no restart occurred
echo -e "\n${YELLOW}Verification:${NC}"
if [ "$INITIAL_RESTART_COUNT" == "$POST_RESTART_COUNT" ] && [ "$INITIAL_START_TIME" == "$POST_START_TIME" ]; then
  echo -e "${GREEN}✓ SUCCESS: Pod was resized in-place without restart!${NC}"
  echo "  - Restart count unchanged: $INITIAL_RESTART_COUNT"
  echo "  - Container start time unchanged: $INITIAL_START_TIME"
else
  echo -e "${RED}✗ FAILURE: Pod appears to have restarted${NC}"
  echo "  - Initial restart count: $INITIAL_RESTART_COUNT, Current: $POST_RESTART_COUNT"
  echo "  - Initial start time: $INITIAL_START_TIME, Current: $POST_START_TIME"
fi

# Check events
echo -e "\n${YELLOW}Recent pod events:${NC}"
kubectl describe pod $POD_NAME -n $NAMESPACE | grep -A 10 "Events:" || echo "No events found"

# Check allocated resources (if metrics available)
echo -e "\n${YELLOW}Checking allocated resources:${NC}"
kubectl get pod $POD_NAME -n $NAMESPACE -o json | jq '.status.resize // "Resize status not available"'

# Show container status
echo -e "\n${YELLOW}Container allocations:${NC}"
kubectl get pod $POD_NAME -n $NAMESPACE -o json | jq '.status.containerStatuses[0].allocatedResources // "Allocated resources not shown"'

# Cleanup
echo -e "\n${YELLOW}Cleanup (press Enter to delete test namespace, or Ctrl+C to keep it)${NC}"
read -r
kubectl delete namespace $NAMESPACE
echo -e "${GREEN}Test namespace deleted${NC}"

echo -e "\n${GREEN}=== Test Complete ===${NC}"
