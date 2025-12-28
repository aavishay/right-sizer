#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# Navigate to project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT/go"

echo "Running coverage check..."

# Run tests with coverage
go test -coverprofile=coverage.out -covermode=atomic ./... > /dev/null

# Get total coverage
# Output format: "total: (statements) 12.3%"
TOTAL_COVERAGE_LINE=$(go tool cover -func=coverage.out | grep total)
TOTAL_COVERAGE=$(echo "$TOTAL_COVERAGE_LINE" | grep -oE '[0-9]+\.[0-9]+')

echo "Total coverage: ${TOTAL_COVERAGE}%"

# Use awk for float comparison to avoid dependency on bc
if awk "BEGIN {exit !($TOTAL_COVERAGE < 33.0)}"; then
    echo -e "${RED}❌ Coverage is below 33.0% threshold!${NC}"
    exit 1
else
    echo -e "${GREEN}✅ Coverage check passed!${NC}"
    exit 0
fi
