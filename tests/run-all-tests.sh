#!/bin/bash

# Right-Sizer Comprehensive Test Runner
# This script runs all tests for the right-sizer operator

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Test tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_DIR="$PROJECT_ROOT/go"
TEST_DIR="$PROJECT_ROOT/tests"
COVERAGE_OUT="$PROJECT_ROOT/coverage.out"
COVERAGE_HTML="$PROJECT_ROOT/coverage.html"

# Test modes
RUN_UNIT=${RUN_UNIT:-true}
RUN_INTEGRATION=${RUN_INTEGRATION:-true}
RUN_SMOKE=${RUN_SMOKE:-true}
RUN_MINIKUBE=${RUN_MINIKUBE:-false}
VERBOSE=${VERBOSE:-false}
COVERAGE=${COVERAGE:-false}

# Functions
print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

print_header() {
  echo ""
  print_color "$CYAN" "========================================="
  print_color "$CYAN" "$1"
  print_color "$CYAN" "========================================="
}

print_test_result() {
  local test_name=$1
  local result=$2

  if [ "$result" = "0" ]; then
    print_color "$GREEN" "✅ $test_name: PASSED"
    ((PASSED_TESTS++))
  elif [ "$result" = "skip" ]; then
    print_color "$YELLOW" "⏭️  $test_name: SKIPPED"
    ((SKIPPED_TESTS++))
  else
    print_color "$RED" "❌ $test_name: FAILED"
    ((FAILED_TESTS++))
  fi
  ((TOTAL_TESTS++))
}

check_prerequisites() {
  print_header "Checking Prerequisites"

  # Check Go
  if ! command -v go &>/dev/null; then
    print_color "$RED" "Error: Go is not installed"
    exit 1
  fi
  print_color "$GREEN" "✓ Go $(go version | awk '{print $3}')"

  # Check kubectl
  if ! command -v kubectl &>/dev/null; then
    print_color "$YELLOW" "⚠ kubectl not found - some tests will be skipped"
    RUN_INTEGRATION=false
    RUN_SMOKE=false
    RUN_MINIKUBE=false
  else
    print_color "$GREEN" "✓ kubectl $(kubectl version --client --short 2>/dev/null | head -1)"
  fi

  # Check Minikube
  if [ "$RUN_MINIKUBE" = "true" ]; then
    if ! command -v minikube &>/dev/null; then
      print_color "$YELLOW" "⚠ minikube not found - minikube tests will be skipped"
      RUN_MINIKUBE=false
    else
      print_color "$GREEN" "✓ minikube $(minikube version --short)"
    fi
  fi

  # Check if cluster is available
  if [ "$RUN_INTEGRATION" = "true" ] || [ "$RUN_SMOKE" = "true" ]; then
    if ! kubectl cluster-info &>/dev/null; then
      print_color "$YELLOW" "⚠ No Kubernetes cluster available - integration tests will be skipped"
      RUN_INTEGRATION=false
      RUN_SMOKE=false
    else
      print_color "$GREEN" "✓ Kubernetes cluster is accessible"
    fi
  fi
}

run_unit_tests() {
  if [ "$RUN_UNIT" != "true" ]; then
    print_test_result "Unit Tests" "skip"
    return
  fi

  print_header "Running Unit Tests"

  # Run tests from both go directory and tests/unit directory
  local test_failed=false

  # Run tests in go directory (if any remain)
  cd "$GO_DIR"
  local test_cmd="go test ./..."
  if [ "$VERBOSE" = "true" ]; then
    test_cmd="$test_cmd -v"
  fi
  if [ "$COVERAGE" = "true" ]; then
    test_cmd="$test_cmd -cover -coverprofile=$COVERAGE_OUT.go"
  fi

  if ! $test_cmd; then
    test_failed=true
  fi

  # Run tests in tests/unit directory
  cd "$TEST_DIR/unit"
  test_cmd="go test ./..."
  if [ "$VERBOSE" = "true" ]; then
    test_cmd="$test_cmd -v"
  fi
  if [ "$COVERAGE" = "true" ]; then
    test_cmd="$test_cmd -cover -coverprofile=$COVERAGE_OUT.unit"
  fi

  if ! $test_cmd; then
    test_failed=true
  fi

  if [ "$test_failed" = "false" ]; then
    print_test_result "Unit Tests" "0"

    if [ "$COVERAGE" = "true" ]; then
      print_color "$BLUE" "Generating coverage report..."
      # Merge coverage files if both exist
      if [ -f "$COVERAGE_OUT.go" ] && [ -f "$COVERAGE_OUT.unit" ]; then
        go run golang.org/x/tools/cmd/gocovmerge@latest "$COVERAGE_OUT.go" "$COVERAGE_OUT.unit" >"$COVERAGE_OUT"
        rm "$COVERAGE_OUT.go" "$COVERAGE_OUT.unit"
      elif [ -f "$COVERAGE_OUT.go" ]; then
        mv "$COVERAGE_OUT.go" "$COVERAGE_OUT"
      elif [ -f "$COVERAGE_OUT.unit" ]; then
        mv "$COVERAGE_OUT.unit" "$COVERAGE_OUT"
      fi

      if [ -f "$COVERAGE_OUT" ]; then
        go tool cover -html="$COVERAGE_OUT" -o "$COVERAGE_HTML"
        print_color "$GREEN" "Coverage report saved to: $COVERAGE_HTML"
      fi
    fi
  else
    print_test_result "Unit Tests" "1"
  fi

  cd "$PROJECT_ROOT"
}

run_integration_tests() {
  if [ "$RUN_INTEGRATION" != "true" ]; then
    print_test_result "Integration Tests" "skip"
    return
  fi

  print_header "Running Integration Tests"

  if [ -f "$TEST_DIR/integration/integration_test.go" ]; then
    cd "$TEST_DIR/integration"

    local test_cmd="go test"
    if [ "$VERBOSE" = "true" ]; then
      test_cmd="$test_cmd -v"
    fi

    if $test_cmd; then
      print_test_result "Integration Tests" "0"
    else
      print_test_result "Integration Tests" "1"
    fi

    cd "$PROJECT_ROOT"
  else
    print_color "$YELLOW" "No integration tests found"
    print_test_result "Integration Tests" "skip"
  fi
}

run_smoke_tests() {
  if [ "$RUN_SMOKE" != "true" ]; then
    print_test_result "Smoke Tests" "skip"
    return
  fi

  print_header "Running Smoke Tests"

  # Deploy a test pod and verify basic functionality
  print_color "$BLUE" "Deploying test resources..."

  if kubectl apply -f "$TEST_DIR/fixtures/root-test-deployment.yaml" &>/dev/null; then
    sleep 5

    # Check if pods are running
    if kubectl get pods -l app=test-app --no-headers | grep -q "Running"; then
      print_color "$GREEN" "✓ Test pods are running"

      # Clean up
      kubectl delete -f "$TEST_DIR/fixtures/root-test-deployment.yaml" &>/dev/null
      print_test_result "Smoke Tests" "0"
    else
      print_color "$RED" "Test pods failed to start"
      kubectl delete -f "$TEST_DIR/fixtures/root-test-deployment.yaml" &>/dev/null
      print_test_result "Smoke Tests" "1"
    fi
  else
    print_color "$RED" "Failed to deploy test resources"
    print_test_result "Smoke Tests" "1"
  fi
}

run_minikube_tests() {
  if [ "$RUN_MINIKUBE" != "true" ]; then
    print_test_result "Minikube Tests" "skip"
    return
  fi

  print_header "Running Minikube Tests"

  if [ -f "$TEST_DIR/scripts/minikube-test.sh" ]; then
    if "$TEST_DIR/scripts/minikube-test.sh"; then
      print_test_result "Minikube Tests" "0"
    else
      print_test_result "Minikube Tests" "1"
    fi
  else
    print_color "$YELLOW" "Minikube test script not found"
    print_test_result "Minikube Tests" "skip"
  fi
}

print_summary() {
  print_header "Test Summary"

  echo ""
  print_color "$BOLD" "Total Tests Run: $TOTAL_TESTS"
  print_color "$GREEN" "✅ Passed: $PASSED_TESTS"

  if [ "$FAILED_TESTS" -gt 0 ]; then
    print_color "$RED" "❌ Failed: $FAILED_TESTS"
  fi

  if [ "$SKIPPED_TESTS" -gt 0 ]; then
    print_color "$YELLOW" "⏭️  Skipped: $SKIPPED_TESTS"
  fi

  echo ""
  if [ "$FAILED_TESTS" -eq 0 ]; then
    print_color "$GREEN$BOLD" "🎉 All tests passed successfully!"
    return 0
  else
    print_color "$RED$BOLD" "⚠️  Some tests failed. Please review the output above."
    return 1
  fi
}

show_help() {
  echo "Right-Sizer Test Runner"
  echo ""
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  -u, --unit          Run only unit tests"
  echo "  -i, --integration   Run only integration tests"
  echo "  -s, --smoke         Run only smoke tests"
  echo "  -m, --minikube      Include minikube tests"
  echo "  -a, --all           Run all tests including minikube"
  echo "  -v, --verbose       Enable verbose output"
  echo "  -c, --coverage      Generate coverage report"
  echo "  -h, --help          Show this help message"
  echo ""
  echo "Environment Variables:"
  echo "  RUN_UNIT=true|false         Enable/disable unit tests"
  echo "  RUN_INTEGRATION=true|false  Enable/disable integration tests"
  echo "  RUN_SMOKE=true|false        Enable/disable smoke tests"
  echo "  RUN_MINIKUBE=true|false     Enable/disable minikube tests"
  echo "  VERBOSE=true|false          Enable/disable verbose output"
  echo "  COVERAGE=true|false         Enable/disable coverage report"
  echo ""
  echo "Examples:"
  echo "  $0                  # Run default tests (unit, integration, smoke)"
  echo "  $0 -u               # Run only unit tests"
  echo "  $0 -a -v -c         # Run all tests with verbose output and coverage"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -u | --unit)
    RUN_UNIT=true
    RUN_INTEGRATION=false
    RUN_SMOKE=false
    RUN_MINIKUBE=false
    shift
    ;;
  -i | --integration)
    RUN_UNIT=false
    RUN_INTEGRATION=true
    RUN_SMOKE=false
    RUN_MINIKUBE=false
    shift
    ;;
  -s | --smoke)
    RUN_UNIT=false
    RUN_INTEGRATION=false
    RUN_SMOKE=true
    RUN_MINIKUBE=false
    shift
    ;;
  -m | --minikube)
    RUN_MINIKUBE=true
    shift
    ;;
  -a | --all)
    RUN_UNIT=true
    RUN_INTEGRATION=true
    RUN_SMOKE=true
    RUN_MINIKUBE=true
    shift
    ;;
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -c | --coverage)
    COVERAGE=true
    shift
    ;;
  -h | --help)
    show_help
    exit 0
    ;;
  *)
    print_color "$RED" "Unknown option: $1"
    show_help
    exit 1
    ;;
  esac
done

# Main execution
main() {
  print_color "$BOLD$CYAN" "🚀 Right-Sizer Test Suite"
  print_color "$BLUE" "$(date)"

  check_prerequisites

  # Run tests
  run_unit_tests
  run_integration_tests
  run_smoke_tests
  run_minikube_tests

  # Print summary
  print_summary
}

# Run main function
main
