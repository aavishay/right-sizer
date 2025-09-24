#!/bin/bash

# Right-Sizer Deployment Script
# Simple wrapper for deploying Right-Sizer to Minikube with multi-platform support

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default configuration
ACTION="${1:-deploy}"
PROFILE="right-sizer"
NAMESPACE="right-sizer"

# Functions
print_header() {
  echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BLUE} $1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
}

print_error() {
  echo -e "${RED}✗${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

check_requirements() {
  local missing=()

  command -v minikube >/dev/null 2>&1 || missing+=("minikube")
  command -v kubectl >/dev/null 2>&1 || missing+=("kubectl")
  command -v helm >/dev/null 2>&1 || missing+=("helm")
  command -v docker >/dev/null 2>&1 || missing+=("docker")

  if [ ${#missing[@]} -ne 0 ]; then
    print_error "Missing required tools: ${missing[*]}"
    echo "Please install the missing tools and try again."
    exit 1
  fi
}

show_usage() {
  cat <<EOF
Usage: $0 [COMMAND]

Commands:
    deploy    - Deploy Right-Sizer to Minikube (default)
    status    - Show deployment status
    logs      - Show Right-Sizer logs
    forward   - Port-forward to access metrics (9090)
    test      - Deploy with test workload
    upgrade   - Upgrade existing deployment
    delete    - Delete Right-Sizer deployment
    destroy   - Delete entire Minikube cluster
    help      - Show this help message

Examples:
    $0              # Deploy Right-Sizer
    $0 test         # Deploy with test workload
    $0 status       # Check deployment status
    $0 logs         # View logs
    $0 delete       # Remove deployment

Environment Variables:
    PROFILE     - Minikube profile name (default: right-sizer)
    NAMESPACE   - Kubernetes namespace (default: right-sizer)

EOF
}

# Main script
case "$ACTION" in
deploy)
  print_header "Deploying Right-Sizer to Minikube"
  check_requirements

  print_info "Using Makefile targets for deployment..."

  # Start Minikube and build image
  make mk-start
  make mk-enable-metrics
  make mk-build-image

  # Deploy with Helm
  make mk-deploy

  print_success "Deployment complete!"

  echo -e "\n${GREEN}Next steps:${NC}"
  echo "  Check status:  $0 status"
  echo "  View logs:     $0 logs"
  echo "  Port forward:  $0 forward"
  ;;

test)
  print_header "Deploying Right-Sizer with Test Workload"
  check_requirements

  # Run full test deployment
  make mk-test

  print_success "Test deployment complete!"
  ;;

status)
  print_header "Right-Sizer Deployment Status"

  echo -e "\n${CYAN}Minikube Status:${NC}"
  minikube status -p "$PROFILE" || {
    print_error "Minikube profile '$PROFILE' not found"
    exit 1
  }

  echo -e "\n${CYAN}Helm Release:${NC}"
  helm list -n "$NAMESPACE" 2>/dev/null || print_warning "No Helm releases found"

  echo -e "\n${CYAN}Pods:${NC}"
  kubectl get pods -n "$NAMESPACE" 2>/dev/null || print_warning "No pods found"

  echo -e "\n${CYAN}Service:${NC}"
  kubectl get svc -n "$NAMESPACE" 2>/dev/null || print_warning "No services found"
  ;;

logs)
  print_header "Right-Sizer Logs"
  make mk-logs
  ;;

forward)
  print_header "Port Forwarding to Right-Sizer"
  print_info "Forwarding ports 8081, 9090, 8080..."
  print_info "Press Ctrl+C to stop"
  make mk-port-forward
  ;;

upgrade)
  print_header "Upgrading Right-Sizer"
  check_requirements

  # Rebuild image and upgrade
  make mk-build-image
  helm upgrade right-sizer ./helm \
    -n "$NAMESPACE" \
    --set image.repository=right-sizer \
    --set image.tag=test \
    --set image.pullPolicy=IfNotPresent

  print_success "Upgrade complete!"
  ;;

delete)
  print_header "Deleting Right-Sizer Deployment"

  make mk-clean || true

  print_success "Right-Sizer deployment removed"
  ;;

destroy)
  print_header "Destroying Minikube Cluster"

  print_warning "This will delete the entire Minikube profile '$PROFILE'"
  read -p "Are you sure? (y/N) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    make mk-destroy
    print_success "Minikube cluster destroyed"
  else
    print_info "Cancelled"
  fi
  ;;

help | --help | -h)
  show_usage
  ;;

*)
  print_error "Unknown command: $ACTION"
  show_usage
  exit 1
  ;;
esac
