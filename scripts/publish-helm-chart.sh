#!/bin/bash

# Script to publish Right-Sizer Helm chart to GitHub Pages
# This script packages the Helm chart and publishes it to the gh-pages branch

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CHART_DIR="helm"
CHARTS_REPO_DIR="charts"
BRANCH="gh-pages"
CURRENT_BRANCH=$(git branch --show-current)
VERSION=$(cat VERSION)
CHART_VERSION=$(grep '^version:' ${CHART_DIR}/Chart.yaml | awk '{print $2}')

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

# Check prerequisites
print_color $BLUE "🔍 Checking prerequisites..."

# Check if helm is installed
if ! command -v helm &>/dev/null; then
  error_exit "Helm is not installed. Please install Helm first."
fi

# Check if we're in the right directory
if [ ! -f "VERSION" ] || [ ! -d "$CHART_DIR" ]; then
  error_exit "This script must be run from the right-sizer root directory"
fi

# Check if VERSION and Chart.yaml versions match
if [ "$VERSION" != "$CHART_VERSION" ]; then
  error_exit "Version mismatch: VERSION file ($VERSION) != Chart.yaml ($CHART_VERSION)"
fi

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
  print_color $YELLOW "⚠️  Warning: You have uncommitted changes"
  read -p "Do you want to continue? (y/n) " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    error_exit "Aborted by user"
  fi
fi

print_color $GREEN "✅ Prerequisites check passed"

# Step 1: Package the Helm chart
print_color $BLUE "\n📦 Packaging Helm chart version $VERSION..."
helm package $CHART_DIR --destination /tmp/
CHART_PACKAGE="/tmp/right-sizer-${VERSION}.tgz"

if [ ! -f "$CHART_PACKAGE" ]; then
  error_exit "Failed to package Helm chart"
fi
print_color $GREEN "✅ Chart packaged: $CHART_PACKAGE"

# Step 2: Lint the chart
print_color $BLUE "\n🔍 Linting Helm chart..."
helm lint $CHART_DIR
print_color $GREEN "✅ Chart validation passed"

# Step 3: Save current branch and stash changes
print_color $BLUE "\n💾 Saving current state..."
git stash push -m "Publishing Helm chart $VERSION"

# Step 4: Switch to gh-pages branch
print_color $BLUE "\n🔄 Switching to $BRANCH branch..."
if git show-ref --verify --quiet refs/heads/$BRANCH; then
  git checkout $BRANCH
  git pull origin $BRANCH
else
  error_exit "$BRANCH branch does not exist. Please create it first."
fi

# Step 5: Create charts directory if it doesn't exist
if [ ! -d "$CHARTS_REPO_DIR" ]; then
  print_color $YELLOW "📁 Creating $CHARTS_REPO_DIR directory..."
  mkdir -p $CHARTS_REPO_DIR
fi

# Step 6: Copy the packaged chart
print_color $BLUE "\n📋 Copying chart package to repository..."
cp $CHART_PACKAGE $CHARTS_REPO_DIR/
print_color $GREEN "✅ Chart copied to $CHARTS_REPO_DIR/"

# Step 7: Generate or update Helm repository index
print_color $BLUE "\n📑 Updating Helm repository index..."
if [ -f "$CHARTS_REPO_DIR/index.yaml" ]; then
  helm repo index $CHARTS_REPO_DIR --merge $CHARTS_REPO_DIR/index.yaml --url https://aavishay.github.io/right-sizer/charts
else
  helm repo index $CHARTS_REPO_DIR --url https://aavishay.github.io/right-sizer/charts
fi
print_color $GREEN "✅ Repository index updated"

# Step 8: Update the main index.html if it exists
if [ -f "index.html" ]; then
  print_color $BLUE "\n📝 Updating index.html with latest version..."
  sed -i.bak "s/Version-[0-9]\+\.[0-9]\+\.[0-9]\+/Version-${VERSION}/g" index.html
  sed -i.bak "s/version [0-9]\+\.[0-9]\+\.[0-9]\+/version ${VERSION}/g" index.html
  rm -f index.html.bak
fi

# Step 9: Commit and push changes
print_color $BLUE "\n📤 Committing and pushing changes..."
git add $CHARTS_REPO_DIR/
if [ -f "index.html" ]; then
  git add index.html
fi
git commit -m "Release Helm chart version $VERSION" || true

if [ "$(git status --porcelain)" ]; then
  git push origin $BRANCH
  print_color $GREEN "✅ Changes pushed to $BRANCH branch"
else
  print_color $YELLOW "ℹ️  No changes to push"
fi

# Step 10: Return to original branch
print_color $BLUE "\n🔄 Returning to $CURRENT_BRANCH branch..."
git checkout $CURRENT_BRANCH

# Restore stashed changes if any
if git stash list | grep -q "Publishing Helm chart $VERSION"; then
  print_color $BLUE "🔄 Restoring stashed changes..."
  git stash pop
fi

# Clean up temporary files
rm -f $CHART_PACKAGE

# Step 11: Create a GitHub release (optional)
print_color $BLUE "\n🎉 Chart published successfully!"
print_color $GREEN "
✨ Helm Chart v$VERSION has been published!

📦 Users can now install it with:

    helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
    helm repo update
    helm install right-sizer right-sizer/right-sizer --version $VERSION

📋 Or upgrade existing installations:

    helm upgrade right-sizer right-sizer/right-sizer --version $VERSION

🔗 Chart URL: https://aavishay.github.io/right-sizer/charts/right-sizer-${VERSION}.tgz

📊 Repository index: https://aavishay.github.io/right-sizer/charts/index.yaml
"

# Ask if user wants to create a GitHub release
read -p "Do you want to create a GitHub release for v$VERSION? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  if command -v gh &>/dev/null; then
    print_color $BLUE "\n📢 Creating GitHub release..."

    # Create release notes
    RELEASE_NOTES="## 🎉 Right-Sizer v$VERSION

### ✨ What's New
- Default RightSizerConfig included in Helm chart
- Comprehensive configuration options via Helm values
- Multiple deployment profiles (conservative, aggressive, adaptive)
- Enhanced documentation and examples

### 📦 Installation

\`\`\`bash
# Add Helm repository
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update

# Install
helm install right-sizer right-sizer/right-sizer --version $VERSION
\`\`\`

### 📋 Changelog
- Added default RightSizerConfig template
- Added values-examples.yaml with 8 use case scenarios
- Updated documentation with configuration examples
- Fixed pod resize issues for partial resource definitions

### 🔗 Resources
- [Documentation](https://github.com/aavishay/right-sizer)
- [Helm Chart](https://aavishay.github.io/right-sizer/charts)
- [Issues](https://github.com/aavishay/right-sizer/issues)
"

    echo "$RELEASE_NOTES" | gh release create "v$VERSION" \
      --title "Release v$VERSION" \
      --notes-file - \
      --draft=false \
      --prerelease=false

    print_color $GREEN "✅ GitHub release created: https://github.com/aavishay/right-sizer/releases/tag/v$VERSION"
  else
    print_color $YELLOW "⚠️  GitHub CLI (gh) not installed. Please create the release manually."
  fi
fi

print_color $GREEN "\n🚀 All done! Happy right-sizing!"
