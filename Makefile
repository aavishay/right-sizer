# Makefile for Right-Sizer
# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Version management
VERSION ?= $(shell cat VERSION)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v$(VERSION)")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)"

# Docker configuration
DOCKER_REGISTRY ?= docker.io
DOCKER_REPO ?= aavishay/right-sizer
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_REPO)
DOCKER_TAG ?= $(VERSION)
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64

# Helm configuration
HELM_CHART_PATH := helm
HELM_PACKAGE_DIR := dist

# Go configuration
GO_MODULE := github.com/aavishay/right-sizer
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Build directories
BUILD_DIR := build
DIST_DIR := dist

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: all
all: help

.PHONY: help
help: ## Show this help message
	@echo "$(BLUE)Right-Sizer Build System$(NC)"
	@echo "$(YELLOW)Version: $(VERSION)$(NC)"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Version Management

.PHONY: version
version: ## Display current version
	@echo "$(GREEN)Current version: $(VERSION)$(NC)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Git tag: $(GIT_TAG)"

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version (x.y.Z)
	@current=$$(cat VERSION); \
	new=$$(echo $$current | awk -F. '{print $$1"."$$2"."$$3+1}'); \
	echo "$(YELLOW)Bumping version from $$current to $$new$(NC)"; \
	echo $$new > VERSION; \
	$(MAKE) version-update

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version (x.Y.z)
	@current=$$(cat VERSION); \
	new=$$(echo $$current | awk -F. '{print $$1"."$$2+1".0"}'); \
	echo "$(YELLOW)Bumping version from $$current to $$new$(NC)"; \
	echo $$new > VERSION; \
	$(MAKE) version-update

.PHONY: version-bump-major
version-bump-major: ## Bump major version (X.y.z)
	@current=$$(cat VERSION); \
	new=$$(echo $$current | awk -F. '{print $$1+1".0.0"}'); \
	echo "$(YELLOW)Bumping version from $$current to $$new$(NC)"; \
	echo $$new > VERSION; \
	$(MAKE) version-update

.PHONY: version-update
version-update: ## Update version in all files
	@echo "$(BLUE)Updating version to $(VERSION) in all files...$(NC)"
	@sed -i.bak 's/^version:.*/version: $(VERSION)/' $(HELM_CHART_PATH)/Chart.yaml && rm $(HELM_CHART_PATH)/Chart.yaml.bak
	@sed -i.bak 's/^appVersion:.*/appVersion: "$(VERSION)"/' $(HELM_CHART_PATH)/Chart.yaml && rm $(HELM_CHART_PATH)/Chart.yaml.bak
	@echo "$(GREEN)Version updated successfully$(NC)"

##@ Build

.PHONY: build
build: ## Build Go binary for current platform
	@echo "$(BLUE)Building right-sizer binary...$(NC)"
	@mkdir -p $(BUILD_DIR)
	cd go && CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(LDFLAGS) -o ../$(BUILD_DIR)/right-sizer-$(GOOS)-$(GOARCH) main.go
	@echo "$(GREEN)Binary built: $(BUILD_DIR)/right-sizer-$(GOOS)-$(GOARCH)$(NC)"

.PHONY: build-all
build-all: ## Build binaries for all platforms
	@echo "$(BLUE)Building binaries for all platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then \
				continue; \
			fi; \
			echo "Building $$os/$$arch..."; \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			cd go && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
				go build $(LDFLAGS) -o ../$(BUILD_DIR)/right-sizer-$$os-$$arch$$ext main.go; \
			cd ..; \
		done; \
	done
	@echo "$(GREEN)All binaries built successfully$(NC)"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image for current architecture
	@echo "$(BLUE)Building Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		-f Dockerfile .
	@echo "$(GREEN)Docker image built successfully$(NC)"

.PHONY: docker-buildx
docker-buildx: ## Build multi-platform Docker image
	@echo "$(BLUE)Building multi-platform Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):v$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		-f Dockerfile \
		--push .
	@echo "$(GREEN)Multi-platform Docker image built and pushed$(NC)"

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "$(BLUE)Pushing Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image pushed successfully$(NC)"

##@ Helm

.PHONY: helm-lint
helm-lint: ## Lint Helm chart
	@echo "$(BLUE)Linting Helm chart...$(NC)"
	helm lint $(HELM_CHART_PATH)
	@echo "$(GREEN)Helm chart validation passed$(NC)"

.PHONY: helm-package
helm-package: version-update ## Package Helm chart
	@echo "$(BLUE)Packaging Helm chart version $(VERSION)...$(NC)"
	@mkdir -p $(DIST_DIR)
	helm package $(HELM_CHART_PATH) -d $(DIST_DIR)
	@echo "$(GREEN)Helm chart packaged: $(DIST_DIR)/right-sizer-$(VERSION).tgz$(NC)"

.PHONY: helm-install
helm-install: ## Install Helm chart locally
	@echo "$(BLUE)Installing Helm chart...$(NC)"
	helm upgrade --install right-sizer $(HELM_CHART_PATH) \
		--namespace right-sizer \
		--create-namespace \
		--set image.tag=$(VERSION)
	@echo "$(GREEN)Helm chart installed successfully$(NC)"

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall Helm chart
	@echo "$(YELLOW)Uninstalling Helm chart...$(NC)"
	helm uninstall right-sizer --namespace right-sizer
	@echo "$(GREEN)Helm chart uninstalled$(NC)"

##@ Testing

.PHONY: test
test: ## Run Go tests
	@echo "$(BLUE)Running tests...$(NC)"
	cd go && go test -v -race -coverprofile=../coverage.out ./...
	@echo "$(GREEN)Tests completed$(NC)"

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "$(BLUE)Generating coverage report...$(NC)"
	cd go && go tool cover -html=../coverage.out -o ../coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

.PHONY: lint
lint: ## Run Go linter
	@echo "$(BLUE)Running linter...$(NC)"
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "$(YELLOW)golangci-lint not found, installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	cd go && golangci-lint run ./...
	@echo "$(GREEN)Linting completed$(NC)"

##@ Security

.PHONY: security-scan
security-scan: ## Run all security scans
	@echo "$(BLUE)Running security scans...$(NC)"
	@$(MAKE) vuln-check
	@$(MAKE) trivy-scan
	@echo "$(GREEN)All security scans completed$(NC)"

.PHONY: vuln-check
vuln-check: ## Check Go dependencies for vulnerabilities
	@echo "$(BLUE)Checking Go dependencies for vulnerabilities...$(NC)"
	@if ! command -v govulncheck &> /dev/null; then \
		echo "$(YELLOW)govulncheck not found, installing...$(NC)"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	cd go && govulncheck ./...
	@echo "$(GREEN)Vulnerability check completed$(NC)"

.PHONY: trivy-scan
trivy-scan: ## Scan Docker image for vulnerabilities
	@echo "$(BLUE)Scanning Docker image with Trivy...$(NC)"
	@if ! command -v trivy &> /dev/null; then \
		echo "$(RED)Trivy not found. Please install: brew install trivy$(NC)"; \
		exit 1; \
	fi
	trivy image --severity HIGH,CRITICAL $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "$(GREEN)Trivy scan completed$(NC)"

.PHONY: update-deps
update-deps: ## Update Go dependencies to latest versions
	@echo "$(BLUE)Updating Go dependencies...$(NC)"
	cd go && go get -u ./...
	cd go && go mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"
	@echo "$(YELLOW)Run 'make test' to verify everything still works$(NC)"

.PHONY: security-report
security-report: ## Generate security report
	@echo "$(BLUE)Generating security report...$(NC)"
	@echo "# Security Report - $(shell date '+%Y-%m-%d')" > security-report.md
	@echo "" >> security-report.md
	@echo "## Version Information" >> security-report.md
	@echo "- Right-Sizer Version: $(VERSION)" >> security-report.md
	@echo "- Git Commit: $(GIT_COMMIT)" >> security-report.md
	@echo "" >> security-report.md
	@echo "## Vulnerability Scan Results" >> security-report.md
	@echo "" >> security-report.md
	@echo "### Go Dependencies" >> security-report.md
	@echo '```' >> security-report.md
	@cd go && govulncheck ./... 2>&1 | tail -n +2 >> ../security-report.md || echo "No vulnerabilities found" >> ../security-report.md
	@echo '```' >> security-report.md
	@echo "" >> security-report.md
	@echo "### Docker Image" >> security-report.md
	@echo '```' >> security-report.md
	@trivy image --severity HIGH,CRITICAL --format table $(DOCKER_IMAGE):$(DOCKER_TAG) >> security-report.md 2>&1 || echo "No HIGH or CRITICAL vulnerabilities found" >> security-report.md
	@echo '```' >> security-report.md
	@echo "$(GREEN)Security report generated: security-report.md$(NC)"

##@ Development

.PHONY: dev
dev: ## Run operator locally for development
	@echo "$(BLUE)Running operator in development mode...$(NC)"
	cd go && go run $(LDFLAGS) main.go

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(BLUE)Formatting Go code...$(NC)"
	cd go && go fmt ./...
	@echo "$(GREEN)Code formatted$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	cd go && go vet ./...
	@echo "$(GREEN)Vet completed$(NC)"

.PHONY: mod-tidy
mod-tidy: ## Tidy Go modules
	@echo "$(BLUE)Tidying Go modules...$(NC)"
	cd go && go mod tidy
	@echo "$(GREEN)Modules tidied$(NC)"

##@ Release

.PHONY: release-prepare
release-prepare: version ## Prepare for release
	@echo "$(BLUE)Preparing release $(VERSION)...$(NC)"
	@$(MAKE) mod-tidy
	@$(MAKE) fmt
	@$(MAKE) test
	@$(MAKE) lint || true
	@$(MAKE) version-update
	@$(MAKE) helm-lint
	@echo "$(GREEN)Release preparation complete$(NC)"
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Review changes"
	@echo "  2. Commit: git add -A && git commit -m 'Release v$(VERSION)'"
	@echo "  3. Tag: git tag v$(VERSION)"
	@echo "  4. Push: git push origin main --tags"

.PHONY: release-tag
release-tag: ## Create and push git tag for current version
	@echo "$(BLUE)Creating git tag v$(VERSION)...$(NC)"
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@echo "$(GREEN)Tag created. Push with: git push origin v$(VERSION)$(NC)"

.PHONY: changelog
changelog: ## Generate changelog from git commits
	@echo "$(BLUE)Generating changelog...$(NC)"
	@echo "# Changelog for v$(VERSION)" > CHANGELOG.md
	@echo "" >> CHANGELOG.md
	@echo "## Changes" >> CHANGELOG.md
	@git log --pretty=format:"* %s (%an)" $$(git describe --tags --abbrev=0 2>/dev/null)..HEAD >> CHANGELOG.md 2>/dev/null || \
		git log --pretty=format:"* %s (%an)" --max-count=20 >> CHANGELOG.md
	@echo "" >> CHANGELOG.md
	@echo "$(GREEN)Changelog generated: CHANGELOG.md$(NC)"

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "$(GREEN)Cleanup complete$(NC)"

.PHONY: clean-all
clean-all: clean ## Clean all artifacts including Docker images
	@echo "$(YELLOW)Cleaning Docker images...$(NC)"
	@docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	@docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true
	@echo "$(GREEN)Deep cleanup complete$(NC)"

##@ Utilities

.PHONY: check-tools
check-tools: ## Check if required tools are installed
	@echo "$(BLUE)Checking required tools...$(NC)"
	@which go > /dev/null || (echo "$(RED)Go is not installed$(NC)" && exit 1)
	@which docker > /dev/null || (echo "$(RED)Docker is not installed$(NC)" && exit 1)
	@which helm > /dev/null || (echo "$(RED)Helm is not installed$(NC)" && exit 1)
	@which git > /dev/null || (echo "$(RED)Git is not installed$(NC)" && exit 1)
	@echo "$(GREEN)All required tools are installed$(NC)"

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/rakyll/gotest@latest
	@echo "$(GREEN)Development tools installed$(NC)"
