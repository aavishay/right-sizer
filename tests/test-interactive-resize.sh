#!/bin/bash

# Interactive Test for Right-Sizer Operator
# This script sets up a test environment and allows you to observe the operator in action

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-right-sizer}
TEST_NAMESPACE=${TEST_NAMESPACE:-test-workloads}

print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

print_header() {
  echo ""
  print_color $CYAN "========================================="
  print_color $CYAN "$1"
  print_color $CYAN "========================================="
}

show_commands() {
  print_header "Useful Commands"
  print_color $YELLOW "Watch pods being resized:"
  echo "  kubectl get pods -n $TEST_NAMESPACE -w"
  echo ""
  print_color $YELLOW "Monitor operator logs:"
  echo "  kubectl logs -n $NAMESPACE deployment/right-sizer -f"
  echo ""
  print_color $YELLOW "Check pod resources:"
  echo "  kubectl get pods -n $TEST_NAMESPACE -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,MEM_LIM:.spec.containers[0].resources.limits.memory"
  echo ""
  print_color $YELLOW "See resize events:"
  echo "  kubectl get events -n $TEST_NAMESPACE --field-selector reason=InPlaceResize"
  echo ""
  print_color $YELLOW "Check metrics for a pod:"
  echo "  kubectl top pod -n $TEST_NAMESPACE"
  echo ""
  print_color $YELLOW "Port-forward to health endpoints:"
  echo "  kubectl port-forward -n $NAMESPACE deployment/right-sizer 8081:8081"
  echo "  Then visit: http://localhost:8081/readyz"
}

cleanup() {
  print_header "Cleanup Instructions"
  print_color $YELLOW "To clean up all test resources, run:"
  echo ""
  echo "kubectl delete namespace $NAMESPACE $TEST_NAMESPACE"
  echo ""
  print_color $YELLOW "Or run this script with 'cleanup' argument:"
  echo "./tests/test-interactive-resize.sh cleanup"
}

do_cleanup() {
  print_header "Cleaning Up Test Environment"

  kubectl delete namespace $TEST_NAMESPACE --ignore-not-found=true
  kubectl delete namespace $NAMESPACE --ignore-not-found=true
  kubectl delete crd rightsizerconfigs.rightsizer.io --ignore-not-found=true
  kubectl delete crd rightsizerpolicies.rightsizer.io --ignore-not-found=true

  print_color $GREEN "✅ Cleanup complete"
  exit 0
}

# Check if cleanup was requested
if [ "$1" == "cleanup" ]; then
  do_cleanup
fi

print_header "Right-Sizer Interactive Test Environment"

# Check Kubernetes version
K8S_VERSION=$(kubectl version -o json | jq -r '.serverVersion.gitVersion')
print_color $YELLOW "📋 Kubernetes version: $K8S_VERSION"

if [[ ! "$K8S_VERSION" =~ v1\.(3[3-9]|[4-9][0-9]) ]]; then
  print_color $RED "❌ Kubernetes 1.33+ is required for in-place resizing"
  exit 1
fi

# Build and load image
print_header "Preparing Docker Image"
print_color $YELLOW "Building image..."
docker build -t right-sizer:interactive . >/dev/null 2>&1
print_color $GREEN "✅ Built"

if command -v minikube &>/dev/null; then
  print_color $YELLOW "Loading into minikube..."
  minikube image load right-sizer:interactive >/dev/null 2>&1
  print_color $GREEN "✅ Loaded"
fi

# Install CRDs
print_header "Installing CRDs"
kubectl apply -f helm/crds/
print_color $GREEN "✅ CRDs installed"
sleep 2

# Create namespaces
print_header "Creating Namespaces"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace $TEST_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
print_color $GREEN "✅ Namespaces created"

# Deploy operator
print_header "Deploying Right-Sizer Operator"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["apps"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["rightsizer.io"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer
subjects:
- kind: ServiceAccount
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer
  template:
    metadata:
      labels:
        app: right-sizer
    spec:
      serviceAccountName: right-sizer
      containers:
      - name: right-sizer
        image: right-sizer:interactive
        imagePullPolicy: Never
        env:
        - name: LOG_LEVEL
          value: "debug"
        ports:
        - containerPort: 8081
          name: health
        - containerPort: 8080
          name: metrics
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "1000m"
EOF

print_color $YELLOW "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=60s deployment/right-sizer -n $NAMESPACE
print_color $GREEN "✅ Operator is running"

# Create configuration
print_header "Creating Right-Sizer Configuration"
cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: interactive-config
  namespace: $NAMESPACE
spec:
  enabled: true
  dryRun: false
  namespaceConfig:
    includeNamespaces:
      - $TEST_NAMESPACE
  observabilityConfig:
    logLevel: debug
    enableMetricsExport: true
    metricsPort: 8080
EOF
print_color $GREEN "✅ Configuration applied"

# Deploy test workloads
print_header "Deploying Test Workloads"

# 1. Memory-hungry pod (will approach OOMKilled)
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-stress
  namespace: $TEST_NAMESPACE
  labels:
    app: memory-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args: ["--vm", "1", "--vm-bytes", "50M", "--vm-hang", "1"]
    resources:
      requests:
        memory: "30Mi"
        cpu: "50m"
      limits:
        memory: "60Mi"  # Will be too low!
        cpu: "100m"
EOF

# 2. CPU-intensive pod
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: cpu-stress
  namespace: $TEST_NAMESPACE
  labels:
    app: cpu-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args: ["--cpu", "1"]
    resources:
      requests:
        memory: "20Mi"
        cpu: "50m"
      limits:
        memory: "50Mi"
        cpu: "100m"  # Will cause throttling!
EOF

# 3. Normal pod (for comparison)
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: normal-app
  namespace: $TEST_NAMESPACE
  labels:
    app: normal
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        memory: "50Mi"
        cpu: "10m"
      limits:
        memory: "100Mi"
        cpu: "50m"
EOF

print_color $GREEN "✅ Test workloads deployed"

# Wait for pods to start
print_color $YELLOW "Waiting for pods to start..."
sleep 10

# Show initial state
print_header "Initial Pod Resources"
kubectl get pods -n $TEST_NAMESPACE -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,CPU_REQ:.spec.containers[0].resources.requests.cpu,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,MEM_LIM:.spec.containers[0].resources.limits.memory

print_header "What's Happening Now"
print_color $CYAN "The Right-Sizer operator is now monitoring the test pods."
print_color $CYAN "Within 1-2 minutes, you should see:"
echo ""
print_color $YELLOW "1. memory-stress pod: Memory limit increased to prevent OOMKilled"
print_color $YELLOW "2. cpu-stress pod: CPU limit increased to prevent throttling"
print_color $YELLOW "3. normal-app pod: Resources adjusted based on actual usage"
echo ""

show_commands

print_header "Watch the Magic Happen!"
print_color $GREEN "The test environment is ready. Use the commands above to observe the operator in action."
print_color $GREEN "The operator will automatically resize pods that are under resource pressure."
echo ""
print_color $CYAN "TIP: Open multiple terminals to watch different aspects simultaneously!"
echo ""

cleanup

print_header "Next Steps"
print_color $YELLOW "1. Watch the operator logs for resize decisions"
print_color $YELLOW "2. Monitor pod resources to see them change"
print_color $YELLOW "3. Check events for resize notifications"
print_color $YELLOW "4. Use 'kubectl top' to see actual usage"
echo ""
print_color $GREEN "Happy testing! 🚀"
