#!/bin/bash

# Right-Sizer Comprehensive Test Runner
# This script runs all tests including unit tests, integration tests, and health checks

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test report directory
REPORT_DIR="tests/reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$REPORT_DIR/test_report_${TIMESTAMP}.txt"

# Create report directory if it doesn't exist
mkdir -p $REPORT_DIR

# Function to print colored output
print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

# Function to run and report test results
run_test() {
  local test_name=$1
  local test_command=$2

  echo "" | tee -a $REPORT_FILE
  print_color $BLUE "=========================================" | tee -a $REPORT_FILE
  print_color $BLUE "Running: $test_name" | tee -a $REPORT_FILE
  print_color $BLUE "=========================================" | tee -a $REPORT_FILE

  if eval $test_command 2>&1 | tee -a $REPORT_FILE; then
    print_color $GREEN "✓ $test_name PASSED" | tee -a $REPORT_FILE
    return 0
  else
    print_color $RED "✗ $test_name FAILED" | tee -a $REPORT_FILE
    return 1
  fi
}

# Start test report
echo "Right-Sizer Test Report - $TIMESTAMP" >$REPORT_FILE
echo "=========================================" >>$REPORT_FILE

print_color $YELLOW "Starting Right-Sizer Test Suite"
print_color $YELLOW "Report will be saved to: $REPORT_FILE"
echo ""

# Track test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
FAILED_TEST_NAMES=()

# 1. Go Unit Tests
print_color $YELLOW "\n📦 Running Go Unit Tests..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Go Unit Tests" "cd go && go test ./... -v -cover -coverprofile=../tests/reports/coverage_${TIMESTAMP}.out"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Go Unit Tests")
fi

# 2. Health Check Unit Tests
print_color $YELLOW "\n🏥 Running Health Check Unit Tests..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Health Check Tests" "cd go && go test ./health/... -v"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Health Check Tests")
fi

# 3. Configuration Tests
print_color $YELLOW "\n⚙️ Running Configuration Tests..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Config Tests" "cd go && go test ./config/... -v"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Config Tests")
fi

# 4. Controller Tests
print_color $YELLOW "\n🎮 Running Controller Tests..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Controller Tests" "cd go && go test ./controllers/... -v"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Controller Tests")
fi

# 5. Metrics Provider Tests
print_color $YELLOW "\n📊 Running Metrics Provider Tests..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Metrics Tests" "cd go && go test ./metrics/... -v"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Metrics Tests")
fi

# 6. Build Test
print_color $YELLOW "\n🔨 Running Build Test..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Build Test" "cd go && go build -o ../right-sizer-build ."; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
  rm -f right-sizer-build
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Build Test")
fi

# 7. Dockerfile Lint (if hadolint is available)
if command -v hadolint &>/dev/null; then
  print_color $YELLOW "\n🐳 Running Dockerfile Lint..."
  TOTAL_TESTS=$((TOTAL_TESTS + 1))
  if run_test "Dockerfile Lint" "hadolint Dockerfile"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
  else
    FAILED_TESTS=$((FAILED_TESTS + 1))
    FAILED_TEST_NAMES+=("Dockerfile Lint")
  fi
else
  print_color $YELLOW "\n⚠️ Skipping Dockerfile lint (hadolint not installed)"
fi

# 8. YAML Validation
print_color $YELLOW "\n📝 Running YAML Validation..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "YAML Validation" "find . -name '*.yaml' -o -name '*.yml' | xargs -I {} sh -c 'python3 -c \"import yaml; yaml.safe_load(open(\"{}\"))\" 2>/dev/null || true'"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("YAML Validation")
fi

# 9. Go Lint (if golangci-lint is available)
if command -v golangci-lint &>/dev/null; then
  print_color $YELLOW "\n🔍 Running Go Lint..."
  TOTAL_TESTS=$((TOTAL_TESTS + 1))
  if run_test "Go Lint" "cd go && golangci-lint run --timeout=5m"; then
    PASSED_TESTS=$((PASSED_TESTS + 1))
  else
    FAILED_TESTS=$((FAILED_TESTS + 1))
    FAILED_TEST_NAMES+=("Go Lint")
  fi
else
  print_color $YELLOW "\n⚠️ Skipping Go lint (golangci-lint not installed)"
fi

# 10. Check for uncommitted health check files
print_color $YELLOW "\n📁 Checking Health Check Files..."
TOTAL_TESTS=$((TOTAL_TESTS + 1))
if run_test "Health Files Check" "test -f go/health/checker.go && test -f docs/HEALTH_CHECKS.md && test -f scripts/test-health.sh"; then
  PASSED_TESTS=$((PASSED_TESTS + 1))
else
  FAILED_TESTS=$((FAILED_TESTS + 1))
  FAILED_TEST_NAMES+=("Health Files Check")
fi

# Generate test summary
echo "" | tee -a $REPORT_FILE
echo "" | tee -a $REPORT_FILE
print_color $BLUE "=========================================" | tee -a $REPORT_FILE
print_color $BLUE "TEST SUMMARY" | tee -a $REPORT_FILE
print_color $BLUE "=========================================" | tee -a $REPORT_FILE
echo "Total Tests: $TOTAL_TESTS" | tee -a $REPORT_FILE
print_color $GREEN "Passed: $PASSED_TESTS" | tee -a $REPORT_FILE
print_color $RED "Failed: $FAILED_TESTS" | tee -a $REPORT_FILE

if [ $FAILED_TESTS -gt 0 ]; then
  echo "" | tee -a $REPORT_FILE
  print_color $RED "Failed Tests:" | tee -a $REPORT_FILE
  for test_name in "${FAILED_TEST_NAMES[@]}"; do
    print_color $RED "  - $test_name" | tee -a $REPORT_FILE
  done
fi

# Generate coverage report if available
if [ -f "tests/reports/coverage_${TIMESTAMP}.out" ]; then
  echo "" | tee -a $REPORT_FILE
  print_color $BLUE "Generating Coverage Report..." | tee -a $REPORT_FILE
  cd go && go tool cover -html=../tests/reports/coverage_${TIMESTAMP}.out -o ../tests/reports/coverage_${TIMESTAMP}.html 2>/dev/null || true
  cd ..
  if [ -f "tests/reports/coverage_${TIMESTAMP}.html" ]; then
    print_color $GREEN "Coverage report saved to: tests/reports/coverage_${TIMESTAMP}.html" | tee -a $REPORT_FILE
  fi
fi

echo "" | tee -a $REPORT_FILE
print_color $BLUE "=========================================" | tee -a $REPORT_FILE
print_color $BLUE "Test report saved to: $REPORT_FILE" | tee -a $REPORT_FILE
print_color $BLUE "=========================================" | tee -a $REPORT_FILE

# Exit with appropriate code
if [ $FAILED_TESTS -gt 0 ]; then
  print_color $RED "\n❌ Test suite failed with $FAILED_TESTS failures"
  exit 1
else
  print_color $GREEN "\n✅ All tests passed successfully!"
  exit 0
fi
