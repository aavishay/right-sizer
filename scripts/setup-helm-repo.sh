#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to set up and test the Helm repository locally
# This helps developers test Helm chart changes before pushing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
HELM_DIR="${ROOT_DIR}/helm"
CHARTS_DIR="${ROOT_DIR}/charts"
LOCAL_PORT=8080

# Parse command line arguments
ACTION="help"
VERSION=""
SERVE_LOCAL=false

while [[ $# -gt 0 ]]; do
  case $1 in
  package)
    ACTION="package"
    shift
    ;;
  index)
    ACTION="index"
    shift
    ;;
  serve)
    ACTION="serve"
    shift
    ;;
  test)
    ACTION="test"
    shift
    ;;
  publish)
    ACTION="publish"
    shift
    ;;
  clean)
    ACTION="clean"
    shift
    ;;
  --version | -v)
    VERSION="$2"
    shift 2
    ;;
  --port | -p)
    LOCAL_PORT="$2"
    shift 2
    ;;
  --help | -h)
    ACTION="help"
    shift
    ;;
  *)
    echo -e "${RED}Unknown option: $1${NC}"
    ACTION="help"
    shift
    ;;
  esac
done

# Functions
show_help() {
  cat <<EOF
${BLUE}Helm Repository Setup Script${NC}

Usage: $0 [command] [options]

${CYAN}Commands:${NC}
  package       Package the Helm chart
  index         Create/update the repository index
  serve         Start a local HTTP server for testing
  test          Test the Helm repository (package, index, and serve)
  publish       Prepare charts for GitHub Pages publishing
  clean         Clean up generated files
  help          Show this help message

${CYAN}Options:${NC}
  --version, -v <version>  Specify chart version (default: from Chart.yaml)
  --port, -p <port>       Port for local server (default: 8080)
  --help, -h              Show help

${CYAN}Examples:${NC}
  # Package the chart with current version
  $0 package

  # Package with specific version
  $0 package --version 1.2.3

  # Test the repository locally
  $0 test

  # Serve repository on custom port
  $0 serve --port 9090

  # Prepare for GitHub Pages
  $0 publish

EOF
}

check_prerequisites() {
  echo -e "${YELLOW}Checking prerequisites...${NC}"

  # Check Helm
  if ! command -v helm &>/dev/null; then
    echo -e "${RED}❌ Helm is not installed${NC}"
    echo -e "${YELLOW}   Install from: https://helm.sh/docs/intro/install/${NC}"
    exit 1
  fi

  # Check Python (for HTTP server)
  if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
    echo -e "${YELLOW}⚠️  Python not found (needed for local server)${NC}"
  fi

  echo -e "${GREEN}✓ Prerequisites satisfied${NC}"
}

get_chart_version() {
  if [[ -n "$VERSION" ]]; then
    echo "$VERSION"
  else
    grep '^version:' "${HELM_DIR}/Chart.yaml" | awk '{print $2}'
  fi
}

package_chart() {
  echo -e "\n${BLUE}📦 Packaging Helm chart...${NC}"

  # Create charts directory if it doesn't exist
  mkdir -p "${CHARTS_DIR}"

  # Get version
  CHART_VERSION=$(get_chart_version)
  echo -e "${CYAN}   Version: ${CHART_VERSION}${NC}"

  # Update Chart.yaml if custom version provided
  if [[ -n "$VERSION" ]]; then
    echo -e "${YELLOW}   Updating Chart.yaml with version ${VERSION}${NC}"
    sed -i.bak "s/^version:.*/version: ${VERSION}/" "${HELM_DIR}/Chart.yaml"
  fi

  # Lint the chart first
  echo -e "${YELLOW}   Linting chart...${NC}"
  helm lint "${HELM_DIR}"

  # Package the chart
  echo -e "${YELLOW}   Packaging...${NC}"
  helm package "${HELM_DIR}" --destination "${CHARTS_DIR}"

  # Restore original Chart.yaml if we modified it
  if [[ -f "${HELM_DIR}/Chart.yaml.bak" ]]; then
    mv "${HELM_DIR}/Chart.yaml.bak" "${HELM_DIR}/Chart.yaml"
  fi

  echo -e "${GREEN}✓ Chart packaged: ${CHARTS_DIR}/right-sizer-${CHART_VERSION}.tgz${NC}"
}

create_index() {
  echo -e "\n${BLUE}📇 Creating repository index...${NC}"

  if [[ ! -d "${CHARTS_DIR}" ]]; then
    echo -e "${RED}❌ Charts directory not found. Run 'package' first.${NC}"
    exit 1
  fi

  # Determine the URL based on whether we're serving locally or for GitHub Pages
  if [[ "$ACTION" == "publish" ]]; then
    REPO_URL="https://aavishay.github.io/right-sizer/charts"
  else
    REPO_URL="http://localhost:${LOCAL_PORT}/charts"
  fi

  echo -e "${CYAN}   Repository URL: ${REPO_URL}${NC}"

  # Generate index
  helm repo index "${CHARTS_DIR}" --url "${REPO_URL}"

  # Create a simple HTML index page
  cat >"${CHARTS_DIR}/index.html" <<EOF
<!DOCTYPE html>
<html>
<head>
    <title>Right-Sizer Helm Repository (Local)</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
        code { background: #f4f4f4; padding: 2px 5px; border-radius: 3px; }
        .command { background: #000; color: #0f0; padding: 10px; border-radius: 5px; margin: 10px 0; }
    </style>
</head>
<body>
    <h1>🚀 Right-Sizer Helm Repository (Local Test)</h1>

    <h2>Available Charts</h2>
    <ul>
EOF

  # List available charts
  for chart in "${CHARTS_DIR}"/*.tgz; do
    if [[ -f "$chart" ]]; then
      basename=$(basename "$chart")
      echo "        <li><a href=\"${basename}\">${basename}</a></li>" >>"${CHARTS_DIR}/index.html"
    fi
  done

  cat >>"${CHARTS_DIR}/index.html" <<EOF
    </ul>

    <h2>Usage</h2>
    <div class="command">
        <pre>
# Add this repository
helm repo add right-sizer-local ${REPO_URL}
helm repo update

# Install the chart
helm install right-sizer right-sizer-local/right-sizer
        </pre>
    </div>

    <h2>Files</h2>
    <ul>
        <li><a href="index.yaml">index.yaml</a> - Repository index</li>
    </ul>
</body>
</html>
EOF

  echo -e "${GREEN}✓ Repository index created at ${CHARTS_DIR}/index.yaml${NC}"
}

serve_repository() {
  echo -e "\n${BLUE}🌐 Starting local Helm repository server...${NC}"

  if [[ ! -f "${CHARTS_DIR}/index.yaml" ]]; then
    echo -e "${YELLOW}   No index found, creating one...${NC}"
    create_index
  fi

  echo -e "${CYAN}   Server URL: http://localhost:${LOCAL_PORT}${NC}"
  echo -e "${CYAN}   Charts URL: http://localhost:${LOCAL_PORT}/charts${NC}"
  echo -e "\n${YELLOW}📝 To test this repository:${NC}"
  echo -e "${GREEN}   helm repo add right-sizer-local http://localhost:${LOCAL_PORT}/charts${NC}"
  echo -e "${GREEN}   helm repo update${NC}"
  echo -e "${GREEN}   helm search repo right-sizer-local${NC}"
  echo -e "${GREEN}   helm install test right-sizer-local/right-sizer --dry-run${NC}"
  echo -e "\n${YELLOW}Press Ctrl+C to stop the server${NC}\n"

  # Change to root directory to serve both charts and other files if needed
  cd "${ROOT_DIR}"

  # Try Python 3 first, then Python 2
  if command -v python3 &>/dev/null; then
    python3 -m http.server ${LOCAL_PORT}
  elif command -v python &>/dev/null; then
    python -m SimpleHTTPServer ${LOCAL_PORT}
  else
    echo -e "${RED}❌ Python is required to run the local server${NC}"
    echo -e "${YELLOW}   Alternatively, you can use any HTTP server to serve the '${CHARTS_DIR}' directory${NC}"
    exit 1
  fi
}

test_repository() {
  echo -e "\n${BLUE}🧪 Testing Helm repository setup...${NC}"

  # Package the chart
  package_chart

  # Create index
  create_index

  # Start server in background
  echo -e "\n${YELLOW}Starting test server in background...${NC}"

  # Start Python server in background
  cd "${ROOT_DIR}"
  if command -v python3 &>/dev/null; then
    python3 -m http.server ${LOCAL_PORT} >/dev/null 2>&1 &
  elif command -v python &>/dev/null; then
    python -m SimpleHTTPServer ${LOCAL_PORT} >/dev/null 2>&1 &
  else
    echo -e "${RED}❌ Python is required for testing${NC}"
    exit 1
  fi
  SERVER_PID=$!

  # Wait for server to start
  sleep 2

  # Test the repository
  echo -e "\n${YELLOW}Testing repository operations...${NC}"

  # Remove old test repo if exists
  helm repo remove right-sizer-test 2>/dev/null || true

  # Add test repository
  echo -e "${CYAN}   Adding repository...${NC}"
  helm repo add right-sizer-test http://localhost:${LOCAL_PORT}/charts

  # Update repositories
  echo -e "${CYAN}   Updating repositories...${NC}"
  helm repo update

  # Search for chart
  echo -e "${CYAN}   Searching for chart...${NC}"
  helm search repo right-sizer-test

  # Show chart info
  echo -e "${CYAN}   Showing chart information...${NC}"
  helm show chart right-sizer-test/right-sizer

  # Test installation (dry-run)
  echo -e "${CYAN}   Testing installation (dry-run)...${NC}"
  helm install test-release right-sizer-test/right-sizer --dry-run >/dev/null 2>&1

  if [[ $? -eq 0 ]]; then
    echo -e "${GREEN}✓ Dry-run installation successful${NC}"
  else
    echo -e "${RED}❌ Dry-run installation failed${NC}"
  fi

  # Cleanup
  echo -e "\n${YELLOW}Cleaning up...${NC}"
  helm repo remove right-sizer-test
  kill $SERVER_PID 2>/dev/null

  echo -e "\n${GREEN}✓ Repository test completed successfully!${NC}"
}

prepare_publish() {
  echo -e "\n${BLUE}📤 Preparing charts for GitHub Pages publishing...${NC}"

  # Package the chart
  package_chart

  # Create index for GitHub Pages
  echo -e "${YELLOW}Creating index for GitHub Pages...${NC}"
  helm repo index "${CHARTS_DIR}" --url "https://aavishay.github.io/right-sizer/charts"

  # Create README for charts directory
  cat >"${CHARTS_DIR}/README.md" <<EOF
# Right-Sizer Helm Charts

This directory contains packaged Helm charts for Right-Sizer.

## Usage

\`\`\`bash
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update
helm install right-sizer right-sizer/right-sizer
\`\`\`

## Files

- \`*.tgz\` - Packaged Helm charts
- \`index.yaml\` - Helm repository index
EOF

  echo -e "${GREEN}✓ Charts prepared for publishing${NC}"
  echo -e "${CYAN}   Directory: ${CHARTS_DIR}${NC}"
  echo -e "${YELLOW}   To publish: Push to GitHub and enable GitHub Pages for the repository${NC}"
}

clean_charts() {
  echo -e "\n${BLUE}🧹 Cleaning up generated files...${NC}"

  if [[ -d "${CHARTS_DIR}" ]]; then
    echo -e "${YELLOW}   Removing ${CHARTS_DIR}...${NC}"
    rm -rf "${CHARTS_DIR}"
  fi

  # Remove backup files
  find "${HELM_DIR}" -name "*.bak" -delete

  echo -e "${GREEN}✓ Cleanup completed${NC}"
}

# Main execution
case $ACTION in
package)
  check_prerequisites
  package_chart
  ;;
index)
  check_prerequisites
  create_index
  ;;
serve)
  check_prerequisites
  serve_repository
  ;;
test)
  check_prerequisites
  test_repository
  ;;
publish)
  check_prerequisites
  prepare_publish
  ;;
clean)
  clean_charts
  ;;
help | *)
  show_help
  ;;
esac
