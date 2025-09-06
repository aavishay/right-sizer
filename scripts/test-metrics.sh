right-sizer/scripts/test-metrics.sh
#!/bin/bash

# Test script for Right-Sizer operator metrics endpoint
# This script verifies connectivity and parses basic metrics

set -e

# Configuration
METRICS_URL="${METRICS_URL:-http://right-sizer.right-sizer.svc.cluster.local:80/metrics}"
TIMEOUT="${TIMEOUT:-10}"

echo "🔍 Testing Right-Sizer Metrics Endpoint"
echo "========================================"
echo "Endpoint: $METRICS_URL"
echo "Timeout: ${TIMEOUT}s"
echo ""

# Test basic connectivity
echo "📡 Testing connectivity..."
if ! curl -s --max-time "$TIMEOUT" --head "$METRICS_URL" >/dev/null 2>&1; then
  echo "❌ Connection failed - endpoint not reachable"
  echo ""
  echo "🔧 Troubleshooting tips:"
  echo "  - Check if right-sizer service is running: kubectl get svc -n right-sizer"
  echo "  - Check if right-sizer pods are running: kubectl get pods -n right-sizer"
  echo "  - Verify network policies allow communication"
  echo "  - Try with different endpoint URL"
  exit 1
fi

echo "✅ Endpoint reachable"

# Fetch and analyze metrics
echo ""
echo "📊 Fetching metrics..."
METRICS_RESPONSE=$(curl -s --max-time "$TIMEOUT" "$METRICS_URL" 2>/dev/null)

if [ -z "$METRICS_RESPONSE" ]; then
  echo "❌ No response from metrics endpoint"
  exit 1
fi

echo "✅ Metrics received (${#METRICS_RESPONSE} characters)"

# Parse key metrics
echo ""
echo "🔢 Key Metrics Analysis:"
echo "-----------------------"

# Function to extract metric value
extract_metric() {
  local metric_name="$1"
  local value=$(echo "$METRICS_RESPONSE" | grep "^$metric_name" | head -1 | awk '{print $2}')
  echo "$value"
}

# Core metrics
PODS_PROCESSED=$(extract_metric "rightsizer_pods_processed_total")
PODS_RESIZED=$(extract_metric "rightsizer_pods_resized_total")
CPU_ADJUSTMENTS=$(extract_metric "rightsizer_cpu_adjustments_total")
MEMORY_ADJUSTMENTS=$(extract_metric "rightsizer_memory_adjustments_total")

echo "Pods Processed: ${PODS_PROCESSED:-0}"
echo "Pods Resized: ${PODS_RESIZED:-0}"
echo "CPU Adjustments: ${CPU_ADJUSTMENTS:-0}"
echo "Memory Adjustments: ${MEMORY_ADJUSTMENTS:-0}"

# Check for processing duration metrics
PROCESSING_DURATION=$(echo "$METRICS_RESPONSE" | grep "rightsizer_processing_duration_seconds" | wc -l)
echo "Processing Duration Metrics: $PROCESSING_DURATION samples"

# Check for error metrics
ERROR_METRICS=$(echo "$METRICS_RESPONSE" | grep "rightsizer.*error" | wc -l)
echo "Error Metrics: $ERROR_METRICS types found"

# Check for safety metrics
SAFETY_METRICS=$(echo "$METRICS_RESPONSE" | grep "rightsizer.*safety" | wc -l)
echo "Safety Metrics: $SAFETY_METRICS types found"

echo ""
echo "📋 Sample Metrics (first 10 lines):"
echo "-----------------------------------"
echo "$METRICS_RESPONSE" | head -10

echo ""
echo "🎯 Connection Test Summary:"
echo "=========================="

if [ "${PODS_PROCESSED:-0}" -gt 0 ]; then
  echo "✅ Right-Sizer operator is active and processing pods"
  OPTIMIZATION_RATE=$((PODS_RESIZED * 100 / PODS_PROCESSED))
  echo "📈 Optimization Rate: $OPTIMIZATION_RATE% ($PODS_RESIZED/$PODS_PROCESSED pods)"
else
  echo "⚠️  Right-Sizer operator appears to be running but no pods processed yet"
fi

if [ "$ERROR_METRICS" -gt 0 ]; then
  echo "⚠️  Some error metrics detected - check operator logs"
fi

echo ""
echo "✅ Metrics endpoint test completed successfully"
echo ""
echo "💡 Next steps:"
echo "  - Check operator logs: kubectl logs -n right-sizer deployment/right-sizer"
echo "  - Monitor metrics: kubectl port-forward -n right-sizer svc/right-sizer 8080:80"
