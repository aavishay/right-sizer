#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to set up GitHub secrets for the right-sizer repository
# Requires: GitHub CLI (gh) to be installed and authenticated

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
REPO="${GITHUB_REPOSITORY:-}"
DRY_RUN="${DRY_RUN:-false}"
INTERACTIVE="${INTERACTIVE:-true}"

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

  # Check if gh CLI is installed
  if ! command -v gh &>/dev/null; then
    print_error "GitHub CLI (gh) is not installed!"
    echo ""
    echo "Installation instructions:"
    echo "  macOS:    brew install gh"
    echo "  Linux:    https://github.com/cli/cli/blob/trunk/docs/install_linux.md"
    echo "  Windows:  winget install --id GitHub.cli"
    echo ""
    exit 1
  fi
  print_success "GitHub CLI is installed ($(gh version 2>/dev/null | head -1))"

  # Check if gh is authenticated
  if ! gh auth status &>/dev/null; then
    print_error "GitHub CLI is not authenticated!"
    echo ""
    echo "Run: gh auth login"
    echo ""
    exit 1
  fi
  print_success "GitHub CLI is authenticated"

  # Get repository if not set
  if [ -z "$REPO" ]; then
    # Try to get from git remote
    if git remote get-url origin &>/dev/null; then
      REPO=$(git remote get-url origin | sed 's/.*github.com[:/]\(.*\)\.git/\1/')
      print_info "Detected repository: $REPO"
    else
      print_error "Could not detect repository. Set GITHUB_REPOSITORY environment variable."
      exit 1
    fi
  fi

  # Verify repository access
  if gh repo view "$REPO" &>/dev/null; then
    print_success "Repository access confirmed: $REPO"
  else
    print_error "Cannot access repository: $REPO"
    echo "Make sure you have admin access to the repository."
    exit 1
  fi
}

# Read secret value securely
read_secret() {
  local prompt="$1"
  local var_name="$2"
  local secret_value=""

  if [ "$INTERACTIVE" = "true" ]; then
    echo -n "$prompt"
    read -s secret_value
    echo ""
  else
    # Read from environment variable
    secret_value="${!var_name}"
  fi

  echo "$secret_value"
}

# Set a GitHub secret
set_github_secret() {
  local secret_name="$1"
  local secret_value="$2"
  local required="${3:-true}"

  if [ -z "$secret_value" ] && [ "$required" = "true" ]; then
    print_warning "Skipping $secret_name (no value provided)"
    return 1
  fi

  if [ -z "$secret_value" ]; then
    return 0
  fi

  print_step "Setting secret: $secret_name"

  if [ "$DRY_RUN" = "true" ]; then
    print_info "DRY RUN - Would set secret: $secret_name"
  else
    if echo "$secret_value" | gh secret set "$secret_name" --repo="$REPO"; then
      print_success "Secret $secret_name set successfully"
    else
      print_error "Failed to set secret $secret_name"
      return 1
    fi
  fi
}

# Setup Docker Hub secrets
setup_docker_secrets() {
  print_header "Docker Hub Configuration"

  echo "Docker Hub credentials are required for pushing images."
  echo "Create a Docker Hub account at: https://hub.docker.com"
  echo ""

  local docker_username=""
  local docker_password=""

  if [ "$INTERACTIVE" = "true" ]; then
    echo -n "Docker Hub Username: "
    read docker_username
    docker_password=$(read_secret "Docker Hub Password: " "DOCKER_PASSWORD")
  else
    docker_username="${DOCKER_USERNAME:-}"
    docker_password="${DOCKER_PASSWORD:-}"
  fi

  if [ -n "$docker_username" ] && [ -n "$docker_password" ]; then
    set_github_secret "DOCKER_USERNAME" "$docker_username"
    set_github_secret "DOCKER_PASSWORD" "$docker_password"
    set_github_secret "DOCKERHUB_USERNAME" "$docker_username"
    set_github_secret "DOCKERHUB_TOKEN" "$docker_password"
  else
    print_warning "Docker Hub credentials not provided"
  fi
}

# Setup optional secrets
setup_optional_secrets() {
  print_header "Optional Services Configuration"

  echo "These services are optional but enhance the CI/CD pipeline."
  echo ""

  # Codecov
  print_step "Codecov (for code coverage reports)"
  echo "Get token from: https://codecov.io/gh/$REPO/settings"
  local codecov_token=""

  if [ "$INTERACTIVE" = "true" ]; then
    echo -n "Codecov Token (press Enter to skip): "
    read -s codecov_token
    echo ""
  else
    codecov_token="${CODECOV_TOKEN:-}"
  fi

  if [ -n "$codecov_token" ]; then
    set_github_secret "CODECOV_TOKEN" "$codecov_token" false
  else
    print_info "Skipping Codecov configuration"
  fi

  # Snyk (security scanning)
  print_step "Snyk (for security scanning)"
  echo "Get token from: https://app.snyk.io/account"
  local snyk_token=""

  if [ "$INTERACTIVE" = "true" ]; then
    echo -n "Snyk Token (press Enter to skip): "
    read -s snyk_token
    echo ""
  else
    snyk_token="${SNYK_TOKEN:-}"
  fi

  if [ -n "$snyk_token" ]; then
    set_github_secret "SNYK_TOKEN" "$snyk_token" false
  else
    print_info "Skipping Snyk configuration"
  fi
}

# List current secrets
list_secrets() {
  print_header "Current GitHub Secrets"

  if gh secret list --repo="$REPO" 2>/dev/null; then
    echo ""
    print_info "Use 'gh secret delete SECRET_NAME --repo=$REPO' to remove a secret"
  else
    print_warning "No secrets found or unable to list secrets"
  fi
}

# Validate secrets
validate_secrets() {
  print_header "Validating Secrets"

  local required_secrets=(
    "DOCKER_USERNAME"
    "DOCKER_PASSWORD"
  )

  local optional_secrets=(
    "DOCKERHUB_USERNAME"
    "DOCKERHUB_TOKEN"
    "CODECOV_TOKEN"
    "SNYK_TOKEN"
  )

  local missing_required=()
  local missing_optional=()

  # Check required secrets
  for secret in "${required_secrets[@]}"; do
    if gh secret list --repo="$REPO" 2>/dev/null | grep -q "^$secret"; then
      print_success "$secret is configured"
    else
      print_error "$secret is missing (required)"
      missing_required+=("$secret")
    fi
  done

  # Check optional secrets
  for secret in "${optional_secrets[@]}"; do
    if gh secret list --repo="$REPO" 2>/dev/null | grep -q "^$secret"; then
      print_success "$secret is configured"
    else
      print_info "$secret is not configured (optional)"
      missing_optional+=("$secret")
    fi
  done

  echo ""
  if [ ${#missing_required[@]} -eq 0 ]; then
    print_success "All required secrets are configured!"
  else
    print_error "Missing required secrets: ${missing_required[*]}"
    return 1
  fi

  if [ ${#missing_optional[@]} -gt 0 ]; then
    print_info "Optional secrets not configured: ${missing_optional[*]}"
  fi
}

# Generate workflow test command
generate_test_command() {
  print_header "Testing Your Setup"

  echo "To test your GitHub Actions locally with act:"
  echo ""
  echo "1. Create a local secrets file:"
  echo -e "   ${CYAN}cat > .secrets.act << EOF"
  echo "   DOCKER_USERNAME=your_docker_username"
  echo "   DOCKER_PASSWORD=your_docker_password"
  echo "   DOCKERHUB_USERNAME=your_docker_username"
  echo "   DOCKERHUB_TOKEN=your_docker_password"
  echo "   CODECOV_TOKEN=your_codecov_token"
  echo -e "   EOF${NC}"
  echo ""
  echo "2. Run workflows locally:"
  echo -e "   ${CYAN}act push -W .github/workflows/docker-build.yml --secret-file .secrets.act${NC}"
  echo ""
  echo "3. Push to GitHub to trigger real workflows:"
  echo -e "   ${CYAN}git push origin main${NC}"
  echo ""
  print_warning "Never commit .secrets.act file! It should be in .gitignore"
}

# Main menu
show_menu() {
  print_header "GitHub Secrets Setup for $REPO"

  echo "Choose an option:"
  echo ""
  echo "  1) Quick Setup (Docker Hub only)"
  echo "  2) Full Setup (all services)"
  echo "  3) List current secrets"
  echo "  4) Validate secrets"
  echo "  5) Show test commands"
  echo "  6) Exit"
  echo ""
  echo -n "Enter choice [1-6]: "
}

# Main execution
main() {
  cd "$ROOT_DIR"

  # Check prerequisites first
  check_prerequisites

  if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    cat <<EOF
${BOLD}Usage:${NC} $0 [options]

${BOLD}Description:${NC}
  Set up GitHub secrets for the right-sizer repository

${BOLD}Options:${NC}
  --repo OWNER/REPO    GitHub repository (default: auto-detect)
  --non-interactive    Run in non-interactive mode (use env vars)
  --dry-run            Show what would be done without making changes
  --validate           Only validate existing secrets
  --help               Show this help message

${BOLD}Environment Variables:${NC}
  GITHUB_REPOSITORY    Repository in OWNER/REPO format
  DOCKER_USERNAME      Docker Hub username
  DOCKER_PASSWORD      Docker Hub password
  CODECOV_TOKEN        Codecov token (optional)
  SNYK_TOKEN          Snyk token (optional)
  DRY_RUN             Set to 'true' for dry run
  INTERACTIVE         Set to 'false' for non-interactive mode

${BOLD}Examples:${NC}
  $0                                    # Interactive setup
  $0 --validate                         # Validate existing secrets
  $0 --repo aavishay/right-sizer       # Specify repository
  DOCKER_USERNAME=myuser $0 --non-interactive  # Non-interactive mode

${BOLD}Required Secrets:${NC}
  DOCKER_USERNAME      Docker Hub username
  DOCKER_PASSWORD      Docker Hub password/token

${BOLD}Optional Secrets:${NC}
  DOCKERHUB_USERNAME   Docker Hub username (mirror)
  DOCKERHUB_TOKEN      Docker Hub token (mirror)
  CODECOV_TOKEN        Codecov integration
  SNYK_TOKEN          Security scanning

EOF
    exit 0
  fi

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case $1 in
    --repo)
      REPO="$2"
      shift 2
      ;;
    --non-interactive)
      INTERACTIVE="false"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --validate)
      validate_secrets
      exit $?
      ;;
    *)
      shift
      ;;
    esac
  done

  if [ "$INTERACTIVE" = "false" ]; then
    # Non-interactive mode
    setup_docker_secrets
    setup_optional_secrets
    validate_secrets
  else
    # Interactive menu
    while true; do
      show_menu
      read -r choice
      case $choice in
      1)
        setup_docker_secrets
        validate_secrets
        ;;
      2)
        setup_docker_secrets
        setup_optional_secrets
        validate_secrets
        ;;
      3)
        list_secrets
        ;;
      4)
        validate_secrets
        ;;
      5)
        generate_test_command
        ;;
      6)
        print_info "Exiting..."
        exit 0
        ;;
      *)
        print_error "Invalid choice"
        ;;
      esac
      echo ""
      echo "Press Enter to continue..."
      read -r
    done
  fi
}

# Run main function
main "$@"
