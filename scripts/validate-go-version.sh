#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to validate Go version consistency across all GitHub Actions workflows
# Ensures all workflows use Go 1.25

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
if [ -t 1 ] && [ -z "$NO_COLOR" ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  MAGENTA='\033[0;35m'
  BOLD='\033[1m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  CYAN=''
  MAGENTA=''
  BOLD=''
  NC=''
fi

# Expected Go version
EXPECTED_GO_VERSION="1.25"

# Counters
TOTAL_FILES=0
VALID_FILES=0
INVALID_FILES=0
WARNINGS=0

# Functions
print_header() {
  echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
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

# Check workflow file for Go version
check_workflow_file() {
  local file="$1"
  local filename=$(basename "$file")
  local has_go=false
  local has_issues=false
  local issues=""

  echo -e "\n${BOLD}Checking: ${filename}${NC}"

  # Check for GO_VERSION environment variable
  if grep -q "GO_VERSION:" "$file"; then
    has_go=true
    local env_version=$(grep "GO_VERSION:" "$file" | sed 's/.*GO_VERSION:[ ]*"//' | sed 's/".*//' | tr -d ' ')
    if [ "$env_version" != "$EXPECTED_GO_VERSION" ]; then
      has_issues=true
      issues="${issues}\n  ${RED}✗${NC} GO_VERSION is set to '$env_version' (expected: $EXPECTED_GO_VERSION)"
    else
      echo -e "  ${GREEN}✓${NC} GO_VERSION correctly set to $EXPECTED_GO_VERSION"
    fi
  fi

  # Check for go-version in setup-go action
  if grep -q "go-version:" "$file"; then
    has_go=true

    # Check direct version specifications
    local direct_versions=$(grep -E "go-version:[ ]*[\"']?[0-9]" "$file" | sed 's/.*go-version:[ ]*//' | sed 's/^"//' | sed 's/".*$//' | tr -d ' ')
    if [ -n "$direct_versions" ]; then
      while IFS= read -r version; do
        if [ -n "$version" ] && [ "$version" != "$EXPECTED_GO_VERSION" ]; then
          has_issues=true
          issues="${issues}\n  ${RED}✗${NC} Found go-version: '$version' (expected: $EXPECTED_GO_VERSION)"
        elif [ "$version" = "$EXPECTED_GO_VERSION" ]; then
          echo -e "  ${GREEN}✓${NC} Found correct go-version: $EXPECTED_GO_VERSION"
        fi
      done <<<"$direct_versions"
    fi

    # Check for matrix versions
    if grep -q "go-version:.*\[" "$file"; then
      local matrix_line=$(grep "go-version:.*\[" "$file")
      if ! echo "$matrix_line" | grep -q "\"$EXPECTED_GO_VERSION\""; then
        has_issues=true
        issues="${issues}\n  ${RED}✗${NC} Matrix doesn't include Go $EXPECTED_GO_VERSION: $matrix_line"
      else
        # Check if matrix has other versions
        local other_versions=$(echo "$matrix_line" | grep -oE '"[0-9]+\.[0-9]+"' | sed 's/"//g' | grep -v "^$EXPECTED_GO_VERSION$")
        if [ -n "$other_versions" ]; then
          print_warning "Matrix includes additional Go versions besides $EXPECTED_GO_VERSION:"
          echo "$other_versions" | while read -r ver; do
            echo -e "    - $ver"
          done
          ((WARNINGS++))
        else
          echo -e "  ${GREEN}✓${NC} Matrix correctly uses only Go $EXPECTED_GO_VERSION"
        fi
      fi
    fi

    # Check for variable references
    if grep -q 'go-version:.*\${{' "$file"; then
      echo -e "  ${CYAN}ℹ${NC} Uses variable reference for go-version (check the referenced variable)"
    fi
  fi

  if [ "$has_go" = false ]; then
    echo -e "  ${CYAN}ℹ${NC} No Go configuration found (workflow might not use Go)"
  elif [ "$has_issues" = true ]; then
    echo -e "${issues}"
    ((INVALID_FILES++))
    return 1
  else
    ((VALID_FILES++))
  fi

  return 0
}

# Main execution
main() {
  cd "${ROOT_DIR}"

  print_header "Go Version Validation for GitHub Actions Workflows"

  echo -e "\n${BOLD}Expected Go Version:${NC} ${CYAN}$EXPECTED_GO_VERSION${NC}"
  echo -e "${BOLD}Workflow Directory:${NC} ${CYAN}.github/workflows${NC}"

  # Find all workflow files
  if [ ! -d ".github/workflows" ]; then
    print_error "No .github/workflows directory found!"
    exit 1
  fi

  local workflow_files=$(find .github/workflows -name "*.yml" -o -name "*.yaml" | sort)

  if [ -z "$workflow_files" ]; then
    print_error "No workflow files found in .github/workflows!"
    exit 1
  fi

  # Check each workflow file
  for file in $workflow_files; do
    ((TOTAL_FILES++))
    check_workflow_file "$file" || true
  done

  # Print summary
  print_header "Validation Summary"

  echo -e "\n${BOLD}Results:${NC}"
  echo -e "  Total workflow files: ${CYAN}$TOTAL_FILES${NC}"
  echo -e "  Valid configurations: ${GREEN}$VALID_FILES${NC}"
  echo -e "  Invalid configurations: ${RED}$INVALID_FILES${NC}"
  echo -e "  Warnings: ${YELLOW}$WARNINGS${NC}"

  # Check go.mod for Go version as well
  if [ -f "go/go.mod" ]; then
    echo -e "\n${BOLD}Checking go.mod:${NC}"
    local go_mod_version=$(grep "^go " go/go.mod | awk '{print $2}')
    if [ -n "$go_mod_version" ]; then
      echo -e "  go.mod specifies Go ${CYAN}$go_mod_version${NC}"
      if [ "$go_mod_version" != "$EXPECTED_GO_VERSION" ]; then
        print_warning "go.mod version ($go_mod_version) differs from workflow version ($EXPECTED_GO_VERSION)"
        echo -e "  Consider updating with: ${CYAN}cd go && go mod edit -go=$EXPECTED_GO_VERSION${NC}"
      else
        print_success "go.mod version matches workflow version"
      fi
    fi
  fi

  # Final status
  echo ""
  if [ $INVALID_FILES -eq 0 ]; then
    if [ $WARNINGS -eq 0 ]; then
      print_success "All workflows use Go $EXPECTED_GO_VERSION correctly! 🎉"
    else
      print_success "All workflows have valid Go versions with $WARNINGS warning(s)"
    fi
    exit 0
  else
    print_error "Found $INVALID_FILES workflow(s) with incorrect Go version!"
    echo -e "\n${BOLD}To fix the issues:${NC}"
    echo -e "  1. Update GO_VERSION environment variable and any go-version fields to '$EXPECTED_GO_VERSION'"
    echo -e "  2. Update go-version in setup-go action to '$EXPECTED_GO_VERSION'"
    echo -e "  3. Update matrix go-version arrays to use only '$EXPECTED_GO_VERSION'"
    echo ""
    echo -e "You can use this command to update all Go versions:"
    echo -e "  ${CYAN}find .github/workflows -name '*.yml' -exec sed -i '' 's/1\\.2[0-9]/$EXPECTED_GO_VERSION/g' {} \\;${NC}"
    exit 1
  fi
}

# Show usage if help is requested
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
  cat <<EOF
${BOLD}Usage:${NC} $0 [options]

${BOLD}Description:${NC}
  Validates that all GitHub Actions workflows use Go version $EXPECTED_GO_VERSION

${BOLD}Options:${NC}
  -h, --help    Show this help message
  --no-color    Disable colored output

${BOLD}Examples:${NC}
  $0                  # Run validation
  NO_COLOR=1 $0       # Run without colors

${BOLD}Exit Codes:${NC}
  0 - All workflows have correct Go version
  1 - One or more workflows have incorrect Go version

EOF
  exit 0
fi

# Run main function
main
