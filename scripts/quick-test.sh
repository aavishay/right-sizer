#!/bin/bash

# Quick Test Runner for Right-Sizer Resource Calculations
# This script runs the unit tests without requiring a full cluster setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
GO_DIR="$PROJECT_ROOT/go"
TEST_TIMEOUT="30s"
VERBOSE=${VERBOSE:-false}

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Functions
print_header() {
  echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${CYAN}  $1${NC}"
  echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

log_info() {
  echo -e "${BLUE}ℹ${NC}  $1"
}

log_success() {
  echo -e "${GREEN}✓${NC}  $1"
}

log_warning() {
  echo -e "${YELLOW}⚠${NC}  $1"
}

log_error() {
  echo -e "${RED}✗${NC}  $1"
}

log_test() {
  echo -e "${MAGENTA}▶${NC}  $1"
}

run_test_suite() {
  local suite_name=$1
  local test_pattern=$2
  local package=${3:-"./controllers"}

  log_test "Running: $suite_name"

  if [ "$VERBOSE" = true ]; then
    go test "$package" -v -run "$test_pattern" -timeout "$TEST_TIMEOUT" 2>&1
  else
    # Run test and capture output
    if output=$(go test "$package" -run "$test_pattern" -timeout "$TEST_TIMEOUT" 2>&1); then
      log_success "$suite_name - PASSED"
      ((TESTS_PASSED++))
      return 0
    else
      log_error "$suite_name - FAILED"
      echo "$output" | grep -E "FAIL|Error|panic" | head -5
      ((TESTS_FAILED++))
      return 1
    fi
  fi
  ((TESTS_RUN++))
}

check_go_env() {
  log_info "Checking Go environment..."

  if ! command -v go &>/dev/null; then
    log_error "Go is not installed or not in PATH"
    exit 1
  fi

  GO_VERSION=$(go version | awk '{print $3}')
  log_success "Go version: $GO_VERSION"

  # Change to go directory
  cd "$GO_DIR" || {
    log_error "Cannot change to directory: $GO_DIR"
    exit 1
  }

  # Check if go.mod exists
  if [ ! -f "go.mod" ]; then
    log_error "go.mod not found in $GO_DIR"
    exit 1
  fi

  log_success "Go environment ready"
}

run_quick_tests() {
  print_header "QUICK TEST SUITE - Resource Calculations"

  # CPU Request Tests
  echo -e "\n${BLUE}▪ CPU Request Calculations${NC}"
  run_test_suite "CPU Request Calculations" "TestCPURequestCalculations"

  # Memory Request Tests
  echo -e "\n${BLUE}▪ Memory Request Calculations${NC}"
  run_test_suite "Memory Request Calculations" "TestMemoryRequestCalculations"

  # CPU Limit Tests
  echo -e "\n${BLUE}▪ CPU Limit Calculations${NC}"
  run_test_suite "CPU Limit Calculations" "TestCPULimitCalculations"

  # Memory Limit Tests
  echo -e "\n${BLUE}▪ Memory Limit Calculations${NC}"
  run_test_suite "Memory Limit Calculations" "TestMemoryLimitCalculations"

  # Complete Resource Calculation
  echo -e "\n${BLUE}▪ Complete Resource Calculations${NC}"
  run_test_suite "Complete Resource Calculation" "TestCompleteResourceCalculation"

  # Scaling Decision Impact
  echo -e "\n${BLUE}▪ Scaling Decision Impact${NC}"
  run_test_suite "Scaling Decision Impact" "TestScalingDecisionImpact"
}

run_edge_case_tests() {
  print_header "EDGE CASE TESTS - Memory Limits"

  # Memory Limit Edge Cases
  echo -e "\n${BLUE}▪ Memory Limit Problematic Scenarios${NC}"
  run_test_suite "Memory Limit Edge Cases" "TestMemoryLimitProblematicScenarios"

  # Memory Limit Ratios
  echo -e "\n${BLUE}▪ Memory Limit to Request Ratios${NC}"
  run_test_suite "Memory Limit Ratios" "TestMemoryLimitToRequestRatios"

  # Memory Burst Patterns
  echo -e "\n${BLUE}▪ Memory Limit Burst Patterns${NC}"
  run_test_suite "Memory Burst Patterns" "TestMemoryLimitWithBurstPatterns"

  # Memory Scaling Decisions
  echo -e "\n${BLUE}▪ Memory Limit Scaling Decisions${NC}"
  run_test_suite "Memory Scaling Decisions" "TestMemoryLimitScalingDecisions"

  # Container Types
  echo -e "\n${BLUE}▪ Memory Limits for Container Types${NC}"
  run_test_suite "Container Type Memory" "TestMemoryLimitForContainerTypes"

  # Validation Tests
  echo -e "\n${BLUE}▪ Memory Limit Validation${NC}"
  run_test_suite "Memory Validation" "TestMemoryLimitValidation"
}

run_metrics_tests() {
  print_header "METRICS TESTS - Calculations & Aggregations"

  # Metrics Aggregation
  echo -e "\n${BLUE}▪ Metrics Aggregation${NC}"
  run_test_suite "Metrics Aggregation" "TestMetricsAggregation" "./metrics"

  # CPU Conversions
  echo -e "\n${BLUE}▪ CPU Metrics Conversions${NC}"
  run_test_suite "CPU Conversions" "TestCPUMetricsConversions" "./metrics"

  # Memory Conversions
  echo -e "\n${BLUE}▪ Memory Metrics Conversions${NC}"
  run_test_suite "Memory Conversions" "TestMemoryMetricsConversions" "./metrics"

  # Time Series
  echo -e "\n${BLUE}▪ Time Series Calculations${NC}"
  run_test_suite "Time Series" "TestMetricsTimeSeriesCalculations" "./metrics"
}

run_specific_test() {
  local test_name=$1
  print_header "RUNNING SPECIFIC TEST: $test_name"

  cd "$GO_DIR"
  go test ./... -v -run "$test_name" -timeout "$TEST_TIMEOUT"
}

show_summary() {
  print_header "TEST SUMMARY"

  echo -e "Tests Run:     ${CYAN}$TESTS_RUN${NC}"
  echo -e "Tests Passed:  ${GREEN}$TESTS_PASSED${NC}"
  echo -e "Tests Failed:  ${RED}$TESTS_FAILED${NC}"

  if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✓ ALL TESTS PASSED!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    return 0
  else
    echo -e "\n${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ✗ SOME TESTS FAILED${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    return 1
  fi
}

show_usage() {
  cat <<EOF
Usage: $0 [OPTIONS] [TEST_NAME]

Quick test runner for Right-Sizer resource calculation tests.

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Show detailed test output
    -q, --quick         Run only quick tests (default)
    -e, --edge-cases    Run edge case tests
    -m, --metrics       Run metrics tests
    -a, --all          Run all tests
    -s, --specific TEST Run specific test by name
    -c, --coverage     Run with coverage report

EXAMPLES:
    $0                  # Run quick tests
    $0 -a              # Run all tests
    $0 -v -e           # Run edge cases with verbose output
    $0 -s TestCPURequestCalculations  # Run specific test

EOF
}

# Parse command line arguments
TEST_MODE="quick"
COVERAGE=false

while [[ $# -gt 0 ]]; do
  case $1 in
  -h | --help)
    show_usage
    exit 0
    ;;
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -q | --quick)
    TEST_MODE="quick"
    shift
    ;;
  -e | --edge-cases)
    TEST_MODE="edge"
    shift
    ;;
  -m | --metrics)
    TEST_MODE="metrics"
    shift
    ;;
  -a | --all)
    TEST_MODE="all"
    shift
    ;;
  -s | --specific)
    TEST_MODE="specific"
    SPECIFIC_TEST="$2"
    shift 2
    ;;
  -c | --coverage)
    COVERAGE=true
    shift
    ;;
  *)
    echo "Unknown option: $1"
    show_usage
    exit 1
    ;;
  esac
done

# Main execution
main() {
  echo -e "${MAGENTA}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${MAGENTA}║                    Right-Sizer Resource Calculation Tests                    ║${NC}"
  echo -e "${MAGENTA}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"

  # Check environment
  check_go_env

  # Run tests based on mode
  case $TEST_MODE in
  quick)
    run_quick_tests
    ;;
  edge)
    run_edge_case_tests
    ;;
  metrics)
    run_metrics_tests
    ;;
  all)
    run_quick_tests
    run_edge_case_tests
    run_metrics_tests
    ;;
  specific)
    run_specific_test "$SPECIFIC_TEST"
    ;;
  esac

  # Show coverage if requested
  if [ "$COVERAGE" = true ]; then
    print_header "COVERAGE REPORT"
    cd "$GO_DIR"
    go test ./controllers ./metrics -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html
    log_success "Coverage report saved to coverage.html"
  fi

  # Show summary
  show_summary
}

# Run main function
main
