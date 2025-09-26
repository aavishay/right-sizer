#!/bin/bash

# Right-Sizer ARM64 Deployment Script for Minikube
# Specifically designed for Apple Silicon and ARM64 systems

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}       Right-Sizer ARM64 Deployment for Minikube               ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Configuration
PROFILE="${MINIKUBE_PROFILE:-right-sizer}"
NAMESPACE="${NAMESPACE:-right-sizer}"
RELEASE_NAME="${RELEASE_NAME:-right-sizer}"

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" != "arm64" && "$ARCH" != "aarch64" ]]; then
  echo -e "${YELLOW}Warning: This script is optimized for ARM64 architecture${NC}"
  echo -e "${YELLOW}Detected architecture: $ARCH${NC}"
fi

echo -e "\n${CYAN}System Information:${NC}"
echo "  Architecture: $ARCH"
echo "  Profile: $PROFILE"
echo "  Namespace: $NAMESPACE"

# Step 1: Ensure Minikube is running
echo -e "\n${BLUE}[Step 1/6] Checking Minikube...${NC}"
if ! minikube -p "$PROFILE" status &>/dev/null; then
  echo -e "${YELLOW}Starting Minikube...${NC}"
  minikube start -p "$PROFILE" \
    --driver=docker \
    --kubernetes-version=stable \
    --cpus=4 \
    --memory=6144 \
    --addons=metrics-server
else
  echo -e "${GREEN}✓ Minikube is running${NC}"
fi

# Verify Minikube architecture
MINIKUBE_ARCH=$(minikube -p "$PROFILE" ssh -- uname -m)
echo -e "${CYAN}Minikube architecture: $MINIKUBE_ARCH${NC}"

# Step 2: Set Docker environment to use Minikube's Docker daemon
echo -e "\n${BLUE}[Step 2/6] Configuring Docker environment...${NC}"
eval $(minikube -p "$PROFILE" docker-env)
echo -e "${GREEN}✓ Using Minikube Docker daemon${NC}"

# Step 3: Build the image directly for ARM64
echo -e "\n${BLUE}[Step 3/6] Building ARM64 Docker image...${NC}"

# Get version information
VERSION=$(cat VERSION 2>/dev/null || echo "0.0.0-dev")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "  Version: $VERSION"
echo "  Git Commit: $GIT_COMMIT"
echo "  Build Date: $BUILD_DATE"

# Create a modified Dockerfile for direct ARM64 build
cat >Dockerfile.arm64 <<'EOF'
# Build stage - using specific ARM64 base image
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go/go.mod go/go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY go/ ./

# Build arguments
ARG VERSION=dev
ARG BUILD_DATE
ARG GIT_COMMIT

# Build the binary specifically for ARM64
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=arm64

RUN go build -ldflags="-w -s \
    -X 'main.Version=${VERSION}' \
    -X 'main.BuildDate=${BUILD_DATE}' \
    -X 'main.GitCommit=${GIT_COMMIT}'" \
    -o right-sizer \
    .

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /build/right-sizer /usr/local/bin/right-sizer

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set user
USER 65532:65532

# Expose ports
EXPOSE 8080 8081 9090

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/right-sizer"]
EOF

# Build the image
docker build \
  -t right-sizer:arm64 \
  --build-arg VERSION="$VERSION" \
  --build-arg GIT_COMMIT="$GIT_COMMIT" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  -f Dockerfile.arm64 \
  .

if [ $? -eq 0 ]; then
  echo -e "${GREEN}✓ Docker image built successfully${NC}"
  docker images | grep right-sizer
else
  echo -e "${RED}✗ Docker build failed${NC}"
  exit 1
fi

# Step 4: Create namespace
echo -e "\n${BLUE}[Step 4/6] Creating namespace...${NC}"
kubectl create namespace "$NAMESPACE" 2>/dev/null || echo -e "${YELLOW}Namespace already exists${NC}"

# Step 5: Deploy with Helm using the ARM64 image
echo -e "\n${BLUE}[Step 5/6] Deploying Right-Sizer with Helm...${NC}"

helm upgrade --install "$RELEASE_NAME" ./helm \
  -n "$NAMESPACE" \
  --set image.repository=right-sizer \
  --set image.tag=arm64 \
  --set image.pullPolicy=IfNotPresent \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=512Mi \
  --wait \
  --timeout 3m

if [ $? -eq 0 ]; then
  echo -e "${GREEN}✓ Helm deployment successful${NC}"
else
  echo -e "${RED}✗ Helm deployment failed${NC}"
  echo -e "${YELLOW}Checking pod status...${NC}"
  kubectl get pods -n "$NAMESPACE"
  kubectl describe pods -n "$NAMESPACE"
  exit 1
fi

# Step 6: Verify deployment
echo -e "\n${BLUE}[Step 6/6] Verifying deployment...${NC}"

# Wait for pod to be ready
echo "Waiting for pod to be ready..."
kubectl wait --for=condition=Ready pods \
  -n "$NAMESPACE" \
  -l app.kubernetes.io/name=right-sizer \
  --timeout=120s

# Check pod status
POD_STATUS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].status.phase}')
if [ "$POD_STATUS" = "Running" ]; then
  echo -e "${GREEN}✓ Pod is running${NC}"

  # Show pod details
  echo -e "\n${CYAN}Pod Details:${NC}"
  kubectl get pods -n "$NAMESPACE" -o wide

  # Check logs
  echo -e "\n${CYAN}Recent Logs:${NC}"
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=20
else
  echo -e "${RED}✗ Pod is not running. Status: $POD_STATUS${NC}"

  # Show pod events for debugging
  echo -e "\n${YELLOW}Pod Events:${NC}"
  kubectl describe pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer

  # Show logs if available
  echo -e "\n${YELLOW}Pod Logs (if available):${NC}"
  kubectl logs -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer --tail=50 || true

  exit 1
fi

# Deployment successful
echo -e "\n${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}           Deployment Complete! (ARM64 Build)                  ${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"

echo -e "\n${CYAN}Quick Commands:${NC}"
echo "  View logs:     kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"
echo "  Port forward:  kubectl port-forward -n $NAMESPACE svc/$RELEASE_NAME 9090:9090"
echo "  Get metrics:   curl http://localhost:9090/metrics  (after port-forward)"
echo "  Check pods:    kubectl get pods -n $NAMESPACE -w"
echo "  Uninstall:     helm uninstall $RELEASE_NAME -n $NAMESPACE"

# Optional: Create test workload
echo -e "\n${YELLOW}Would you like to create a test workload? (y/N)${NC} "
read -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  kubectl create namespace test-workloads 2>/dev/null || true
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  namespace: test-workloads
spec:
  replicas: 3
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
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
EOF
  echo -e "${GREEN}✓ Test workload created in 'test-workloads' namespace${NC}"
  echo -e "${CYAN}Watch the workload: kubectl get pods -n test-workloads -w${NC}"
fi

# Cleanup temporary Dockerfile
rm -f Dockerfile.arm64

echo -e "\n${GREEN}Done! Right-Sizer is running on your ARM64 Minikube cluster.${NC}"
