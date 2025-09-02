#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Script to test the published Helm repository from GitHub Pages
# This script verifies that the Helm chart is properly published and accessible

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="aavishay"
REPO_NAME="right-sizer"
HELM_REPO_URL="https://${REPO_OWNER}.github.io/${REPO_NAME}/charts"
HELM_REPO_NAME="right-sizer"
TEST_NAMESPACE="test-right-sizer"
DRY_RUN=true

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  --install)
    DRY_RUN=false
    shift
    ;;
  --namespace | -n)
    TEST_NAMESPACE="$2"
    shift 2
    ;;
  --repo-name | -r)
    HELM_REPO_NAME="$2"
    shift 2
    ;;
  --help | -h)
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --install              Actually install the chart (default: dry-run only)"
    echo "  --namespace, -n NAME   Namespace for test installation (default: test-right-sizer)"
    echo "  --repo-name, -r NAME   Name for the Helm repository (default: right-sizer)"
    echo "  --help, -h             Show this help message"
    echo ""
    echo "This script tests the published Helm repository from GitHub Pages."
    exit 0
    ;;
  *)
    echo -e "${RED}Unknown option: $1${NC}"
    exit 1
    ;;
  esac
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Testing Published Helm Repository${NC}"
echo -e "${BLUE}========================================${NC}"

# Function to check URL availability
check_url() {
  local url=$1
  local description=$2

  echo -e "\n${YELLOW}Checking ${description}...${NC}"

  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$url")

  if [[ "$HTTP_CODE" == "200" ]]; then
    echo -e "${GREEN}✓ ${description} is available (HTTP ${HTTP_CODE})${NC}"
    return 0
  elif [[ "$HTTP_CODE" == "404" ]]; then
    echo -e "${RED}✗ ${description} not found (HTTP ${HTTP_CODE})${NC}"
    return 1
  else
    echo -e "${YELLOW}⚠ ${description} returned HTTP ${HTTP_CODE}${NC}"
    return 1
  fi
}

# Step 1: Check if GitHub Pages is live
echo -e "\n${CYAN}Step 1: Checking GitHub Pages availability${NC}"

PAGES_URL="https://${REPO_OWNER}.github.io/${REPO_NAME}/"
if ! check_url "$PAGES_URL" "GitHub Pages site"; then
  echo -e "${RED}GitHub Pages is not accessible.${NC}"
  echo -e "${YELLOW}Please ensure:${NC}"
  echo -e "${YELLOW}  1. GitHub Pages is enabled in repository settings${NC}"
  echo -e "${YELLOW}  2. Source is set to 'gh-pages' branch${NC}"
  echo -e "${YELLOW}  3. The workflow has completed successfully${NC}"
  echo -e "${YELLOW}  4. Wait a few minutes for GitHub Pages to deploy${NC}"
  echo -e ""
  echo -e "${CYAN}To enable GitHub Pages:${NC}"
  echo -e "  1. Go to: https://github.com/${REPO_OWNER}/${REPO_NAME}/settings/pages"
  echo -e "  2. Under 'Source', select 'Deploy from a branch'"
  echo -e "  3. Choose 'gh-pages' branch and '/ (root)' folder"
  echo -e "  4. Click 'Save'"
  exit 1
fi

# Step 2: Check if Helm repository index is available
echo -e "\n${CYAN}Step 2: Checking Helm repository index${NC}"

INDEX_URL="${HELM_REPO_URL}/index.yaml"
if ! check_url "$INDEX_URL" "Helm repository index"; then
  echo -e "${RED}Helm repository index is not accessible.${NC}"
  echo -e "${YELLOW}The workflow may still be running or needs to be triggered.${NC}"
  exit 1
fi

# Download and check index content
echo -e "${YELLOW}Downloading index.yaml...${NC}"
curl -sL "$INDEX_URL" -o /tmp/right-sizer-index.yaml

if grep -q "right-sizer" /tmp/right-sizer-index.yaml; then
  echo -e "${GREEN}✓ Index contains right-sizer chart${NC}"

  # Extract version information
  CHART_VERSION=$(grep -A1 "version:" /tmp/right-sizer-index.yaml | head -2 | tail -1 | awk '{print $2}')
  echo -e "${CYAN}  Latest version: ${CHART_VERSION}${NC}"
else
  echo -e "${RED}✗ Index does not contain right-sizer chart${NC}"
  exit 1
fi

# Step 3: Add Helm repository
echo -e "\n${CYAN}Step 3: Adding Helm repository${NC}"

# Remove existing repository if it exists
helm repo remove ${HELM_REPO_NAME} 2>/dev/null || true

echo -e "${YELLOW}Adding repository: ${HELM_REPO_URL}${NC}"
if helm repo add ${HELM_REPO_NAME} ${HELM_REPO_URL}; then
  echo -e "${GREEN}✓ Repository added successfully${NC}"
else
  echo -e "${RED}✗ Failed to add repository${NC}"
  exit 1
fi

# Step 4: Update repositories
echo -e "\n${CYAN}Step 4: Updating Helm repositories${NC}"

if helm repo update; then
  echo -e "${GREEN}✓ Repositories updated${NC}"
else
  echo -e "${RED}✗ Failed to update repositories${NC}"
  exit 1
fi

# Step 5: Search for the chart
echo -e "\n${CYAN}Step 5: Searching for right-sizer chart${NC}"

echo -e "${YELLOW}Available charts:${NC}"
helm search repo ${HELM_REPO_NAME} --versions

if helm search repo ${HELM_REPO_NAME}/right-sizer | grep -q right-sizer; then
  echo -e "${GREEN}✓ Chart found in repository${NC}"
else
  echo -e "${RED}✗ Chart not found in repository${NC}"
  exit 1
fi

# Step 6: Show chart information
echo -e "\n${CYAN}Step 6: Getting chart information${NC}"

echo -e "${YELLOW}Chart details:${NC}"
helm show chart ${HELM_REPO_NAME}/right-sizer

# Step 7: Test installation
echo -e "\n${CYAN}Step 7: Testing chart installation${NC}"

if [[ "$DRY_RUN" == true ]]; then
  echo -e "${YELLOW}Running dry-run installation...${NC}"

  if helm install test-right-sizer ${HELM_REPO_NAME}/right-sizer \
    --namespace ${TEST_NAMESPACE} \
    --create-namespace \
    --dry-run \
    --debug >/tmp/right-sizer-dry-run.log 2>&1; then
    echo -e "${GREEN}✓ Dry-run installation successful${NC}"
    echo -e "${CYAN}  Installation would create resources in namespace: ${TEST_NAMESPACE}${NC}"
  else
    echo -e "${RED}✗ Dry-run installation failed${NC}"
    echo -e "${YELLOW}Check /tmp/right-sizer-dry-run.log for details${NC}"
    tail -20 /tmp/right-sizer-dry-run.log
    exit 1
  fi
else
  echo -e "${YELLOW}Installing chart in namespace ${TEST_NAMESPACE}...${NC}"

  if helm install test-right-sizer ${HELM_REPO_NAME}/right-sizer \
    --namespace ${TEST_NAMESPACE} \
    --create-namespace \
    --wait \
    --timeout 5m; then
    echo -e "${GREEN}✓ Chart installed successfully${NC}"

    # Show installation status
    echo -e "\n${YELLOW}Installation status:${NC}"
    helm list -n ${TEST_NAMESPACE}

    echo -e "\n${YELLOW}Pods status:${NC}"
    kubectl get pods -n ${TEST_NAMESPACE}

    # Offer to uninstall
    echo -e "\n${YELLOW}To uninstall the test installation:${NC}"
    echo -e "${CYAN}  helm uninstall test-right-sizer -n ${TEST_NAMESPACE}${NC}"
    echo -e "${CYAN}  kubectl delete namespace ${TEST_NAMESPACE}${NC}"
  else
    echo -e "${RED}✗ Installation failed${NC}"
    exit 1
  fi
fi

# Step 8: Show usage instructions
echo -e "\n${BLUE}========================================${NC}"
echo -e "${GREEN}✅ All tests passed!${NC}"
echo -e "${BLUE}========================================${NC}"

echo -e "\n${CYAN}The Helm repository is working correctly!${NC}"
echo -e "\n${YELLOW}Users can now install right-sizer with:${NC}"
echo -e "${GREEN}  helm repo add ${HELM_REPO_NAME} ${HELM_REPO_URL}${NC}"
echo -e "${GREEN}  helm repo update${NC}"
echo -e "${GREEN}  helm install right-sizer ${HELM_REPO_NAME}/right-sizer --namespace right-sizer --create-namespace${NC}"

echo -e "\n${YELLOW}To view all available versions:${NC}"
echo -e "${GREEN}  helm search repo ${HELM_REPO_NAME} --versions${NC}"

echo -e "\n${YELLOW}To get default values:${NC}"
echo -e "${GREEN}  helm show values ${HELM_REPO_NAME}/right-sizer${NC}"

# Cleanup
rm -f /tmp/right-sizer-index.yaml /tmp/right-sizer-dry-run.log

echo -e "\n${CYAN}Repository URL: ${HELM_REPO_URL}${NC}"
echo -e "${CYAN}Documentation: https://github.com/${REPO_OWNER}/${REPO_NAME}${NC}"
