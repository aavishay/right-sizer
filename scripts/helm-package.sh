#!/bin/bash

# Right-Sizer Helm Chart Packaging Script
# This script packages the Helm chart for local testing and distribution

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHART_DIR="$PROJECT_ROOT/helm"
OUTPUT_DIR="$PROJECT_ROOT/dist"
CHART_NAME="right-sizer"
VERSION=""
DRY_RUN=false
LINT=true
SIGN=false
KEY=""
REPO_INDEX=false
REPO_URL="https://aavishay.github.io/right-sizer/charts"

# Function to print usage
usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Package the Right-Sizer Helm chart for distribution.

OPTIONS:
    -v, --version VERSION    Override chart version
    -o, --output DIR        Output directory (default: dist/)
    -d, --dry-run           Perform a dry run without packaging
    -l, --no-lint           Skip chart linting
    -s, --sign              Sign the chart package
    -k, --key KEY           GPG key to use for signing
    -i, --index             Generate/update repository index
    -u, --url URL           Repository URL for index (default: $REPO_URL)
    -h, --help              Show this help message

EXAMPLES:
    # Package chart with default version
    $0

    # Package with specific version
    $0 --version 1.2.3

    # Package and sign
    $0 --sign --key maintainer@example.com

    # Package and create repository index
    $0 --index --url https://charts.example.com

EOF
  exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -v | --version)
    VERSION="$2"
    shift 2
    ;;
  -o | --output)
    OUTPUT_DIR="$2"
    shift 2
    ;;
  -d | --dry-run)
    DRY_RUN=true
    shift
    ;;
  -l | --no-lint)
    LINT=false
    shift
    ;;
  -s | --sign)
    SIGN=true
    shift
    ;;
  -k | --key)
    KEY="$2"
    shift 2
    ;;
  -i | --index)
    REPO_INDEX=true
    shift
    ;;
  -u | --url)
    REPO_URL="$2"
    shift 2
    ;;
  -h | --help)
    usage
    ;;
  *)
    echo -e "${RED}Unknown option: $1${NC}"
    usage
    ;;
  esac
done

# Check if Helm is installed
if ! command -v helm &>/dev/null; then
  echo -e "${RED}❌ Helm is not installed. Please install Helm first.${NC}"
  echo "Visit: https://helm.sh/docs/intro/install/"
  exit 1
fi

# Check if chart directory exists
if [ ! -d "$CHART_DIR" ]; then
  echo -e "${RED}❌ Chart directory not found: $CHART_DIR${NC}"
  exit 1
fi

# Get chart version from Chart.yaml if not specified
if [ -z "$VERSION" ]; then
  VERSION=$(grep '^version:' "$CHART_DIR/Chart.yaml" | awk '{print $2}')
  echo -e "${BLUE}📦 Using chart version from Chart.yaml: $VERSION${NC}"
else
  echo -e "${BLUE}📦 Using specified version: $VERSION${NC}"
  # Update Chart.yaml with the specified version
  if [ "$DRY_RUN" = false ]; then
    sed -i.bak "s/^version:.*/version: $VERSION/" "$CHART_DIR/Chart.yaml"
    rm -f "$CHART_DIR/Chart.yaml.bak"
    echo -e "${GREEN}✅ Updated Chart.yaml with version $VERSION${NC}"
  fi
fi

# Get app version from VERSION file if it exists
if [ -f "$PROJECT_ROOT/VERSION" ]; then
  APP_VERSION=$(cat "$PROJECT_ROOT/VERSION")
  echo -e "${BLUE}🏷️  Using app version from VERSION file: $APP_VERSION${NC}"
  if [ "$DRY_RUN" = false ]; then
    sed -i.bak "s/^appVersion:.*/appVersion: \"$APP_VERSION\"/" "$CHART_DIR/Chart.yaml"
    rm -f "$CHART_DIR/Chart.yaml.bak"
  fi
fi

# Create output directory
if [ "$DRY_RUN" = false ]; then
  mkdir -p "$OUTPUT_DIR"
  echo -e "${GREEN}✅ Created output directory: $OUTPUT_DIR${NC}"
fi

# Update dependencies
echo -e "${BLUE}🔄 Updating chart dependencies...${NC}"
if [ "$DRY_RUN" = false ]; then
  helm dependency update "$CHART_DIR" 2>/dev/null || true
fi

# Lint the chart
if [ "$LINT" = true ]; then
  echo -e "${BLUE}🔍 Linting chart...${NC}"
  if helm lint "$CHART_DIR"; then
    echo -e "${GREEN}✅ Chart validation passed${NC}"
  else
    echo -e "${YELLOW}⚠️  Chart has linting warnings/errors${NC}"
    if [ "$DRY_RUN" = false ]; then
      read -p "Continue packaging? (y/n) " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
      fi
    fi
  fi
fi

# Perform dry run
if [ "$DRY_RUN" = true ]; then
  echo -e "${YELLOW}🔍 Dry run mode - no files will be created${NC}"
  echo -e "${BLUE}Would package: ${CHART_NAME}-${VERSION}.tgz${NC}"
  echo -e "${BLUE}Output directory: $OUTPUT_DIR${NC}"

  if [ "$SIGN" = true ]; then
    echo -e "${BLUE}Would sign with key: ${KEY:-default}${NC}"
  fi

  if [ "$REPO_INDEX" = true ]; then
    echo -e "${BLUE}Would generate repository index with URL: $REPO_URL${NC}"
  fi

  exit 0
fi

# Package the chart
echo -e "${GREEN}📦 Packaging Helm chart...${NC}"
PACKAGE_FILE="${CHART_NAME}-${VERSION}.tgz"

if helm package "$CHART_DIR" --destination "$OUTPUT_DIR"; then
  echo -e "${GREEN}✅ Chart packaged successfully: $OUTPUT_DIR/$PACKAGE_FILE${NC}"
else
  echo -e "${RED}❌ Failed to package chart${NC}"
  exit 1
fi

# Sign the package if requested
if [ "$SIGN" = true ]; then
  echo -e "${BLUE}🔐 Signing chart package...${NC}"

  if [ -n "$KEY" ]; then
    SIGN_ARGS="--key $KEY"
  else
    SIGN_ARGS=""
  fi

  if helm gpg sign "$OUTPUT_DIR/$PACKAGE_FILE" $SIGN_ARGS; then
    echo -e "${GREEN}✅ Chart signed successfully${NC}"
  else
    echo -e "${RED}❌ Failed to sign chart${NC}"
    exit 1
  fi
fi

# Generate repository index if requested
if [ "$REPO_INDEX" = true ]; then
  echo -e "${BLUE}📋 Generating repository index...${NC}"

  # Check if index.yaml already exists
  if [ -f "$OUTPUT_DIR/index.yaml" ]; then
    echo -e "${BLUE}📋 Updating existing repository index...${NC}"
    helm repo index "$OUTPUT_DIR" --url "$REPO_URL" --merge "$OUTPUT_DIR/index.yaml"
  else
    echo -e "${BLUE}📋 Creating new repository index...${NC}"
    helm repo index "$OUTPUT_DIR" --url "$REPO_URL"
  fi

  if [ -f "$OUTPUT_DIR/index.yaml" ]; then
    echo -e "${GREEN}✅ Repository index generated: $OUTPUT_DIR/index.yaml${NC}"
  else
    echo -e "${RED}❌ Failed to generate repository index${NC}"
    exit 1
  fi
fi

# Verify the package
echo -e "${BLUE}🔍 Verifying package...${NC}"
tar -tzf "$OUTPUT_DIR/$PACKAGE_FILE" | head -10
echo "..."
echo -e "${GREEN}✅ Package contents verified${NC}"

# Calculate checksums
echo -e "${BLUE}🔐 Calculating checksums...${NC}"
cd "$OUTPUT_DIR"
sha256sum "$PACKAGE_FILE" >"$PACKAGE_FILE.sha256"
md5sum "$PACKAGE_FILE" >"$PACKAGE_FILE.md5"
cd - >/dev/null
echo -e "${GREEN}✅ Checksums calculated${NC}"

# Summary
echo ""
echo -e "${GREEN}📊 Packaging Summary${NC}"
echo "===================="
echo -e "Chart Name:    ${BLUE}$CHART_NAME${NC}"
echo -e "Version:       ${BLUE}$VERSION${NC}"
echo -e "Package:       ${BLUE}$OUTPUT_DIR/$PACKAGE_FILE${NC}"
echo -e "Size:          ${BLUE}$(du -h "$OUTPUT_DIR/$PACKAGE_FILE" | cut -f1)${NC}"
echo -e "SHA256:        ${BLUE}$(cut -d' ' -f1 "$OUTPUT_DIR/$PACKAGE_FILE.sha256")${NC}"

if [ "$SIGN" = true ]; then
  echo -e "Signature:     ${BLUE}$OUTPUT_DIR/$PACKAGE_FILE.prov${NC}"
fi

if [ "$REPO_INDEX" = true ]; then
  echo -e "Index:         ${BLUE}$OUTPUT_DIR/index.yaml${NC}"
fi

echo ""
echo -e "${YELLOW}📝 Next Steps:${NC}"
echo ""

# Installation instructions
echo "1. Install locally:"
echo "   helm install $CHART_NAME $OUTPUT_DIR/$PACKAGE_FILE"
echo ""

# Push to OCI registry
echo "2. Push to OCI registry:"
echo "   helm push $OUTPUT_DIR/$PACKAGE_FILE oci://registry-1.docker.io/aavishay"
echo ""

# Upload to GitHub releases
echo "3. Upload to GitHub release:"
echo "   gh release upload v$VERSION $OUTPUT_DIR/$PACKAGE_FILE"
echo ""

# Serve local repository
if [ "$REPO_INDEX" = true ]; then
  echo "4. Serve as local repository:"
  echo "   cd $OUTPUT_DIR && python3 -m http.server 8080"
  echo "   helm repo add local http://localhost:8080"
  echo ""
fi

echo -e "${GREEN}✨ Packaging complete!${NC}"
