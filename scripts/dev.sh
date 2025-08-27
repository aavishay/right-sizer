#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# dev.sh - Development workflow helper script
# This script provides convenient commands for common development tasks

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Change to project root
cd "$PROJECT_ROOT"

# Functions
print_header() {
  echo -e "\n${BLUE}=== $1 ===${NC}\n"
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

print_task() {
  echo -e "${MAGENTA}▶${NC} $1"
}

# Check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Setup development environment
setup_env() {
  print_header "Setting up development environment"

  # Check required tools
  print_task "Checking required tools..."

  local missing_tools=()

  if ! command_exists go; then
    missing_tools+=("go")
  fi

  if ! command_exists docker; then
    missing_tools+=("docker")
  fi

  if ! command_exists kubectl; then
    missing_tools+=("kubectl")
  fi

  if ! command_exists helm; then
    missing_tools+=("helm")
  fi

  if [ ${#missing_tools[@]} -gt 0 ]; then
    print_error "Missing required tools: ${missing_tools[*]}"
    echo "Please install the missing tools and try again."
    exit 1
  fi

  print_success "All required tools are installed"

  # Install Go dependencies
  print_task "Installing Go dependencies..."
  go mod download
  go mod verify
  print_success "Go dependencies installed"

  # Install development tools
  print_task "Installing development tools..."

  # Install golangci-lint if not present
  if ! command_exists golangci-lint; then
    print_info "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    print_success "golangci-lint installed"
  else
    print_info "golangci-lint already installed"
  fi

  # Install gofumpt for better formatting
  if ! command_exists gofumpt; then
    print_info "Installing gofumpt..."
    go install mvdan.cc/gofumpt@latest
    print_success "gofumpt installed"
  else
    print_info "gofumpt already installed"
  fi

  print_success "Development environment setup complete"
}

# Watch for changes and rebuild
watch() {
  print_header "Watching for changes"

  # Check if fswatch or entr is available
  if command_exists fswatch; then
    print_info "Using fswatch to watch for changes..."
    fswatch -o . -e ".*" -i "\\.go$" | xargs -n1 -I{} sh -c 'clear; echo "Changes detected, rebuilding..."; ./make build'
  elif command_exists entr; then
    print_info "Using entr to watch for changes..."
    find . -name "*.go" | entr -c sh -c 'clear; echo "Changes detected, rebuilding..."; ./make build'
  else
    print_warning "Neither fswatch nor entr found. Install one for file watching:"
    echo "  macOS: brew install fswatch"
    echo "  Linux: apt-get install entr or yum install entr"
    echo ""
    print_info "Running manual watch loop (less efficient)..."

    local last_mod=""
    while true; do
      local current_mod=$(find . -name "*.go" -exec stat -f "%m" {} \; 2>/dev/null | sort -n | tail -1)
      if [ "$current_mod" != "$last_mod" ]; then
        clear
        echo "Changes detected, rebuilding..."
        ./make build
        last_mod="$current_mod"
      fi
      sleep 2
    done
  fi
}

# Run tests with coverage
test_coverage() {
  print_header "Running tests with coverage"

  go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
  go tool cover -html=coverage.out -o coverage.html

  # Calculate coverage percentage
  local coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')

  print_success "Test coverage: $coverage"
  print_info "Coverage report saved to coverage.html"

  # Open coverage report if possible
  if [[ "$OSTYPE" == "darwin"* ]]; then
    open coverage.html
  elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    xdg-open coverage.html 2>/dev/null || print_info "Open coverage.html in your browser"
  fi
}

# Run linters
lint() {
  print_header "Running linters"

  print_task "Running go fmt..."
  if [ -z "$(gofmt -l .)" ]; then
    print_success "Code is properly formatted"
  else
    print_warning "The following files need formatting:"
    gofmt -l .
    print_info "Run './scripts/dev.sh fmt' to fix"
  fi

  if command_exists golangci-lint; then
    print_task "Running golangci-lint..."
    golangci-lint run --timeout 5m
    print_success "Lint checks passed"
  else
    print_warning "golangci-lint not installed, skipping advanced linting"
  fi

  print_task "Running go vet..."
  go vet ./...
  print_success "go vet passed"
}

# Format code
fmt() {
  print_header "Formatting code"

  if command_exists gofumpt; then
    print_task "Running gofumpt..."
    gofumpt -l -w .
    print_success "Code formatted with gofumpt"
  else
    print_task "Running go fmt..."
    go fmt ./...
    print_success "Code formatted with go fmt"
  fi

  print_task "Running go mod tidy..."
  go mod tidy
  print_success "go.mod tidied"
}

# Quick check before committing
precommit() {
  print_header "Running pre-commit checks"

  # Format code
  fmt

  # Run tests
  print_task "Running tests..."
  go test ./...
  print_success "Tests passed"

  # Run linters
  lint

  # Build binary
  print_task "Building binary..."
  ./make build
  print_success "Build successful"

  print_success "All pre-commit checks passed!"
}

# Start local development cluster
start_cluster() {
  print_header "Starting local development cluster"

  if ! command_exists minikube; then
    print_error "Minikube not installed"
    exit 1
  fi

  # Check if minikube is running
  if minikube status &>/dev/null; then
    print_info "Minikube is already running"
  else
    print_task "Starting Minikube..."
    minikube start --kubernetes-version=v1.33.1 --memory=4096 --cpus=2
    print_success "Minikube started"
  fi

  # Enable metrics-server
  print_task "Enabling metrics-server..."
  minikube addons enable metrics-server
  print_success "metrics-server enabled"

  # Set docker env
  print_task "Configuring Docker environment..."
  eval $(minikube docker-env)
  print_success "Docker environment configured"
  print_info "Run 'eval \$(minikube docker-env)' in your shell to use Minikube's Docker"

  # Build and deploy
  print_task "Building Docker image..."
  ./make minikube-build

  print_task "Deploying to Minikube..."
  ./make helm-deploy

  print_success "Development cluster ready!"
  print_info "Run 'kubectl get pods' to see running pods"
}

# Stop local development cluster
stop_cluster() {
  print_header "Stopping local development cluster"

  if command_exists minikube; then
    print_task "Stopping Minikube..."
    minikube stop
    print_success "Minikube stopped"
  else
    print_info "Minikube not installed"
  fi
}

# Clean everything
clean_all() {
  print_header "Cleaning everything"

  print_task "Cleaning build artifacts..."
  ./make clean

  print_task "Removing coverage files..."
  rm -f coverage.out coverage.html

  print_task "Removing vendor directory..."
  rm -rf vendor/

  print_task "Cleaning go cache..."
  go clean -cache -testcache -modcache

  print_success "Everything cleaned"
}

# Run integration tests
integration_test() {
  print_header "Running integration tests"

  # Ensure cluster is running
  if ! minikube status &>/dev/null; then
    print_error "Minikube not running. Run './scripts/dev.sh start-cluster' first"
    exit 1
  fi

  # Run test scripts
  print_task "Running Helm deployment test..."
  ./scripts/test-helm-deployment.sh

  print_task "Running configuration tests..."
  ./test/scripts/quick-test-config.sh

  print_success "Integration tests completed"
}

# Show development status
status() {
  print_header "Development Environment Status"

  # Go version
  if command_exists go; then
    echo -e "${CYAN}Go:${NC} $(go version | awk '{print $3}')"
  else
    echo -e "${CYAN}Go:${NC} ${RED}Not installed${NC}"
  fi

  # Docker status
  if command_exists docker; then
    if docker version &>/dev/null; then
      echo -e "${CYAN}Docker:${NC} ${GREEN}Running${NC} ($(docker version --format '{{.Server.Version}}'))"
    else
      echo -e "${CYAN}Docker:${NC} ${YELLOW}Installed but not running${NC}"
    fi
  else
    echo -e "${CYAN}Docker:${NC} ${RED}Not installed${NC}"
  fi

  # Kubernetes status
  if command_exists kubectl; then
    local k8s_version=$(kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion' 2>/dev/null || echo "unknown")
    echo -e "${CYAN}kubectl:${NC} $k8s_version"
  else
    echo -e "${CYAN}kubectl:${NC} ${RED}Not installed${NC}"
  fi

  # Minikube status
  if command_exists minikube; then
    if minikube status &>/dev/null; then
      echo -e "${CYAN}Minikube:${NC} ${GREEN}Running${NC} (K8s $(kubectl version -o json 2>/dev/null | jq -r '.serverVersion.gitVersion' 2>/dev/null || echo "unknown"))"
    else
      echo -e "${CYAN}Minikube:${NC} ${YELLOW}Stopped${NC}"
    fi
  else
    echo -e "${CYAN}Minikube:${NC} ${RED}Not installed${NC}"
  fi

  # Helm status
  if command_exists helm; then
    echo -e "${CYAN}Helm:${NC} $(helm version --short 2>/dev/null | cut -d: -f2 | cut -d+ -f1)"
  else
    echo -e "${CYAN}Helm:${NC} ${RED}Not installed${NC}"
  fi

  # Project status
  echo ""
  echo -e "${CYAN}Project:${NC}"
  echo "  Git branch: $(git branch --show-current 2>/dev/null || echo 'not in git repo')"
  echo "  Git status: $(git status --porcelain 2>/dev/null | wc -l | xargs) uncommitted changes"

  # Binary status
  if [ -f "right-sizer" ]; then
    echo "  Binary: ${GREEN}Built${NC} ($(ls -lh right-sizer | awk '{print $5}'))"
  else
    echo "  Binary: ${YELLOW}Not built${NC}"
  fi
}

# Show help
help() {
  cat <<EOF
${BLUE}Right-Sizer Development Helper${NC}

Usage: $0 <command>

${CYAN}Environment Commands:${NC}
  setup           Setup development environment and install tools
  status          Show development environment status

${CYAN}Development Commands:${NC}
  watch           Watch for changes and auto-rebuild
  fmt             Format code
  lint            Run linters
  test-coverage   Run tests with coverage report
  precommit       Run all checks before committing

${CYAN}Cluster Commands:${NC}
  start-cluster   Start local Minikube cluster and deploy
  stop-cluster    Stop local Minikube cluster
  integration     Run integration tests

${CYAN}Utility Commands:${NC}
  clean           Clean all build artifacts and caches
  help            Show this help message

${CYAN}Examples:${NC}
  $0 setup           # Setup development environment
  $0 status          # Check environment status
  $0 watch           # Auto-rebuild on changes
  $0 precommit       # Run checks before git commit
  $0 start-cluster   # Start local testing cluster

${CYAN}Tips:${NC}
  • Run 'setup' first to install all development tools
  • Use 'precommit' before committing code
  • Use 'watch' for continuous development
  • Use 'start-cluster' for local testing

EOF
}

# Main command handler
main() {
  if [ $# -eq 0 ]; then
    help
    exit 0
  fi

  case "$1" in
  setup | setup-env)
    setup_env
    ;;
  watch)
    watch
    ;;
  test | test-coverage)
    test_coverage
    ;;
  lint)
    lint
    ;;
  fmt | format)
    fmt
    ;;
  precommit | pre-commit)
    precommit
    ;;
  start | start-cluster)
    start_cluster
    ;;
  stop | stop-cluster)
    stop_cluster
    ;;
  clean | clean-all)
    clean_all
    ;;
  integration | integration-test)
    integration_test
    ;;
  status)
    status
    ;;
  help | --help | -h)
    help
    ;;
  *)
    print_error "Unknown command: $1"
    echo ""
    help
    exit 1
    ;;
  esac
}

# Run main function with all arguments
main "$@"
