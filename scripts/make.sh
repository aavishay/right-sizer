#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output (only if stdout is a terminal and NO_COLOR is not set)
if [ -t 1 ] && [ -z "$NO_COLOR" ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  CYAN=''
  NC=''
fi

# Variables
BINARY_NAME="right-sizer"
IMAGE_NAME="right-sizer"
IMAGE_TAG="${IMAGE_TAG:-latest}"
GO="${GO:-go}"
DOCKER="${DOCKER:-docker}"
KUBECTL="${KUBECTL:-kubectl}"

# Build variables
GOOS=$(go env GOOS 2>/dev/null || echo "linux")
GOARCH=$(go env GOARCH 2>/dev/null || echo "amd64")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-ldflags \"-X main.Version=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}\""

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Functions
print_header() {
  echo -e "${BLUE}$1${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
}

print_error() {
  echo -e "${RED}✗${NC} $1"
  exit 1
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

# Command functions
cmd_all() {
  print_header "Building everything..."
  cmd_build
}

cmd_build() {
  print_header "Building ${BINARY_NAME}..."
  cd "${ROOT_DIR}/go"
  eval "${GO} build ${LDFLAGS} -o ../${BINARY_NAME} main.go"
  cd "${ROOT_DIR}"
  print_success "Binary built successfully: ${BINARY_NAME}"
}

cmd_clean() {
  print_header "Cleaning build artifacts..."
  cd "${ROOT_DIR}/go"
  ${GO} clean 2>/dev/null || true
  cd "${ROOT_DIR}"
  rm -f "${BINARY_NAME}"
  rm -rf bin/ dist/ go/vendor/ go/coverage.out go/coverage.html
  print_success "Clean completed"
}

cmd_test() {
  print_header "Running tests..."
  if [ -f "${ROOT_DIR}/tests/run-all-tests.sh" ]; then
    "${ROOT_DIR}/tests/run-all-tests.sh"
  else
    # Fallback to direct go test if new test runner doesn't exist
    cd "${ROOT_DIR}/go"
    ${GO} test -v ./...
    cd "${ROOT_DIR}"
  fi
  print_success "Tests completed"
}

cmd_test_coverage() {
  print_header "Running tests with coverage..."
  if [ -f "${ROOT_DIR}/tests/run-all-tests.sh" ]; then
    COVERAGE=true "${ROOT_DIR}/tests/run-all-tests.sh"
    print_success "Coverage report generated: coverage.html"
    print_info "Open coverage.html in your browser to view the report"
  else
    # Fallback to direct go test if new test runner doesn't exist
    cd "${ROOT_DIR}/go"
    ${GO} test -v -coverprofile=coverage.out ./...
    ${GO} tool cover -html=coverage.out -o coverage.html
    cd "${ROOT_DIR}"
    print_success "Coverage report generated: go/coverage.html"
    print_info "Open go/coverage.html in your browser to view the report"
  fi
}

cmd_fmt() {
  print_header "Formatting code..."
  cd "${ROOT_DIR}/go"
  ${GO} fmt ./...
  ${GO} mod tidy
  cd "${ROOT_DIR}"
  print_success "Code formatted"
}

cmd_lint() {
  print_header "Running linters..."
  if ! command -v golangci-lint &>/dev/null; then
    print_error "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"
  fi
  golangci-lint run
  print_success "Linting completed"
}

cmd_docker() {
  print_header "Building Docker image..."
  ${DOCKER} build -t "${IMAGE_NAME}:${IMAGE_TAG}" .
  print_success "Docker image built: ${IMAGE_NAME}:${IMAGE_TAG}"
}

cmd_docker_push() {
  print_header "Pushing Docker image..."
  cmd_docker
  ${DOCKER} push "${IMAGE_NAME}:${IMAGE_TAG}"
  print_success "Docker image pushed: ${IMAGE_NAME}:${IMAGE_TAG}"
}

cmd_deploy() {
  print_header "Deploying to Kubernetes..."
  if [ -d "deploy/kubernetes" ]; then
    ${KUBECTL} apply -f deploy/kubernetes/
  elif [ -d "deploy" ]; then
    ${KUBECTL} apply -f deploy/
  else
    print_error "Deployment manifests not found in deploy/ or deploy/kubernetes/"
  fi
  print_success "Deployed to Kubernetes"
}

cmd_undeploy() {
  print_header "Removing from Kubernetes..."
  if [ -d "deploy/kubernetes" ]; then
    ${KUBECTL} delete -f deploy/kubernetes/ --ignore-not-found=true
  elif [ -d "deploy" ]; then
    ${KUBECTL} delete -f deploy/ --ignore-not-found=true
  else
    print_error "Deployment manifests not found in deploy/ or deploy/kubernetes/"
  fi
  print_success "Removed from Kubernetes"
}

cmd_run() {
  print_header "Building and running ${BINARY_NAME}..."
  cmd_build
  print_info "Running ${BINARY_NAME}..."
  ./"${BINARY_NAME}"
}

cmd_install() {
  print_header "Installing ${BINARY_NAME}..."
  cmd_build
  cd "${ROOT_DIR}/go"
  ${GO} install
  cd "${ROOT_DIR}"
  print_success "${BINARY_NAME} installed to GOPATH/bin"
}

cmd_vendor() {
  print_header "Vendoring dependencies..."
  cd "${ROOT_DIR}/go"
  ${GO} mod vendor
  cd "${ROOT_DIR}"
  print_success "Dependencies vendored"
}

cmd_verify() {
  print_header "Verifying dependencies..."
  cd "${ROOT_DIR}/go"
  ${GO} mod verify
  cd "${ROOT_DIR}"
  print_success "Dependencies verified"
}

cmd_minikube_build() {
  print_header "Building Docker image in Minikube..."
  eval $(minikube docker-env)
  cmd_docker
  print_success "Docker image built in Minikube environment"
}

cmd_helm_deploy() {
  print_header "Deploying with Helm..."
  if [ ! -d "helm" ]; then
    print_error "Helm chart not found in ./helm directory"
  fi

  NAMESPACE="${NAMESPACE:-default}"
  RELEASE_NAME="${RELEASE_NAME:-right-sizer}"

  helm install "${RELEASE_NAME}" ./helm \
    --namespace "${NAMESPACE}" \
    --create-namespace \
    --set image.repository="${IMAGE_NAME}" \
    --set image.tag="${IMAGE_TAG}" \
    --set image.pullPolicy=IfNotPresent

  print_success "Deployed with Helm: ${RELEASE_NAME} in namespace ${NAMESPACE}"
}

cmd_helm_upgrade() {
  print_header "Upgrading Helm deployment..."
  if [ ! -d "helm" ]; then
    print_error "Helm chart not found in ./helm directory"
  fi

  NAMESPACE="${NAMESPACE:-default}"
  RELEASE_NAME="${RELEASE_NAME:-right-sizer}"

  helm upgrade "${RELEASE_NAME}" ./helm \
    --namespace "${NAMESPACE}" \
    --set image.repository="${IMAGE_NAME}" \
    --set image.tag="${IMAGE_TAG}" \
    --set image.pullPolicy=IfNotPresent

  print_success "Upgraded Helm deployment: ${RELEASE_NAME} in namespace ${NAMESPACE}"
}

cmd_helm_uninstall() {
  print_header "Uninstalling Helm deployment..."
  NAMESPACE="${NAMESPACE:-default}"
  RELEASE_NAME="${RELEASE_NAME:-right-sizer}"

  helm uninstall "${RELEASE_NAME}" --namespace "${NAMESPACE}"
  print_success "Uninstalled Helm deployment: ${RELEASE_NAME} from namespace ${NAMESPACE}"
}

cmd_quick_test() {
  print_header "Running quick tests..."
  cmd_fmt
  cmd_test
  cmd_build
  print_success "Quick tests completed successfully"
}

cmd_full_test() {
  print_header "Running full test suite..."
  cmd_fmt
  cmd_test
  cmd_lint
  cmd_build
  cmd_docker
  print_success "Full test suite completed successfully"
}

cmd_version() {
  print_header "Version Information"
  echo "Binary Name: ${BINARY_NAME}"
  echo "Git Commit: ${GIT_COMMIT}"
  echo "Build Time: ${BUILD_TIME}"
  echo "Go Version: $(${GO} version | awk '{print $3}')"
  echo "Platform: ${GOOS}/${GOARCH}"
}

cmd_help() {
  cat <<EOF
${BLUE}Right-Sizer Build Tool${NC}

Usage: $0 [command] [options]

${CYAN}Basic Commands:${NC}
  all              Build everything (default: build)
  build            Build the binary
  clean            Clean build artifacts
  test             Run tests
  test-coverage    Run tests with coverage report
  fmt              Format code and tidy modules
  lint             Run linters (requires golangci-lint)

${CYAN}Docker Commands:${NC}
  docker           Build Docker image
  docker-push      Build and push Docker image
  minikube-build   Build Docker image in Minikube environment

${CYAN}Deployment Commands:${NC}
  deploy           Deploy to Kubernetes (using manifests)
  undeploy         Remove from Kubernetes
  helm-deploy      Deploy using Helm chart
  helm-upgrade     Upgrade Helm deployment
  helm-uninstall   Uninstall Helm deployment

${CYAN}Development Commands:${NC}
  run              Build and run locally
  install          Install binary to GOPATH/bin
  vendor           Download dependencies to vendor/
  verify           Verify dependencies
  test-coverage    Run tests with coverage report
  test-integration Run integration tests (requires cluster)
  test-all         Run comprehensive test suite

${CYAN}Utility Commands:${NC}
  quick-test       Run format, test, and build
  full-test        Run complete test suite
  version          Show version information
  help             Show this help message

${CYAN}Environment Variables:${NC}
  IMAGE_TAG        Docker image tag (default: latest)
  IMAGE_NAME       Docker image name (default: right-sizer)
  NAMESPACE        Kubernetes namespace for Helm (default: default)
  RELEASE_NAME     Helm release name (default: right-sizer)
  GO               Go binary path (default: go)
  DOCKER           Docker binary path (default: docker)
  KUBECTL          Kubectl binary path (default: kubectl)

${CYAN}Examples:${NC}
  $0 build                    # Build the binary
  $0 test                     # Run tests
  $0 docker                   # Build Docker image
  $0 minikube-build          # Build in Minikube
  $0 helm-deploy             # Deploy with Helm
  IMAGE_TAG=v1.0.0 $0 docker # Build with custom tag

EOF
}

# Test with coverage report
cmd_test_coverage() {
  print_info "Running tests with coverage..."
  "${SCRIPT_DIR}/test.sh" -v -c
}

# Integration tests
cmd_test_integration() {
  print_info "Running integration tests..."
  "${SCRIPT_DIR}/test.sh" -v -i
}

# Run all tests
cmd_test_all() {
  print_info "Running comprehensive test suite..."
  "${SCRIPT_DIR}/test.sh" -v -c -i -b
}

# Main execution
main() {
  # If no arguments, show help
  if [ $# -eq 0 ]; then
    cmd_help
    exit 0
  fi

  # Parse command
  COMMAND=$1
  shift

  case "${COMMAND}" in
  all) cmd_all "$@" ;;
  build) cmd_build "$@" ;;
  clean) cmd_clean "$@" ;;
  test) cmd_test "$@" ;;
  test-coverage) cmd_test_coverage "$@" ;;
  fmt | format) cmd_fmt "$@" ;;
  lint) cmd_lint "$@" ;;
  docker) cmd_docker "$@" ;;
  docker-push) cmd_docker_push "$@" ;;
  minikube-build) cmd_minikube_build "$@" ;;
  deploy) cmd_deploy "$@" ;;
  undeploy) cmd_undeploy "$@" ;;
  helm-deploy) cmd_helm_deploy "$@" ;;
  helm-upgrade) cmd_helm_upgrade "$@" ;;
  helm-uninstall) cmd_helm_uninstall "$@" ;;
  run) cmd_run "$@" ;;
  install) cmd_install "$@" ;;
  vendor) cmd_vendor "$@" ;;
  verify) cmd_verify "$@" ;;
  quick-test) cmd_quick_test "$@" ;;
  full-test) cmd_full_test "$@" ;;
  test-coverage) cmd_test_coverage "$@" ;;
  test-integration) cmd_test_integration "$@" ;;
  test-all) cmd_test_all "$@" ;;
  version) cmd_version "$@" ;;
  help | --help | -h) cmd_help "$@" ;;
  *)
    print_error "Unknown command: ${COMMAND}"
    echo ""
    cmd_help
    exit 1
    ;;
  esac
}

# Run main function
main "$@"
