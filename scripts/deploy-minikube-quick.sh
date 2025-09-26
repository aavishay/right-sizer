#!/bin/bash

# Right-Sizer Quick Deploy to Minikube
# Simple one-command deployment script with multi-platform support

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}     Right-Sizer Quick Deploy to Minikube (Multi-Platform)     ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Configuration
PROFILE="right-sizer"
NAMESPACE="right-sizer"

# Step 1: Start Minikube
echo -e "\n${BLUE}[1/5] Starting Minikube...${NC}"
if minikube status -p "$PROFILE" &>/dev/null; then
  echo -e "${YELLOW}Minikube profile '$PROFILE' already exists, starting...${NC}"
  minikube start -p "$PROFILE"
else
  minikube start -p "$PROFILE" \
    --kubernetes-version=stable \
    --driver=docker \
    --cpus=4 \
    --memory=6144 \
    --addons=metrics-server
fi
echo -e "${GREEN}✓ Minikube ready${NC}"

# Step 2: Set kubectl context
echo -e "\n${BLUE}[2/5] Setting kubectl context...${NC}"
kubectl config use-context "$PROFILE"
echo -e "${GREEN}✓ Context set to '$PROFILE'${NC}"

# Step 3: Build multi-platform image
echo -e "\n${BLUE}[3/5] Building multi-platform Docker image (linux/amd64, linux/arm64)...${NC}"
eval $(minikube -p "$PROFILE" docker-env)

# Get version info
VERSION=$(cat VERSION 2>/dev/null || echo "0.0.0-dev")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Create buildx builder if it doesn't exist
docker buildx create --use --name right-sizer-builder --driver docker-container 2>/dev/null ||
  docker buildx use right-sizer-builder 2>/dev/null || true

# Build multi-platform image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t right-sizer:local \
  --build-arg VERSION="$VERSION" \
  --build-arg GIT_COMMIT="$GIT_COMMIT" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  -f Dockerfile \
  --load \
  . || {
  echo -e "${YELLOW}Multi-platform build failed, trying single platform...${NC}"
  docker build \
    -t right-sizer:local \
    --build-arg VERSION="$VERSION" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    -f Dockerfile \
    .
}

echo -e "${GREEN}✓ Docker image built${NC}"

# Step 4: Deploy with Helm
echo -e "\n${BLUE}[4/5] Deploying Right-Sizer with Helm...${NC}"
helm upgrade --install right-sizer ./helm \
  -n "$NAMESPACE" \
  --create-namespace \
  --set image.repository=right-sizer \
  --set image.tag=local \
  --set image.pullPolicy=IfNotPresent \
  --set rightsizerConfig.dryRun=false \
  --set rightsizerConfig.mode=balanced \
  --wait \
  --timeout 2m

echo -e "${GREEN}✓ Right-Sizer deployed${NC}"

# Step 5: Wait for pod to be ready
echo -e "\n${BLUE}[5/5] Waiting for pod to be ready...${NC}"
kubectl wait --for=condition=Ready pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --timeout=60s
echo -e "${GREEN}✓ Pod is ready${NC}"

# Display status
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                    Deployment Complete!                        ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "\n${GREEN}Status:${NC}"
kubectl get pods -n "$NAMESPACE"

echo -e "\n${GREEN}Quick Commands:${NC}"
echo "  View logs:        kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"
echo "  Port forward:     kubectl port-forward -n $NAMESPACE svc/right-sizer 9090:9090"
echo "  Check metrics:    curl http://localhost:9090/metrics  (after port-forward)"
echo "  Deploy test app:  kubectl create deployment nginx --image=nginx --replicas=3"
echo "  Uninstall:        helm uninstall right-sizer -n $NAMESPACE"
echo "  Delete cluster:   minikube delete -p $PROFILE"

# Create test deployment
echo -e "\n${YELLOW}Create a test deployment to see Right-Sizer in action? (y/N)${NC} "
read -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  kubectl create namespace test-apps 2>/dev/null || true
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-nginx
  namespace: test-apps
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-nginx
  template:
    metadata:
      labels:
        app: test-nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        ports:
        - containerPort: 80
EOF
  echo -e "${GREEN}✓ Test deployment created in 'test-apps' namespace${NC}"
  echo -e "${YELLOW}Watch the pods: kubectl get pods -n test-apps -w${NC}"
  echo -e "${YELLOW}Watch Right-Sizer logs to see it in action!${NC}"
fi

echo -e "\n${GREEN}Done! Right-Sizer is now running on Minikube.${NC}"
