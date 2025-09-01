#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Quick setup script for GitHub secrets
# Run this after: gh auth login

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}=================================${NC}"
echo -e "${BLUE}GitHub Secrets Quick Setup${NC}"
echo -e "${BLUE}=================================${NC}"
echo ""

# Check if gh is authenticated
if ! gh auth status &>/dev/null; then
  echo -e "${RED}✗ GitHub CLI not authenticated!${NC}"
  echo ""
  echo "Please run: gh auth login"
  echo ""
  exit 1
fi

# Get repository
REPO=$(git remote get-url origin 2>/dev/null | sed 's/.*github.com[:/]\(.*\)\.git/\1/' || echo "")
if [ -z "$REPO" ]; then
  echo -e "${YELLOW}Enter repository (format: owner/repo):${NC}"
  read -r REPO
fi

echo -e "${CYAN}Repository: $REPO${NC}"
echo ""

# Docker Hub Credentials
echo -e "${YELLOW}=== Docker Hub Credentials ===${NC}"
echo "These are REQUIRED for Docker image builds"
echo ""

echo -n "Docker Hub Username: "
read -r DOCKER_USERNAME

echo -n "Docker Hub Password/Token: "
read -rs DOCKER_PASSWORD
echo ""

if [ -z "$DOCKER_USERNAME" ] || [ -z "$DOCKER_PASSWORD" ]; then
  echo -e "${RED}✗ Docker credentials are required!${NC}"
  exit 1
fi

# Set the secrets
echo ""
echo -e "${BLUE}Setting GitHub Secrets...${NC}"

# Set Docker secrets
echo "$DOCKER_USERNAME" | gh secret set DOCKER_USERNAME --repo="$REPO" 2>/dev/null &&
  echo -e "${GREEN}✓ DOCKER_USERNAME set${NC}" ||
  echo -e "${RED}✗ Failed to set DOCKER_USERNAME${NC}"

echo "$DOCKER_PASSWORD" | gh secret set DOCKER_PASSWORD --repo="$REPO" 2>/dev/null &&
  echo -e "${GREEN}✓ DOCKER_PASSWORD set${NC}" ||
  echo -e "${RED}✗ Failed to set DOCKER_PASSWORD${NC}"

# Mirror for Docker Hub (some workflows use these names)
echo "$DOCKER_USERNAME" | gh secret set DOCKERHUB_USERNAME --repo="$REPO" 2>/dev/null &&
  echo -e "${GREEN}✓ DOCKERHUB_USERNAME set${NC}" ||
  echo -e "${RED}✗ Failed to set DOCKERHUB_USERNAME${NC}"

echo "$DOCKER_PASSWORD" | gh secret set DOCKERHUB_TOKEN --repo="$REPO" 2>/dev/null &&
  echo -e "${GREEN}✓ DOCKERHUB_TOKEN set${NC}" ||
  echo -e "${RED}✗ Failed to set DOCKERHUB_TOKEN${NC}"

# Optional: Codecov
echo ""
echo -e "${YELLOW}=== Optional: Codecov Token ===${NC}"
echo "Get token from: https://codecov.io/gh/$REPO/settings"
echo -n "Codecov Token (press Enter to skip): "
read -rs CODECOV_TOKEN
echo ""

if [ -n "$CODECOV_TOKEN" ]; then
  echo "$CODECOV_TOKEN" | gh secret set CODECOV_TOKEN --repo="$REPO" 2>/dev/null &&
    echo -e "${GREEN}✓ CODECOV_TOKEN set${NC}" ||
    echo -e "${RED}✗ Failed to set CODECOV_TOKEN${NC}"
fi

# Verify secrets
echo ""
echo -e "${BLUE}=== Verifying Secrets ===${NC}"
echo ""

SECRETS=$(gh secret list --repo="$REPO" 2>/dev/null | awk '{print $1}')

for secret in DOCKER_USERNAME DOCKER_PASSWORD DOCKERHUB_USERNAME DOCKERHUB_TOKEN; do
  if echo "$SECRETS" | grep -q "^$secret$"; then
    echo -e "${GREEN}✓ $secret is configured${NC}"
  else
    echo -e "${RED}✗ $secret is missing${NC}"
  fi
done

if echo "$SECRETS" | grep -q "^CODECOV_TOKEN$"; then
  echo -e "${GREEN}✓ CODECOV_TOKEN is configured (optional)${NC}"
fi

# Instructions
echo ""
echo -e "${BLUE}=== Next Steps ===${NC}"
echo ""
echo "1. Push to GitHub to trigger workflows:"
echo -e "   ${CYAN}git push origin main${NC}"
echo ""
echo "2. Check workflow status:"
echo -e "   ${CYAN}gh run list --repo=$REPO${NC}"
echo ""
echo "3. View workflow logs:"
echo -e "   ${CYAN}gh run view --repo=$REPO${NC}"
echo ""
echo "4. Your Docker images will be available at:"
echo -e "   ${CYAN}docker.io/aavishay/right-sizer${NC}"
echo ""
echo -e "${GREEN}✓ Setup complete!${NC}"
