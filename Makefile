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

##@ Testing

.PHONY: test-k8s-compliance
test-k8s-compliance: ## Run K8s 1.33+ in-place resize compliance check
	@echo "$(BLUE)Running Kubernetes 1.33+ In-Place Resize Compliance Check...$(NC)"
	@./scripts/check-k8s-compliance.sh

.PHONY: test-compliance-integration
test-compliance-integration: ## Run comprehensive K8s compliance integration tests
	@echo "$(BLUE)Running K8s compliance integration tests...$(NC)"
	cd tests/integration && go test -v -tags=integration -run TestK8sSpecCompliance -timeout=30m

.PHONY: test-inplace-resize
test-inplace-resize: ## Run in-place resize compliance tests
	@echo "$(BLUE)Running in-place resize compliance tests...$(NC)"
	cd tests/integration && go test -v -tags=integration -run TestK8sInPlaceResizeCompliance -timeout=20m

.PHONY: test-resize-policy
test-resize-policy: ## Run resize policy validation unit tests
	@echo "$(BLUE)Running resize policy validation tests...$(NC)"
	cd tests/unit && go test -v -run TestResizePolicyValidation

.PHONY: test-qos-validation
test-qos-validation: ## Run QoS class preservation tests
	@echo "$(BLUE)Running QoS class preservation tests...$(NC)"
	cd tests/unit && go test -v -run TestQoSClassPreservationValidation

.PHONY: test-compliance-full
test-compliance-full: test-k8s-compliance test-resize-policy test-qos-validation test-inplace-resize test-compliance-integration ## Run full K8s compliance test suite
	@echo "$(GREEN)✅ Full K8s compliance test suite completed$(NC)"

.PHONY: test-unit
test-unit: ## Run all unit tests
	@echo "$(BLUE)Running unit tests...$(NC)"
	cd go && go test -v ./... -race -coverprofile=coverage.out
	cd tests/unit && go test -v ./...

.PHONY: test-integration
test-integration: ## Run all integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	cd tests/integration && go test -v -tags=integration ./... -timeout=30m

.PHONY: test-all
test-all: test-unit test-integration test-compliance-full ## Run all tests including compliance checks
	@echo "$(GREEN)✅ All tests completed$(NC)"

##@ Development Tools

.PHONY: check-k8s-prereqs
check-k8s-prereqs: ## Check K8s cluster prerequisites for in-place resizing
	@echo "$(BLUE)Checking Kubernetes prerequisites...$(NC)"
	@echo "Kubernetes version:"
	@kubectl version --short 2>/dev/null || echo "$(RED)❌ kubectl not available$(NC)"
	@echo "Checking for resize subresource support..."
	@kubectl api-resources --subresource=resize 2>/dev/null | grep -q resize && echo "$(GREEN)✅ Resize subresource supported$(NC)" || echo "$(YELLOW)⚠️  Resize subresource may not be supported$(NC)"

.PHONY: create-test-pod
create-test-pod: ## Create a test pod for manual resize testing
	@echo "$(BLUE)Creating test pod for resize testing...$(NC)"
	@kubectl apply -f - <<EOF || true
	apiVersion: v1
	kind: Pod
	metadata:
	  name: resize-test-pod
	  namespace: default
	spec:
	  containers:
	  - name: test-container
	    image: registry.k8s.io/pause:3.8
	    resources:
	      requests:
	        cpu: "100m"
	        memory: "128Mi"
	      limits:
	        cpu: "200m"
	        memory: "256Mi"
	    resizePolicy:
	    - resourceName: cpu
	      restartPolicy: NotRequired
	    - resourceName: memory
	      restartPolicy: NotRequired
	EOF
	@echo "$(GREEN)✅ Test pod created. Test resize with:$(NC)"
	@echo "kubectl patch pod resize-test-pod --subresource resize --patch '{\"spec\":{\"containers\":[{\"name\":\"test-container\", \"resources\":{\"requests\":{\"cpu\":\"150m\"}, \"limits\":{\"cpu\":\"300m\"}}}]}}'"

.PHONY: cleanup-test-resources
cleanup-test-resources: ## Clean up test resources
	@echo "$(BLUE)Cleaning up test resources...$(NC)"
	@kubectl delete pod resize-test-pod --ignore-not-found=true
	@kubectl delete namespace rightsizer-compliance-test k8s-resize-compliance-test k8s-spec-compliance-test --ignore-not-found=true
	@echo "$(GREEN)✅ Test resources cleaned up$(NC)"

##@ Compliance Reporting

.PHONY: generate-compliance-report
generate-compliance-report: ## Generate detailed compliance report
	@echo "$(BLUE)Generating K8s compliance report...$(NC)"
	@./scripts/check-k8s-compliance.sh > compliance-report-$(shell date +%Y%m%d-%H%M%S).txt
	@echo "$(GREEN)✅ Compliance report generated$(NC)"

.PHONY: validate-implementation
validate-implementation: ## Validate current implementation against K8s spec
	@echo "$(BLUE)Validating implementation against K8s 1.33+ spec...$(NC)"
	@echo "Checking resize subresource usage in code..."
	@grep -r "resize.*subresource\|SubResource.*resize" go/ && echo "$(GREEN)✅ Found resize subresource usage$(NC)" || echo "$(RED)❌ No resize subresource usage found$(NC)"
	@echo "Checking for status condition handling..."
	@grep -r "PodResizePending\|PodResizeInProgress" go/ && echo "$(GREEN)✅ Found status condition handling$(NC)" || echo "$(YELLOW)⚠️  No status condition handling found$(NC)"
	@echo "Checking for QoS validation..."
	@grep -r "QOSClass\|qos.*validation" go/ && echo "$(GREEN)✅ Found QoS handling$(NC)" || echo "$(YELLOW)⚠️  Limited QoS validation found$(NC)"

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
docker-build: ## Build multi-platform Docker image
	@echo "$(BLUE)Building multi-platform Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	docker buildx create --use --name right-sizer-builder 2>/dev/null || true
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		-f Dockerfile \
		--load .
	@echo "$(GREEN)Multi-platform Docker image built successfully$(NC)"

.PHONY: docker-buildx
docker-buildx: ## Build and push multi-platform Docker image
	@echo "$(BLUE)Building and pushing multi-platform Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	docker buildx create --use --name right-sizer-builder 2>/dev/null || true
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

##@ Testing

.PHONY: test
test: ## Run all tests
	@echo "$(BLUE)Running tests...$(NC)"
	cd go && go test -v -race ./...
	@echo "$(GREEN)Tests completed$(NC)"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@mkdir -p $(BUILD_DIR)/coverage
	cd go && go test -v -race -coverprofile=../$(BUILD_DIR)/coverage/coverage.out -covermode=atomic ./...
	cd go && go tool cover -func=../$(BUILD_DIR)/coverage/coverage.out -o ../$(BUILD_DIR)/coverage/coverage.txt
	@echo "$(GREEN)Coverage report generated$(NC)"
	@echo ""
	@echo "$(YELLOW)Coverage Summary:$(NC)"
	@tail -n 1 $(BUILD_DIR)/coverage/coverage.txt

.PHONY: test-coverage-html
test-coverage-html: test-coverage ## Generate HTML coverage report
	@echo "$(BLUE)Generating HTML coverage report...$(NC)"
	cd go && go tool cover -html=../$(BUILD_DIR)/coverage/coverage.out -o ../$(BUILD_DIR)/coverage/coverage.html
	@echo "$(GREEN)HTML report generated: $(BUILD_DIR)/coverage/coverage.html$(NC)"
	@echo "$(YELLOW)Opening coverage report...$(NC)"
	@if command -v open > /dev/null; then \
		open $(BUILD_DIR)/coverage/coverage.html; \
	elif command -v xdg-open > /dev/null; then \
		xdg-open $(BUILD_DIR)/coverage/coverage.html; \
	else \
		echo "Please open $(BUILD_DIR)/coverage/coverage.html in your browser"; \
	fi

.PHONY: test-benchmark
test-benchmark: ## Run benchmark tests
	@echo "$(BLUE)Running benchmarks...$(NC)"
	cd go && go test -bench=. -benchmem ./...
	@echo "$(GREEN)Benchmarks completed$(NC)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	cd go && go test -v -tags=integration ./...
	@echo "$(GREEN)Integration tests completed$(NC)"

.PHONY: test-lint
test-lint: ## Run linting checks
	@echo "$(BLUE)Running linting checks...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		cd go && golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint not installed, installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		cd go && golangci-lint run ./...; \
	fi
	@echo "$(GREEN)Linting completed$(NC)"

.PHONY: test-all
test-all: test-lint test test-coverage ## Run all tests, linting, and coverage
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

.PHONY: prune-archive
prune-archive: ## Interactively prune contents of archive/ directory
	@echo "$(YELLOW)Pruning archive directory (interactive)...$(NC)"
	@if [ ! -d archive ]; then echo "archive/ directory not found (nothing to prune)"; exit 0; fi
	@echo "Contents to remove:" && ls -1 archive || true
	@read -p "This will permanently delete files inside archive/. Continue? [y/N] " ans; \
	  if [ "$$ans" = "y" ] || [ "$$ans" = "Y" ]; then \
	    rm -rf archive/*; \
	    echo "$(GREEN)Archive directory pruned$(NC)"; \
	  else \
	    echo "Aborted"; \
	  fi

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

##@ Local E2E / Minikube

.PHONY: mk-start
mk-start: ## Start (or ensure) a minikube cluster for local testing
	@echo "$(BLUE)Starting minikube (profile: right-sizer)...$(NC)"
	minikube start -p right-sizer --kubernetes-version=stable --cpus=4 --memory=6144
	@echo "$(GREEN)Minikube started$(NC)"

.PHONY: mk-enable-metrics
mk-enable-metrics: ## Enable metrics-server addon in minikube
	@echo "$(BLUE)Enabling metrics-server addon...$(NC)"
	minikube -p right-sizer addons enable metrics-server
	@echo "$(GREEN)metrics-server enabled (it may take ~30s to become Ready)$(NC)"

.PHONY: mk-build-image
mk-build-image: ## Build multi-platform operator image inside minikube Docker daemon
	@echo "$(BLUE)Building multi-platform image inside minikube Docker daemon...$(NC)"
	eval $$(minikube -p right-sizer docker-env) && \
	  docker buildx create --use --name minikube-builder --driver docker-container 2>/dev/null || true && \
	  docker buildx build \
	    --platform linux/amd64,linux/arm64 \
	    --build-arg VERSION=$(VERSION) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) \
	    --build-arg BUILD_DATE=$(BUILD_DATE) \
	    -t right-sizer:test \
	    -f Dockerfile \
	    --load .
	@echo "$(GREEN)Multi-platform image right-sizer:test built inside minikube$(NC)"

.PHONY: mk-deploy
mk-deploy: mk-start mk-build-image ## Deploy Helm chart using locally built image
	@echo "$(BLUE)Deploying Helm chart to minikube...$(NC)"
	helm upgrade --install right-sizer ./helm \
	  -n right-sizer --create-namespace \
	  --set image.repository=right-sizer \
	  --set image.tag=test \
	  --set image.pullPolicy=IfNotPresent
	kubectl wait --for=condition=available deployment/right-sizer -n right-sizer --timeout=120s
	@echo "$(GREEN)Deployment available$(NC)"

.PHONY: mk-policy
mk-policy: ## Apply sample RightSizerPolicy and demo workload
	@echo "$(BLUE)Applying demo workload & policy...$(NC)"
	./hack/apply-demo.sh
	@echo "$(GREEN)Demo workload & policy applied$(NC)"

.PHONY: mk-port-forward
mk-port-forward: ## Port-forward operator (health:8081, metrics:9090, controller-runtime:8080)
	@echo "$(BLUE)Starting port-forward (Ctrl+C to stop)...$(NC)"
	kubectl -n right-sizer port-forward deploy/right-sizer 8081:8081 9090:9090 8080:8080

.PHONY: mk-logs
mk-logs: ## Tail operator logs
	kubectl logs -n right-sizer -f deploy/right-sizer

.PHONY: mk-status
mk-status: ## Show quick status (pods & policies)
	@echo "$(BLUE)Operator status:$(NC)"
	kubectl get pods -n right-sizer
	@echo ""
	@echo "$(BLUE)Policies:$(NC)"
	-kubectl get rightsizerpolicies -A || true

.PHONY: mk-test
mk-test: mk-deploy mk-enable-metrics mk-policy ## Full local e2e (cluster → image → deploy → policy)
	@echo "$(BLUE)Waiting briefly for metrics-server (15s)...$(NC)"; sleep 15
	$(MAKE) mk-status
	@echo ""
	@echo "$(BLUE)Recent operator logs:$(NC)"
	kubectl logs -n right-sizer deploy/right-sizer --tail=40
	@echo "$(GREEN)Local e2e sequence completed$(NC)"

.PHONY: mk-clean
mk-clean: ## Remove demo namespaces & uninstall operator (keeps cluster)
	@echo "$(YELLOW)Cleaning demo resources...$(NC)"
	-helm uninstall right-sizer -n right-sizer 2>/dev/null || true
	-kubectl delete ns rs-demo 2>/dev/null || true
	@echo "$(GREEN)Demo resources removed$(NC)"

.PHONY: mk-destroy
mk-destroy: mk-clean ## Delete entire minikube profile
	@echo "$(YELLOW)Deleting minikube profile 'right-sizer'...$(NC)"
	minikube delete -p right-sizer
	@echo "$(GREEN)Minikube profile deleted$(NC)"

.PHONY: local-e2e
local-e2e: mk-test ## Alias for mk-test
	@echo "$(GREEN)local-e2e completed$(NC)"
