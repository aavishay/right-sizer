#!/bin/bash

# Simple Self-Protection Test for Working Right-Sizer Pod
# This script tests the specific working right-sizer pod for self-protection

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_success() { printf "${GREEN}✅ $1${NC}\n"; }
print_error() { printf "${RED}❌ $1${NC}\n"; }
print_warning() { printf "${YELLOW}⚠️ $1${NC}\n"; }
print_info() { printf "${BLUE}ℹ️ $1${NC}\n"; }

NAMESPACE="right-sizer"
LOG_FILE="/tmp/working-pod-test.log"
TEST_DURATION=120

echo "========================================"
echo "Testing Working Right-Sizer Pod"
echo "========================================"

# Find the working pod
WORKING_POD=$(kubectl get pods -n $NAMESPACE | grep "1/1.*Running" | awk '{print $1}' | head -1)

if [ -z "$WORKING_POD" ]; then
  print_error "No working right-sizer pod found"
  exit 1
fi

print_success "Found working pod: $WORKING_POD"

# Check current resources
print_info "Current pod resources:"
CPU_REQ=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
MEM_REQ=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')
CPU_LIM=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.cpu}')
MEM_LIM=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.memory}')

echo "  CPU Request: $CPU_REQ"
echo "  Memory Request: $MEM_REQ"
echo "  CPU Limit: $CPU_LIM"
echo "  Memory Limit: $MEM_LIM"

# Store initial resources
INITIAL_RESOURCES="$CPU_REQ|$MEM_REQ|$CPU_LIM|$MEM_LIM"

# Create test workload
print_info "Creating test workload with high resources..."
kubectl create namespace test-protection --dry-run=client -o yaml | kubectl apply -f -

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: high-resource-app
  namespace: test-protection
spec:
  replicas: 1
  selector:
    matchLabels:
      app: high-resource-app
  template:
    metadata:
      labels:
        app: high-resource-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 300m
            memory: 300Mi
          limits:
            cpu: 600m
            memory: 600Mi
EOF

# Wait for test workload
print_info "Waiting for test workload..."
kubectl wait --for=condition=Ready pod -l app=high-resource-app -n test-protection --timeout=60s

# Monitor logs
print_info "Monitoring logs for $TEST_DURATION seconds..."
echo "=== Right-Sizer Self-Protection Test - $(date) ===" >$LOG_FILE

# Start log monitoring
kubectl logs -n $NAMESPACE $WORKING_POD -f --since=10s >>$LOG_FILE 2>&1 &
LOG_PID=$!

# Monitor for events
START_TIME=$(date +%s)
CONFIG_PROTECTION=false
POD_PROTECTION=false
SELF_RESIZE=false
TEST_RESIZE=false
RESOURCE_ERROR=false

while [ $(($(date +%s) - START_TIME)) -lt $TEST_DURATION ]; do
  if [ -f "$LOG_FILE" ]; then
    # Check for protection events
    if grep -q "Added operator namespace.*exclude list\|Preserved operator namespace.*exclude list" $LOG_FILE 2>/dev/null; then
      if [ "$CONFIG_PROTECTION" = false ]; then
        print_success "Configuration self-protection detected"
        CONFIG_PROTECTION=true
      fi
    fi

    if grep -q "Skipping self-pod.*prevent self-modification" $LOG_FILE 2>/dev/null; then
      if [ "$POD_PROTECTION" = false ]; then
        print_success "Pod-level self-protection detected"
        POD_PROTECTION=true
      fi
    fi

    # Check for bad events
    if grep -q "Resizing.*pod right-sizer/" $LOG_FILE 2>/dev/null; then
      if [ "$SELF_RESIZE" = false ]; then
        print_error "CRITICAL: Self-resize attempt detected!"
        SELF_RESIZE=true
      fi
    fi

    if grep -q "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null; then
      if [ "$RESOURCE_ERROR" = false ]; then
        print_error "CRITICAL: Resource error detected!"
        RESOURCE_ERROR=true
      fi
    fi

    # Check for good events
    if grep -q "Resizing.*pod test-protection/" $LOG_FILE 2>/dev/null; then
      if [ "$TEST_RESIZE" = false ]; then
        print_success "Right-sizer processing test workloads"
        TEST_RESIZE=true
      fi
    fi
  fi

  # Show progress every 20 seconds
  ELAPSED=$(($(date +%s) - START_TIME))
  if [ $((ELAPSED % 20)) -eq 0 ] && [ $ELAPSED -gt 0 ]; then
    printf "\rProgress: ${ELAPSED}s | Config:%s Pod:%s Test:%s Self:%s Error:%s" \
      "$([ "$CONFIG_PROTECTION" = true ] && echo "✅" || echo "⏳")" \
      "$([ "$POD_PROTECTION" = true ] && echo "✅" || echo "⏳")" \
      "$([ "$TEST_RESIZE" = true ] && echo "✅" || echo "⏳")" \
      "$([ "$SELF_RESIZE" = true ] && echo "❌" || echo "✅")" \
      "$([ "$RESOURCE_ERROR" = true ] && echo "❌" || echo "✅")"
  fi

  sleep 2
done

echo "" # New line after progress

# Stop log monitoring
kill $LOG_PID 2>/dev/null || true

# Final resource check
print_info "Checking final resources..."
FINAL_CPU_REQ=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
FINAL_MEM_REQ=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.requests.memory}')
FINAL_CPU_LIM=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.cpu}')
FINAL_MEM_LIM=$(kubectl get pod $WORKING_POD -n $NAMESPACE -o jsonpath='{.spec.containers[0].resources.limits.memory}')

FINAL_RESOURCES="$FINAL_CPU_REQ|$FINAL_MEM_REQ|$FINAL_CPU_LIM|$FINAL_MEM_LIM"

echo "========================================"
echo "Test Results Summary"
echo "========================================"

# Count events
CONFIG_EVENTS=$(grep -c "Added operator namespace.*exclude list\|Preserved operator namespace.*exclude list" $LOG_FILE 2>/dev/null || echo 0)
POD_EVENTS=$(grep -c "Skipping self-pod.*prevent self-modification" $LOG_FILE 2>/dev/null || echo 0)
SELF_EVENTS=$(grep -c "Resizing.*pod right-sizer/" $LOG_FILE 2>/dev/null || echo 0)
TEST_EVENTS=$(grep -c "Resizing.*pod test-protection/" $LOG_FILE 2>/dev/null || echo 0)
ERROR_EVENTS=$(grep -c "CPU resize failed: the server could not find the requested resource" $LOG_FILE 2>/dev/null || echo 0)

print_info "Event counts:"
echo "  Configuration protection: $CONFIG_EVENTS"
echo "  Pod-level protection: $POD_EVENTS"
echo "  Self-resize attempts: $SELF_EVENTS"
echo "  Test workload resizes: $TEST_EVENTS"
echo "  Resource errors: $ERROR_EVENTS"

print_info "Resource comparison:"
echo "  Initial: $INITIAL_RESOURCES"
echo "  Final:   $FINAL_RESOURCES"

# Evaluate results
PASSED=0
TOTAL=3

if [ "$INITIAL_RESOURCES" = "$FINAL_RESOURCES" ]; then
  print_success "Pod resources unchanged (GOOD)"
  PASSED=$((PASSED + 1))
else
  print_error "Pod resources changed (BAD)"
fi

if [ $SELF_EVENTS -eq 0 ]; then
  print_success "No self-resize attempts (GOOD)"
  PASSED=$((PASSED + 1))
else
  print_error "Self-resize attempts detected (BAD)"
fi

if [ $ERROR_EVENTS -eq 0 ]; then
  print_success "No resource errors (GOOD)"
  PASSED=$((PASSED + 1))
else
  print_error "Resource errors detected (BAD)"
fi

echo "========================================"
echo "Final Score: $PASSED/$TOTAL critical checks passed"

if [ $CONFIG_EVENTS -gt 0 ] || [ $POD_EVENTS -gt 0 ]; then
  print_success "Self-protection mechanisms active"
fi

if [ $TEST_EVENTS -gt 0 ]; then
  print_success "Right-sizer processing other workloads normally"
fi

# Show key log entries
echo "========================================"
print_info "Key log entries:"
if [ -s "$LOG_FILE" ]; then
  echo "Protection messages:"
  grep -i "self-protection\|skipping self-pod" $LOG_FILE 2>/dev/null | head -3 || echo "  None found"
  echo ""
  echo "Recent activity:"
  tail -5 $LOG_FILE | sed 's/^/  /'
else
  echo "  No significant log activity during monitoring"
fi

# Cleanup
print_info "Cleaning up test resources..."
kubectl delete namespace test-protection --ignore-not-found=true >/dev/null 2>&1

echo "========================================"
if [ $PASSED -ge 2 ] && [ $SELF_EVENTS -eq 0 ] && [ $ERROR_EVENTS -eq 0 ]; then
  print_success "🛡️ SELF-PROTECTION TEST PASSED"
  print_success "Right-sizer is protected from self-modification"
  exit 0
else
  print_error "❌ SELF-PROTECTION TEST FAILED"
  print_error "Issues detected with self-protection"
  print_info "Review logs at: $LOG_FILE"
  exit 1
fi
