#!/bin/bash
# Runtime testing script for metrics registration fix
# Tests the metrics system in a live Kubernetes cluster

set -e

echo "=========================================="
echo "🧪 Right-Sizer Metrics Fix - Runtime Tests"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -n "Test $TESTS_TOTAL: $test_name... "
    
    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Function to run a test with output
run_test_with_output() {
    local test_name="$1"
    local test_command="$2"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo ""
    echo "Test $TESTS_TOTAL: $test_name"
    echo "----------------------------------------"
    
    if eval "$test_command"; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "📋 Pre-flight Checks"
echo "----------------------------------------"

# Check if kubectl is available
run_test "kubectl available" "command -v kubectl"

# Check if cluster is accessible
run_test "Kubernetes cluster accessible" "kubectl cluster-info"

# Check if right-sizer namespace exists
if ! kubectl get namespace right-sizer > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  right-sizer namespace not found, creating...${NC}"
    kubectl create namespace right-sizer
fi

echo ""
echo "🔨 Building and Deploying Operator"
echo "----------------------------------------"

# Build the operator
echo "Building operator..."
cd go
if go build -o ../bin/right-sizer .; then
    echo -e "${GREEN}✓ Build successful${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi
cd ..

# Check if operator is already running
if kubectl get deployment right-sizer -n right-sizer > /dev/null 2>&1; then
    echo "Operator already deployed, restarting..."
    kubectl rollout restart deployment/right-sizer -n right-sizer
    kubectl rollout status deployment/right-sizer -n right-sizer --timeout=60s
else
    echo -e "${YELLOW}⚠️  Operator not deployed. Please deploy using: make deploy${NC}"
    echo "Skipping runtime tests (operator not running)"
    exit 0
fi

echo ""
echo "🧪 Runtime Tests"
echo "----------------------------------------"

# Test 1: Check operator pod is running
run_test_with_output "Operator pod is running" "
    kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer --field-selector=status.phase=Running | grep -q right-sizer
"

# Test 2: Check for panic in logs
run_test_with_output "No panic in operator logs" "
    ! kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=100 | grep -i 'panic'
"

# Test 3: Check for metrics registration errors
run_test_with_output "No metrics registration errors" "
    ! kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=100 | grep -i 'failed to register'
"

# Test 4: Check metrics endpoint is accessible
echo ""
echo "Test: Metrics endpoint accessible"
echo "----------------------------------------"
POD_NAME=$(kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')
if [ -n "$POD_NAME" ]; then
    echo "Port-forwarding to pod: $POD_NAME"
    kubectl port-forward -n right-sizer "$POD_NAME" 8080:8080 > /dev/null 2>&1 &
    PF_PID=$!
    sleep 3
    
    if curl -s http://localhost:8080/metrics > /tmp/metrics.txt; then
        echo -e "${GREEN}✓ Metrics endpoint accessible${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ Metrics endpoint not accessible${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    kill $PF_PID 2>/dev/null || true
else
    echo -e "${RED}✗ No operator pod found${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
fi

# Test 5: Check specific metrics are present
if [ -f /tmp/metrics.txt ]; then
    echo ""
    echo "Test: Verify specific metrics are exposed"
    echo "----------------------------------------"
    
    EXPECTED_METRICS=(
        "rightsizer_pods_processed_total"
        "rightsizer_pods_resized_total"
        "rightsizer_cpu_adjustments_total"
        "rightsizer_memory_adjustments_total"
        "rightsizer_cpu_usage_percent"
        "rightsizer_memory_usage_percent"
        "rightsizer_active_pods_total"
    )
    
    for metric in "${EXPECTED_METRICS[@]}"; do
        if grep -q "$metric" /tmp/metrics.txt; then
            echo -e "  ${GREEN}✓${NC} $metric"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "  ${RED}✗${NC} $metric (missing)"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
        TESTS_TOTAL=$((TESTS_TOTAL + 1))
    done
    
    # Show sample metrics
    echo ""
    echo "Sample metrics output:"
    echo "----------------------------------------"
    head -20 /tmp/metrics.txt
    echo "..."
fi

# Test 6: Check operator is processing pods
echo ""
echo "Test: Operator is processing pods"
echo "----------------------------------------"
sleep 5  # Wait for operator to process
if kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=50 | grep -q "pods processed\|Analyzing pod\|Processing pod"; then
    echo -e "${GREEN}✓ Operator is processing pods${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠️  No pod processing activity detected (may be normal if no pods to process)${NC}"
fi
TESTS_TOTAL=$((TESTS_TOTAL + 1))

# Test 7: Multi-replica test (if enabled)
echo ""
echo "Test: Multi-replica deployment"
echo "----------------------------------------"
REPLICA_COUNT=$(kubectl get deployment right-sizer -n right-sizer -o jsonpath='{.spec.replicas}')
echo "Current replicas: $REPLICA_COUNT"

if [ "$REPLICA_COUNT" -gt 1 ]; then
    echo "Testing multi-replica scenario..."
    RUNNING_PODS=$(kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer --field-selector=status.phase=Running --no-headers | wc -l)
    
    if [ "$RUNNING_PODS" -eq "$REPLICA_COUNT" ]; then
        echo -e "${GREEN}✓ All $REPLICA_COUNT replicas running${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        
        # Check each replica for panics
        echo "Checking each replica for errors..."
        kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer -o name | while read pod; do
            POD_NAME=$(basename "$pod")
            if kubectl logs -n right-sizer "$POD_NAME" --tail=50 | grep -qi "panic"; then
                echo -e "  ${RED}✗${NC} $POD_NAME has panic"
            else
                echo -e "  ${GREEN}✓${NC} $POD_NAME healthy"
            fi
        done
    else
        echo -e "${RED}✗ Only $RUNNING_PODS/$REPLICA_COUNT replicas running${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
else
    echo "Single replica deployment (skipping multi-replica test)"
fi

# Test 8: Health check endpoints
echo ""
echo "Test: Health check endpoints"
echo "----------------------------------------"
POD_NAME=$(kubectl get pods -n right-sizer -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}')
if [ -n "$POD_NAME" ]; then
    kubectl port-forward -n right-sizer "$POD_NAME" 8081:8081 > /dev/null 2>&1 &
    PF_PID=$!
    sleep 3
    
    # Test /healthz
    if curl -s http://localhost:8081/healthz | grep -q "ok\|healthy"; then
        echo -e "${GREEN}✓ /healthz endpoint working${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ /healthz endpoint failed${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    # Test /readyz
    if curl -s http://localhost:8081/readyz | grep -q "ok\|ready"; then
        echo -e "${GREEN}✓ /readyz endpoint working${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ /readyz endpoint failed${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    kill $PF_PID 2>/dev/null || true
fi

# Cleanup
rm -f /tmp/metrics.txt

echo ""
echo "=========================================="
echo "📊 Test Results Summary"
echo "=========================================="
echo ""
echo "Total Tests: $TESTS_TOTAL"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "🎉 Metrics registration fix verified successfully!"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    echo "Please review the failures above and check operator logs:"
    echo "  kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer"
    exit 1
fi
