#!/bin/bash

# Script to create a release for Right-Sizer
# This script creates a git tag based on the VERSION file,
# which triggers the release workflow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_color() {
  local color=$1
  shift
  echo -e "${color}$@${NC}"
}

error_exit() {
  print_color $RED "Error: $1"
  exit 1
}

# Check if VERSION file exists
if [ ! -f "VERSION" ]; then
  error_exit "VERSION file not found. Please run this script from the project root."
fi

# Read version from VERSION file
VERSION=$(cat VERSION)
TAG="v${VERSION}"

print_color $BLUE "🚀 Right-Sizer Release Script"
print_color $BLUE "============================="
echo ""
print_color $YELLOW "Version to release: ${VERSION}"
print_color $YELLOW "Git tag to create: ${TAG}"
echo ""

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
  print_color $YELLOW "⚠️  Warning: You are not on the main branch (current: $CURRENT_BRANCH)"
  read -p "Do you want to continue? (y/n) " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    error_exit "Aborted by user"
  fi
fi

# Check if there are uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
  print_color $YELLOW "⚠️  Warning: You have uncommitted changes"
  git status --short
  echo ""
  read -p "Do you want to commit these changes first? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    read -p "Enter commit message: " commit_msg
    git add -A
    git commit -m "$commit_msg"
    git push origin main
  else
    read -p "Continue without committing? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      error_exit "Aborted by user"
    fi
  fi
fi

# Pull latest changes
print_color $BLUE "📥 Pulling latest changes from origin..."
git pull origin main --rebase || true

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
  print_color $RED "❌ Tag $TAG already exists!"
  echo ""
  print_color $YELLOW "Existing tag points to:"
  git log -1 --format="%H %s" "$TAG"
  echo ""
  read -p "Do you want to delete and recreate the tag? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_color $YELLOW "Deleting existing tag locally and remotely..."
    git tag -d "$TAG"
    git push origin --delete "$TAG" || true
  else
    error_exit "Cannot create release with existing tag"
  fi
fi

# Create annotated tag
print_color $BLUE "🏷️  Creating annotated tag $TAG..."
git tag -a "$TAG" -m "Release $VERSION

## Highlights
- Updated to version $VERSION
- Docker images will be tagged as: $VERSION
- Helm chart version: $VERSION

## Installation

### Docker
\`\`\`bash
docker pull aavishay/right-sizer:$VERSION
\`\`\`

### Helm
\`\`\`bash
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update
helm install right-sizer right-sizer/right-sizer --version $VERSION
\`\`\`

For full changelog, see the release notes on GitHub."

print_color $GREEN "✅ Tag created successfully"

# Push the tag
print_color $BLUE "📤 Pushing tag to origin..."
git push origin "$TAG"

if [ $? -eq 0 ]; then
  print_color $GREEN "✅ Tag pushed successfully!"
  echo ""
  print_color $GREEN "🎉 Release process initiated!"
  echo ""
  print_color $BLUE "The following will happen automatically:"
  echo "  1. GitHub Actions will build Docker images with tag: $VERSION"
  echo "  2. Docker images will be pushed to Docker Hub"
  echo "  3. Helm chart will be packaged and published"
  echo "  4. GitHub Release will be created with artifacts"
  echo ""
  print_color $YELLOW "📊 Monitor the progress at:"
  echo "  https://github.com/aavishay/right-sizer/actions"
  echo ""
  print_color $YELLOW "📦 Once complete, the release will be available at:"
  echo "  https://github.com/aavishay/right-sizer/releases/tag/$TAG"
  echo ""
  print_color $BLUE "🐳 Docker images will be available at:"
  echo "  https://hub.docker.com/r/aavishay/right-sizer/tags"
  echo ""
  print_color $BLUE "⎈ Helm chart will be available at:"
  echo "  https://aavishay.github.io/right-sizer/charts"
else
  error_exit "Failed to push tag to origin"
fi

# Optional: Open the Actions page in browser
if command -v open >/dev/null 2>&1; then
  read -p "Do you want to open GitHub Actions in your browser? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    open "https://github.com/aavishay/right-sizer/actions"
  fi
elif command -v xdg-open >/dev/null 2>&1; then
  read -p "Do you want to open GitHub Actions in your browser? (y/n) " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    xdg-open "https://github.com/aavishay/right-sizer/actions"
  fi
fi

print_color $GREEN "✨ Release script completed successfully!"
