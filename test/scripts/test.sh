#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERBOSE=false
INTEGRATION=false
COVERAGE=false
BENCHMARK=false
WATCH=false
CLEAN=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -i | --integration)
    INTEGRATION=true
    shift
    ;;
  -c | --coverage)
    COVERAGE=true
    shift
    ;;
  -b | --benchmark)
    BENCHMARK=true
    shift
    ;;
  -w | --watch)
    WATCH=true
    shift
    ;;
  --clean)
    CLEAN=true
    shift
    ;;
  -h | --help)
    cat <<EOF
Usage: $0 [OPTIONS]

Test script for the right-sizer operator

OPTIONS:
    -v, --verbose      Enable verbose output
    -i, --integration  Run integration tests (requires running cluster)
    -c, --coverage     Generate test coverage report
    -b, --benchmark    Run benchmark tests
    -w, --watch        Watch mode - re-run tests on file changes
    --clean           Clean test cache and artifacts before running
    -h, --help        Show this help message

EXAMPLES:
    $0                    # Run unit tests
    $0 -c                # Run tests with coverage
    $0 -i                # Run integration tests
    $0 -v -c -i          # Run all tests with verbose output and coverage
    $0 -w                # Watch mode for development

EOF
    exit 0
    ;;
  *)
    echo "Unknown option $1"
    exit 1
    ;;
  esac
done

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_DIR="${ROOT_DIR}/go"

# Log functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
check_go() {
  if ! command -v go &>/dev/null; then
    log_error "Go is not installed or not in PATH"
    exit 1
  fi

  GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
  log_info "Using Go version: $GO_VERSION"
}

# Check if kubectl is available (for integration tests)
check_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    log_warning "kubectl not found - integration tests will be skipped"
    return 1
  fi

  # Check if cluster is accessible
  if ! kubectl cluster-info &>/dev/null; then
    log_warning "kubectl cluster not accessible - integration tests will be skipped"
    return 1
  fi

  log_info "kubectl cluster is accessible"
  return 0
}

# Clean test artifacts
clean_artifacts() {
  log_info "Cleaning test artifacts..."

  cd "$GO_DIR"

  # Remove test binaries
  find . -name "*.test" -delete 2>/dev/null || true

  # Remove coverage files
  rm -f coverage.out coverage.html 2>/dev/null || true

  # Clean Go test cache
  go clean -testcache

  # Clean Go module cache if requested
  if [[ "$CLEAN" == "true" ]]; then
    go clean -modcache
  fi

  cd "$ROOT_DIR"
  log_success "Test artifacts cleaned"
}

# Install test dependencies
install_dependencies() {
  log_info "Installing test dependencies..."

  cd "$GO_DIR"

  # Install testify if not present
  if ! go list -m github.com/stretchr/testify &>/dev/null; then
    go get github.com/stretchr/testify/assert
    go get github.com/stretchr/testify/require
  fi

  # Install fake client for testing
  if ! go list -m sigs.k8s.io/controller-runtime/pkg/client/fake &>/dev/null; then
    go get sigs.k8s.io/controller-runtime/pkg/client/fake
  fi

  cd "$ROOT_DIR"
  log_success "Dependencies installed"
}

# Run unit tests
run_unit_tests() {
  log_info "Running unit tests..."

  cd "$GO_DIR"

  local test_args=()

  if [[ "$VERBOSE" == "true" ]]; then
    test_args+=("-v")
  fi

  if [[ "$COVERAGE" == "true" ]]; then
    test_args+=("-coverprofile=coverage.out" "-covermode=atomic")
  fi

  # Run tests for all packages except integration tests
  local packages
  packages=$(go list ./... | grep -v /test)

  if go test "${test_args[@]}" $packages; then
    cd "$ROOT_DIR"
    log_success "Unit tests passed"
  else
    cd "$ROOT_DIR"
    log_error "Unit tests failed"
    return 1
  fi
}

# Generate coverage report
generate_coverage() {
  if [[ "$COVERAGE" == "true" ]] && [[ -f "$GO_DIR/coverage.out" ]]; then
    log_info "Generating coverage report..."

    cd "$GO_DIR"

    # Generate HTML coverage report
    go tool cover -html=coverage.out -o coverage.html

    # Show coverage summary
    local coverage_percent
    coverage_percent=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}')

    cd "$ROOT_DIR"
    log_info "Total coverage: $coverage_percent"
    log_success "Coverage report generated: go/coverage.html"

    # Open coverage report if on macOS and not in CI
    if [[ "$OSTYPE" == "darwin"* ]] && [[ -z "$CI" ]]; then
      open go/coverage.html
    fi
  fi
}

# Run integration tests
run_integration_tests() {
  if [[ "$INTEGRATION" != "true" ]]; then
    return 0
  fi

  log_info "Running integration tests..."

  if ! check_kubectl; then
    log_warning "Skipping integration tests - kubectl not available"
    return 0
  fi

  cd "$GO_DIR"

  local test_args=("-v")

  # Set environment variable for integration tests
  export INTEGRATION_TESTS=true

  # Run integration tests
  if go test ./test/... "${test_args[@]}"; then
    cd "$ROOT_DIR"
    log_success "Integration tests passed"
  else
    cd "$ROOT_DIR"
    log_error "Integration tests failed"
    return 1
  fi

  unset INTEGRATION_TESTS
}

# Run benchmark tests
run_benchmarks() {
  if [[ "$BENCHMARK" != "true" ]]; then
    return 0
  fi

  log_info "Running benchmark tests..."

  cd "$GO_DIR"

  # Set environment variable for benchmarks
  export INTEGRATION_TESTS=true

  local packages
  packages=$(go list ./... | grep -E "(test|benchmark)")

  if go test -bench=. -benchmem $packages; then
    cd "$ROOT_DIR"
    log_success "Benchmarks completed"
  else
    cd "$ROOT_DIR"
    log_error "Benchmarks failed"
    return 1
  fi

  unset INTEGRATION_TESTS
}

# Watch mode for development
watch_tests() {
  if [[ "$WATCH" != "true" ]]; then
    return 0
  fi

  log_info "Starting watch mode..."
  log_info "Press Ctrl+C to stop"

  # Check if fswatch is available
  if command -v fswatch &>/dev/null; then
    # Use fswatch for file watching
    fswatch -o go --exclude=".*\\.git.*" --exclude=".*\\.test" --exclude="coverage\\.*" | while read f; do
      log_info "Files changed, re-running tests..."
      run_unit_tests || true
      log_info "Waiting for changes..."
    done
  elif command -v inotifywait &>/dev/null; then
    # Use inotifywait on Linux
    while true; do
      inotifywait -r -e modify,create,delete --exclude='.*\.(git|test).*|coverage\..*' go 2>/dev/null
      log_info "Files changed, re-running tests..."
      run_unit_tests || true
      log_info "Waiting for changes..."
    done
  else
    log_warning "No file watcher available (fswatch/inotifywait). Falling back to polling."
    local last_mod=0
    while true; do
      local current_mod
      current_mod=$(find go -name "*.go" -not -path "./go/.git/*" -not -name "*.test" -exec stat -f "%m" {} \; 2>/dev/null | sort -n | tail -1)

      if [[ "$current_mod" != "$last_mod" ]]; then
        last_mod=$current_mod
        log_info "Files changed, re-running tests..."
        run_unit_tests || true
        log_info "Waiting for changes..."
      fi

      sleep 2
    done
  fi
}

# Validate test workload deployment
validate_test_workload() {
  log_info "Validating test workload deployment..."

  if ! kubectl get deployment demo-app -n default &>/dev/null; then
    log_warning "Demo app not found. Deploying test workload..."
    kubectl apply -f examples/in-place-resize-demo.yaml || {
      log_error "Failed to deploy test workload"
      return 1
    }

    # Wait for deployment to be ready
    kubectl wait --for=condition=available --timeout=60s deployment/demo-app -n default || {
      log_error "Test workload deployment timed out"
      return 1
    }
  fi

  log_success "Test workload is ready"
}

# Run all tests
run_all_tests() {
  local start_time
  start_time=$(date +%s)

  log_info "Starting right-sizer test suite..."

  # Clean artifacts if requested
  if [[ "$CLEAN" == "true" ]]; then
    clean_artifacts
  fi

  # Install dependencies
  install_dependencies

  # Run unit tests
  run_unit_tests || exit 1

  # Generate coverage report
  generate_coverage

  # Run integration tests if requested
  if [[ "$INTEGRATION" == "true" ]]; then
    validate_test_workload || exit 1
    run_integration_tests || exit 1
  fi

  # Run benchmarks if requested
  run_benchmarks

  # Start watch mode if requested
  watch_tests

  local end_time
  end_time=$(date +%s)
  local duration=$((end_time - start_time))

  log_success "All tests completed successfully in ${duration}s"
}

# Main execution
main() {
  # Check prerequisites
  check_go

  # Run tests
  run_all_tests
}

# Handle script interruption
trap 'log_info "Test run interrupted"; exit 130' INT

# Run main function
main "$@"
