#!/bin/bash

# Quick Memory Metrics Test for Right-Sizer
# This script performs focused memory metrics testing with minimal setup

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
TEST_NAMESPACE=${TEST_NAMESPACE:-memory-quick-test}
MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-minikube}
SKIP_BUILD=${SKIP_BUILD:-false}
SKIP_DEPLOY=${SKIP_DEPLOY:-false}

print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

print_header() {
  echo ""
  print_color $BLUE "========================================="
  print_color $BLUE "$1"
  print_color $BLUE "========================================="
  echo ""
}

check_prerequisites() {
  print_header "Checking Prerequisites"

  local missing=0

  for cmd in kubectl minikube docker curl jq; do
    if command -v $cmd &>/dev/null; then
      print_color $GREEN "✅ $cmd found"
    else
      print_color $RED "❌ $cmd not found"
      missing=$((missing + 1))
    fi
  done

  if [ $missing -gt 0 ]; then
    print_color $RED "Please install missing prerequisites"
    exit 1
  fi

  # Check if minikube is running
  if minikube status -p $MINIKUBE_PROFILE &>/dev/null; then
    print_color $GREEN "✅ Minikube is running"
  else
    print_color $RED "❌ Minikube is not running"
    print_color $YELLOW "Start it with: minikube start"
    exit 1
  fi

  # Check metrics-server
  if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
    print_color $GREEN "✅ Metrics-server is available"
  else
    print_color $YELLOW "⚠️  Metrics-server not found, enabling..."
    minikube addons enable metrics-server -p $MINIKUBE_PROFILE
  fi
}

deploy_operator() {
  if [ "$SKIP_DEPLOY" = "true" ]; then
    print_color $YELLOW "Skipping operator deployment (SKIP_DEPLOY=true)"
    return
  fi

  print_header "Deploying Right-Sizer Operator"

  # Build and load image if needed
  if [ "$SKIP_BUILD" != "true" ]; then
    print_color $YELLOW "Building Docker image..."
    docker build -t right-sizer:quick-test . >/dev/null 2>&1

    print_color $YELLOW "Loading image into Minikube..."
    minikube image load right-sizer:quick-test -p $MINIKUBE_PROFILE
  fi

  # Create namespace
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl create namespace $TEST_NAMESPACE --dry-run=client -o yaml | kubectl apply -f - >/dev/null

  # Apply CRDs
  kubectl apply -f helm/crds/ >/dev/null 2>&1

  # Deploy operator with minimal configuration
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
# Core API resources
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "update", "patch"]
# Pod subresources for in-place resizing
- apiGroups: [""]
  resources: ["pods/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: [""]
  resources: ["pods/resize"]
  verbs: ["update", "patch"]
# Node information for resource availability
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
# Events for audit trail
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch", "update"]
# ConfigMaps and Secrets for configuration
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
# Namespaces for filtering
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
# Apps API resources
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["apps"]
  resources: ["deployments/scale", "statefulsets/scale"]
  verbs: ["get", "update", "patch"]
# Batch API resources
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch"]
# Metrics API for resource usage
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list"]
# HPA for coordination
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch"]
# VPA for coordination (if installed)
- apiGroups: ["autoscaling.k8s.io"]
  resources: ["verticalpodautoscalers"]
  verbs: ["get", "list", "watch"]
# PDB for safe operations
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list", "watch"]
# Custom Resources
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs", "rightsizerpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs/status", "rightsizerpolicies/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerconfigs/finalizers", "rightsizerpolicies/finalizers"]
  verbs: ["update"]
# Admission webhooks (if enabled)
- apiGroups: ["admissionregistration.k8s.io"]
  resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
# Coordination for leader election
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
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
        image: right-sizer:quick-test
        imagePullPolicy: Never
        env:
        - name: LOG_LEVEL
          value: debug
        - name: ENABLE_MEMORY_METRICS
          value: "true"
        - name: METRICS_INTERVAL
          value: "15"
        ports:
        - containerPort: 8081
          name: health
        - containerPort: 8080
          name: metrics
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
EOF

  print_color $YELLOW "Waiting for operator to be ready..."
  kubectl wait --for=condition=available --timeout=60s deployment/right-sizer -n $NAMESPACE >/dev/null 2>&1
  print_color $GREEN "✅ Operator deployed"
}

deploy_test_pod() {
  print_header "Deploying Test Pod"

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-test-pod
  namespace: $TEST_NAMESPACE
  labels:
    app: memory-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args:
    - "--vm"
    - "1"
    - "--vm-bytes"
    - "150M"
    - "--vm-hang"
    - "1"
    - "--timeout"
    - "300s"
    resources:
      requests:
        memory: "128Mi"
        cpu: "50m"
      limits:
        memory: "256Mi"
        cpu: "100m"
EOF

  kubectl wait --for=condition=ready pod/memory-test-pod -n $TEST_NAMESPACE --timeout=30s >/dev/null 2>&1
  print_color $GREEN "✅ Test pod deployed"
}

check_memory_metrics() {
  print_header "Checking Memory Metrics"

  # Get operator pod
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  print_color $CYAN "Operator Pod: $POD_NAME"
  echo ""

  # Wait for metrics collection
  print_color $YELLOW "Waiting 20 seconds for metrics collection..."
  sleep 20

  # Check pod metrics from metrics-server
  print_color $CYAN "📊 Metrics Server Data:"
  kubectl top pod memory-test-pod -n $TEST_NAMESPACE || print_color $YELLOW "Metrics not yet available"
  echo ""

  # Check operator logs for memory processing
  print_color $CYAN "📝 Operator Memory Logs:"
  kubectl logs $POD_NAME -n $NAMESPACE --tail=20 | grep -i "memory\|mem" | head -5 || print_color $YELLOW "No memory logs yet"
  echo ""

  # Port-forward and check metrics endpoint
  print_color $CYAN "📈 Checking Prometheus Metrics:"
  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8080:8080 &
  PF_PID=$!
  sleep 3

  # Check for memory metrics
  METRICS_OUTPUT=$(curl -s http://localhost:8080/metrics 2>/dev/null || echo "")

  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true

  if echo "$METRICS_OUTPUT" | grep -q "memory"; then
    print_color $GREEN "✅ Memory metrics found in endpoint"
    echo "$METRICS_OUTPUT" | grep "memory" | head -5
  else
    print_color $RED "❌ No memory metrics in endpoint"
  fi
  echo ""
}

test_memory_recommendation() {
  print_header "Testing Memory Recommendations"

  # Apply configuration to trigger recommendations
  cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: quick-test-config
  namespace: $NAMESPACE
spec:
  enabled: true
  dryRun: false
  resizeInterval: 15s
  defaultResourceStrategy:
    memory:
      requestMultiplier: 1.2
      limitMultiplier: 1.5
      scaleUpThreshold: 0.7
      scaleDownThreshold: 0.3
  namespaceConfig:
    includeNamespaces:
    - $TEST_NAMESPACE
EOF

  print_color $YELLOW "Waiting 30 seconds for recommendation cycle..."
  sleep 30

  # Check for recommendations in logs
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  print_color $CYAN "📋 Checking for Recommendations:"
  if kubectl logs $POD_NAME -n $NAMESPACE --tail=50 | grep -i "recommendation\|suggest\|resize"; then
    print_color $GREEN "✅ Recommendations generated"
  else
    print_color $YELLOW "⚠️  No recommendations yet"
  fi
  echo ""
}

stress_test_memory() {
  print_header "Memory Stress Test"

  print_color $YELLOW "Deploying memory stress pods..."

  for i in {1..3}; do
    cat <<EOF | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: stress-pod-$i
  namespace: $TEST_NAMESPACE
  labels:
    app: stress-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    args:
    - "--vm"
    - "1"
    - "--vm-bytes"
    - "$((50 * i))M"
    - "--timeout"
    - "60s"
    resources:
      requests:
        memory: "$((64 * i))Mi"
        cpu: "10m"
      limits:
        memory: "$((128 * i))Mi"
        cpu: "50m"
EOF
  done

  print_color $GREEN "✅ Stress pods deployed"

  # Monitor for 30 seconds
  print_color $YELLOW "Monitoring stress test for 30 seconds..."

  for i in {1..6}; do
    sleep 5
    echo -n "."

    # Quick check of pod statuses
    PODS_STATUS=$(kubectl get pods -n $TEST_NAMESPACE -l app=stress-test --no-headers 2>/dev/null | awk '{print $3}' | sort | uniq -c)

    if echo "$PODS_STATUS" | grep -q "OOMKilled\|Error"; then
      print_color $YELLOW " Some pods experiencing memory issues"
      break
    fi
  done
  echo ""

  # Show final status
  print_color $CYAN "Final Pod Status:"
  kubectl get pods -n $TEST_NAMESPACE -l app=stress-test
  echo ""

  # Check operator response
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  print_color $CYAN "Operator Response to Stress:"
  kubectl logs $POD_NAME -n $NAMESPACE --tail=10 | grep -i "stress\|memory\|oom" | head -5 || print_color $YELLOW "No stress-related logs"
  echo ""
}

show_summary() {
  print_header "Quick Test Summary"

  # Get test pod current resources
  print_color $CYAN "📊 Current Resource Usage:"
  kubectl top pods -n $TEST_NAMESPACE 2>/dev/null || print_color $YELLOW "Metrics not available"
  echo ""

  # Show any events
  print_color $CYAN "📌 Recent Events:"
  kubectl get events -n $TEST_NAMESPACE --sort-by='.lastTimestamp' | tail -5
  echo ""

  # Show operator status
  print_color $CYAN "🤖 Operator Status:"
  kubectl get deployment right-sizer -n $NAMESPACE
  echo ""

  print_color $GREEN "✅ Quick memory test completed!"
  print_color $YELLOW "To see detailed logs: kubectl logs -n $NAMESPACE deployment/right-sizer -f"
}

cleanup() {
  print_header "Cleaning Up"

  kubectl delete namespace $TEST_NAMESPACE --ignore-not-found=true >/dev/null 2>&1 || true

  if [ "${KEEP_OPERATOR:-false}" != "true" ]; then
    kubectl delete namespace $NAMESPACE --ignore-not-found=true >/dev/null 2>&1 || true
  fi

  print_color $GREEN "✅ Cleanup completed"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --skip-build)
    SKIP_BUILD=true
    shift
    ;;
  --skip-deploy)
    SKIP_DEPLOY=true
    shift
    ;;
  --keep-operator)
    KEEP_OPERATOR=true
    shift
    ;;
  --cleanup-only)
    cleanup
    exit 0
    ;;
  --help)
    cat <<EOF
Usage: $0 [OPTIONS]

Quick memory metrics test for Right-Sizer operator

Options:
  --skip-build      Skip building Docker image
  --skip-deploy     Skip operator deployment (use existing)
  --keep-operator   Don't delete operator namespace during cleanup
  --cleanup-only    Only run cleanup and exit
  --help           Show this help message

Examples:
  # Full test from scratch
  $0

  # Use existing operator
  $0 --skip-deploy

  # Keep operator after test
  $0 --keep-operator

EOF
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use --help for usage"
    exit 1
    ;;
  esac
done

# Set trap for cleanup
trap cleanup EXIT

# Main execution
print_header "Right-Sizer Quick Memory Test"
print_color $CYAN "Fast memory metrics validation"
echo ""

check_prerequisites
deploy_operator
deploy_test_pod
check_memory_metrics
test_memory_recommendation
stress_test_memory
show_summary

print_color $GREEN "🎉 Test completed successfully!"
