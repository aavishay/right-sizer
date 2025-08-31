#!/bin/bash

# Simple verification script for right-sizer logging improvements
# This script checks if the duplicate logging issue has been fixed

set -e

echo "================================================"
echo "    Right-Sizer Logging Fix Verification"
echo "================================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACE="verify-logging"
TEST_POD_NAME="test-app"
LOG_DURATION=60
TEMP_LOG="/tmp/rightsizer-verify-$$.log"

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
        cpu: "50m"
        memory: "64Mi"
      limits:
        cpu: "200m"
        memory: "256Mi"
EOF

echo "⏳ Step 2: Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod/${TEST_POD_NAME} -n ${TEST_NAMESPACE} --timeout=30s

echo "📊 Step 3: Monitoring right-sizer logs for ${LOG_DURATION} seconds..."
echo ""

# Start capturing logs
kubectl logs -f deployment/right-sizer -n right-sizer --tail=0 >${TEMP_LOG} 2>&1 &
LOG_PID=$!

# Create some load on the test pod to trigger scaling analysis
kubectl exec -n ${TEST_NAMESPACE} ${TEST_POD_NAME} -- sh -c '
for i in $(seq 1 5); do
  dd if=/dev/zero of=/tmp/test bs=10M count=10 2>/dev/null
  sleep 2
  rm -f /tmp/test
  sleep 2
done
' &

# Wait for log collection
sleep ${LOG_DURATION}
kill ${LOG_PID} 2>/dev/null || true

echo ""
echo "📈 Step 4: Analyzing logs for duplicates..."
echo "==========================================="

# Count occurrences of key log patterns
SCALING_ANALYSIS=$(grep -c "🔍 Scaling analysis" ${TEMP_LOG} 2>/dev/null || echo "0")
CONTAINER_RESIZE=$(grep -c "📈 Container.*will be resized" ${TEMP_LOG} 2>/dev/null || echo "0")
SUCCESS_RESIZE=$(grep -c "✅ Successfully resized pod" ${TEMP_LOG} 2>/dev/null || echo "0")
BATCH_PROCESSING=$(grep -c "📦 Processing batch" ${TEMP_LOG} 2>/dev/null || echo "0")

echo "Log Pattern Counts:"
echo "-------------------"
echo "  🔍 Scaling analysis logs: ${SCALING_ANALYSIS}"
echo "  📈 Container resize logs: ${CONTAINER_RESIZE}"
echo "  ✅ Success logs: ${SUCCESS_RESIZE}"
echo "  📦 Batch processing logs: ${BATCH_PROCESSING}"

# Check for exact duplicate lines (excluding timestamps)
echo ""
echo "Checking for duplicate lines..."
DUPLICATE_COUNT=$(cat ${TEMP_LOG} | sed 's/^[0-9\/]* [0-9:]* //' | sort | uniq -d | wc -l)

echo ""
echo "================================================"
echo "                  RESULTS"
echo "================================================"

ISSUES_FOUND=0

# Check if scaling analysis appears more than resize notifications
if [ ${SCALING_ANALYSIS} -gt 0 ] && [ ${CONTAINER_RESIZE} -gt 0 ]; then
  RATIO=$(echo "scale=2; ${SCALING_ANALYSIS} / ${CONTAINER_RESIZE}" | bc 2>/dev/null || echo "1")
  if [ "$(echo "${RATIO} > 1.2" | bc 2>/dev/null || echo "0")" = "1" ]; then
    echo -e "${RED}❌ Issue detected: Scaling analysis appears ${RATIO}x more than resize logs${NC}"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
  else
    echo -e "${GREEN}✅ Scaling analysis to resize ratio is good (${RATIO}:1)${NC}"
  fi
fi

# Check for duplicate lines
if [ ${DUPLICATE_COUNT} -gt 0 ]; then
  echo -e "${RED}❌ Found ${DUPLICATE_COUNT} duplicate log lines${NC}"
  echo "   Duplicates:"
  cat ${TEMP_LOG} | sed 's/^[0-9\/]* [0-9:]* //' | sort | uniq -d | head -5
  ISSUES_FOUND=$((ISSUES_FOUND + 1))
else
  echo -e "${GREEN}✅ No exact duplicate log lines found${NC}"
fi

# Check if logs show the new pattern (scaling analysis with resize notification)
if grep -q "🔍 Scaling analysis.*\n.*📈 Container.*will be resized" ${TEMP_LOG} 2>/dev/null; then
  echo -e "${GREEN}✅ New logging pattern detected (analysis + resize together)${NC}"
else
  if [ ${SCALING_ANALYSIS} -gt 0 ] || [ ${CONTAINER_RESIZE} -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Could not verify new logging pattern${NC}"
  fi
fi

echo ""
echo "================================================"
if [ ${ISSUES_FOUND} -eq 0 ]; then
  echo -e "${GREEN}✅ VERIFICATION PASSED${NC}"
  echo "The duplicate logging fix appears to be working correctly!"
else
  echo -e "${RED}❌ VERIFICATION FAILED${NC}"
  echo "Found ${ISSUES_FOUND} issue(s) with logging"
fi
echo "================================================"

# Cleanup
echo ""
echo "🧹 Cleaning up..."
kubectl delete namespace ${TEST_NAMESPACE} --ignore-not-found=true
rm -f ${TEMP_LOG}

echo ""
echo "✅ Verification complete!"

# Exit with appropriate code
exit ${ISSUES_FOUND}
