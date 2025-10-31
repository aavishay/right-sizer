#!/bin/bash

# Pre-commit Setup Script for Right-Sizer
# Installs and configures pre-commit hooks with 90% coverage requirement

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
  echo -e "${BLUE}ℹ️  ${NC} $1"
}

print_success() {
  echo -e "${GREEN}✅ ${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}⚠️  ${NC} $1"
}

print_error() {
  echo -e "${RED}❌ ${NC} $1"
}

# Get project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

print_info "Setting up pre-commit hooks for right-sizer..."
echo ""

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    print_warning "pre-commit is not installed"
    print_info "Installing pre-commit..."
    pip install pre-commit
fi

print_success "pre-commit is installed"
echo ""

# Install pre-commit hooks
print_info "Installing pre-commit git hooks..."
pre-commit install --hook-type pre-commit
pre-commit install --hook-type commit-msg

print_success "Git hooks installed"
echo ""

# Run pre-commit on all files to validate setup
print_info "Running pre-commit on all files to validate setup..."
print_warning "Note: Some checks may fail - this is normal for initial setup"
echo ""

pre-commit run --all-files || print_warning "Some pre-commit checks failed (expected on initial run)"

echo ""
print_success "Pre-commit setup complete!"
echo ""
print_info "Coverage requirement: 90%"
print_info "Coverage check runs on every commit"
print_info ""
print_info "Manual commands:"
echo "  • Run tests with coverage:   make test-coverage"
echo "  • Run specific pre-commit check: pre-commit run go-test-coverage --all-files"
echo "  • View coverage report:      open build/coverage/coverage.html"
echo ""
