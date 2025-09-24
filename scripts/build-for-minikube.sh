#!/bin/bash

# Build Right-Sizer Binary for Minikube
# This script ensures the binary is built for the correct architecture

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_success() { printf "${GREEN}✅ $1${NC}\n"; }
print_error() { printf "${RED}❌ $1${NC}\n"; }
print_warning() { printf "${YELLOW}⚠️  $1${NC}\n"; }
print_info() { printf "${BLUE}ℹ️  $1${NC}\n"; }

print_header() {
  echo "========================================"
  printf "${BLUE}$1${NC}\n"
  echo "========================================"
}

# Configuration
BINARY_NAME="right-sizer"
BUILD_DIR="./build"
GO_DIR="./go"

print_header "Building Right-Sizer for Minikube"

# Check prerequisites
if ! command -v go &>/dev/null; then
  print_error "Go is not installed or not in PATH"
  exit 1
fi

if ! command -v minikube &>/dev/null; then
  print_error "Minikube is not installed or not in PATH"
  exit 1
fi

if [ ! -d "$GO_DIR" ]; then
  print_error "Go source directory not found: $GO_DIR"
  exit 1
fi

print_success "Prerequisites check passed"

# Detect Minikube architecture
print_info "Detecting Minikube architecture..."
MINIKUBE_ARCH=$(minikube ssh -- uname -m 2>/dev/null || echo "unknown")

print_info "Raw architecture output: '$MINIKUBE_ARCH'"

case "$MINIKUBE_ARCH" in
"x86_64")
  TARGET_ARCH="amd64"
  print_info "Using amd64 for x86_64"
  ;;
"aarch64" | "arm64")
  TARGET_ARCH="arm64"
  print_info "Using arm64 for aarch64/arm64"
  ;;
*)
  print_warning "Unknown Minikube architecture: $MINIKUBE_ARCH"
  print_info "Defaulting to amd64"
  TARGET_ARCH="amd64"
  ;;
esac

print_success "Target architecture: linux/$TARGET_ARCH (Minikube: $MINIKUBE_ARCH)"

# Create build directory
mkdir -p "$BUILD_DIR"

# Build binary
print_info "Building binary..."
cd "$GO_DIR"

# Get version info
VERSION=$(cat ../VERSION 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

print_info "Build info:"
echo "  Version: $VERSION"
echo "  Build Date: $BUILD_DATE"
echo "  Git Commit: $GIT_COMMIT"

# Build with explicit architecture targeting
CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=$TARGET_ARCH \
  go build -a -installsuffix cgo \
  -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
  -o "../${BUILD_DIR}/${BINARY_NAME}-linux-${TARGET_ARCH}" \
  main.go

cd ..

if [ ! -f "${BUILD_DIR}/${BINARY_NAME}-linux-${TARGET_ARCH}" ]; then
  print_error "Build failed - binary not created"
  exit 1
fi

print_success "Binary built successfully"

# Verify binary
print_info "Verifying binary..."
file "${BUILD_DIR}/${BINARY_NAME}-linux-${TARGET_ARCH}"

# Create symlink for easy access
ln -sf "${BINARY_NAME}-linux-${TARGET_ARCH}" "${BUILD_DIR}/${BINARY_NAME}"
print_success "Created symlink: ${BUILD_DIR}/${BINARY_NAME}"

# Build Docker image for Minikube
print_header "Building Docker Image for Minikube"

# Set Minikube Docker environment
eval $(minikube docker-env)
print_success "Minikube Docker environment configured"

# Create temporary Dockerfile
cat >Dockerfile.minikube <<EOF
FROM gcr.io/distroless/static-debian12:nonroot
COPY build/${BINARY_NAME}-linux-${TARGET_ARCH} /usr/local/bin/right-sizer
ENTRYPOINT ["/usr/local/bin/right-sizer"]
EOF

# Build image
IMAGE_TAG="right-sizer:minikube-${TARGET_ARCH}"
docker build -f Dockerfile.minikube -t "$IMAGE_TAG" .

print_success "Docker image built: $IMAGE_TAG"

# Verify image
print_info "Verifying Docker image..."
if docker run --rm "$IMAGE_TAG" --version 2>/dev/null; then
  print_success "Image verification passed"
else
  print_warning "Image verification failed, but this might be normal if --version is not implemented"
fi

# Clean up
rm -f Dockerfile.minikube

# Create deployment helper
print_header "Creating Deployment Files"

# Create values file for Helm deployment
cat >helm-values-minikube.yaml <<EOF
# Minikube-specific Helm values
replicaCount: 1

image:
  repository: right-sizer
  tag: minikube-${TARGET_ARCH}
  pullPolicy: IfNotPresent

# Resources suitable for Minikube
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

# Fast intervals for testing
rightsizerConfig:
  enabled: true
  dryRun: false
  defaultMode: "balanced"
  resizeInterval: "30s"

  # Self-protection: exclude right-sizer namespace
  namespaceConfig:
    includeNamespaces: []
    excludeNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"
      - "right-sizer"  # Critical for self-protection
    systemNamespaces:
      - "kube-system"
      - "kube-public"
      - "kube-node-lease"

  # Debugging enabled
  logging:
    level: "debug"
    format: "json"

  # Conservative resource strategy for testing
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 0.8
      limitMultiplier: 1.2
      minRequest: "10m"
      maxLimit: "1000m"
    memory:
      requestMultiplier: 0.9
      limitMultiplier: 1.1
      minRequest: "32Mi"
      maxLimit: "1Gi"
EOF

print_success "Created helm-values-minikube.yaml"

# Create deployment script
cat >scripts/deploy-to-minikube.sh <<'EOF'
#!/bin/bash

set -e

NAMESPACE="right-sizer"
RELEASE_NAME="right-sizer"

echo "🚀 Deploying Right-Sizer to Minikube..."

# Ensure Minikube Docker environment
eval $(minikube docker-env)

# Create namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Deploy with Helm
helm upgrade --install $RELEASE_NAME ./helm \
    --namespace $NAMESPACE \
    --values helm-values-minikube.yaml \
    --wait \
    --timeout 5m

echo "✅ Deployment complete!"
echo ""
echo "📋 Check status:"
echo "  kubectl get pods -n $NAMESPACE"
echo ""
echo "📜 View logs:"
echo "  kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=right-sizer -f"
echo ""
echo "🧪 Test self-protection:"
echo "  ./scripts/test-working-pod.sh"
EOF

chmod +x scripts/deploy-to-minikube.sh
print_success "Created scripts/deploy-to-minikube.sh"

print_header "Build Complete!"
print_success "Binary: ${BUILD_DIR}/${BINARY_NAME}-linux-${TARGET_ARCH}"
print_success "Docker Image: $IMAGE_TAG"
print_success "Helm Values: helm-values-minikube.yaml"
print_success "Deploy Script: scripts/deploy-to-minikube.sh"

echo ""
print_info "Next steps:"
echo "1. Deploy to Minikube:"
echo "   ./scripts/deploy-to-minikube.sh"
echo ""
echo "2. Test self-protection:"
echo "   ./scripts/test-working-pod.sh"
echo ""
echo "3. Monitor logs:"
echo "   kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer -f"
