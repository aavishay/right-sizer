#!/bin/bash
# Version management script for Right-Sizer
# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VERSION_FILE="VERSION"
HELM_CHART_FILE="helm/Chart.yaml"
DOCKERFILE="Dockerfile"

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

# Function to display usage
usage() {
  cat <<EOF
Usage: $0 [COMMAND] [OPTIONS]

Commands:
    current         Show current version
    bump            Bump version (major|minor|patch|prerelease)
    set VERSION     Set specific version
    tag             Create git tag for current version
    release         Prepare release (bump, update files, create tag)
    check           Verify version consistency across files

Options:
    -h, --help      Show this help message
    -d, --dry-run   Show what would be done without making changes
    -p, --push      Push git tag after creation
    -f, --force     Force action (e.g., overwrite existing tag)

Examples:
    $0 current                    # Show current version
    $0 bump patch                 # Bump patch version (0.1.0 -> 0.1.1)
    $0 bump minor                 # Bump minor version (0.1.0 -> 0.2.0)
    $0 bump major                 # Bump major version (0.1.0 -> 1.0.0)
    $0 bump prerelease alpha      # Create prerelease (0.1.0 -> 0.1.1-alpha.1)
    $0 set 1.2.3                  # Set specific version
    $0 tag --push                 # Create and push git tag
    $0 release patch --push       # Full release cycle

EOF
  exit 0
}

# Function to read current version
get_current_version() {
  if [ -f "$VERSION_FILE" ]; then
    cat "$VERSION_FILE"
  else
    echo "0.0.0"
  fi
}

# Function to validate version format
validate_version() {
  local version=$1
  if ! echo "$version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[0-9]+)?)?$'; then
    echo -e "${RED}Error: Invalid version format: $version${NC}"
    echo "Version must be in format: MAJOR.MINOR.PATCH[-PRERELEASE]"
    exit 1
  fi
}

# Function to bump version
bump_version() {
  local current_version=$(get_current_version)
  local bump_type=$1
  local prerelease_type=${2:-}

  # Remove any prerelease suffix for version parsing
  local base_version=$(echo "$current_version" | sed 's/-.*$//')

  IFS='.' read -r major minor patch <<<"$base_version"

  case "$bump_type" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    new_version="$major.$minor.$patch"
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    new_version="$major.$minor.$patch"
    ;;
  patch)
    patch=$((patch + 1))
    new_version="$major.$minor.$patch"
    ;;
  prerelease)
    if [ -z "$prerelease_type" ]; then
      echo -e "${RED}Error: Prerelease type required (alpha|beta|rc)${NC}"
      exit 1
    fi

    # Check if current version already has a prerelease
    if echo "$current_version" | grep -q -- "-$prerelease_type"; then
      # Increment prerelease number
      local prerelease_num=$(echo "$current_version" | sed -n "s/.*-$prerelease_type\.\([0-9]*\).*/\1/p")
      prerelease_num=$((prerelease_num + 1))
      new_version="$base_version-$prerelease_type.$prerelease_num"
    else
      # Start new prerelease
      patch=$((patch + 1))
      new_version="$major.$minor.$patch-$prerelease_type.1"
    fi
    ;;
  *)
    echo -e "${RED}Error: Invalid bump type: $bump_type${NC}"
    echo "Valid types: major, minor, patch, prerelease"
    exit 1
    ;;
  esac

  echo "$new_version"
}

# Function to update version in files
update_version_in_files() {
  local new_version=$1
  local dry_run=${2:-false}

  echo -e "${BLUE}Updating version to $new_version${NC}"

  # Update VERSION file
  if [ "$dry_run" = true ]; then
    echo "Would update $VERSION_FILE to $new_version"
  else
    echo "$new_version" >"$VERSION_FILE"
    echo -e "${GREEN}✓${NC} Updated $VERSION_FILE"
  fi

  # Update Helm Chart.yaml
  if [ -f "$HELM_CHART_FILE" ]; then
    if [ "$dry_run" = true ]; then
      echo "Would update $HELM_CHART_FILE version and appVersion to $new_version"
    else
      # Use sed with different syntax for macOS and Linux compatibility
      if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/^version:.*/version: $new_version/" "$HELM_CHART_FILE"
        sed -i '' "s/^appVersion:.*/appVersion: \"$new_version\"/" "$HELM_CHART_FILE"
      else
        sed -i "s/^version:.*/version: $new_version/" "$HELM_CHART_FILE"
        sed -i "s/^appVersion:.*/appVersion: \"$new_version\"/" "$HELM_CHART_FILE"
      fi
      echo -e "${GREEN}✓${NC} Updated $HELM_CHART_FILE"
    fi
  fi

  # Update README if it contains version badges
  if [ -f "README.md" ] && grep -q "img.shields.io.*version" "README.md"; then
    if [ "$dry_run" = true ]; then
      echo "Would update version badges in README.md"
    else
      if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/version-[0-9.]*-/version-$new_version-/g" "README.md"
      else
        sed -i "s/version-[0-9.]*-/version-$new_version-/g" "README.md"
      fi
      echo -e "${GREEN}✓${NC} Updated README.md badges"
    fi
  fi
}

# Function to create git tag
create_git_tag() {
  local version=$1
  local push=${2:-false}
  local force=${3:-false}

  local tag_name="v$version"

  # Check if tag already exists
  if git rev-parse "$tag_name" >/dev/null 2>&1; then
    if [ "$force" = true ]; then
      echo -e "${YELLOW}Warning: Tag $tag_name already exists. Overwriting...${NC}"
      git tag -d "$tag_name"
    else
      echo -e "${RED}Error: Tag $tag_name already exists. Use --force to overwrite.${NC}"
      exit 1
    fi
  fi

  # Create annotated tag
  git tag -a "$tag_name" -m "Release version $version"
  echo -e "${GREEN}✓${NC} Created git tag: $tag_name"

  # Push tag if requested
  if [ "$push" = true ]; then
    git push origin "$tag_name"
    echo -e "${GREEN}✓${NC} Pushed tag to origin"
  fi
}

# Function to check version consistency
check_version_consistency() {
  local current_version=$(get_current_version)
  local errors=0

  echo -e "${BLUE}Checking version consistency...${NC}"
  echo "Expected version: $current_version"
  echo ""

  # Check Helm Chart
  if [ -f "$HELM_CHART_FILE" ]; then
    local chart_version=$(grep "^version:" "$HELM_CHART_FILE" | awk '{print $2}')
    local app_version=$(grep "^appVersion:" "$HELM_CHART_FILE" | awk '{print $2}' | tr -d '"')

    if [ "$chart_version" != "$current_version" ]; then
      echo -e "${RED}✗${NC} Helm chart version mismatch: $chart_version"
      errors=$((errors + 1))
    else
      echo -e "${GREEN}✓${NC} Helm chart version: $chart_version"
    fi

    if [ "$app_version" != "$current_version" ]; then
      echo -e "${RED}✗${NC} Helm appVersion mismatch: $app_version"
      errors=$((errors + 1))
    else
      echo -e "${GREEN}✓${NC} Helm appVersion: $app_version"
    fi
  fi

  # Check git tag
  local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "none")
  if [ "$latest_tag" != "none" ]; then
    local tag_version=${latest_tag#v}
    echo -e "${BLUE}ℹ${NC}  Latest git tag: $latest_tag"
  fi

  echo ""
  if [ $errors -eq 0 ]; then
    echo -e "${GREEN}All version references are consistent!${NC}"
    return 0
  else
    echo -e "${RED}Found $errors version inconsistencies${NC}"
    return 1
  fi
}

# Function to prepare release
prepare_release() {
  local bump_type=$1
  local push=${2:-false}

  echo -e "${BLUE}Preparing release...${NC}"
  echo ""

  # Get current and new version
  local current_version=$(get_current_version)
  local new_version=$(bump_version "$bump_type")

  echo "Current version: $current_version"
  echo "New version: $new_version"
  echo ""

  # Update version in files
  update_version_in_files "$new_version" false

  # Check for uncommitted changes
  if ! git diff-index --quiet HEAD --; then
    echo -e "${YELLOW}Committing version changes...${NC}"
    git add -A
    git commit -m "chore: bump version to $new_version"
    echo -e "${GREEN}✓${NC} Committed version changes"
  fi

  # Create tag
  create_git_tag "$new_version" "$push" false

  echo ""
  echo -e "${GREEN}Release $new_version prepared successfully!${NC}"
  echo ""
  echo "Next steps:"
  echo "  1. Review the changes: git show HEAD"
  echo "  2. Push commits: git push origin"
  echo "  3. Push tag: git push origin v$new_version"
  echo "  4. Run CI/CD pipeline to build and publish artifacts"
}

# Main script logic
main() {
  local command=${1:-}
  shift || true

  case "$command" in
  current)
    version=$(get_current_version)
    echo -e "${GREEN}Current version: $version${NC}"
    ;;

  bump)
    bump_type=${1:-}
    prerelease_type=${2:-}
    if [ -z "$bump_type" ]; then
      echo -e "${RED}Error: Bump type required${NC}"
      usage
    fi
    new_version=$(bump_version "$bump_type" "$prerelease_type")
    update_version_in_files "$new_version" false
    echo -e "${GREEN}Version bumped to: $new_version${NC}"
    ;;

  set)
    new_version=${1:-}
    if [ -z "$new_version" ]; then
      echo -e "${RED}Error: Version required${NC}"
      usage
    fi
    validate_version "$new_version"
    update_version_in_files "$new_version" false
    echo -e "${GREEN}Version set to: $new_version${NC}"
    ;;

  tag)
    push=false
    force=false
    while [[ $# -gt 0 ]]; do
      case $1 in
      -p | --push)
        push=true
        shift
        ;;
      -f | --force)
        force=true
        shift
        ;;
      *) shift ;;
      esac
    done
    version=$(get_current_version)
    create_git_tag "$version" "$push" "$force"
    ;;

  release)
    bump_type=${1:-patch}
    shift || true
    push=false
    while [[ $# -gt 0 ]]; do
      case $1 in
      -p | --push)
        push=true
        shift
        ;;
      *) shift ;;
      esac
    done
    prepare_release "$bump_type" "$push"
    ;;

  check)
    check_version_consistency
    ;;

  -h | --help | help)
    usage
    ;;

  *)
    echo -e "${RED}Error: Unknown command: $command${NC}"
    usage
    ;;
  esac
}

# Run main function
main "$@"
