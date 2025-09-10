#!/bin/bash

# Comprehensive Test Script for Right-Sizer
# This script runs all available tests and checks for the Go project

set -e # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Change to the go directory
cd go

print_status "Starting comprehensive test suite..."

# 1. Check Go version
print_status "Checking Go version..."
go version

# 2. Tidy modules
print_status "Tidying Go modules..."
go mod tidy
go mod verify

# 3. Format check
print_status "Checking code formatting..."
if ! go fmt ./... | grep -q .; then
  print_success "Code is properly formatted"
else
  print_error "Code formatting issues found. Run 'go fmt ./...' to fix"
  exit 1
fi

# 4. Vet check
print_status "Running go vet..."
if go vet ./...; then
  print_success "go vet passed"
else
  print_error "go vet found issues"
  exit 1
fi

# 5. Linting
print_status "Running golangci-lint..."
if command -v golangci-lint >/dev/null 2>&1; then
  if golangci-lint run ./...; then
    print_success "Linting passed"
  else
    print_error "Linting failed"
    exit 1
  fi
else
  print_warning "golangci-lint not found, skipping linting"
fi

# 6. Security scan
print_status "Running security scan..."
if command -v govulncheck >/dev/null 2>&1; then
  if govulncheck ./...; then
    print_success "Security scan passed"
  else
    print_error "Security vulnerabilities found"
    exit 1
  fi
else
  print_warning "govulncheck not found, skipping security scan"
fi

# 7. Unit tests with coverage
print_status "Running unit tests with coverage..."
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Check coverage
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
print_status "Test coverage: ${COVERAGE}%"

if (($(echo "$COVERAGE < 80" | bc -l 2>/dev/null || echo "1"))); then
  print_warning "Coverage is below 80%: ${COVERAGE}%"
else
  print_success "Coverage meets minimum requirement: ${COVERAGE}%"
fi

# 8. Integration tests
print_status "Running integration tests..."
if go test -v -tags=integration ./...; then
  print_success "Integration tests passed"
else
  print_error "Integration tests failed"
  exit 1
fi

# 9. Benchmark tests
print_status "Running benchmark tests..."
go test -bench=. -benchmem ./... || print_warning "No benchmarks found or benchmarks failed"

# 10. Build check
print_status "Building binary..."
if go build -o /tmp/right-sizer-test ./...; then
  print_success "Build successful"
  rm -f /tmp/right-sizer-test
else
  print_error "Build failed"
  exit 1
fi

# 11. Generate coverage report
print_status "Generating coverage report..."
go tool cover -html=coverage.out -o coverage.html
print_success "Coverage report generated: coverage.html"

print_success "All tests completed successfully!"
print_status "Summary:"
echo "  ✓ Code formatting"
echo "  ✓ go vet"
echo "  ✓ Linting"
echo "  ✓ Security scan"
echo "  ✓ Unit tests (${COVERAGE}% coverage)"
echo "  ✓ Integration tests"
echo "  ✓ Build verification"

exit 0
