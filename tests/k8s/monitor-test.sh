#!/bin/bash

# Right-Sizer Test Monitoring Script
# This script monitors test deployments and verifies QoS preservation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test configuration
TEST_NAMESPACES=("test-guaranteed" "test-apps")
MONITORING_DURATION=${1:-300} # Default 5 minutes
CHECK_INTERVAL=10

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}Right-Sizer Guaranteed QoS Test Monitor${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Function to check if all pods are ready
check_pods_ready() {
  local ready=true
  for ns in "${TEST_NAMESPACES[@]}"; do
    local not_ready=$(kubectl get pods -n "$ns" --no-headers | grep -v "Running\|Completed" | wc -l)
    if [ "$not_ready" -gt 0 ]; then
      ready=false
    fi
  done
  echo $ready
}

# Function to get QoS class of a pod
get_pod_qos() {
  local namespace=$1
  local pod=$2
  kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.status.qosClass}' 2>/dev/null
}

# Function to check if resources are equal (for Guaranteed QoS)
check_guaranteed_resources() {
  local namespace=$1
  local pod=$2

  # Get CPU requests and limits
  local cpu_req=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.requests.cpu}' 2>/dev/null)
  local cpu_lim=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.limits.cpu}' 2>/dev/null)

  # Get memory requests and limits
  local mem_req=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.requests.memory}' 2>/dev/null)
  local mem_lim=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.limits.memory}' 2>/dev/null)

  if [ "$cpu_req" == "$cpu_lim" ] && [ "$mem_req" == "$mem_lim" ]; then
    echo "true"
  else
    echo "false"
  fi
}

# Function to generate load on test pods
generate_load() {
  echo -e "${YELLOW}Generating load on test pods...${NC}"

  # Generate HTTP load on nginx pods
  for i in {1..10}; do
    kubectl run -n test-guaranteed load-generator-$i --image=busybox --rm -it --restart=Never -- \
      sh -c "for i in \$(seq 1 100); do wget -q -O- http://guaranteed-qos-app/; done" 2>/dev/null &
  done

  # Generate Redis load
  kubectl run -n test-guaranteed redis-load --image=redis:alpine --rm -it --restart=Never -- \
    redis-cli -h critical-workload SET test "data" 2>/dev/null &

  wait
  echo -e "${GREEN}Load generation completed${NC}"
}

# Function to monitor right-sizer logs
monitor_rightsizer_logs() {
  echo -e "${BLUE}Recent Right-Sizer Activity:${NC}"
  kubectl logs -n right-sizer deployment/right-sizer --tail=20 | grep -E "(Maintaining Guaranteed QoS|Error updating|will be resized|QoS class)" || true
  echo ""
}

# Function to display pod resources table
display_pod_resources() {
  local namespace=$1
  echo -e "${CYAN}Namespace: $namespace${NC}"
  echo "----------------------------------------"
  printf "%-40s %-12s %-8s %-8s %-8s %-8s %-10s\n" "POD" "QOS" "CPU_REQ" "CPU_LIM" "MEM_REQ" "MEM_LIM" "GUARANTEED"
  echo "----------------------------------------"

  local pods=$(kubectl get pods -n "$namespace" --no-headers -o custom-columns=":metadata.name")
  for pod in $pods; do
    local qos=$(get_pod_qos "$namespace" "$pod")
    local cpu_req=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.requests.cpu}' 2>/dev/null || echo "N/A")
    local cpu_lim=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.limits.cpu}' 2>/dev/null || echo "N/A")
    local mem_req=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.requests.memory}' 2>/dev/null || echo "N/A")
    local mem_lim=$(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.containers[0].resources.limits.memory}' 2>/dev/null || echo "N/A")
    local is_guaranteed=$(check_guaranteed_resources "$namespace" "$pod")

    # Color code based on QoS
    local color=$NC
    if [ "$qos" == "Guaranteed" ]; then
      color=$GREEN
      if [ "$is_guaranteed" != "true" ]; then
        color=$RED # Red if QoS says Guaranteed but resources don't match
      fi
    elif [ "$qos" == "Burstable" ]; then
      color=$YELLOW
    fi

    printf "${color}%-40s %-12s %-8s %-8s %-8s %-8s %-10s${NC}\n" \
      "${pod:0:40}" "$qos" "$cpu_req" "$cpu_lim" "$mem_req" "$mem_lim" "$is_guaranteed"
  done
  echo ""
}

# Function to check for QoS violations
check_qos_violations() {
  local violations=0
  echo -e "${YELLOW}Checking for QoS violations...${NC}"

  for ns in "${TEST_NAMESPACES[@]}"; do
    local pods=$(kubectl get pods -n "$ns" --no-headers -o custom-columns=":metadata.name")
    for pod in $pods; do
      local qos=$(get_pod_qos "$ns" "$pod")
      local expected_guaranteed=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.annotations.rightsizer\.io/qos-class}' 2>/dev/null)

      if [ "$expected_guaranteed" == "Guaranteed" ] && [ "$qos" != "Guaranteed" ]; then
        echo -e "${RED}VIOLATION: Pod $ns/$pod expected to be Guaranteed but is $qos${NC}"
        violations=$((violations + 1))
      fi

      if [ "$qos" == "Guaranteed" ]; then
        local is_guaranteed=$(check_guaranteed_resources "$ns" "$pod")
        if [ "$is_guaranteed" != "true" ]; then
          echo -e "${RED}VIOLATION: Pod $ns/$pod is marked as Guaranteed but resources don't match${NC}"
          violations=$((violations + 1))
        fi
      fi
    done
  done

  if [ $violations -eq 0 ]; then
    echo -e "${GREEN}✓ No QoS violations detected${NC}"
  else
    echo -e "${RED}✗ Found $violations QoS violations${NC}"
  fi

  return $violations
}

# Function to get metrics for a pod
get_pod_metrics() {
  local namespace=$1
  local pod=$2
  kubectl top pod "$pod" -n "$namespace" --no-headers 2>/dev/null || echo "N/A"
}

# Function to display metrics
display_metrics() {
  echo -e "${CYAN}Pod Metrics:${NC}"
  echo "----------------------------------------"

  for ns in "${TEST_NAMESPACES[@]}"; do
    echo -e "${BLUE}Namespace: $ns${NC}"
    kubectl top pods -n "$ns" 2>/dev/null || echo "Metrics not available yet"
    echo ""
  done
}

# Main monitoring loop
main() {
  local start_time=$(date +%s)
  local end_time=$((start_time + MONITORING_DURATION))
  local iteration=0
  local total_violations=0

  # Wait for pods to be ready
  echo -e "${YELLOW}Waiting for all pods to be ready...${NC}"
  while [ "$(check_pods_ready)" != "true" ]; do
    sleep 2
    echo -n "."
  done
  echo -e "\n${GREEN}All pods are ready!${NC}\n"

  # Initial state
  echo -e "${CYAN}=== Initial State ===${NC}"
  for ns in "${TEST_NAMESPACES[@]}"; do
    display_pod_resources "$ns"
  done

  # Generate some load
  generate_load

  # Monitor loop
  while [ $(date +%s) -lt $end_time ]; do
    iteration=$((iteration + 1))
    echo -e "${CYAN}=== Monitoring Iteration $iteration ===${NC}"
    echo "Time remaining: $((end_time - $(date +%s))) seconds"
    echo ""

    # Display current state
    for ns in "${TEST_NAMESPACES[@]}"; do
      display_pod_resources "$ns"
    done

    # Display metrics if available
    display_metrics

    # Monitor right-sizer activity
    monitor_rightsizer_logs

    # Check for violations
    violations=0
    check_qos_violations || violations=$?
    total_violations=$((total_violations + violations))

    echo -e "${CYAN}========================================${NC}"

    # Wait before next check
    sleep $CHECK_INTERVAL
  done

  # Final report
  echo ""
  echo -e "${CYAN}=== Final Report ===${NC}"
  echo -e "Test Duration: ${MONITORING_DURATION} seconds"
  echo -e "Total Iterations: ${iteration}"
  echo -e "Total QoS Violations: ${total_violations}"

  if [ $total_violations -eq 0 ]; then
    echo -e "${GREEN}✓ TEST PASSED: No QoS violations detected during monitoring${NC}"
    exit 0
  else
    echo -e "${RED}✗ TEST FAILED: $total_violations QoS violations detected${NC}"
    exit 1
  fi
}

# Handle script termination
trap 'echo -e "\n${YELLOW}Monitoring interrupted${NC}"; exit 130' INT TERM

# Check prerequisites
if ! kubectl get ns right-sizer &>/dev/null; then
  echo -e "${RED}Error: right-sizer namespace not found${NC}"
  exit 1
fi

for ns in "${TEST_NAMESPACES[@]}"; do
  if ! kubectl get ns "$ns" &>/dev/null; then
    echo -e "${RED}Error: test namespace $ns not found${NC}"
    exit 1
  fi
done

# Start monitoring
main
