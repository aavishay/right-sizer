#!/bin/bash

# Test script to verify reduced duplicate logging in right-sizer
# This script deploys test pods and monitors the logs for duplicate entries

set -e

NAMESPACE="logging-test"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="tests/reports/duplicate-logging-test-${TIMESTAMP}.log"

echo "🧪 Testing Duplicate Logging Reduction"
echo "======================================"
echo "Timestamp: ${TIMESTAMP}"
echo "Log file: ${LOG_FILE}"
echo ""

# Create test namespace
echo "📦 Creating test namespace..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Deploy a test pod with specific resource requirements
echo "🚀 Deploying test pod..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-logging-app
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-logging
  template:
    metadata:
      labels:
        app: test-logging
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
            # Simulate variable CPU and memory usage
            while true; do
              # CPU intensive work for 10 seconds
              (while true; do echo "scale=5000; 4*a(1)" | bc -l > /dev/null; done) &
              CPU_PID=$!
              sleep 10
              kill $CPU_PID 2>/dev/null || true
              # Memory allocation for 10 seconds
              dd if=/dev/zero of=/tmp/memfile bs=100M count=1 2>/dev/null
              sleep 5
              rm -f /tmp/memfile
              sleep 5
            done
EOF

# Wait for pod to be ready
echo "⏳ Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod -l app=test-logging -n ${NAMESPACE} --timeout=60s

# Deploy right-sizer if not already running
echo "🔧 Ensuring right-sizer is deployed..."
if ! kubectl get deployment right-sizer -n right-sizer >/dev/null 2>&1; then
  echo "   Deploying right-sizer..."
  helm install right-sizer ./helm/right-sizer \
    --namespace right-sizer \
    --create-namespace \
    --set config.resizeMode=InPlaceResize \
    --set config.recommendationFrequency=30s \
    --set config.cpuScaleUpThreshold=0.7 \
    --set config.cpuScaleDownThreshold=0.3 \
    --set config.memoryScaleUpThreshold=0.8 \
    --set config.memoryScaleDownThreshold=0.4 \
    --set logLevel=INFO

  kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=right-sizer -n right-sizer --timeout=60s
else
  echo "   Right-sizer already deployed"
fi

# Start collecting logs
echo "📊 Collecting logs for 2 minutes..."
echo "=====================================" >${LOG_FILE}
echo "Test started at: $(date)" >>${LOG_FILE}
echo "=====================================" >>${LOG_FILE}
echo "" >>${LOG_FILE}

# Monitor logs for 2 minutes
kubectl logs -f deployment/right-sizer -n right-sizer --tail=0 >>${LOG_FILE} 2>&1 &
LOG_PID=$!

# Wait for logging to complete
sleep 120
kill $LOG_PID 2>/dev/null || true

echo ""
echo "📈 Analyzing logs for duplicate patterns..."
echo "==========================================="

# Check for duplicate scaling analysis logs
echo ""
echo "1. Scaling Analysis Logs:"
echo "-------------------------"
SCALING_COUNT=$(grep -c "🔍 Scaling analysis" ${LOG_FILE} || true)
echo "   Total scaling analysis logs: ${SCALING_COUNT}"

# Show unique scaling analysis patterns
echo "   Unique patterns:"
grep "🔍 Scaling analysis" ${LOG_FILE} | sed 's/[0-9]\+\(\.[0-9]\+\)\?[a-zA-Z]*//g' | sort -u | head -5 || true

# Check for duplicate resize logs
echo ""
echo "2. Container Resize Logs:"
echo "-------------------------"
RESIZE_COUNT=$(grep -c "📈 Container.*will be resized" ${LOG_FILE} || true)
echo "   Total resize logs: ${RESIZE_COUNT}"

# Check for duplicate success logs
echo ""
echo "3. Success Logs:"
echo "----------------"
SUCCESS_COUNT=$(grep -c "✅ Successfully resized pod" ${LOG_FILE} || true)
echo "   Total success logs: ${SUCCESS_COUNT}"

# Check for duplicate batch processing logs
echo ""
echo "4. Batch Processing Logs:"
echo "-------------------------"
BATCH_COUNT=$(grep -c "📦 Processing batch" ${LOG_FILE} || true)
echo "   Total batch logs: ${BATCH_COUNT}"

# Check for duplicate completion logs
echo ""
echo "5. Completion Logs:"
echo "-------------------"
COMPLETE_COUNT=$(grep -c "✅ Completed processing all" ${LOG_FILE} || true)
echo "   Total completion logs: ${COMPLETE_COUNT}"

# Analyze duplicate lines
echo ""
echo "6. Duplicate Line Analysis:"
echo "---------------------------"
echo "   Lines that appear more than once (excluding timestamps):"
# Remove timestamps and count duplicates
cat ${LOG_FILE} | sed 's/^[0-9\/]* [0-9:]* //' | sort | uniq -c | sort -rn | grep -v "^      1 " | head -10 || echo "   No exact duplicate lines found!"

# Check specific patterns that were problematic before
echo ""
echo "7. Checking Previously Problematic Patterns:"
echo "--------------------------------------------"

# Check if scaling analysis and resize logs appear together (should now)
COMBINED_LOGS=$(grep -B1 "📈 Container.*will be resized" ${LOG_FILE} | grep -c "🔍 Scaling analysis" || true)
echo "   Scaling analysis immediately before resize: ${COMBINED_LOGS} occurrences"

# Check if we have multiple success logs for the same pod
echo "   Checking for multiple success logs per pod:"
grep "✅ Successfully resized pod" ${LOG_FILE} | awk '{print $NF}' | sort | uniq -c | sort -rn | head -5 || echo "   No pods found"

echo ""
echo "8. Log Summary:"
echo "--------------"
TOTAL_LINES=$(wc -l <${LOG_FILE})
UNIQUE_LINES=$(cat ${LOG_FILE} | sed 's/^[0-9\/]* [0-9:]* //' | sort -u | wc -l)
echo "   Total log lines: ${TOTAL_LINES}"
echo "   Unique log lines (excluding timestamps): ${UNIQUE_LINES}"
echo "   Duplication ratio: $(echo "scale=2; ($TOTAL_LINES - $UNIQUE_LINES) * 100 / $TOTAL_LINES" | bc)%"

echo ""
echo "📋 Test Results:"
echo "==============="
if [ ${SCALING_COUNT} -gt 0 ] && [ ${RESIZE_COUNT} -gt 0 ]; then
  RATIO=$(echo "scale=2; ${SCALING_COUNT} / ${RESIZE_COUNT}" | bc)
  if (($(echo "$RATIO > 1.5" | bc -l))); then
    echo "⚠️  WARNING: High ratio of scaling analysis to resize logs (${RATIO}:1)"
    echo "   This might indicate duplicate logging"
  else
    echo "✅ Logging appears optimized (ratio ${RATIO}:1)"
  fi
else
  echo "ℹ️  No resize operations detected during test period"
fi

# Cleanup
echo ""
echo "🧹 Cleaning up test resources..."
kubectl delete namespace ${NAMESPACE} --ignore-not-found=true

echo ""
echo "✅ Test completed. Full logs saved to: ${LOG_FILE}"
echo ""
echo "💡 To view the full log file:"
echo "   cat ${LOG_FILE}"
echo ""
echo "💡 To check for specific patterns:"
echo "   grep '🔍 Scaling analysis' ${LOG_FILE}"
echo "   grep '📈 Container.*will be resized' ${LOG_FILE}"
