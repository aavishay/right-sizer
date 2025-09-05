#!/bin/bash
# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Update versions across all README files and configuration
# Usage: ./scripts/update-versions.sh <new-version>

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }

# Function to show usage
usage() {
  echo "Usage: $0 <new-version>"
  echo ""
  echo "Examples:"
  echo "  $0 0.1.7           # Update to version 0.1.7"
  echo "  $0 1.0.0           # Update to version 1.0.0"
  echo "  $0 1.0.0-beta.1    # Update to pre-release 1.0.0-beta.1"
  echo ""
  echo "This script will:"
  echo "  • Update all version references in README.md files"
  echo "  • Update Helm Chart.yaml (version and appVersion)"
  echo "  • Update Docker image tags in examples"
  echo "  • Update badge versions"
  echo "  • Create a summary of changes"
  exit 1
}

# Check if version is provided
if [[ $# -eq 0 ]]; then
  print_error "No version specified"
  usage
fi

NEW_VERSION="$1"

# Validate version format (semantic versioning)
if ! echo "$NEW_VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$'; then
  print_error "Invalid version format: $NEW_VERSION"
  echo "Version must follow semantic versioning (e.g., 1.0.0, 1.0.0-beta.1, 1.0.0+build.1)"
  exit 1
fi

# Get current version from Chart.yaml
CURRENT_VERSION=""
if [[ -f "helm/Chart.yaml" ]]; then
  CURRENT_VERSION=$(grep '^version:' helm/Chart.yaml | sed 's/version: *//' | tr -d '"')
fi

if [[ -z "$CURRENT_VERSION" ]]; then
  print_warning "Could not detect current version from helm/Chart.yaml"
  CURRENT_VERSION="unknown"
fi

print_info "Updating versions from $CURRENT_VERSION to $NEW_VERSION"

# Backup directory
BACKUP_DIR="./version-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Files to update
declare -a FILES_TO_UPDATE=(
  "README.md"
  "helm/README.md"
  "helm/Chart.yaml"
  "helm/values.yaml"
  "docs/CHANGELOG.md"
)

# Function to backup file
backup_file() {
  local file="$1"
  if [[ -f "$file" ]]; then
    cp "$file" "$BACKUP_DIR/$(basename "$file")"
    print_info "Backed up $file"
  fi
}

# Function to update file with version replacements
update_file() {
  local file="$1"
  local temp_file="${file}.tmp"

  if [[ ! -f "$file" ]]; then
    print_warning "File not found: $file"
    return 0
  fi

  print_info "Updating $file..."

  # Create backup
  backup_file "$file"

  # Perform replacements based on file type
  case "$file" in
  "README.md" | "helm/README.md")
    # Update version badges
    sed "s|Version-[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^-]*-green|Version-${NEW_VERSION}-green|g" "$file" >"$temp_file"

    # Update Docker image tags
    sed -i.bak "s|aavishay/right-sizer:[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^[:space:]]*|aavishay/right-sizer:${NEW_VERSION}|g" "$temp_file"

    # Update Helm chart version references
    sed -i.bak "s|--version [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^[:space:]]*|--version ${NEW_VERSION}|g" "$temp_file"

    # Update image.tag references
    sed -i.bak "s|--set image\.tag=[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^[:space:]]*|--set image.tag=${NEW_VERSION}|g" "$temp_file"

    # Update targetRevision in ArgoCD examples
    sed -i.bak "s|targetRevision: [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^[:space:]]*|targetRevision: ${NEW_VERSION}|g" "$temp_file"

    # Update OCI registry references
    sed -i.bak "s|oci://registry-1\.docker\.io/aavishay/right-sizer --version [0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[^[:space:]]*|oci://registry-1.docker.io/aavishay/right-sizer --version ${NEW_VERSION}|g" "$temp_file"

    # Clean up sed backup files
    rm -f "${temp_file}.bak"
    ;;

  "helm/Chart.yaml")
    # Update both version and appVersion
    sed "s|^version:.*|version: ${NEW_VERSION}|g" "$file" >"$temp_file"
    sed -i.bak "s|^appVersion:.*|appVersion: \"${NEW_VERSION}\"|g" "$temp_file"
    rm -f "${temp_file}.bak"
    ;;

  "helm/values.yaml")
    # Update default image tag comment if present
    sed "s|# Overrides the image tag whose default is the chart appVersion\.|# Overrides the image tag whose default is the chart appVersion (${NEW_VERSION})|g" "$file" >"$temp_file"
    ;;

  "docs/CHANGELOG.md")
    # Add new version entry at the top if it's a CHANGELOG
    if [[ -f "$file" ]] && ! grep -q "## \[${NEW_VERSION}\]" "$file"; then
      {
        echo "## [${NEW_VERSION}] - $(date +%Y-%m-%d)"
        echo ""
        echo "### Added"
        echo "- Version update to ${NEW_VERSION}"
        echo ""
        cat "$file"
      } >"$temp_file"
    else
      cp "$file" "$temp_file"
    fi
    ;;

  *)
    # Generic version replacement for other files
    sed "s|${CURRENT_VERSION}|${NEW_VERSION}|g" "$file" >"$temp_file"
    ;;
  esac

  # Replace original file with updated version
  mv "$temp_file" "$file"
  print_success "Updated $file"
}

# Function to validate updates
validate_updates() {
  local errors=0

  print_info "Validating updates..."

  # Check Helm Chart.yaml
  if [[ -f "helm/Chart.yaml" ]]; then
    local chart_version=$(grep '^version:' helm/Chart.yaml | sed 's/version: *//' | tr -d '"')
    local app_version=$(grep '^appVersion:' helm/Chart.yaml | sed 's/appVersion: *//' | tr -d '"')

    if [[ "$chart_version" != "$NEW_VERSION" ]]; then
      print_error "Helm chart version mismatch: expected $NEW_VERSION, got $chart_version"
      errors=$((errors + 1))
    fi

    if [[ "$app_version" != "$NEW_VERSION" ]]; then
      print_error "Helm appVersion mismatch: expected $NEW_VERSION, got $app_version"
      errors=$((errors + 1))
    fi
  fi

  # Check README files for version badges
  for readme in README.md helm/README.md; do
    if [[ -f "$readme" ]]; then
      if ! grep -q "Version-${NEW_VERSION}-green" "$readme"; then
        print_error "Version badge not updated in $readme"
        errors=$((errors + 1))
      fi
    fi
  done

  # Check for Docker image tag updates in README
  if [[ -f "README.md" ]]; then
    local docker_refs=$(grep -c "aavishay/right-sizer:${NEW_VERSION}" README.md || echo "0")
    if [[ "$docker_refs" -eq 0 ]]; then
      print_error "No Docker image references updated in README.md"
      errors=$((errors + 1))
    else
      print_success "Found $docker_refs Docker image references updated"
    fi
  fi

  return $errors
}

# Function to show summary
show_summary() {
  echo ""
  echo "=================================================="
  print_success "Version Update Summary"
  echo "=================================================="
  echo "Old Version: $CURRENT_VERSION"
  echo "New Version: $NEW_VERSION"
  echo "Backup Location: $BACKUP_DIR"
  echo ""
  echo "Updated Files:"

  for file in "${FILES_TO_UPDATE[@]}"; do
    if [[ -f "$file" ]]; then
      echo "  ✅ $file"
    else
      echo "  ⚠️  $file (not found)"
    fi
  done

  echo ""
  echo "Git Status:"
  git status --porcelain | sed 's/^/  /'

  echo ""
  print_info "Next steps:"
  echo "  1. Review changes: git diff"
  echo "  2. Test the changes locally"
  echo "  3. Commit changes: git add . && git commit -m 'chore: update version to $NEW_VERSION'"
  echo "  4. Create and push tag: git tag v$NEW_VERSION && git push origin v$NEW_VERSION"
  echo ""
  print_warning "Remember to restore from $BACKUP_DIR if needed!"
}

# Function to cleanup
cleanup() {
  # Remove any temporary files that might be left
  find . -name "*.tmp" -type f -delete 2>/dev/null || true
}

# Trap for cleanup
trap cleanup EXIT

# Main execution
main() {
  print_info "Starting version update process..."

  # Ensure we're in the project root
  if [[ ! -f "helm/Chart.yaml" ]]; then
    print_error "Please run this script from the project root directory"
    exit 1
  fi

  # Update all files
  for file in "${FILES_TO_UPDATE[@]}"; do
    update_file "$file"
  done

  # Validate updates
  if validate_updates; then
    print_success "All validations passed!"
  else
    print_error "Some validations failed. Please review the changes."
    exit 1
  fi

  # Show summary
  show_summary

  print_success "Version update completed successfully!"
}

# Run main function
main "$@"
