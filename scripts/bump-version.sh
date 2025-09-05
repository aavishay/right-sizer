#!/bin/bash

# bump-version.sh - Simple script to bump version and trigger release pipeline
# Usage: ./scripts/bump-version.sh [patch|minor|major]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default bump type
BUMP_TYPE=${1:-patch}

echo -e "${BLUE}🏷️ Right-Sizer Version Bumper${NC}"
echo "================================="
echo ""

# Validate bump type
case $BUMP_TYPE in
patch | minor | major)
  echo -e "${GREEN}✅ Bump type: $BUMP_TYPE${NC}"
  ;;
*)
  echo -e "${RED}❌ Invalid bump type: $BUMP_TYPE${NC}"
  echo "Usage: $0 [patch|minor|major]"
  exit 1
  ;;
esac

# Get current version
if [[ -f "VERSION" ]]; then
  CURRENT_VERSION=$(cat VERSION)
elif [[ -f "helm/Chart.yaml" ]]; then
  CURRENT_VERSION=$(grep '^version:' helm/Chart.yaml | sed 's/version: *//' | tr -d '"')
else
  echo -e "${RED}❌ No VERSION file or helm/Chart.yaml found${NC}"
  exit 1
fi

echo "Current version: $CURRENT_VERSION"

# Parse semantic version
if [[ $CURRENT_VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)(-.*)?$ ]]; then
  MAJOR=${BASH_REMATCH[1]}
  MINOR=${BASH_REMATCH[2]}
  PATCH=${BASH_REMATCH[3]}
  PRERELEASE=${BASH_REMATCH[4]}
else
  echo -e "${RED}❌ Invalid version format: $CURRENT_VERSION${NC}"
  echo "Version must be in semantic versioning format (e.g., 1.2.3)"
  exit 1
fi

# Bump version
case $BUMP_TYPE in
major)
  MAJOR=$((MAJOR + 1))
  MINOR=0
  PATCH=0
  ;;
minor)
  MINOR=$((MINOR + 1))
  PATCH=0
  ;;
patch)
  PATCH=$((PATCH + 1))
  ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"
echo -e "${GREEN}New version: $NEW_VERSION${NC}"

# Check if version already exists
if git tag | grep -q "^v$NEW_VERSION$"; then
  echo -e "${YELLOW}⚠️ Warning: Tag v$NEW_VERSION already exists${NC}"
  read -p "Do you want to continue anyway? (y/N): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${RED}❌ Aborted${NC}"
    exit 1
  fi
fi

# Check for uncommitted changes
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo -e "${YELLOW}⚠️ Warning: You have uncommitted changes${NC}"
  git status --porcelain | sed 's/^/  /'
  echo
  read -p "Do you want to commit these changes first? (y/N): " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    git add -A
    git commit -m "chore: prepare for version bump to $NEW_VERSION"
    echo -e "${GREEN}✅ Changes committed${NC}"
  fi
fi

echo ""
echo -e "${BLUE}📝 Updating version files...${NC}"

# Update VERSION file
echo "$NEW_VERSION" >VERSION
echo "✅ Updated VERSION file"

# Update Helm Chart.yaml
if [[ -f "helm/Chart.yaml" ]]; then
  sed -i.bak "s/^version:.*/version: $NEW_VERSION/" helm/Chart.yaml
  sed -i.bak "s/^appVersion:.*/appVersion: \"$NEW_VERSION\"/" helm/Chart.yaml
  rm -f helm/Chart.yaml.bak
  echo "✅ Updated helm/Chart.yaml"
fi

# Update version references in documentation
if [[ -f "scripts/update-versions.sh" ]]; then
  echo "🔄 Updating documentation references..."
  chmod +x scripts/update-versions.sh
  if ./scripts/update-versions.sh "$NEW_VERSION"; then
    echo "✅ Updated documentation versions"
  else
    echo -e "${YELLOW}⚠️ Documentation update script failed (continuing anyway)${NC}"
  fi
fi

echo ""
echo -e "${BLUE}📦 Committing and tagging...${NC}"

# Add all changed files
git add VERSION
git add helm/Chart.yaml || true
git add README.md helm/README.md docs/ || true

# Create commit
COMMIT_MSG="chore: bump version to $NEW_VERSION ($BUMP_TYPE)

- Updated VERSION: $CURRENT_VERSION → $NEW_VERSION
- Updated Helm Chart version and appVersion
- Updated documentation references

Release type: $BUMP_TYPE bump"

git commit -m "$COMMIT_MSG"
echo "✅ Changes committed"

# Create and push tag
git tag "v$NEW_VERSION" -m "Release v$NEW_VERSION"
echo "✅ Tag v$NEW_VERSION created"

echo ""
echo -e "${YELLOW}🚀 Ready to push...${NC}"
echo "This will:"
echo "  1. Push the commit to main branch"
echo "  2. Push the tag v$NEW_VERSION"
echo "  3. Trigger the automated release pipeline"
echo ""

read -p "Push changes and trigger release? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  # Push commit and tag
  git push origin main
  git push origin "v$NEW_VERSION"

  echo ""
  echo -e "${GREEN}🎉 Version bump complete!${NC}"
  echo ""
  echo "📊 Summary:"
  echo "  • Version: $CURRENT_VERSION → $NEW_VERSION"
  echo "  • Tag: v$NEW_VERSION"
  echo "  • Commit: $(git rev-parse --short HEAD)"
  echo ""
  echo "🔄 The following will happen automatically:"
  echo "  • 🐳 Docker images will be built for linux/amd64 and linux/arm64"
  echo "  • 📦 Helm chart will be packaged and published to OCI registry"
  echo "  • 📋 GitHub release will be created with artifacts"
  echo "  • 🌍 Images will be distributed to Docker Hub and GHCR"
  echo ""
  echo "📈 Monitor progress:"
  echo "  • GitHub Actions: https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\([^/]*\/[^/.]*\).*/\1/')/actions"
  echo "  • Releases: https://github.com/$(git remote get-url origin | sed 's/.*github.com[:/]\([^/]*\/[^/.]*\).*/\1/')/releases"
  echo "  • Docker Hub: https://hub.docker.com/r/aavishay/right-sizer/tags"
  echo ""
else
  echo -e "${YELLOW}❌ Push cancelled${NC}"
  echo "To push manually later:"
  echo "  git push origin main"
  echo "  git push origin v$NEW_VERSION"
  echo ""
  echo "To undo the local changes:"
  echo "  git reset --hard HEAD~1"
  echo "  git tag -d v$NEW_VERSION"
fi
