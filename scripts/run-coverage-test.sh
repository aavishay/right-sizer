#!/bin/bash

# Manual Coverage Test Runner
# Run this to check coverage without pre-commit hooks

set -e

# Colors
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

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║        Right-Sizer Unit Test Coverage Report (90% min)       ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

# Run the check-coverage script
if ./scripts/check-coverage.sh; then
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════╗"
    print_success "All coverage requirements met!"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo ""
    print_info "Next steps:"
    echo "  • Commit your changes: git commit -m 'your message'"
    echo "  • View full report: open build/coverage/coverage.html"
    exit 0
else
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════╗"
    print_error "Coverage requirements not met!"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo ""
    print_warning "Actions required:"
    echo "  • Add more unit tests"
    echo "  • Review the per-package breakdown above"
    echo "  • Check the HTML report: build/coverage/coverage.html"
    exit 1
fi
