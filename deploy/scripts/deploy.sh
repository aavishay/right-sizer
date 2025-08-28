#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Deploy script for right-sizer operator with Kubernetes 1.33 in-place resize support
# This script builds and deploys the operator to your current Kubernetes context

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
OPERATOR_NAME="right-sizer"
NAMESPACE="default"
IMAGE_TAG="latest"
BUILD_LOCAL=true
USE_HELM=false

# Project root detection (two levels up from this script)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --namespace | -n)
    NAMESPACE="$2"
    shift 2
    ;;
  --tag | -t)
    IMAGE_TAG="$2"
    shift 2
    ;;
  --no-build)
    BUILD_LOCAL=false
    shift
    ;;
  --helm)
    USE_HELM=true
    shift
    ;;
  --help | -h)
    echo "Usage: $0 [options]"
    echo "Options:"
    echo "  --namespace, -n <namespace>  Namespace to deploy to (default: default)"
    echo "  --tag, -t <tag>             Image tag (default: latest)"
    echo "  --no-build                  Skip building the image locally"
    echo "  --helm                      Use Helm for deployment"
    echo "  --help, -h                  Show this help message"
    exit 0
    ;;
  *)
    echo -e "${RED}Unknown option: $1${NC}"
    exit 1
    ;;
  esac
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Right-Sizer Operator Deployment${NC}"
echo -e "${BLUE}  Kubernetes 1.33+ In-Place Resize${NC}"
echo -e "${BLUE}========================================${NC}"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

# Check kubectl
if ! command -v kubectl &>/dev/null; then
  echo -e "${RED}kubectl is not installed. Please install kubectl first.${NC}"
  exit 1
fi

KUBECTL_VERSION=$(kubectl version --client -o json | jq -r '.clientVersion.gitVersion')
echo -e "kubectl version: ${GREEN}${KUBECTL_VERSION}${NC}"

# Check Kubernetes cluster connection
if ! kubectl cluster-info &>/dev/null; then
  echo -e "${RED}Cannot connect to Kubernetes cluster. Please check your kubeconfig.${NC}"
  exit 1
fi

# Get Kubernetes server version
K8S_VERSION=$(kubectl version -o json | jq -r '.serverVersion.gitVersion')
echo -e "Kubernetes server version: ${GREEN}${K8S_VERSION}${NC}"

# Extract major and minor version
K8S_MAJOR=$(echo $K8S_VERSION | cut -d'.' -f1 | sed 's/v//')
K8S_MINOR=$(echo $K8S_VERSION | cut -d'.' -f2)

# Check if Kubernetes version supports in-place resize
if [[ $K8S_MAJOR -ge 1 && $K8S_MINOR -ge 33 ]]; then
  echo -e "${GREEN}✓ Kubernetes ${K8S_VERSION} supports in-place pod resize${NC}"

  # Check if resize subresource is available
  if kubectl api-resources | grep -q "pods/resize"; then
    echo -e "${GREEN}✓ Resize subresource is available${NC}"
  else
    echo -e "${YELLOW}⚠ Resize subresource not found. Feature may need to be enabled.${NC}"
  fi
else
  echo -e "${YELLOW}⚠ Kubernetes ${K8S_VERSION} does not support in-place resize (requires v1.33+)${NC}"
  echo -e "${YELLOW}  The operator will use fallback methods for resizing.${NC}"
fi

# Check current context
CURRENT_CONTEXT=$(kubectl config current-context)
echo -e "Current context: ${GREEN}${CURRENT_CONTEXT}${NC}"

# Check if using Minikube
if [[ "$CURRENT_CONTEXT" == "minikube" ]] && command -v minikube &>/dev/null; then
  echo -e "${YELLOW}Detected Minikube environment${NC}"
  if [[ "$BUILD_LOCAL" == true ]]; then
    echo -e "${YELLOW}Configuring Docker to use Minikube's daemon...${NC}"
    eval $(minikube docker-env)
  fi
fi

# Build the operator image if requested
if [[ "$BUILD_LOCAL" == true ]]; then
  echo -e "\n${YELLOW}Building operator image...${NC}"

  # Check if Go is installed for local build
  if ! command -v go &>/dev/null; then
    echo -e "${YELLOW}Go is not installed. Using Docker build instead.${NC}"
    docker build -t ${OPERATOR_NAME}:${IMAGE_TAG} .
  else
    # Build Go binary
    echo "Running go mod tidy..."
    cd "$ROOT_DIR/go"
    go mod tidy

    echo "Building binary..."
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ../${OPERATOR_NAME} main.go
    cd "$ROOT_DIR"

    # Build Docker image
    echo "Building Docker image..."
    docker build -t ${OPERATOR_NAME}:${IMAGE_TAG} .
  fi

  echo -e "${GREEN}✓ Image built: ${OPERATOR_NAME}:${IMAGE_TAG}${NC}"
else
  echo -e "\n${YELLOW}Skipping image build (using existing image)${NC}"
fi

# Create namespace if it doesn't exist
echo -e "\n${YELLOW}Checking namespace...${NC}"
if ! kubectl get namespace ${NAMESPACE} &>/dev/null; then
  echo -e "${YELLOW}Creating namespace ${NAMESPACE}...${NC}"
  kubectl create namespace ${NAMESPACE}
fi
echo -e "${GREEN}✓ Namespace ${NAMESPACE} is ready${NC}"

# Deploy the operator
echo -e "\n${YELLOW}Deploying operator...${NC}"

if [[ "$USE_HELM" == true ]]; then
  # Deploy using Helm
  if ! command -v helm &>/dev/null; then
    echo -e "${RED}Helm is not installed. Please install Helm or use manifest deployment.${NC}"
    exit 1
  fi

  echo -e "${YELLOW}Installing with Helm...${NC}"

  helm upgrade --install ${OPERATOR_NAME} ./helm \
    --namespace ${NAMESPACE} \
    --set image.repository=${OPERATOR_NAME} \
    --set image.tag=${IMAGE_TAG} \
    --set image.pullPolicy=IfNotPresent \
    --wait --timeout 5m

  echo -e "${GREEN}✓ Helm deployment complete${NC}"
else
  # Deploy using kubectl manifests
  echo -e "${YELLOW}Applying RBAC permissions...${NC}"
  kubectl apply -f "$ROOT_DIR/deploy/kubernetes/rbac.yaml" -n ${NAMESPACE}

  echo -e "${YELLOW}Deploying operator...${NC}"

  # Create temporary deployment manifest with correct image
  cat >/tmp/right-sizer-deployment.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${OPERATOR_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${OPERATOR_NAME}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${OPERATOR_NAME}
  template:
    metadata:
      labels:
        app: ${OPERATOR_NAME}
    spec:
      serviceAccountName: ${OPERATOR_NAME}
      containers:
      - name: ${OPERATOR_NAME}
        image: ${OPERATOR_NAME}:${IMAGE_TAG}
        imagePullPolicy: IfNotPresent
        env:
        - name: OPERATOR_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: ENABLE_INPLACE_RESIZE
          value: "true"
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
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
EOF

  kubectl apply -f /tmp/right-sizer-deployment.yaml
  rm /tmp/right-sizer-deployment.yaml

  echo -e "${GREEN}✓ Manifest deployment complete${NC}"
fi

# Wait for deployment to be ready
echo -e "\n${YELLOW}Waiting for operator to be ready...${NC}"
kubectl rollout status deployment/${OPERATOR_NAME} -n ${NAMESPACE} --timeout=120s

# Get pod status
POD_NAME=$(kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=${OPERATOR_NAME} -o jsonpath='{.items[0].metadata.name}')
if [[ -n "$POD_NAME" ]]; then
  echo -e "${GREEN}✓ Operator pod is running: ${POD_NAME}${NC}"

  # Show recent logs
  echo -e "\n${YELLOW}Recent operator logs:${NC}"
  kubectl logs ${POD_NAME} -n ${NAMESPACE} --tail=20
else
  echo -e "${RED}✗ Operator pod not found${NC}"
  exit 1
fi

# Deployment summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${GREEN}Deployment Summary:${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Operator: ${GREEN}${OPERATOR_NAME}${NC}"
echo -e "Namespace: ${GREEN}${NAMESPACE}${NC}"
echo -e "Image: ${GREEN}${OPERATOR_NAME}:${IMAGE_TAG}${NC}"
echo -e "Pod: ${GREEN}${POD_NAME}${NC}"
echo -e "Kubernetes: ${GREEN}${K8S_VERSION}${NC}"

if [[ $K8S_MAJOR -ge 1 && $K8S_MINOR -ge 33 ]]; then
  echo -e "In-Place Resize: ${GREEN}Enabled${NC}"
else
  echo -e "In-Place Resize: ${YELLOW}Not Available (K8s < 1.33)${NC}"
fi

echo -e "\n${BLUE}Useful Commands:${NC}"
echo -e "Watch logs: ${YELLOW}kubectl logs -f ${POD_NAME} -n ${NAMESPACE}${NC}"
echo -e "Check status: ${YELLOW}kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=${OPERATOR_NAME}${NC}"
echo -e "Describe pod: ${YELLOW}kubectl describe pod ${POD_NAME} -n ${NAMESPACE}${NC}"
echo -e "Test resize: ${YELLOW}./test-inplace-resize.sh${NC}"
echo -e "View metrics: ${YELLOW}kubectl top pods -n ${NAMESPACE}${NC}"

echo -e "\n${GREEN}✓ Deployment complete!${NC}"
