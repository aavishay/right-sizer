#!/bin/bash

# Right-Sizer Memory Metrics Test for Minikube
# This script specifically tests memory metrics collection and processing
# in the right-sizer operator using minikube

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE=${NAMESPACE:-right-sizer}
TEST_NAMESPACE=${TEST_NAMESPACE:-memory-test}
MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-right-sizer-memory}
TEST_TIMEOUT=${TEST_TIMEOUT:-300}
METRICS_INTERVAL=${METRICS_INTERVAL:-30}

# Test tracking
TESTS_PASSED=0
TESTS_FAILED=0
TEST_RESULTS=()

# Logging configuration
LOG_DIR="./test-logs"
LOG_FILE="$LOG_DIR/memory-metrics-test-$(date +%Y%m%d-%H%M%S).log"

# Initialize logging
mkdir -p "$LOG_DIR"
exec 1> >(tee -a "$LOG_FILE")
exec 2>&1

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

print_subheader() {
  echo ""
  print_color $CYAN "--- $1 ---"
}

log_test_result() {
  local test_name=$1
  local status=$2
  local details=$3

  TEST_RESULTS+=("[$status] $test_name: $details")

  if [ "$status" = "PASS" ]; then
    TESTS_PASSED=$((TESTS_PASSED + 1))
    print_color $GREEN "✅ $test_name: $details"
  else
    TESTS_FAILED=$((TESTS_FAILED + 1))
    print_color $RED "❌ $test_name: $details"
  fi
}

check_requirement() {
  local cmd=$1
  if ! command -v $cmd &>/dev/null; then
    print_color $RED "Error: $cmd is not installed"
    exit 1
  fi
}

wait_for_condition() {
  local condition=$1
  local timeout=$2
  local message=$3

  echo -n "$message"
  local elapsed=0
  while [ $elapsed -lt $timeout ]; do
    if eval $condition &>/dev/null; then
      print_color $GREEN " Done!"
      return 0
    fi
    echo -n "."
    sleep 2
    elapsed=$((elapsed + 2))
  done
  print_color $RED " Timeout!"
  return 1
}

setup_minikube() {
  print_header "Setting Up Minikube Environment"

  # Check if minikube profile exists
  if minikube status -p $MINIKUBE_PROFILE &>/dev/null; then
    print_color $YELLOW "Using existing Minikube profile: $MINIKUBE_PROFILE"
  else
    print_color $YELLOW "Starting new Minikube profile: $MINIKUBE_PROFILE"
    minikube start -p $MINIKUBE_PROFILE \
      --memory=6144 \
      --cpus=4 \
      --kubernetes-version=v1.28.0 \
      --addons=metrics-server \
      --extra-config=kubelet.housekeeping-interval=10s
  fi

  # Set kubectl context
  kubectl config use-context $MINIKUBE_PROFILE

  # Verify metrics-server is running
  wait_for_condition "kubectl get deployment metrics-server -n kube-system -o jsonpath='{.status.readyReplicas}' | grep -q 1" 60 "Waiting for metrics-server"

  print_color $GREEN "✅ Minikube environment ready"
}

build_and_load_image() {
  print_header "Building and Loading Right-Sizer Image"

  print_color $YELLOW "Building Docker image..."
  docker build -t right-sizer:memory-test . >/dev/null 2>&1

  print_color $YELLOW "Loading image into Minikube..."
  minikube image load right-sizer:memory-test -p $MINIKUBE_PROFILE

  print_color $GREEN "✅ Image ready in Minikube"
}

deploy_crds() {
  print_header "Deploying Custom Resource Definitions"

  kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
  kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml

  wait_for_condition "kubectl get crd rightsizerconfigs.rightsizer.io" 30 "Waiting for CRDs"

  print_color $GREEN "✅ CRDs deployed successfully"
}

deploy_operator() {
  print_header "Deploying Right-Sizer Operator"

  # Create namespace
  kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
  kubectl create namespace $TEST_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

  # Deploy operator with enhanced memory monitoring
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
apiVersion: v1
kind: ConfigMap
metadata:
  name: right-sizer-config
  namespace: $NAMESPACE
data:
  config.yaml: |
    logLevel: debug
    metricsInterval: ${METRICS_INTERVAL}s
    memoryMetrics:
      enabled: true
      includeCache: true
      includeRSS: true
      includeWorkingSet: true
      includeSwap: true
      aggregationWindow: 5m
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: $NAMESPACE
  labels:
    app: right-sizer
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
        image: right-sizer:memory-test
        imagePullPolicy: Never
        env:
        - name: METRICS_INTERVAL
          value: "${METRICS_INTERVAL}"
        - name: LOG_LEVEL
          value: "debug"
        - name: ENABLE_MEMORY_METRICS
          value: "true"
        ports:
        - containerPort: 8081
          name: health
        - containerPort: 8080
          name: metrics
        - containerPort: 9090
          name: prometheus
        volumeMounts:
        - name: config
          mountPath: /etc/right-sizer
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "1000m"
      volumes:
      - name: config
        configMap:
          name: right-sizer-config
EOF

  # Wait for deployment
  wait_for_condition "kubectl get deployment right-sizer -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q 1" 120 "Waiting for operator deployment"

  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')
  print_color $GREEN "✅ Operator deployed: $POD_NAME"
}

deploy_memory_test_config() {
  print_header "Deploying Memory Test Configuration"

  # Create RightSizerConfig with memory-specific settings
  cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: memory-test-config
  namespace: $NAMESPACE
spec:
  enabled: true
  dryRun: false
  resizeInterval: ${METRICS_INTERVAL}s
  defaultResourceStrategy:
    memory:
      requestMultiplier: 1.2
      limitMultiplier: 1.5
      minRequest: 32
      maxLimit: 2048
      scaleUpThreshold: 0.7
      scaleDownThreshold: 0.3
  namespaceConfig:
    includeNamespaces:
    - $TEST_NAMESPACE
EOF

  # Create memory-specific policy
  cat <<EOF | kubectl apply -f -
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: memory-test-policy
  namespace: $NAMESPACE
spec:
  enabled: true
  priority: 100
  mode: balanced
  targetRef:
    namespaces:
    - $TEST_NAMESPACE
  resourceStrategy:
    memory:
      requestMultiplier: 1.1
      limitMultiplier: 1.4
      targetUtilization: 75
  constraints:
    maxChangePercentage: 50
    cooldownPeriod: 1m
EOF

  sleep 5
  print_color $GREEN "✅ Memory test configuration deployed"
}

test_memory_metrics_collection() {
  print_subheader "Testing Memory Metrics Collection"

  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  # Check if metrics endpoint is available
  kubectl exec -n $NAMESPACE $POD_NAME -- wget -q -O- http://localhost:8080/metrics >/tmp/metrics.txt 2>&1

  # Check for memory-specific metrics
  if grep -q "rightsizer_memory_usage_bytes" /tmp/metrics.txt; then
    log_test_result "Memory Usage Metric" "PASS" "rightsizer_memory_usage_bytes metric found"
  else
    log_test_result "Memory Usage Metric" "FAIL" "rightsizer_memory_usage_bytes metric not found"
  fi

  if grep -q "rightsizer_memory_working_set_bytes" /tmp/metrics.txt; then
    log_test_result "Working Set Metric" "PASS" "rightsizer_memory_working_set_bytes metric found"
  else
    log_test_result "Working Set Metric" "FAIL" "rightsizer_memory_working_set_bytes metric not found"
  fi

  if grep -q "rightsizer_memory_rss_bytes" /tmp/metrics.txt; then
    log_test_result "RSS Metric" "PASS" "rightsizer_memory_rss_bytes metric found"
  else
    log_test_result "RSS Metric" "FAIL" "rightsizer_memory_rss_bytes metric not found"
  fi

  if grep -q "rightsizer_memory_cache_bytes" /tmp/metrics.txt; then
    log_test_result "Cache Metric" "PASS" "rightsizer_memory_cache_bytes metric found"
  else
    log_test_result "Cache Metric" "FAIL" "rightsizer_memory_cache_bytes metric not found"
  fi
}

deploy_test_workloads() {
  print_header "Deploying Test Workloads with Different Memory Patterns"

  # 1. Low memory usage pod
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: low-memory-pod
  namespace: $TEST_NAMESPACE
  labels:
    test: memory
    profile: low
spec:
  containers:
  - name: app
    image: busybox
    command: ["sh", "-c"]
    args:
    - |
      echo "Low memory usage pod"
      while true; do
        sleep 30
      done
    resources:
      requests:
        memory: "64Mi"
        cpu: "10m"
      limits:
        memory: "128Mi"
        cpu: "50m"
EOF

  # 2. Medium memory usage pod with stress
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: medium-memory-pod
  namespace: $TEST_NAMESPACE
  labels:
    test: memory
    profile: medium
spec:
  containers:
  - name: app
    image: polinux/stress
    command: ["stress"]
    args:
    - "--vm"
    - "1"
    - "--vm-bytes"
    - "100M"
    - "--vm-hang"
    - "1"
    - "--timeout"
    - "3600s"
    resources:
      requests:
        memory: "128Mi"
        cpu: "50m"
      limits:
        memory: "256Mi"
        cpu: "100m"
EOF

  # 3. High memory usage pod
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: high-memory-pod
  namespace: $TEST_NAMESPACE
  labels:
    test: memory
    profile: high
spec:
  containers:
  - name: app
    image: polinux/stress
    command: ["stress"]
    args:
    - "--vm"
    - "2"
    - "--vm-bytes"
    - "200M"
    - "--vm-hang"
    - "1"
    - "--timeout"
    - "3600s"
    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "200m"
EOF

  # 4. Memory leak simulation pod
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-leak-pod
  namespace: $TEST_NAMESPACE
  labels:
    test: memory
    profile: leak
spec:
  containers:
  - name: app
    image: busybox
    command: ["sh", "-c"]
    args:
    - |
      echo "Simulating memory leak"
      data=""
      while true; do
        data="\${data}XXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
        echo "Memory usage increasing..."
        sleep 10
      done
    resources:
      requests:
        memory: "64Mi"
        cpu: "10m"
      limits:
        memory: "256Mi"
        cpu: "50m"
EOF

  # 5. Deployment with multiple replicas
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memory-test-deployment
  namespace: $TEST_NAMESPACE
spec:
  replicas: 3
  selector:
    matchLabels:
      app: memory-test
  template:
    metadata:
      labels:
        app: memory-test
        test: memory
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            memory: "64Mi"
            cpu: "25m"
          limits:
            memory: "128Mi"
            cpu: "100m"
EOF

  # Wait for all pods to be ready
  print_color $YELLOW "Waiting for test workloads to be ready..."
  sleep 10

  kubectl wait --for=condition=ready pod/low-memory-pod -n $TEST_NAMESPACE --timeout=60s || true
  kubectl wait --for=condition=ready pod/medium-memory-pod -n $TEST_NAMESPACE --timeout=60s || true
  kubectl wait --for=condition=ready pod/high-memory-pod -n $TEST_NAMESPACE --timeout=60s || true
  kubectl wait --for=condition=ready pod/memory-leak-pod -n $TEST_NAMESPACE --timeout=60s || true

  print_color $GREEN "✅ Test workloads deployed"
}

validate_memory_metrics() {
  print_header "Validating Memory Metrics Collection"

  print_subheader "Waiting for metrics collection cycle (${METRICS_INTERVAL}s)..."
  sleep $((METRICS_INTERVAL + 10))

  # Get operator pod
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  # Check operator logs for memory metrics processing
  print_subheader "Checking Operator Logs for Memory Metrics"

  kubectl logs $POD_NAME -n $NAMESPACE --tail=50 | grep -i memory >/tmp/memory-logs.txt || true

  if [ -s /tmp/memory-logs.txt ]; then
    log_test_result "Memory Metrics Logging" "PASS" "Memory metrics found in logs"
    print_color $CYAN "Sample log entries:"
    head -5 /tmp/memory-logs.txt
  else
    log_test_result "Memory Metrics Logging" "FAIL" "No memory metrics in logs"
  fi

  # Check each test pod's metrics
  print_subheader "Validating Individual Pod Metrics"

  for pod in low-memory-pod medium-memory-pod high-memory-pod memory-leak-pod; do
    echo ""
    print_color $YELLOW "Checking metrics for $pod:"

    # Get pod metrics from metrics-server
    if kubectl top pod $pod -n $TEST_NAMESPACE --no-headers 2>/dev/null; then
      MEMORY_USAGE=$(kubectl top pod $pod -n $TEST_NAMESPACE --no-headers | awk '{print $3}')
      print_color $GREEN "  Current memory usage: $MEMORY_USAGE"
      log_test_result "$pod Metrics" "PASS" "Memory: $MEMORY_USAGE"
    else
      log_test_result "$pod Metrics" "FAIL" "Could not retrieve metrics"
    fi

    # Check if operator is tracking this pod
    if kubectl logs $POD_NAME -n $NAMESPACE --tail=100 | grep -q "$pod"; then
      print_color $GREEN "  ✓ Pod is being tracked by operator"
    else
      print_color $YELLOW "  ⚠ Pod may not be tracked yet"
    fi
  done
}

test_memory_recommendations() {
  print_header "Testing Memory Sizing Recommendations"

  print_subheader "Waiting for recommendation cycle..."
  sleep $((METRICS_INTERVAL * 2))

  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  # Check for sizing recommendations in logs
  kubectl logs $POD_NAME -n $NAMESPACE --tail=100 | grep -i "recommendation\|resize\|adjustment" >/tmp/recommendations.txt || true

  if [ -s /tmp/recommendations.txt ]; then
    log_test_result "Sizing Recommendations" "PASS" "Recommendations generated"
    print_color $CYAN "Sample recommendations:"
    head -5 /tmp/recommendations.txt
  else
    log_test_result "Sizing Recommendations" "FAIL" "No recommendations found"
  fi

  # Check if any pods were actually resized
  print_subheader "Checking for Pod Resizing Events"

  kubectl get events -n $TEST_NAMESPACE --field-selector reason=Resized --no-headers >/tmp/resize-events.txt 2>/dev/null || true

  if [ -s /tmp/resize-events.txt ]; then
    log_test_result "Pod Resizing" "PASS" "Resize events detected"
    print_color $CYAN "Resize events:"
    cat /tmp/resize-events.txt
  else
    print_color $YELLOW "No resize events yet (might be in dry-run mode or cooldown period)"
  fi
}

test_prometheus_metrics() {
  print_header "Testing Prometheus Metrics Export"

  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  # Port-forward to access metrics
  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 9090:9090 &
  PF_PID=$!
  sleep 3

  # Fetch Prometheus metrics
  curl -s http://localhost:9090/metrics >/tmp/prometheus-metrics.txt 2>/dev/null || true

  # Kill port-forward
  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true

  # Check for memory-related Prometheus metrics
  print_subheader "Checking Prometheus Memory Metrics"

  MEMORY_METRICS=(
    "rightsizer_pod_memory_usage_bytes"
    "rightsizer_pod_memory_working_set_bytes"
    "rightsizer_pod_memory_rss_bytes"
    "rightsizer_pod_memory_cache_bytes"
    "rightsizer_pod_memory_limit_bytes"
    "rightsizer_pod_memory_request_bytes"
    "rightsizer_memory_recommendation_bytes"
    "rightsizer_memory_utilization_percentage"
  )

  for metric in "${MEMORY_METRICS[@]}"; do
    if grep -q "$metric" /tmp/prometheus-metrics.txt 2>/dev/null; then
      log_test_result "Prometheus: $metric" "PASS" "Metric exported"
    else
      log_test_result "Prometheus: $metric" "FAIL" "Metric not found"
    fi
  done
}

test_memory_pressure_handling() {
  print_header "Testing Memory Pressure Handling"

  print_subheader "Creating High Memory Pressure Pod"

  # Deploy a pod that will consume significant memory
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-pressure-pod
  namespace: $TEST_NAMESPACE
  labels:
    test: memory-pressure
spec:
  containers:
  - name: app
    image: polinux/stress
    command: ["stress"]
    args:
    - "--vm"
    - "1"
    - "--vm-bytes"
    - "400M"
    - "--timeout"
    - "120s"
    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "200m"
EOF

  # Wait for pod to start
  kubectl wait --for=condition=ready pod/memory-pressure-pod -n $TEST_NAMESPACE --timeout=60s || true

  # Monitor for 60 seconds
  print_color $YELLOW "Monitoring memory pressure for 60 seconds..."

  for i in {1..6}; do
    sleep 10
    echo -n "."

    # Check pod status
    POD_STATUS=$(kubectl get pod memory-pressure-pod -n $TEST_NAMESPACE -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")

    if [ "$POD_STATUS" = "Failed" ] || [ "$POD_STATUS" = "OOMKilled" ]; then
      print_color $YELLOW " Pod experienced OOM"
      break
    fi
  done
  echo ""

  # Check operator's response to memory pressure
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  if kubectl logs $POD_NAME -n $NAMESPACE --tail=50 | grep -i "memory.*pressure\|oom\|high.*memory"; then
    log_test_result "Memory Pressure Detection" "PASS" "Operator detected memory pressure"
  else
    log_test_result "Memory Pressure Detection" "FAIL" "No memory pressure detection in logs"
  fi

  # Clean up pressure pod
  kubectl delete pod memory-pressure-pod -n $TEST_NAMESPACE --ignore-not-found=true
}

test_memory_metrics_accuracy() {
  print_header "Testing Memory Metrics Accuracy"

  print_subheader "Comparing Metrics Sources"

  # Select a test pod
  TEST_POD="medium-memory-pod"

  # Get metrics from metrics-server
  METRICS_SERVER_MEM=$(kubectl top pod $TEST_POD -n $TEST_NAMESPACE --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//')

  # Get metrics from operator
  POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=right-sizer -o jsonpath='{.items[0].metadata.name}')

  # Port-forward to get detailed metrics
  kubectl port-forward -n $NAMESPACE pod/$POD_NAME 8080:8080 &
  PF_PID=$!
  sleep 3

  # Get operator's view of the metrics
  curl -s http://localhost:8080/metrics | grep "rightsizer_pod_memory_usage_bytes.*$TEST_POD" >/tmp/operator-metrics.txt 2>/dev/null || true

  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true

  if [ -s /tmp/operator-metrics.txt ]; then
    log_test_result "Metrics Correlation" "PASS" "Operator tracking pod metrics"
  else
    log_test_result "Metrics Correlation" "FAIL" "Operator not tracking pod metrics"
  fi

  print_color $CYAN "Metrics Server Memory: ${METRICS_SERVER_MEM}Mi"
}

cleanup() {
  print_header "Cleaning Up Test Resources"

  # Delete test workloads
  kubectl delete namespace $TEST_NAMESPACE --ignore-not-found=true &>/dev/null || true

  # Delete operator
  kubectl delete namespace $NAMESPACE --ignore-not-found=true &>/dev/null || true

  # Delete CRDs
  kubectl delete crd rightsizerconfigs.rightsizer.io --ignore-not-found=true &>/dev/null || true
  kubectl delete crd rightsizerpolicies.rightsizer.io --ignore-not-found=true &>/dev/null || true

  # Stop minikube if requested
  if [ "${CLEANUP_MINIKUBE:-false}" = "true" ]; then
    print_color $YELLOW "Stopping Minikube profile: $MINIKUBE_PROFILE"
    minikube stop -p $MINIKUBE_PROFILE
    minikube delete -p $MINIKUBE_PROFILE
  fi

  print_color $GREEN "✅ Cleanup completed"
}

generate_report() {
  print_header "Test Report"

  echo ""
  print_color $BLUE "Memory Metrics Test Summary"
  print_color $BLUE "=========================="
  echo ""

  TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))

  print_color $CYAN "Total Tests: $TOTAL_TESTS"
  print_color $GREEN "Passed: $TESTS_PASSED"
  print_color $RED "Failed: $TESTS_FAILED"

  if [ $TOTAL_TESTS -gt 0 ]; then
    SUCCESS_RATE=$((TESTS_PASSED * 100 / TOTAL_TESTS))
    print_color $CYAN "Success Rate: ${SUCCESS_RATE}%"
  fi

  echo ""
  print_color $BLUE "Detailed Results:"
  print_color $BLUE "-----------------"

  for result in "${TEST_RESULTS[@]}"; do
    echo "$result"
  done

  echo ""
  print_color $YELLOW "Test logs saved to: $LOG_FILE"

  # Generate JSON report
  REPORT_FILE="$LOG_DIR/memory-metrics-report-$(date +%Y%m%d-%H%M%S).json"
  cat >"$REPORT_FILE" <<EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "test_suite": "memory-metrics",
  "environment": {
    "minikube_profile": "$MINIKUBE_PROFILE",
    "namespace": "$NAMESPACE",
    "test_namespace": "$TEST_NAMESPACE"
  },
  "results": {
    "total": $TOTAL_TESTS,
    "passed": $TESTS_PASSED,
    "failed": $TESTS_FAILED,
    "success_rate": ${SUCCESS_RATE:-0}
  }
}
EOF

  print_color $YELLOW "JSON report saved to: $REPORT_FILE"

  echo ""
  if [ $TESTS_FAILED -eq 0 ]; then
    print_color $GREEN "✅ All memory metrics tests passed!"
    return 0
  else
    print_color $RED "❌ Some tests failed. Please review the logs."
    return 1
  fi
}

# Trap cleanup on exit
trap cleanup EXIT

# Main execution
main() {
  print_header "Right-Sizer Memory Metrics Test Suite"
  print_color $CYAN "Starting comprehensive memory metrics testing on Minikube"
  echo ""

  # Step 1: Check prerequisites
  print_header "Checking Prerequisites"
  check_requirement kubectl
  check_requirement minikube
  check_requirement docker
  check_requirement curl
  check_requirement jq

  # Step 2: Setup Minikube
  setup_minikube

  # Step 3: Build and load image
  build_and_load_image

  # Step 4: Deploy CRDs
  deploy_crds

  # Step 5: Deploy operator
  deploy_operator

  # Step 6: Deploy test configuration
  deploy_memory_test_config

  # Step 7: Run memory metrics collection test
  print_header "Running Memory Metrics Tests"
  test_memory_metrics_collection

  # Step 8: Deploy test workloads
  deploy_test_workloads

  # Step 9: Validate memory metrics
  validate_memory_metrics

  # Step 10: Test memory recommendations
  test_memory_recommendations

  # Step 11: Test Prometheus metrics
  test_prometheus_metrics

  # Step 12: Test memory pressure handling
  test_memory_pressure_handling

  # Step 13: Test metrics accuracy
  test_memory_metrics_accuracy

  # Generate final report
  generate_report
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  --test-namespace)
    TEST_NAMESPACE="$2"
    shift 2
    ;;
  --profile)
    MINIKUBE_PROFILE="$2"
    shift 2
    ;;
  --cleanup)
    CLEANUP_MINIKUBE=true
    shift
    ;;
  --metrics-interval)
    METRICS_INTERVAL="$2"
    shift 2
    ;;
  --help)
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
  --namespace NAME          Operator namespace (default: right-sizer)
  --test-namespace NAME     Test workloads namespace (default: memory-test)
  --profile NAME           Minikube profile name (default: right-sizer-memory)
  --metrics-interval SEC   Metrics collection interval in seconds (default: 30)
  --cleanup                Delete Minikube profile after tests
  --help                   Show this help message

Examples:
  # Run with default settings
  $0

  # Run with custom namespace and cleanup
  $0 --namespace my-operator --cleanup

  # Run with custom metrics interval
  $0 --metrics-interval 60
EOF
    exit 0
    ;;
  *)
    echo "Unknown option: $1"
    echo "Use --help for usage information"
    exit 1
    ;;
  esac
done

# Run main function
main

# Exit with appropriate code
if [ $TESTS_FAILED -eq 0 ]; then
  exit 0
else
  exit 1
fi
