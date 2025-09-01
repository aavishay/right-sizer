#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to test GitHub Actions workflows locally using act
# Requires: act (https://github.com/nektos/act)

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
ACT_IMAGE="${ACT_IMAGE:-catthehacker/ubuntu:act-latest}"
WORKFLOW_DIR="${ROOT_DIR}/.github/workflows"
ENV_FILE="${ROOT_DIR}/.env.act"
SECRETS_FILE="${ROOT_DIR}/.secrets.act"
EVENT_FILE="${ROOT_DIR}/.github/events/push.json"

# Test modes
TEST_MODE="${1:-all}"
VERBOSE="${VERBOSE:-false}"
DRY_RUN="${DRY_RUN:-false}"
PULL_IMAGES="${PULL_IMAGES:-true}"

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

print_step() {
  echo -e "${MAGENTA}▶${NC} $1"
}

# Check if act is installed
check_act() {
  if ! command -v act &>/dev/null; then
    print_error "act is not installed!"
    echo ""
    echo "Please install act from: https://github.com/nektos/act"
    echo ""
    echo "Installation methods:"
    echo "  macOS:    brew install act"
    echo "  Linux:    curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash"
    echo "  Windows:  choco install act-cli"
    echo ""
    exit 1
  fi

  local act_version=$(act --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
  print_success "act is installed (version: ${act_version})"
}

# Check Docker
check_docker() {
  if ! docker info &>/dev/null; then
    print_error "Docker is not running!"
    echo "Please start Docker Desktop or Docker daemon."
    exit 1
  fi
  print_success "Docker is running"
}

# Create necessary directories and files
setup_environment() {
  print_step "Setting up environment..."

  # Create events directory if it doesn't exist
  mkdir -p "${ROOT_DIR}/.github/events"

  # Create a push event file if it doesn't exist
  if [ ! -f "${EVENT_FILE}" ]; then
    cat >"${EVENT_FILE}" <<EOF
{
  "ref": "refs/heads/main",
  "before": "0000000000000000000000000000000000000000",
  "after": "$(git rev-parse HEAD 2>/dev/null || echo '0000000000000000000000000000000000000000')",
  "repository": {
    "name": "right-sizer",
    "full_name": "aavishay/right-sizer",
    "owner": {
      "name": "aavishay",
      "login": "aavishay"
    }
  },
  "pusher": {
    "name": "act-user",
    "email": "act@localhost"
  },
  "sender": {
    "login": "act-user"
  },
  "created": false,
  "deleted": false,
  "forced": false,
  "compare": "https://github.com/aavishay/right-sizer/compare/main",
  "commits": [],
  "head_commit": {
    "id": "$(git rev-parse HEAD 2>/dev/null || echo '0000000000000000000000000000000000000000')",
    "message": "Test commit",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "author": {
      "name": "act-user",
      "email": "act@localhost"
    }
  }
}
EOF
    print_success "Created push event file"
  fi

  # Create secrets file if it doesn't exist (with dummy values)
  if [ ! -f "${SECRETS_FILE}" ]; then
    cat >"${SECRETS_FILE}" <<EOF
# Secrets for act (add real values as needed)
GITHUB_TOKEN=ghp_dummy_token_for_local_testing
DOCKER_USERNAME=dummy_user
DOCKER_PASSWORD=dummy_password
DOCKERHUB_USERNAME=dummy_user
DOCKERHUB_TOKEN=dummy_token
CODECOV_TOKEN=dummy_codecov_token
EOF
    print_info "Created secrets file with dummy values at ${SECRETS_FILE}"
    print_warning "Edit ${SECRETS_FILE} with real values if needed for authenticated operations"
  fi
}

# Build act command with common flags
build_act_command() {
  local workflow="$1"
  local job="$2"
  local event="${3:-push}"

  local cmd="act"

  # Add event type
  cmd="${cmd} ${event}"

  # Add workflow file if specified
  if [ -n "${workflow}" ]; then
    cmd="${cmd} -W ${workflow}"
  fi

  # Add job if specified
  if [ -n "${job}" ]; then
    cmd="${cmd} -j ${job}"
  fi

  # Add common flags
  cmd="${cmd} --container-architecture linux/amd64"
  cmd="${cmd} -P ubuntu-latest=${ACT_IMAGE}"
  cmd="${cmd} --artifact-server-path /tmp/act-artifacts"

  # Add environment file if it exists
  if [ -f "${ENV_FILE}" ]; then
    cmd="${cmd} --env-file ${ENV_FILE}"
  fi

  # Add secrets file if it exists
  if [ -f "${SECRETS_FILE}" ]; then
    cmd="${cmd} --secret-file ${SECRETS_FILE}"
  fi

  # Add event file
  if [ -f "${EVENT_FILE}" ]; then
    cmd="${cmd} -e ${EVENT_FILE}"
  fi

  # Pull images if requested
  if [ "${PULL_IMAGES}" = "true" ]; then
    cmd="${cmd} --pull"
  fi

  # Add verbose flag if requested
  if [ "${VERBOSE}" = "true" ]; then
    cmd="${cmd} -v"
  fi

  # Add dry-run flag if requested
  if [ "${DRY_RUN}" = "true" ]; then
    cmd="${cmd} -n"
  fi

  echo "${cmd}"
}

# Test Docker build workflow
test_docker_build() {
  print_header "Testing Docker Build Workflow"

  local workflow="${WORKFLOW_DIR}/docker-build.yml"

  if [ ! -f "${workflow}" ]; then
    print_warning "Docker build workflow not found, skipping..."
    return
  fi

  print_step "Running docker-build workflow..."

  # Test the build job (skip multi-platform for local testing)
  local cmd=$(build_act_command "${workflow}" "build" "push")

  print_info "Command: ${cmd}"

  if ${cmd}; then
    print_success "Docker build workflow completed successfully!"
  else
    print_error "Docker build workflow failed!"
    return 1
  fi
}

# Test test workflow
test_test_workflow() {
  print_header "Testing Test Workflow"

  local workflow="${WORKFLOW_DIR}/test.yml"

  if [ ! -f "${workflow}" ]; then
    print_warning "Test workflow not found, skipping..."
    return
  fi

  print_step "Running test workflow..."

  # Test the main test job
  local cmd=$(build_act_command "${workflow}" "test" "push")

  print_info "Command: ${cmd}"

  if ${cmd}; then
    print_success "Test workflow completed successfully!"
  else
    print_error "Test workflow failed!"
    return 1
  fi
}

# Test release workflow
test_release_workflow() {
  print_header "Testing Release Workflow"

  local workflow="${WORKFLOW_DIR}/release.yml"

  if [ ! -f "${workflow}" ]; then
    print_warning "Release workflow not found, skipping..."
    return
  fi

  # Create a tag event for release workflow
  cat >"${ROOT_DIR}/.github/events/tag.json" <<EOF
{
  "ref": "refs/tags/v1.0.0",
  "ref_type": "tag",
  "repository": {
    "name": "right-sizer",
    "full_name": "aavishay/right-sizer",
    "owner": {
      "name": "aavishay",
      "login": "aavishay"
    }
  }
}
EOF

  print_step "Running release workflow (dry-run)..."

  # Test build-binaries job in dry-run mode
  local cmd="act push -W ${workflow} -j build-binaries -e ${ROOT_DIR}/.github/events/tag.json"
  cmd="${cmd} --container-architecture linux/amd64"
  cmd="${cmd} -P ubuntu-latest=${ACT_IMAGE}"
  cmd="${cmd} --env-file ${ENV_FILE}"
  cmd="${cmd} --secret-file ${SECRETS_FILE}"
  cmd="${cmd} -n" # Always dry-run for release

  print_info "Command: ${cmd}"

  if ${cmd}; then
    print_success "Release workflow validation completed successfully!"
  else
    print_error "Release workflow validation failed!"
    return 1
  fi
}

# List available workflows
list_workflows() {
  print_header "Available GitHub Actions Workflows"

  if [ -d "${WORKFLOW_DIR}" ]; then
    echo ""
    for workflow in "${WORKFLOW_DIR}"/*.yml "${WORKFLOW_DIR}"/*.yaml; do
      if [ -f "${workflow}" ]; then
        local name=$(basename "${workflow}" | sed 's/\.[^.]*$//')
        local jobs=$(grep -E '^[[:space:]]+[a-zA-Z0-9_-]+:$' "${workflow}" 2>/dev/null | head -5 | sed 's/://g' | tr '\n' ', ' | sed 's/, $//')
        echo -e "  ${CYAN}${name}${NC}"
        echo -e "    Jobs: ${jobs}"
        echo ""
      fi
    done
  else
    print_error "No workflows directory found!"
  fi
}

# Test all workflows
test_all() {
  local failed=0

  test_docker_build || ((failed++))
  test_test_workflow || ((failed++))
  test_release_workflow || ((failed++))

  echo ""
  print_header "Test Summary"

  if [ ${failed} -eq 0 ]; then
    print_success "All workflows passed!"
    return 0
  else
    print_error "${failed} workflow(s) failed!"
    return 1
  fi
}

# Show usage
usage() {
  cat <<EOF
${BOLD}Usage:${NC} $0 [command] [options]

${BOLD}Commands:${NC}
  all                Test all workflows (default)
  docker-build       Test Docker build workflow
  test              Test test workflow
  release           Test release workflow
  list              List available workflows
  help              Show this help message

${BOLD}Options:${NC}
  VERBOSE=true      Enable verbose output
  DRY_RUN=true      Perform dry run without executing
  PULL_IMAGES=false Skip pulling Docker images

${BOLD}Examples:${NC}
  $0                           # Test all workflows
  $0 docker-build             # Test only Docker build workflow
  VERBOSE=true $0 test        # Test with verbose output
  DRY_RUN=true $0             # Dry run all workflows

${BOLD}Files:${NC}
  .env.act          Environment variables for act
  .secrets.act      Secrets for act (create this with your tokens)
  .actrc            Act configuration file

${BOLD}Requirements:${NC}
  - act (https://github.com/nektos/act)
  - Docker
  - Git

EOF
}

# Main execution
main() {
  cd "${ROOT_DIR}"

  case "${TEST_MODE}" in
  all)
    check_act
    check_docker
    setup_environment
    test_all
    ;;
  docker-build | docker)
    check_act
    check_docker
    setup_environment
    test_docker_build
    ;;
  test | tests)
    check_act
    check_docker
    setup_environment
    test_test_workflow
    ;;
  release)
    check_act
    check_docker
    setup_environment
    test_release_workflow
    ;;
  list | ls)
    list_workflows
    ;;
  help | --help | -h)
    usage
    ;;
  *)
    print_error "Unknown command: ${TEST_MODE}"
    echo ""
    usage
    exit 1
    ;;
  esac
}

# Run main function
main
