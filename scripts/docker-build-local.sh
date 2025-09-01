#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to build and test Docker images locally with build ID tagging
# Simulates GitHub Actions Docker build workflow

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
REGISTRY="${REGISTRY:-docker.io}"
IMAGE_NAME="${IMAGE_NAME:-aavishay/right-sizer}"
BUILD_NUMBER_FILE="${ROOT_DIR}/.build-number"
DOCKERFILE="${DOCKERFILE:-Dockerfile.alpine}"
FALLBACK_DOCKERFILE="${FALLBACK_DOCKERFILE:-Dockerfile}"
PLATFORM="${PLATFORM:-linux/amd64}"
PUSH="${PUSH:-false}"
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# Get or generate build number
get_build_number() {
  if [ -f "${BUILD_NUMBER_FILE}" ]; then
    BUILD_NUMBER=$(cat "${BUILD_NUMBER_FILE}")
    BUILD_NUMBER=$((BUILD_NUMBER + 1))
  else
    BUILD_NUMBER=1
  fi

  if [ "${DRY_RUN}" != "true" ]; then
    echo "${BUILD_NUMBER}" >"${BUILD_NUMBER_FILE}"
  fi

  echo "${BUILD_NUMBER}"
}

# Get git information
get_git_info() {
  GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
  GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
  GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || echo "")
  GIT_DIRTY=$(git diff --quiet 2>/dev/null || echo "-dirty")
}

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

  # Check Docker
  if ! command -v docker &>/dev/null; then
    print_error "Docker is not installed!"
    exit 1
  fi
  print_success "Docker is installed"

  # Check if Docker is running
  if ! docker info &>/dev/null; then
    print_error "Docker is not running!"
    exit 1
  fi
  print_success "Docker daemon is running"

  # Check Docker Buildx
  if docker buildx version &>/dev/null; then
    print_success "Docker Buildx is available"
    BUILDX_AVAILABLE=true
  else
    print_warning "Docker Buildx not available, using standard build"
    BUILDX_AVAILABLE=false
  fi

  # Check Dockerfile exists
  if [ -f "${ROOT_DIR}/${DOCKERFILE}" ]; then
    print_success "Dockerfile found: ${DOCKERFILE}"
  else
    print_error "Dockerfile not found: ${ROOT_DIR}/${DOCKERFILE}"
    if [ -f "${ROOT_DIR}/${FALLBACK_DOCKERFILE}" ]; then
      print_info "Using fallback: ${FALLBACK_DOCKERFILE}"
      DOCKERFILE="${FALLBACK_DOCKERFILE}"
    else
      exit 1
    fi
  fi
}

# Generate tags
generate_tags() {
  local build_num="$1"
  local tags=()

  # Build ID based tags
  tags+=("${REGISTRY}/${IMAGE_NAME}:v${build_num}")
  tags+=("${REGISTRY}/${IMAGE_NAME}:v${build_num}-${GIT_COMMIT}")

  # Date-based tag
  local date_tag=$(date +%Y%m%d)
  tags+=("${REGISTRY}/${IMAGE_NAME}:${date_tag}-v${build_num}")

  # Branch-based tags
  if [ "${GIT_BRANCH}" != "unknown" ]; then
    tags+=("${REGISTRY}/${IMAGE_NAME}:${GIT_BRANCH}-v${build_num}")
    tags+=("${REGISTRY}/${IMAGE_NAME}:${GIT_BRANCH}-${GIT_COMMIT}")

    # Latest tag for main branch
    if [ "${GIT_BRANCH}" = "main" ] || [ "${GIT_BRANCH}" = "master" ]; then
      tags+=("${REGISTRY}/${IMAGE_NAME}:latest")
    fi
  fi

  # SHA-based tag
  tags+=("${REGISTRY}/${IMAGE_NAME}:sha-${GIT_COMMIT}")

  # Version tag if on a git tag
  if [ -n "${GIT_TAG}" ]; then
    tags+=("${REGISTRY}/${IMAGE_NAME}:${GIT_TAG}")
    tags+=("${REGISTRY}/${IMAGE_NAME}:${GIT_TAG}-v${build_num}")
  fi

  # Add dirty suffix if working directory is dirty
  if [ -n "${GIT_DIRTY}" ]; then
    tags+=("${REGISTRY}/${IMAGE_NAME}:local-v${build_num}")
  fi

  echo "${tags[@]}"
}

# Build Docker image
build_image() {
  local build_num="$1"
  shift
  local tags=("$@")

  print_header "Building Docker Image"

  echo -e "${BOLD}Build Configuration:${NC}"
  echo -e "  Registry:     ${CYAN}${REGISTRY}${NC}"
  echo -e "  Image:        ${CYAN}${IMAGE_NAME}${NC}"
  echo -e "  Build Number: ${CYAN}${build_num}${NC}"
  echo -e "  Git Commit:   ${CYAN}${GIT_COMMIT}${NC}"
  echo -e "  Git Branch:   ${CYAN}${GIT_BRANCH}${NC}"
  echo -e "  Platform:     ${CYAN}${PLATFORM}${NC}"
  echo -e "  Dockerfile:   ${CYAN}${DOCKERFILE}${NC}"
  echo ""
  echo -e "${BOLD}Tags to be created:${NC}"
  for tag in "${tags[@]}"; do
    echo -e "  - ${CYAN}${tag}${NC}"
  done
  echo ""

  # Build command
  local build_cmd="docker"

  if [ "${BUILDX_AVAILABLE}" = "true" ]; then
    build_cmd="${build_cmd} buildx build"
    build_cmd="${build_cmd} --platform ${PLATFORM}"
    build_cmd="${build_cmd} --load"
  else
    build_cmd="${build_cmd} build"
  fi

  # Add tags
  for tag in "${tags[@]}"; do
    build_cmd="${build_cmd} -t ${tag}"
  done

  # Add build args
  build_cmd="${build_cmd} --build-arg VERSION=${GIT_TAG:-${GIT_COMMIT}}"
  build_cmd="${build_cmd} --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  build_cmd="${build_cmd} --build-arg BUILD_NUMBER=${build_num}"
  build_cmd="${build_cmd} --build-arg GIT_COMMIT=${GIT_COMMIT}"
  build_cmd="${build_cmd} --build-arg GIT_BRANCH=${GIT_BRANCH}"

  # Add Dockerfile and context
  build_cmd="${build_cmd} -f ${DOCKERFILE}"
  build_cmd="${build_cmd} ${ROOT_DIR}"

  if [ "${VERBOSE}" = "true" ]; then
    echo -e "${BOLD}Build Command:${NC}"
    echo "${build_cmd}"
    echo ""
  fi

  if [ "${DRY_RUN}" = "true" ]; then
    print_warning "DRY RUN - Would execute:"
    echo "${build_cmd}"
  else
    print_step "Building image..."
    if ${build_cmd}; then
      print_success "Docker image built successfully!"
    else
      print_error "Docker build failed!"
      return 1
    fi
  fi
}

# Test image
test_image() {
  local primary_tag="$1"

  print_header "Testing Docker Image"

  if [ "${DRY_RUN}" = "true" ]; then
    print_warning "DRY RUN - Skipping tests"
    return 0
  fi

  print_step "Testing image: ${primary_tag}"

  # Test 1: Image exists
  if docker image inspect "${primary_tag}" &>/dev/null; then
    print_success "Image exists"
  else
    print_error "Image not found!"
    return 1
  fi

  # Test 2: Run with --version
  print_step "Testing --version flag..."
  if docker run --rm "${primary_tag}" --version 2>/dev/null | grep -q "right-sizer"; then
    print_success "Version check passed"
  else
    print_warning "Version check failed (non-critical)"
  fi

  # Test 3: Check user
  print_step "Testing container user..."
  local user=$(docker run --rm --entrypoint sh "${primary_tag}" -c "whoami" 2>/dev/null || echo "unknown")
  if [ "${user}" = "nonroot" ] || [ "${user}" = "nobody" ]; then
    print_success "Running as non-root user: ${user}"
  else
    print_warning "Running as user: ${user}"
  fi

  # Test 4: Check binary
  print_step "Testing binary existence..."
  if docker run --rm --entrypoint sh "${primary_tag}" -c "test -x /app/right-sizer && echo 'OK'" 2>/dev/null | grep -q "OK"; then
    print_success "Binary is executable"
  else
    print_warning "Binary check failed"
  fi

  # Test 5: Get image size
  local size=$(docker image inspect "${primary_tag}" --format='{{.Size}}' | numfmt --to=iec-i --suffix=B 2>/dev/null || echo "unknown")
  print_info "Image size: ${size}"

  print_success "All tests completed!"
}

# Push images
push_images() {
  local tags=("$@")

  print_header "Pushing Docker Images"

  if [ "${PUSH}" != "true" ]; then
    print_warning "Push is disabled. Use PUSH=true to enable"
    return 0
  fi

  if [ "${DRY_RUN}" = "true" ]; then
    print_warning "DRY RUN - Would push the following tags:"
    for tag in "${tags[@]}"; do
      echo "  - ${tag}"
    done
    return 0
  fi

  # Check if logged in to registry
  if ! docker pull "${REGISTRY}/library/hello-world" &>/dev/null; then
    print_warning "Not logged in to ${REGISTRY}"
    print_info "Run: docker login ${REGISTRY}"
    return 1
  fi

  for tag in "${tags[@]}"; do
    print_step "Pushing ${tag}..."
    if docker push "${tag}"; then
      print_success "Pushed ${tag}"
    else
      print_error "Failed to push ${tag}"
      return 1
    fi
  done

  print_success "All images pushed successfully!"
}

# Summary
print_summary() {
  local build_num="$1"
  shift
  local tags=("$@")

  print_header "Build Summary"

  echo -e "${BOLD}Build Information:${NC}"
  echo -e "  Build Number:  ${CYAN}${build_num}${NC}"
  echo -e "  Git Commit:    ${CYAN}${GIT_COMMIT}${NC}"
  echo -e "  Git Branch:    ${CYAN}${GIT_BRANCH}${NC}"
  if [ -n "${GIT_TAG}" ]; then
    echo -e "  Git Tag:       ${CYAN}${GIT_TAG}${NC}"
  fi
  echo ""

  echo -e "${BOLD}Docker Commands:${NC}"
  echo ""
  echo "Pull the image:"
  echo -e "  ${CYAN}docker pull ${tags[0]}${NC}"
  echo ""
  echo "Run the container:"
  echo -e "  ${CYAN}docker run --rm ${tags[0]}${NC}"
  echo ""
  echo "Tag as latest (if on main branch):"
  echo -e "  ${CYAN}docker tag ${tags[0]} ${REGISTRY}/${IMAGE_NAME}:latest${NC}"
  echo ""

  if [ "${PUSH}" != "true" ]; then
    echo -e "${BOLD}To push images to registry:${NC}"
    echo -e "  ${CYAN}PUSH=true $0${NC}"
  fi
}

# Main execution
main() {
  cd "${ROOT_DIR}"

  # Parse arguments
  case "${1:-}" in
  --help | -h)
    cat <<EOF
${BOLD}Usage:${NC} $0 [options]

${BOLD}Description:${NC}
  Build and test Docker images locally with build ID tagging

${BOLD}Environment Variables:${NC}
  REGISTRY=...        Docker registry (default: docker.io)
  IMAGE_NAME=...      Image name (default: aavishay/right-sizer)
  DOCKERFILE=...      Dockerfile to use (default: Dockerfile.alpine)
  PLATFORM=...        Build platform (default: linux/amd64)
  PUSH=true          Push images to registry (default: false)
  DRY_RUN=true       Dry run mode (default: false)
  VERBOSE=true       Verbose output (default: false)

${BOLD}Examples:${NC}
  $0                                    # Build locally
  PUSH=true $0                          # Build and push
  DRY_RUN=true $0                       # Dry run
  PLATFORM=linux/arm64 $0               # Build for ARM64
  DOCKERFILE=Dockerfile $0              # Use different Dockerfile

${BOLD}Build Number:${NC}
  Build numbers are tracked in: ${BUILD_NUMBER_FILE}
  Delete this file to reset the counter.

EOF
    exit 0
    ;;
  esac

  # Run build process
  check_prerequisites

  BUILD_NUMBER=$(get_build_number)
  get_git_info

  # Generate tags
  TAGS=($(generate_tags "${BUILD_NUMBER}"))

  # Build image
  build_image "${BUILD_NUMBER}" "${TAGS[@]}"

  # Test image
  if [ "${DRY_RUN}" != "true" ]; then
    test_image "${TAGS[0]}"
  fi

  # Push if requested
  if [ "${PUSH}" = "true" ]; then
    push_images "${TAGS[@]}"
  fi

  # Print summary
  print_summary "${BUILD_NUMBER}" "${TAGS[@]}"

  print_success "Docker build process completed! 🐳"
}

# Run main function
main "$@"
