#!/bin/bash

# Quick Right-Sizer Deployment Script
# Bypasses slow Docker builds with faster alternatives

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

NAMESPACE="right-sizer"
CLUSTER_NAME="right-sizer-cluster"

print_info "🚀 Quick Right-Sizer Deployment"

# Check if cluster exists, create if not
if ! minikube profile list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
  print_info "Creating minikube cluster..."
  minikube start -p $CLUSTER_NAME \
    --kubernetes-version=v1.29.0 \
    --memory=4096 \
    --cpus=2 \
    --feature-gates=InPlacePodVerticalScaling=true
else
  print_info "Using existing cluster..."
  minikube profile $CLUSTER_NAME
  if ! minikube status -p $CLUSTER_NAME | grep -q "Running"; then
    minikube start -p $CLUSTER_NAME
  fi
fi

# Set context
kubectl config use-context $CLUSTER_NAME

# Enable metrics-server
print_info "Enabling metrics-server..."
minikube addons enable metrics-server -p $CLUSTER_NAME

# Wait for metrics server
print_info "Waiting for metrics-server..."
kubectl wait --for=condition=available deployment/metrics-server -n kube-system --timeout=120s

# Create namespace
print_info "Creating namespace..."
kubectl create namespace $NAMESPACE 2>/dev/null || true

# Configure Docker environment
print_info "Configuring Docker environment..."
eval $(minikube -p $CLUSTER_NAME docker-env)

# Build with simple Dockerfile (much faster)
print_info "Building operator image (fast build)..."
docker build -f Dockerfile.simple -t right-sizer:latest .

# Build dashboard image
print_info "Building dashboard image..."
cd ../right-sizer-dashboard
if [ -f Dockerfile ]; then
  docker build -t right-sizer-dashboard:latest .
else
  # Fallback: create a simple nginx-based dashboard
  cat >Dockerfile.simple <<EOF
FROM nginx:alpine
COPY . /usr/share/nginx/html/
EXPOSE 80
EOF
  docker build -f Dockerfile.simple -t right-sizer-dashboard:latest .
fi
cd ../right-sizer

print_success "Images built successfully"

# Deploy using Helm
print_info "Deploying with Helm..."
helm upgrade --install right-sizer ./helm \
  --namespace $NAMESPACE \
  --set image.repository=right-sizer \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent \
  --wait --timeout=300s

# Deploy dashboard separately if not in Helm chart
print_info "Deploying dashboard..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-dashboard
  template:
    metadata:
      labels:
        app: right-sizer-dashboard
    spec:
      containers:
      - name: dashboard
        image: right-sizer-dashboard:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
spec:
  type: NodePort
  ports:
  - port: 80
    targetPort: 80
    nodePort: 30080
  selector:
    app: right-sizer-dashboard
EOF

# Wait for deployments
print_info "Waiting for deployments..."
kubectl wait --for=condition=available deployment/right-sizer -n $NAMESPACE --timeout=180s
kubectl wait --for=condition=available deployment/right-sizer-dashboard -n $NAMESPACE --timeout=180s || true

# Create test workloads
print_info "Creating test workloads..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: test-workloads
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: test-workloads
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 500m  # Over-provisioned
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        ports:
        - containerPort: 80
EOF

# Setup port forwarding in background
print_info "Setting up port forwarding..."
kubectl port-forward -n $NAMESPACE svc/right-sizer-dashboard 3000:80 >/dev/null 2>&1 &
kubectl port-forward -n $NAMESPACE svc/right-sizer 8081:8080 >/dev/null 2>&1 &

# Wait a moment for port forwards to establish
sleep 3

# Show status
print_success "✅ Deployment complete!"
echo ""
echo "📊 Dashboard: http://localhost:3000"
echo "📈 Metrics: http://localhost:8081/metrics"
echo "🔗 NodePort: http://$(minikube -p $CLUSTER_NAME ip):30080"
echo ""
echo "🔍 Useful commands:"
echo "  kubectl logs -f deployment/right-sizer -n $NAMESPACE"
echo "  kubectl get pods -A"
echo "  kubectl top pods -A"
echo ""
echo "🧪 Test the resize functionality:"
echo "  kubectl get pods -n test-workloads -w"
echo ""

# Verify RBAC
if kubectl get clusterrole right-sizer -o yaml | grep -q "pods/resize"; then
  print_success "✅ RBAC includes pods/resize permission"
else
  print_warning "⚠️  pods/resize permission may be missing"
fi

# Check for forbidden errors
print_info "Checking for RBAC errors..."
sleep 5 # Let operator start
if kubectl logs deployment/right-sizer -n $NAMESPACE --tail=50 2>/dev/null | grep -i forbidden; then
  print_warning "Found forbidden errors in logs"
else
  print_success "No forbidden errors detected"
fi

print_info "Press Ctrl+C to stop port forwarding and exit"
wait
