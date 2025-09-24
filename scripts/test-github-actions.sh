#!/bin/bash

# GitHub Actions Testing Script for Right-Sizer
# This script helps test GitHub Actions workflows locally using act

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
  printf "${1}${2}${NC}\n"
}

print_header() {
  echo "================================"
  print_color $BLUE "$1"
  echo "================================"
}

print_success() {
  print_color $GREEN "✅ $1"
}

print_warning() {
  print_color $YELLOW "⚠️  $1"
}

print_error() {
  print_color $RED "❌ $1"
}

# Check if act is installed
check_act() {
  if ! command -v act &>/dev/null; then
    print_error "act is not installed. Please install it first:"
    echo "  On macOS: brew install act"
    echo "  On Linux: curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash"
    exit 1
  fi
  print_success "act is installed: $(act --version)"
}

# Check if Docker is running
check_docker() {
  if ! docker info &>/dev/null; then
    print_error "Docker is not running. Please start Docker first."
    exit 1
  fi
  print_success "Docker is running"
}

# List all available workflows and jobs
list_workflows() {
  print_header "Available Workflows and Jobs"
  act --list
}

# Test individual workflows
test_helm_lint() {
  print_header "Testing Helm Lint Workflow"
  print_warning "This will pull Docker images and may take some time..."

  if act -j helm-lint -W .github/workflows/helm.yml --dryrun; then
    print_success "Helm lint workflow dry-run passed"

    read -p "Run actual helm lint test? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      if act -j helm-lint -W .github/workflows/helm.yml; then
        print_success "Helm lint workflow test passed"
      else
        print_error "Helm lint workflow test failed"
        return 1
      fi
    fi
  else
    print_error "Helm lint workflow dry-run failed"
    return 1
  fi
}

test_docker_build() {
  print_header "Testing Docker Build Workflow (Dry Run Only)"
  print_warning "Docker build test will only run in dry-run mode to avoid pushing to registry"

  if act -j docker-build-and-push -W .github/workflows/docker.yml --dryrun; then
    print_success "Docker build workflow dry-run passed"
  else
    print_error "Docker build workflow dry-run failed"
    return 1
  fi
}

test_go_build() {
  print_header "Testing Go Build from Release Workflow"
  print_warning "Testing only the build-binaries job from release workflow"

  if act -j build-binaries -W .github/workflows/release.yml --dryrun; then
    print_success "Go build workflow dry-run passed"

    read -p "Run actual Go build test? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      if act -j build-binaries -W .github/workflows/release.yml; then
        print_success "Go build workflow test passed"
      else
        print_error "Go build workflow test failed"
        return 1
      fi
    fi
  else
    print_error "Go build workflow dry-run failed"
    return 1
  fi
}

# Run local validation (non-act tests)
run_local_validation() {
  print_header "Running Local Validation Tests"

  # Check if Helm chart lints locally
  print_color $BLUE "Testing Helm chart lint locally..."
  if helm lint helm/ && helm template test helm/; then
    print_success "Helm chart validation passed"
  else
    print_error "Helm chart validation failed"
    return 1
  fi

  # Check Go build
  print_color $BLUE "Testing Go build locally..."
  if (cd go && go build -o ../right-sizer-test main.go); then
    print_success "Go build passed"
    rm -f right-sizer-test
  else
    print_error "Go build failed"
    return 1
  fi

  # Check Go tests
  print_color $BLUE "Running Go tests..."
  if (cd go && go test ./...); then
    print_success "Go tests passed"
  else
    print_error "Go tests failed"
    return 1
  fi

  # Check VERSION file exists
  if [[ -f VERSION ]]; then
    VERSION=$(cat VERSION)
    print_success "VERSION file found: $VERSION"
  else
    print_error "VERSION file not found"
    return 1
  fi
}

# Setup act configuration
setup_act_config() {
  print_header "Setting up act Configuration"

  if [[ -f .actrc ]]; then
    print_success ".actrc configuration file already exists"
  else
    print_warning "Creating .actrc configuration file..."
    cat >.actrc <<'EOF'
--container-architecture linux/amd64
--platform ubuntu-latest=catthehacker/ubuntu:act-latest
--artifact-server-path /tmp/artifacts
--verbose
EOF
    print_success ".actrc created"
  fi

  if [[ -f .env.act ]]; then
    print_success ".env.act environment file already exists"
  else
    print_warning "Creating .env.act environment file..."
    cat >.env.act <<'EOF'
# Test environment variables for act
DOCKER_USERNAME=test-user
DOCKER_PASSWORD=test-password
GITHUB_TOKEN=ghp_test_token_placeholder
REGISTRY=docker.io
IMAGE_NAME=aavishay/right-sizer
GO_VERSION=1.23
ACT_TEST=true
CI=true
GITHUB_ACTIONS=true
EOF
    print_success ".env.act created"
  fi
}

# Show usage
show_usage() {
  echo "Usage: $0 [command]"
  echo ""
  echo "Commands:"
  echo "  setup       - Setup act configuration files"
  echo "  list        - List all available workflows"
  echo "  local       - Run local validation tests (no Docker)"
  echo "  helm        - Test Helm lint workflow"
  echo "  docker      - Test Docker build workflow (dry-run only)"
  echo "  go          - Test Go build from release workflow"
  echo "  all         - Run all tests (interactive)"
  echo "  help        - Show this help message"
  echo ""
  echo "Examples:"
  echo "  $0 setup     # Setup configuration first"
  echo "  $0 local     # Run quick local tests"
  echo "  $0 helm      # Test Helm workflow with act"
  echo "  $0 all       # Run comprehensive tests"
}

# Main execution
main() {
  case "${1:-help}" in
  "setup")
    check_act
    check_docker
    setup_act_config
    ;;
  "list")
    check_act
    list_workflows
    ;;
  "local")
    run_local_validation
    ;;
  "helm")
    check_act
    check_docker
    test_helm_lint
    ;;
  "docker")
    check_act
    check_docker
    test_docker_build
    ;;
  "go")
    check_act
    check_docker
    test_go_build
    ;;
  "all")
    print_header "Comprehensive GitHub Actions Testing"
    check_act
    check_docker
    setup_act_config

    print_color $BLUE "Running all tests..."

    if run_local_validation; then
      print_success "Local validation passed"
    else
      print_error "Local validation failed - stopping here"
      exit 1
    fi

    read -p "Continue with act-based tests? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      test_helm_lint
      test_docker_build
      test_go_build

      print_header "Test Summary"
      print_success "All tests completed! Check output above for any failures."
    fi
    ;;
  "help" | *)
    show_usage
    ;;
  esac
}

# Run main function with all arguments
main "$@"
