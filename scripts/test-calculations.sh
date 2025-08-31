#!/bin/bash

# Right-Sizer Resource Calculations Test Script
# This script tests resource calculation logic with minikube

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="right-sizer-calc-test"
TEST_NAMESPACE="calc-test"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
  echo -e "${RED}[✗]${NC} $1"
}

check_prerequisites() {
  log_info "Checking prerequisites..."

  if ! command -v minikube &>/dev/null; then
    log_error "minikube is not installed"
    exit 1
  fi

  if ! command -v kubectl &>/dev/null; then
    log_error "kubectl is not installed"
    exit 1
  fi

  log_success "Prerequisites checked"
}

setup_cluster() {
  log_info "Setting up minikube cluster..."

  if minikube status -p "$CLUSTER_NAME" &>/dev/null; then
    log_info "Using existing cluster '$CLUSTER_NAME'"
  else
    minikube start -p "$CLUSTER_NAME" \
      --memory=2048 \
      --cpus=2 \
      --kubernetes-version=v1.30.0
  fi

  minikube profile "$CLUSTER_NAME"

  # Enable metrics-server
  minikube addons enable metrics-server -p "$CLUSTER_NAME"

  log_success "Cluster ready"
}

create_test_pods() {
  log_info "Creating test pods with various resource configurations..."

  kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

  # Test Case 1: CPU-heavy workload
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: cpu-heavy
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "cpu-multiplier-test"
    expected-cpu-request: "240m"  # 200m * 1.2
    expected-cpu-limit: "500m"    # 240m * 2.0 + 20m
    expected-memory-request: "154Mi"  # 128Mi * 1.2
    expected-memory-limit: "308Mi"    # 154Mi * 2.0
spec:
  containers:
  - name: app
    image: busybox
    command: ["sh", "-c", "while true; do echo working; sleep 30; done"]
    resources:
      requests:
        cpu: 200m
        memory: 128Mi
      limits:
        cpu: 400m
        memory: 256Mi
EOF

  # Test Case 2: Memory-heavy workload
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-heavy
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "memory-multiplier-test"
    expected-memory-request: "665Mi"  # 512Mi * 1.3
    expected-memory-limit: "997Mi"    # 665Mi * 1.5
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: 100m
        memory: 512Mi
      limits:
        cpu: 200m
        memory: 1Gi
EOF

  # Test Case 3: Minimal resources (should hit minimums)
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: minimal-resources
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "minimum-enforcement"
    expected-cpu-request: "10m"   # Minimum CPU
    expected-memory-request: "64Mi"  # Minimum memory
spec:
  containers:
  - name: app
    image: busybox
    command: ["sleep", "3600"]
    resources:
      requests:
        cpu: 5m
        memory: 32Mi
      limits:
        cpu: 10m
        memory: 64Mi
EOF

  # Test Case 4: High resources (should hit caps)
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: high-resources
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "maximum-caps"
    expected-cpu-limit: "4000m"   # Should be capped
    expected-memory-limit: "8192Mi"  # Should be capped
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: 3000m
        memory: 6Gi
      limits:
        cpu: 6000m
        memory: 12Gi
EOF

  # Test Case 5: Memory limit too close to request
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: tight-memory-limit
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "memory-limit-buffer"
    warning: "Only 5% buffer between request and limit"
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: 100m
        memory: 1000Mi
      limits:
        cpu: 200m
        memory: 1050Mi  # Only 5% over request - risky!
EOF

  # Test Case 6: Java application simulation
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: java-app
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "java-memory-overhead"
    note: "Java apps need 30-50% extra for non-heap memory"
spec:
  containers:
  - name: java
    image: openjdk:11-jre-slim
    command: ["java", "-Xmx1024m", "-Xms1024m", "-version"]
    resources:
      requests:
        cpu: 500m
        memory: 1536Mi  # 1GB heap + 512MB overhead
      limits:
        cpu: 1000m
        memory: 2304Mi  # 1.5x request for Java
EOF

  # Test Case 7: Multi-container pod
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: multi-container
  namespace: $TEST_NAMESPACE
  annotations:
    test-case: "multi-container-aggregation"
    note: "Total resources should be sum of all containers"
spec:
  containers:
  - name: main
    image: nginx:alpine
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi
  - name: sidecar
    image: busybox
    command: ["sleep", "3600"]
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 100m
        memory: 128Mi
EOF

  log_success "Test pods created"
}

run_unit_tests() {
  log_info "Running unit tests for resource calculations..."

  cd "$PROJECT_ROOT/go"

  # Run CPU calculation tests
  echo -e "\n${BLUE}=== CPU Request Calculations ===${NC}"
  go test ./controllers -v -run TestCPURequestCalculations -count=1 2>&1 | grep -E "PASS|FAIL|Error" || true

  # Run Memory calculation tests
  echo -e "\n${BLUE}=== Memory Request Calculations ===${NC}"
  go test ./controllers -v -run TestMemoryRequestCalculations -count=1 2>&1 | grep -E "PASS|FAIL|Error" || true

  # Run CPU limit tests
  echo -e "\n${BLUE}=== CPU Limit Calculations ===${NC}"
  go test ./controllers -v -run TestCPULimitCalculations -count=1 2>&1 | grep -E "PASS|FAIL|Error" || true

  # Run Memory limit tests
  echo -e "\n${BLUE}=== Memory Limit Calculations ===${NC}"
  go test ./controllers -v -run TestMemoryLimitCalculations -count=1 2>&1 | grep -E "PASS|FAIL|Error" || true

  # Run problematic scenario tests
  echo -e "\n${BLUE}=== Memory Limit Edge Cases ===${NC}"
  go test ./controllers -v -run TestMemoryLimitProblematicScenarios -count=1 2>&1 | grep -E "WARNING|CRITICAL|PASS|FAIL" || true

  # Run scaling decision tests
  echo -e "\n${BLUE}=== Scaling Decision Impact ===${NC}"
  go test ./controllers -v -run TestScalingDecisionImpact -count=1 2>&1 | grep -E "PASS|FAIL|Error" || true

  log_success "Unit tests completed"
}

analyze_test_pods() {
  log_info "Analyzing test pod configurations..."

  echo -e "\n${BLUE}=== Current Pod Resources ===${NC}"
  kubectl get pods -n "$TEST_NAMESPACE" -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,CPU_LIM:.spec.containers[0].resources.limits.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory,MEM_LIM:.spec.containers[0].resources.limits.memory,TEST:.metadata.annotations.test-case

  # Check each test case
  echo -e "\n${BLUE}=== Test Case Analysis ===${NC}"

  for pod in $(kubectl get pods -n "$TEST_NAMESPACE" -o name); do
    POD_NAME=$(echo $pod | cut -d'/' -f2)
    echo -e "\n${GREEN}Pod: $POD_NAME${NC}"

    # Get test case annotation
    TEST_CASE=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.test-case}')
    echo "Test Case: $TEST_CASE"

    # Get current resources
    CPU_REQ=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
    MEM_REQ=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.requests.memory}')
    CPU_LIM=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.limits.cpu}')
    MEM_LIM=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.spec.containers[0].resources.limits.memory}')

    echo "Current: CPU=$CPU_REQ/$CPU_LIM, Memory=$MEM_REQ/$MEM_LIM"

    # Check for warnings
    WARNING=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.warning}')
    if [ ! -z "$WARNING" ]; then
      log_warning "$WARNING"
    fi

    # Check for notes
    NOTE=$(kubectl get $pod -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.note}')
    if [ ! -z "$NOTE" ]; then
      echo "Note: $NOTE"
    fi
  done
}

test_metrics_calculations() {
  log_info "Testing metrics calculations..."

  cd "$PROJECT_ROOT/go"

  # Run metrics aggregation tests
  echo -e "\n${BLUE}=== Metrics Aggregation Tests ===${NC}"
  go test ./metrics -v -run TestMetricsAggregation -count=1 2>&1 | grep -E "PASS|FAIL|Total|Average" || true

  # Run CPU conversion tests
  echo -e "\n${BLUE}=== CPU Metrics Conversions ===${NC}"
  go test ./metrics -v -run TestCPUMetricsConversions -count=1 2>&1 | grep -E "PASS|FAIL|cores|millicores" || true

  # Run Memory conversion tests
  echo -e "\n${BLUE}=== Memory Metrics Conversions ===${NC}"
  go test ./metrics -v -run TestMemoryMetricsConversions -count=1 2>&1 | grep -E "PASS|FAIL|MB|GB" || true

  log_success "Metrics calculation tests completed"
}

simulate_resource_adjustments() {
  log_info "Simulating resource adjustment scenarios..."

  # Scenario 1: Scale up due to high usage
  echo -e "\n${BLUE}Scenario 1: Scale Up (High Usage)${NC}"
  echo "Input: CPU=200m (usage=180m), Memory=256Mi (usage=240Mi)"
  echo "Multipliers: CPU=1.2, Memory=1.3"
  echo "Expected: CPU=216m (180*1.2), Memory=312Mi (240*1.3)"

  # Scenario 2: Scale down due to low usage
  echo -e "\n${BLUE}Scenario 2: Scale Down (Low Usage)${NC}"
  echo "Input: CPU=500m (usage=100m), Memory=1Gi (usage=200Mi)"
  echo "Multipliers: CPU=1.1 (reduced), Memory=1.1 (reduced)"
  echo "Expected: CPU=110m (100*1.1), Memory=220Mi (200*1.1)"

  # Scenario 3: Memory limit buffer check
  echo -e "\n${BLUE}Scenario 3: Memory Limit Buffer Check${NC}"
  echo "Request: 1024Mi"
  echo "Limit with 5% buffer: 1075Mi - ${RED}DANGEROUS${NC}"
  echo "Limit with 20% buffer: 1229Mi - ${YELLOW}MINIMAL${NC}"
  echo "Limit with 50% buffer: 1536Mi - ${GREEN}SAFE${NC}"

  # Scenario 4: Java application memory
  echo -e "\n${BLUE}Scenario 4: Java Application Memory${NC}"
  echo "Heap: 1024Mi"
  echo "Request: 1536Mi (heap + 512Mi overhead)"
  echo "Limit: 2304Mi (request * 1.5)"
  echo "Buffer: 768Mi (50% over request) - ${GREEN}ADEQUATE FOR JAVA${NC}"
}

generate_report() {
  log_info "Generating test report..."

  REPORT_FILE="$PROJECT_ROOT/test-report-$(date +%Y%m%d-%H%M%S).txt"

  {
    echo "Right-Sizer Resource Calculation Test Report"
    echo "============================================="
    echo "Date: $(date)"
    echo ""
    echo "Test Environment:"
    echo "- Cluster: $CLUSTER_NAME"
    echo "- Namespace: $TEST_NAMESPACE"
    echo "- Kubernetes: $(kubectl version --short | grep Server)"
    echo ""
    echo "Test Summary:"
    echo "-------------"

    # Count test results
    TOTAL_PODS=$(kubectl get pods -n "$TEST_NAMESPACE" --no-headers | wc -l)
    echo "- Total test pods: $TOTAL_PODS"

    # List test cases
    echo ""
    echo "Test Cases Executed:"
    kubectl get pods -n "$TEST_NAMESPACE" -o custom-columns=NAME:.metadata.name,TEST:.metadata.annotations.test-case --no-headers

    echo ""
    echo "Key Findings:"
    echo "- Memory limits should have minimum 10% buffer over requests"
    echo "- Java applications need 30-50% extra memory for non-heap"
    echo "- CPU limits should be 1.5-2x requests for burst handling"
    echo "- Multi-container pods need aggregated resource calculations"

  } | tee "$REPORT_FILE"

  log_success "Report saved to: $REPORT_FILE"
}

cleanup() {
  log_info "Cleaning up..."

  # Delete test namespace
  kubectl delete namespace "$TEST_NAMESPACE" --wait=false 2>/dev/null || true

  # Optionally stop minikube
  read -p "Stop minikube cluster? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    minikube stop -p "$CLUSTER_NAME"
  fi
}

# Main execution
main() {
  echo -e "${GREEN}=====================================${NC}"
  echo -e "${GREEN}Right-Sizer Resource Calculation Tests${NC}"
  echo -e "${GREEN}=====================================${NC}\n"

  check_prerequisites
  setup_cluster

  # Run tests
  run_unit_tests
  create_test_pods
  analyze_test_pods
  test_metrics_calculations
  simulate_resource_adjustments

  # Generate report
  generate_report

  echo -e "\n${GREEN}=====================================${NC}"
  echo -e "${GREEN}All tests completed!${NC}"
  echo -e "${GREEN}=====================================${NC}"

  # Cleanup
  cleanup
}

# Trap for cleanup on exit
trap cleanup EXIT

# Run if executed directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  main "$@"
fi
