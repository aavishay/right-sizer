#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to validate GitHub Actions workflows
# Checks syntax, structure, and Go version consistency

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

# Configuration
EXPECTED_GO_VERSION="1.24"
WORKFLOW_DIR="${ROOT_DIR}/.github/workflows"
TEMP_DIR="/tmp/workflow-validation-$$"
FAILED_TESTS=0
PASSED_TESTS=0
WARNINGS=0

# Cleanup on exit
trap 'rm -rf "$TEMP_DIR"' EXIT

# Functions
print_header() {
  echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
  ((PASSED_TESTS++))
}

print_error() {
  echo -e "${RED}✗${NC} $1"
  ((FAILED_TESTS++))
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
  ((WARNINGS++))
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

print_step() {
  echo -e "${MAGENTA}▶${NC} $1"
}

# Check if required tools are installed
check_dependencies() {
  print_header "Checking Dependencies"

  local deps_ok=true

  # Check for yq (YAML processor)
  if command -v yq &>/dev/null; then
    print_success "yq is installed ($(yq --version 2>/dev/null | head -1))"
  else
    print_warning "yq is not installed (optional, but recommended for YAML validation)"
    echo "  Install with: brew install yq (macOS) or snap install yq (Linux)"
    deps_ok=false
  fi

  # Check for yamllint
  if command -v yamllint &>/dev/null; then
    print_success "yamllint is installed ($(yamllint --version 2>/dev/null))"
  else
    print_warning "yamllint is not installed (optional, but recommended)"
    echo "  Install with: brew install yamllint (macOS) or pip install yamllint"
    deps_ok=false
  fi

  # Check for actionlint
  if command -v actionlint &>/dev/null; then
    print_success "actionlint is installed ($(actionlint --version 2>/dev/null | head -1))"
  else
    print_warning "actionlint is not installed (optional, but recommended)"
    echo "  Install from: https://github.com/rhysd/actionlint"
    deps_ok=false
  fi

  # Check for git
  if command -v git &>/dev/null; then
    print_success "git is installed"
  else
    print_error "git is not installed (required)"
    deps_ok=false
  fi

  if [ "$deps_ok" = false ]; then
    echo ""
    print_info "Some optional tools are missing. The validation will continue with limited checks."
  fi

  return 0
}

# Validate YAML syntax
validate_yaml_syntax() {
  local file="$1"
  local filename=$(basename "$file")

  print_step "Validating YAML syntax: $filename"

  # Basic YAML syntax check using Python
  if python3 -c "import yaml, sys; yaml.safe_load(open('$file'))" 2>/dev/null; then
    print_success "Valid YAML syntax"
  else
    print_error "Invalid YAML syntax in $filename"
    return 1
  fi

  # Advanced validation with yamllint if available
  if command -v yamllint &>/dev/null; then
    if yamllint -d relaxed "$file" &>/dev/null; then
      print_success "Passed yamllint validation"
    else
      print_warning "yamllint found issues:"
      yamllint -d relaxed "$file" 2>&1 | head -10
    fi
  fi

  return 0
}

# Validate workflow structure
validate_workflow_structure() {
  local file="$1"
  local filename=$(basename "$file")

  print_step "Validating workflow structure: $filename"

  # Check for required fields
  if grep -q "^name:" "$file"; then
    local workflow_name=$(grep "^name:" "$file" | head -1 | sed 's/name:[ ]*//')
    print_success "Has workflow name: $workflow_name"
  else
    print_error "Missing workflow name"
  fi

  if grep -q "^on:" "$file"; then
    print_success "Has trigger events defined"
  else
    print_error "Missing trigger events (on:)"
  fi

  if grep -q "^jobs:" "$file"; then
    local job_count=$(grep -E "^[[:space:]]{2}[a-zA-Z0-9_-]+:" "$file" | wc -l)
    print_success "Has $job_count job(s) defined"
  else
    print_error "Missing jobs definition"
  fi

  return 0
}

# Validate Go version consistency
validate_go_version() {
  local file="$1"
  local filename=$(basename "$file")

  print_step "Checking Go version consistency: $filename"

  local has_go=false
  local all_versions_correct=true

  # Check GO_VERSION environment variable
  if grep -q "GO_VERSION:" "$file"; then
    has_go=true
    local env_version=$(grep "GO_VERSION:" "$file" | sed 's/.*GO_VERSION:[ ]*"\([^"]*\)".*/\1/' | head -1)
    if [ "$env_version" = "$EXPECTED_GO_VERSION" ]; then
      print_success "GO_VERSION is set to $EXPECTED_GO_VERSION"
    else
      print_error "GO_VERSION is set to '$env_version' (expected: $EXPECTED_GO_VERSION)"
      all_versions_correct=false
    fi
  fi

  # Check go-version in setup-go actions
  if grep -q "uses:.*actions/setup-go" "$file"; then
    has_go=true

    # Check direct version specifications
    if grep -q "go-version:.*\"$EXPECTED_GO_VERSION\"" "$file"; then
      print_success "Found correct go-version: $EXPECTED_GO_VERSION"
    elif grep -q "go-version:.*\[\s*\"$EXPECTED_GO_VERSION\"" "$file"; then
      print_success "Matrix includes Go $EXPECTED_GO_VERSION"
    elif grep -q 'go-version:.*\${{' "$file"; then
      print_info "Uses variable reference for go-version"
    else
      local found_version=$(grep -oE "go-version:.*\"[0-9]+\.[0-9]+\"" "$file" | head -1)
      if [ -n "$found_version" ]; then
        print_error "Incorrect Go version: $found_version"
        all_versions_correct=false
      fi
    fi
  fi

  if [ "$has_go" = false ]; then
    print_info "No Go configuration found (workflow might not use Go)"
  elif [ "$all_versions_correct" = false ]; then
    return 1
  fi

  return 0
}

# Validate with actionlint
validate_with_actionlint() {
  local file="$1"
  local filename=$(basename "$file")

  if ! command -v actionlint &>/dev/null; then
    return 0
  fi

  print_step "Running actionlint: $filename"

  if actionlint "$file" &>/dev/null; then
    print_success "Passed actionlint validation"
  else
    print_warning "actionlint found issues:"
    actionlint "$file" 2>&1 | head -10
  fi

  return 0
}

# Check for common issues
check_common_issues() {
  local file="$1"
  local filename=$(basename "$file")

  print_step "Checking for common issues: $filename"

  # Check for hardcoded secrets
  if grep -qE "(password|token|secret|key)[ ]*[:=][ ]*['\"]" "$file"; then
    print_warning "Possible hardcoded secrets detected"
  fi

  # Check for deprecated actions
  if grep -q "actions/checkout@v[12]" "$file"; then
    print_warning "Using deprecated checkout action (consider updating to v4)"
  fi

  if grep -q "actions/setup-go@v[1234]" "$file"; then
    print_warning "Using older setup-go action (consider updating to v5)"
  fi

  # Check for missing timeout-minutes
  local jobs_without_timeout=$(grep -E "^[[:space:]]{2}[a-zA-Z0-9_-]+:" "$file" | while read -r job; do
    job_name=$(echo "$job" | sed 's/://g' | tr -d ' ')
    if ! grep -A 20 "$job" "$file" | grep -q "timeout-minutes:"; then
      echo "$job_name"
    fi
  done)

  if [ -n "$jobs_without_timeout" ]; then
    print_warning "Jobs without timeout-minutes: $(echo $jobs_without_timeout | tr '\n' ', ')"
  fi

  # Check for missing permissions
  if ! grep -q "permissions:" "$file"; then
    print_info "No explicit permissions defined (using defaults)"
  fi

  return 0
}

# Validate a single workflow file
validate_workflow() {
  local file="$1"
  local filename=$(basename "$file")

  echo ""
  print_header "Validating: $filename"

  # Run all validation checks
  validate_yaml_syntax "$file" || true
  validate_workflow_structure "$file" || true
  validate_go_version "$file" || true
  validate_with_actionlint "$file" || true
  check_common_issues "$file" || true
}

# Test workflow execution with act (dry-run)
test_workflow_execution() {
  local file="$1"
  local filename=$(basename "$file")

  if ! command -v act &>/dev/null; then
    return 0
  fi

  print_step "Testing execution with act (dry-run): $filename"

  # Create a minimal event file
  mkdir -p "$TEMP_DIR"
  cat >"$TEMP_DIR/event.json" <<EOF
{
  "ref": "refs/heads/main",
  "repository": {
    "name": "right-sizer",
    "owner": {
      "login": "test"
    }
  }
}
EOF

  # Try to list jobs
  if act -W "$file" -l &>/dev/null; then
    print_success "Workflow can be parsed by act"
  else
    print_warning "act cannot parse workflow (might need specific event type)"
  fi

  return 0
}

# Main validation function
main() {
  cd "$ROOT_DIR"

  print_header "GitHub Actions Workflow Validation"

  # Check dependencies
  check_dependencies

  # Check if workflows directory exists
  if [ ! -d "$WORKFLOW_DIR" ]; then
    print_error "No .github/workflows directory found!"
    exit 1
  fi

  # Find all workflow files
  local workflow_files=$(find "$WORKFLOW_DIR" -name "*.yml" -o -name "*.yaml" | sort)

  if [ -z "$workflow_files" ]; then
    print_error "No workflow files found in .github/workflows!"
    exit 1
  fi

  local total_files=$(echo "$workflow_files" | wc -l)
  print_info "Found $total_files workflow file(s)"

  # Validate each workflow
  for file in $workflow_files; do
    validate_workflow "$file"
    test_workflow_execution "$file"
  done

  # Summary
  print_header "Validation Summary"

  echo -e "\n${BOLD}Results:${NC}"
  echo -e "  ${GREEN}Passed tests:${NC} $PASSED_TESTS"
  echo -e "  ${RED}Failed tests:${NC} $FAILED_TESTS"
  echo -e "  ${YELLOW}Warnings:${NC} $WARNINGS"

  # Check go.mod version
  if [ -f "$ROOT_DIR/go/go.mod" ]; then
    echo -e "\n${BOLD}Go Module Version:${NC}"
    local go_mod_version=$(grep "^go " "$ROOT_DIR/go/go.mod" | awk '{print $2}')
    if [ "$go_mod_version" = "$EXPECTED_GO_VERSION" ]; then
      print_success "go.mod version matches expected: $EXPECTED_GO_VERSION"
    else
      print_warning "go.mod version ($go_mod_version) differs from expected ($EXPECTED_GO_VERSION)"
    fi
  fi

  # Final status
  echo ""
  if [ $FAILED_TESTS -eq 0 ]; then
    print_success "All critical tests passed! 🎉"
    if [ $WARNINGS -gt 0 ]; then
      print_info "Found $WARNINGS warning(s) - review them for potential improvements"
    fi
    exit 0
  else
    print_error "Validation failed with $FAILED_TESTS error(s)"
    exit 1
  fi
}

# Show help
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
  cat <<EOF
${BOLD}Usage:${NC} $0 [options]

${BOLD}Description:${NC}
  Validates GitHub Actions workflows for syntax, structure, and Go version consistency

${BOLD}Options:${NC}
  -h, --help    Show this help message
  --no-color    Disable colored output

${BOLD}Checks performed:${NC}
  - YAML syntax validation
  - Workflow structure validation
  - Go version consistency (expects $EXPECTED_GO_VERSION)
  - Common issues and best practices
  - Execution testing with act (if available)

${BOLD}Optional tools for enhanced validation:${NC}
  - yq: YAML processor
  - yamllint: YAML linter
  - actionlint: GitHub Actions linter
  - act: Local GitHub Actions runner

${BOLD}Exit codes:${NC}
  0 - All validations passed
  1 - One or more validations failed

EOF
  exit 0
fi

# Run main function
main "$@"
