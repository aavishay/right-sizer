#!/bin/bash

# Coverage Check Script for Right-Sizer
# This script runs Go tests with coverage and checks if minimum coverage is met

set -e

# Configuration
MIN_COVERAGE=80
COVERAGE_FILE="coverage.out"
HTML_COVERAGE_FILE="coverage.html"

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

# Ensure we're in the right directory (script might be called from different locations)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT/go"

print_status "Running tests with coverage..."

# Run tests with coverage
if go test -race -coverprofile="$COVERAGE_FILE" -covermode=atomic ./...; then
  print_success "Tests completed successfully"
else
  print_error "Tests failed"
  exit 1
fi

# Extract coverage percentage
if command -v gawk >/dev/null 2>&1; then
  COVERAGE=$(go tool cover -func="$COVERAGE_FILE" | grep total | gawk '{print int($3)}')
else
  COVERAGE=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print int($3)}')
fi

print_status "Coverage: ${COVERAGE}% (minimum required: ${MIN_COVERAGE}%)"

# Generate HTML coverage report
print_status "Generating HTML coverage report..."
go tool cover -html="$COVERAGE_FILE" -o "$HTML_COVERAGE_FILE"
print_success "HTML coverage report generated: $HTML_COVERAGE_FILE"

# Check if coverage meets minimum requirement
if [ "$COVERAGE" -ge "$MIN_COVERAGE" ]; then
  print_success "Coverage requirement met: ${COVERAGE}% >= ${MIN_COVERAGE}%"
  exit 0
else
  print_error "Coverage requirement not met: ${COVERAGE}% < ${MIN_COVERAGE}%"
  print_status "Please add more tests to increase coverage"
  exit 1
fi
