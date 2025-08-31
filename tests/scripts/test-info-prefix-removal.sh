#!/bin/bash

# Test script to verify [INFO] prefix removal from right-sizer logs
# This script creates a test pod and monitors logs to confirm the fix

set -e

echo "================================================"
echo "    Testing [INFO] Prefix Removal"
echo "================================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="info-prefix-test"
TEST_POD_NAME="test-prefix-app"
LOG_DURATION=30
TEMP_LOG="/tmp/rightsizer-info-test-$$.log"

echo "📋 Step 1: Creating test namespace and pod..."
kubectl create namespace ${TEST_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD_NAME}
  namespace: ${TEST_NAMESPACE}
spec:
  containers:
  - name: app
    image: nginx:alpine
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
      limits:
        cpu: "500m"
        memory: "512Mi"
    command:
      - sh
      - -c
      - |
        # Generate some CPU and memory load
        while true; do
          # CPU load
          for i in \$(seq 1 100000); do echo \$i > /dev/null; done
          # Memory allocation
          dd if=/dev/zero of=/tmp/test bs=50M count=1 2>/dev/null
          sleep 5
          rm -f /tmp/test
          sleep 5
        done
EOF

echo "⏳ Step 2: Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod/${TEST_POD_NAME} -n ${TEST_NAMESPACE} --timeout=30s

echo "📊 Step 3: Capturing right-sizer logs for ${LOG_DURATION} seconds..."
echo ""

# Start capturing logs
kubectl logs -f deployment/right-sizer -n right-sizer --tail=0 >${TEMP_LOG} 2>&1 &
LOG_PID=$!

# Wait for log collection
sleep ${LOG_DURATION}
kill ${LOG_PID} 2>/dev/null || true

echo ""
echo "📈 Step 4: Analyzing logs for [INFO] prefix..."
echo "==========================================="

# Count different patterns
TOTAL_LINES=$(wc -l <${TEMP_LOG} 2>/dev/null || echo "0")
INFO_PREFIX_COUNT=$(grep -c "\[INFO\]" ${TEMP_LOG} 2>/dev/null || echo "0")
SCALING_LOGS=$(grep -c "🔍 Scaling analysis" ${TEMP_LOG} 2>/dev/null || echo "0")
RESIZE_LOGS=$(grep -c "📈 Container.*will be resized" ${TEMP_LOG} 2>/dev/null || echo "0")

echo "Log Analysis:"
echo "-------------"
echo "  Total log lines: ${TOTAL_LINES}"
echo "  Lines with [INFO] prefix: ${INFO_PREFIX_COUNT}"
echo "  Scaling analysis logs: ${SCALING_LOGS}"
echo "  Container resize logs: ${RESIZE_LOGS}"
echo ""

# Show sample of logs
echo "Sample Log Lines:"
echo "-----------------"
if [ -s ${TEMP_LOG} ]; then
  head -5 ${TEMP_LOG}
else
  echo "  (No logs captured)"
fi

echo ""
echo "================================================"
echo "                  RESULTS"
echo "================================================"

if [ ${INFO_PREFIX_COUNT} -eq 0 ]; then
  echo -e "${GREEN}✅ SUCCESS: No [INFO] prefix found in logs${NC}"
  echo "   The [INFO] prefix has been successfully removed!"
else
  echo -e "${RED}❌ FAILED: Found ${INFO_PREFIX_COUNT} lines with [INFO] prefix${NC}"
  echo ""
  echo "Examples of lines with [INFO]:"
  grep "\[INFO\]" ${TEMP_LOG} | head -3
fi

# Check if the important logs are still being generated
echo ""
if [ ${SCALING_LOGS} -gt 0 ] || [ ${RESIZE_LOGS} -gt 0 ]; then
  echo -e "${GREEN}✅ Logging functionality intact${NC}"
  echo "   Important logs are still being generated without [INFO] prefix"
else
  echo -e "${YELLOW}⚠️  No scaling/resize logs captured during test period${NC}"
  echo "   This might be normal if no resizing was needed"
fi

# Show the expected format
echo ""
echo "Expected Log Format (without [INFO]):"
echo "-------------------------------------"
echo "2025/08/31 21:00:00 🔍 Scaling analysis - CPU: scale up..."
echo "2025/08/31 21:00:00 📈 Container namespace/pod/container will be resized..."
echo ""
echo "Instead of:"
echo "-----------"
echo "2025/08/31 21:00:00 [INFO] 🔍 Scaling analysis - CPU: scale up..."
echo "2025/08/31 21:00:00 [INFO] 📈 Container namespace/pod/container will be resized..."

# Cleanup
echo ""
echo "🧹 Cleaning up..."
kubectl delete namespace ${TEST_NAMESPACE} --ignore-not-found=true
rm -f ${TEMP_LOG}

echo ""
echo "================================================"
if [ ${INFO_PREFIX_COUNT} -eq 0 ]; then
  echo -e "${GREEN}✅ Test PASSED${NC}"
  exit 0
else
  echo -e "${RED}❌ Test FAILED${NC}"
  exit 1
fi
