#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_DIR="${ROOT_DIR}/go"

# Colors for output (only if stdout is a terminal and NO_COLOR is not set)
if [ -t 1 ] && [ -z "$NO_COLOR" ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  CYAN=''
  NC=''
fi

# Test configuration
COVERAGE="${COVERAGE:-false}"
VERBOSE="${VERBOSE:-false}"
INTEGRATION="${INTEGRATION:-false}"
BENCHMARK="${BENCHMARK:-false}"
SHORT="${SHORT:-false}"
TIMEOUT="${TIMEOUT:-10m}"
PARALLEL="${PARALLEL:-4}"

# Functions
print_header() {
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
}

print_error() {
  echo -e "${RED}✗${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -c | --coverage)
    COVERAGE=true
    shift
    ;;
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -i | --integration)
    INTEGRATION=true
    shift
    ;;
  -b | --benchmark)
    BENCHMARK=true
    shift
    ;;
  -s | --short)
    SHORT=true
    shift
    ;;
  -t | --timeout)
    TIMEOUT="$2"
    shift 2
    ;;
  -p | --parallel)
    PARALLEL="$2"
    shift 2
    ;;
  -h | --help)
    cat <<EOF
Usage: $0 [OPTIONS]

Run all tests for the right-sizer project.

OPTIONS:
  -c, --coverage       Enable coverage reporting
  -v, --verbose        Enable verbose output
  -i, --integration    Run integration tests
  -b, --benchmark      Run benchmark tests
  -s, --short          Run only short tests
  -t, --timeout TIME   Set test timeout (default: 10m)
  -p, --parallel N     Set parallel test execution (default: 4)
  -h, --help          Show this help message

EXAMPLES:
  $0                   # Run standard tests
  $0 -c               # Run tests with coverage
  $0 -v -i            # Run verbose with integration tests
  $0 -c -i -b         # Run all tests with coverage

EOF
    exit 0
    ;;
  *)
    print_error "Unknown option: $1"
    exit 1
    ;;
  esac
done

# Change to Go directory
cd "${GO_DIR}"

# Build test flags
TEST_FLAGS="-timeout=${TIMEOUT} -parallel=${PARALLEL}"

if [ "$VERBOSE" = true ]; then
  TEST_FLAGS="${TEST_FLAGS} -v"
fi

if [ "$SHORT" = true ]; then
  TEST_FLAGS="${TEST_FLAGS} -short"
fi

if [ "$BENCHMARK" = true ]; then
  TEST_FLAGS="${TEST_FLAGS} -bench=."
fi

# Clean up any previous test artifacts
print_info "Cleaning previous test artifacts..."
rm -f coverage.out coverage.html coverage.txt

# Run unit tests
print_header "Running Unit Tests"

if [ "$COVERAGE" = true ]; then
  print_info "Running tests with coverage enabled..."

  # Run tests with coverage
  go test ${TEST_FLAGS} \
    -coverprofile=coverage.out \
    -covermode=atomic \
    ./...

  TEST_RESULT=$?

  if [ $TEST_RESULT -eq 0 ]; then
    print_success "Unit tests passed!"

    # Generate coverage report
    print_info "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html

    # Calculate and display coverage percentage
    COVERAGE_PCT=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    print_success "Coverage: ${COVERAGE_PCT}"

    # Generate text coverage report
    go tool cover -func=coverage.out >coverage.txt

    print_info "Coverage reports generated:"
    print_info "  - HTML: ${GO_DIR}/coverage.html"
    print_info "  - Text: ${GO_DIR}/coverage.txt"
  else
    print_error "Unit tests failed!"
    exit $TEST_RESULT
  fi
else
  # Run tests without coverage
  go test ${TEST_FLAGS} ./...

  TEST_RESULT=$?

  if [ $TEST_RESULT -eq 0 ]; then
    print_success "Unit tests passed!"
  else
    print_error "Unit tests failed!"
    exit $TEST_RESULT
  fi
fi

# Run integration tests if requested
if [ "$INTEGRATION" = true ]; then
  print_header "Running Integration Tests"

  # Check if integration test directory exists
  if [ -d "${ROOT_DIR}/tests/integration" ]; then
    print_info "Running integration tests..."

    # Set integration test tag
    INTEGRATION_FLAGS="${TEST_FLAGS} -tags=integration"

    if [ "$COVERAGE" = true ]; then
      go test ${INTEGRATION_FLAGS} \
        -coverprofile=integration_coverage.out \
        -covermode=atomic \
        ../tests/integration/...

      INTEGRATION_RESULT=$?

      if [ $INTEGRATION_RESULT -eq 0 ]; then
        print_success "Integration tests passed!"

        # Merge coverage reports if both exist
        if [ -f "coverage.out" ] && [ -f "integration_coverage.out" ]; then
          print_info "Merging coverage reports..."
          echo "mode: atomic" >combined_coverage.out
          tail -n +2 coverage.out >>combined_coverage.out
          tail -n +2 integration_coverage.out >>combined_coverage.out
          mv combined_coverage.out coverage.out

          # Regenerate HTML report with combined coverage
          go tool cover -html=coverage.out -o coverage.html

          # Recalculate total coverage
          COVERAGE_PCT=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
          print_success "Combined coverage: ${COVERAGE_PCT}"
        fi
      else
        print_error "Integration tests failed!"
        exit $INTEGRATION_RESULT
      fi
    else
      go test ${INTEGRATION_FLAGS} ../tests/integration/...

      INTEGRATION_RESULT=$?

      if [ $INTEGRATION_RESULT -eq 0 ]; then
        print_success "Integration tests passed!"
      else
        print_error "Integration tests failed!"
        exit $INTEGRATION_RESULT
      fi
    fi
  else
    print_warning "Integration test directory not found, skipping..."
  fi
fi

# Run benchmark tests if requested
if [ "$BENCHMARK" = true ]; then
  print_header "Running Benchmark Tests"

  print_info "Running benchmarks..."
  go test -bench=. -benchmem -run=^$ ./...

  BENCH_RESULT=$?

  if [ $BENCH_RESULT -eq 0 ]; then
    print_success "Benchmarks completed!"
  else
    print_error "Benchmarks failed!"
    exit $BENCH_RESULT
  fi
fi

# Run go vet
print_header "Running Static Analysis"

print_info "Running go vet..."
go vet ./... 2>&1 | tee vet_output.txt || true

if [ -s vet_output.txt ]; then
  print_warning "go vet found issues (see above)"
else
  print_success "go vet passed!"
fi
rm -f vet_output.txt

# Check for race conditions if not in short mode
if [ "$SHORT" != true ]; then
  print_info "Running race detector..."
  go test -race -short ./...

  RACE_RESULT=$?

  if [ $RACE_RESULT -eq 0 ]; then
    print_success "No race conditions detected!"
  else
    print_error "Race conditions detected!"
    exit $RACE_RESULT
  fi
fi

# Final summary
print_header "Test Summary"

print_success "All tests completed successfully!"

if [ "$COVERAGE" = true ]; then
  print_info "Coverage report: ${GO_DIR}/coverage.html"
  print_info "View coverage: open ${GO_DIR}/coverage.html"
fi

# Return to original directory
cd "${ROOT_DIR}"

exit 0
