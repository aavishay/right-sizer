#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to run all GitHub Actions workflows using act
# Requires: act, Docker, and a valid GitHub token in .secrets.act

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
SECRETS_FILE="${ROOT_DIR}/.secrets.act"
EVENT_FILE="${ROOT_DIR}/.github/events/push.json"
LOG_DIR="${ROOT_DIR}/act-logs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Workflow selection
RUN_MODE="${1:-all}"
PARALLEL="${PARALLEL:-false}"
VERBOSE="${VERBOSE:-false}"

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

# Check prerequisites
check_prerequisites() {
  print_header "Checking Prerequisites"

  # Check act
  if ! command -v act &>/dev/null; then
    print_error "act is not installed!"
    echo "Install from: https://github.com/nektos/act"
    exit 1
  fi
  print_success "act is installed ($(act --version 2>/dev/null | head -1))"

  # Check Docker
  if ! docker info &>/dev/null; then
    print_error "Docker is not running!"
    exit 1
  fi
  print_success "Docker is running"

  # Check secrets file
  if [ ! -f "${SECRETS_FILE}" ]; then
    print_error "Secrets file not found: ${SECRETS_FILE}"
    echo "Create it with your GitHub token:"
    echo "  echo 'GITHUB_TOKEN=your_token_here' > ${SECRETS_FILE}"
    exit 1
  fi

  # Check if GitHub token is set
  if ! grep -q "GITHUB_TOKEN=" "${SECRETS_FILE}"; then
    print_error "GITHUB_TOKEN not found in ${SECRETS_FILE}"
    exit 1
  fi

  # Verify token is not a dummy value
  if grep -q "GITHUB_TOKEN=.*dummy" "${SECRETS_FILE}"; then
    print_warning "GitHub token appears to be a dummy value"
    echo "Some workflows may fail without a valid token"
  else
    print_success "GitHub token is configured"
  fi

  # Create log directory
  mkdir -p "${LOG_DIR}"
  print_success "Log directory ready: ${LOG_DIR}"
}

# Build act command
build_act_cmd() {
  local workflow="$1"
  local job="${2:-}"
  local event="${3:-push}"

  local cmd="act ${event}"
  cmd="${cmd} -W ${workflow}"

  if [ -n "${job}" ]; then
    cmd="${cmd} -j ${job}"
  fi

  cmd="${cmd} --container-architecture linux/amd64"
  cmd="${cmd} -P ubuntu-latest=${ACT_IMAGE}"
  cmd="${cmd} --secret-file ${SECRETS_FILE}"
  cmd="${cmd} --artifact-server-path /tmp/act-artifacts"

  if [ -f "${EVENT_FILE}" ]; then
    cmd="${cmd} -e ${EVENT_FILE}"
  fi

  if [ "${VERBOSE}" = "true" ]; then
    cmd="${cmd} -v"
  fi

  echo "${cmd}"
}

# Run workflow
run_workflow() {
  local workflow="$1"
  local job="${2:-}"
  local name="${3:-$(basename ${workflow} .yml)}"
  local log_file="${LOG_DIR}/${name}_${TIMESTAMP}.log"

  print_step "Running ${name}..."
  echo "Log: ${log_file}"

  local cmd=$(build_act_cmd "${workflow}" "${job}")

  if [ "${VERBOSE}" = "true" ]; then
    echo "Command: ${cmd}"
  fi

  # Run the workflow
  if ${cmd} 2>&1 | tee "${log_file}"; then
    print_success "${name} completed successfully"
    return 0
  else
    print_error "${name} failed! Check log: ${log_file}"
    return 1
  fi
}

# Run test workflow
run_test_workflow() {
  print_header "Running Test Workflow"

  local workflow="${WORKFLOW_DIR}/test.yml"
  local log_file="${LOG_DIR}/test_${TIMESTAMP}.log"

  print_info "Running unit tests with Go 1.24..."

  # Run only the test job (not all jobs)
  local cmd=$(build_act_cmd "${workflow}" "test")
  cmd="${cmd} --matrix go-version:1.24"

  echo "Log: ${log_file}"

  if ${cmd} 2>&1 | tee "${log_file}" | grep -E "Run |Success|Error|Test|Coverage|PASS|FAIL"; then
    print_success "Test workflow completed"

    # Show summary
    echo ""
    print_info "Test Summary:"
    grep -E "PASS|FAIL|coverage:" "${log_file}" | tail -10 || true
  else
    print_error "Test workflow failed"
    return 1
  fi
}

# Run Docker build workflow
run_docker_workflow() {
  print_header "Running Docker Build Workflow"

  local workflow="${WORKFLOW_DIR}/docker-build.yml"
  local log_file="${LOG_DIR}/docker-build_${TIMESTAMP}.log"

  print_info "Building Docker image (AMD64 only for local testing)..."

  # Run the build job
  local cmd=$(build_act_cmd "${workflow}" "build")

  echo "Log: ${log_file}"

  if ${cmd} 2>&1 | tee "${log_file}" | grep -E "Step |Success|Error|Building|Pushing|digest"; then
    print_success "Docker build workflow completed"
  else
    print_error "Docker build workflow failed"
    return 1
  fi
}

# Run release workflow (dry-run)
run_release_workflow() {
  print_header "Running Release Workflow (Dry Run)"

  local workflow="${WORKFLOW_DIR}/release.yml"
  local log_file="${LOG_DIR}/release_${TIMESTAMP}.log"

  print_warning "Release workflow will run in dry-run mode (no actual release)"

  # Create tag event
  cat >"${ROOT_DIR}/.github/events/tag.json" <<EOF
{
  "ref": "refs/tags/v1.0.0-test",
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

  # Run build-binaries job only
  local cmd="act push -W ${workflow} -j build-binaries"
  cmd="${cmd} -e ${ROOT_DIR}/.github/events/tag.json"
  cmd="${cmd} --container-architecture linux/amd64"
  cmd="${cmd} -P ubuntu-latest=${ACT_IMAGE}"
  cmd="${cmd} --secret-file ${SECRETS_FILE}"
  cmd="${cmd} --matrix goos:linux --matrix goarch:amd64"

  echo "Log: ${log_file}"

  if ${cmd} 2>&1 | tee "${log_file}" | grep -E "Step |Success|Error|Building|binary"; then
    print_success "Release workflow (dry-run) completed"
  else
    print_error "Release workflow (dry-run) failed"
    return 1
  fi
}

# Run Go version test workflow
run_go_version_test() {
  print_header "Running Go Version Test Workflow"

  local workflow="${WORKFLOW_DIR}/test-go-version.yml"

  if [ ! -f "${workflow}" ]; then
    print_warning "Go version test workflow not found, skipping..."
    return 0
  fi

  local log_file="${LOG_DIR}/go-version-test_${TIMESTAMP}.log"

  print_info "Testing Go 1.24 configuration..."

  # Run the direct version test
  local cmd=$(build_act_cmd "${workflow}" "test-direct-version" "workflow_dispatch")

  echo "Log: ${log_file}"

  if ${cmd} 2>&1 | tee "${log_file}" | grep -E "Go version|Success|Error|matches|mismatch"; then
    print_success "Go version test completed"

    # Extract version info
    echo ""
    print_info "Go Version Results:"
    grep -E "go version|✅|❌" "${log_file}" | tail -5 || true
  else
    print_error "Go version test failed"
    return 1
  fi
}

# Run all workflows
run_all_workflows() {
  local failed=0
  local passed=0

  # Run workflows sequentially
  run_test_workflow && ((passed++)) || ((failed++))
  run_docker_workflow && ((passed++)) || ((failed++))
  run_release_workflow && ((passed++)) || ((failed++))
  run_go_version_test && ((passed++)) || ((failed++))

  # Summary
  print_header "Workflow Execution Summary"

  echo -e "\n${BOLD}Results:${NC}"
  echo -e "  ${GREEN}Passed:${NC} $passed"
  echo -e "  ${RED}Failed:${NC} $failed"
  echo -e "  ${CYAN}Total:${NC} $((passed + failed))"

  echo -e "\n${BOLD}Logs:${NC}"
  ls -la "${LOG_DIR}"/*_${TIMESTAMP}.log 2>/dev/null || echo "No logs generated"

  if [ $failed -eq 0 ]; then
    print_success "All workflows executed successfully! 🎉"
    return 0
  else
    print_error "$failed workflow(s) failed"
    return 1
  fi
}

# Show usage
usage() {
  cat <<EOF
${BOLD}Usage:${NC} $0 [workflow] [options]

${BOLD}Workflows:${NC}
  all          Run all workflows (default)
  test         Run test workflow only
  docker       Run Docker build workflow only
  release      Run release workflow only (dry-run)
  go-version   Run Go version test only
  help         Show this help message

${BOLD}Environment Variables:${NC}
  VERBOSE=true     Enable verbose output
  PARALLEL=true    Run workflows in parallel (experimental)
  ACT_IMAGE=...    Docker image for act (default: catthehacker/ubuntu:act-latest)

${BOLD}Examples:${NC}
  $0                    # Run all workflows
  $0 test              # Run test workflow only
  VERBOSE=true $0      # Run all with verbose output

${BOLD}Requirements:${NC}
  - act (https://github.com/nektos/act)
  - Docker
  - Valid GitHub token in .secrets.act

${BOLD}Notes:${NC}
  - Logs are saved to: ${LOG_DIR}
  - Secrets file: ${SECRETS_FILE}
  - Uses container architecture: linux/amd64

EOF
}

# Main execution
main() {
  cd "${ROOT_DIR}"

  case "${RUN_MODE}" in
  all)
    check_prerequisites
    run_all_workflows
    ;;
  test)
    check_prerequisites
    run_test_workflow
    ;;
  docker)
    check_prerequisites
    run_docker_workflow
    ;;
  release)
    check_prerequisites
    run_release_workflow
    ;;
  go-version)
    check_prerequisites
    run_go_version_test
    ;;
  help | --help | -h)
    usage
    ;;
  *)
    print_error "Unknown workflow: ${RUN_MODE}"
    echo ""
    usage
    exit 1
    ;;
  esac
}

# Run main function
main
