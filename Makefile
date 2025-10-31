VERSION ?= $(shell cat VERSION)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v$(VERSION)")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)"

DOCKER_REGISTRY ?= docker.io
DOCKER_REPO ?= aavishay/rightsizer
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_REPO)
DOCKER_TAG ?= $(VERSION)
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64

HELM_CHART_PATH := helm
HELM_PACKAGE_DIR := dist

GO_MODULE := github.com/aavishay/rightsizer
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

BUILD_DIR := build
DIST_DIR := dist

RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m

.PHONY: mk-start
mk-start:
	@echo "$(BLUE)Starting minikube (profile: rightsizer)...$(NC)"
	minikube start -p rightsizer --kubernetes-version=stable --cpus=4 --memory=6144
	@echo "$(GREEN)Minikube started$(NC)"

.PHONY: mk-enable-metrics
mk-enable-metrics:
	@echo "$(BLUE)Enabling metrics-server addon...$(NC)"
	minikube -p rightsizer addons enable metrics-server
	@echo "$(GREEN)metrics-server enabled (it may take ~30s to become Ready)$(NC)"

.PHONY: mk-build-image
mk-build-image:
	@echo "$(BLUE)Building multi-platform image inside minikube Docker daemon...$(NC)"
	eval $$(minikube -p rightsizer docker-env) && \
	  docker buildx create --use --name minikube-builder --driver docker-container 2>/dev/null || true && \
	  docker buildx build \
	    --platform linux/arm64 \
	    --build-arg VERSION=$(VERSION) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) \
	    --build-arg BUILD_DATE=$(BUILD_DATE) \
	    -t rightsizer:test \
	    -f Dockerfile \
	    --load .
	@echo "$(GREEN)Multi-platform image rightsizer:test built inside minikube$(NC)"

.PHONY: mk-deploy
mk-deploy: mk-start mk-build-image
	@echo "$(BLUE)Deploying Helm chart to minikube...$(NC)"
	helm upgrade --install rightsizer ./helm \
	  -n rightsizer --create-namespace \
	  --set image.repository=rightsizer \
	  --set image.tag=test \
	  --set image.pullPolicy=IfNotPresent
	kubectl wait --for=condition=available deployment/rightsizer -n rightsizer --timeout=120s
	@echo "$(GREEN)Deployment available$(NC)"

.PHONY: mk-policy
mk-policy:
	@echo "$(BLUE)Applying demo workload & policy...$(NC)"
	kubectl apply -f k8s/demo-workload.yaml
	kubectl apply -f examples/rightsizerconfig-full.yaml
	@echo "$(GREEN)Demo workload & policy applied$(NC)"

.PHONY: mk-status
mk-status:
	@echo "$(BLUE)Operator status:$(NC)"
	kubectl get pods -n rightsizer
	@echo ""
	@echo "$(BLUE)Policies:$(NC)"
	-kubectl get rightsizerpolicies -A || true

.PHONY: mk-test
mk-test: mk-deploy mk-enable-metrics mk-policy
	@echo "$(BLUE)Waiting briefly for metrics-server (15s)...$(NC)"; sleep 15
	$(MAKE) mk-status
	@echo ""
	@echo "$(BLUE)Recent operator logs:$(NC)"
	kubectl logs -n rightsizer deploy/rightsizer --tail=40
	@echo "$(GREEN)Local e2e sequence completed$(NC)"

.PHONY: mk-clean
mk-clean:
	@echo "$(YELLOW)Cleaning demo resources...$(NC)"
	-helm uninstall rightsizer -n rightsizer 2>/dev/null || true
	-kubectl delete ns rs-demo 2>/dev/null || true
	@echo "$(GREEN)Demo resources removed$(NC)"

.PHONY: mk-destroy
mk-destroy: mk-clean
	@echo "$(YELLOW)Deleting minikube profile 'rightsizer'...$(NC)"
	minikube delete -p rightsizer
	@echo "$(GREEN)Minikube profile deleted$(NC)"

# Testing and Coverage Targets

.PHONY: test
test:
	@echo "$(BLUE)Running unit tests...$(NC)"
	cd go && go test -race -v ./...
	@echo "$(GREEN)✅ All tests passed$(NC)"

.PHONY: test-coverage
test-coverage:
	@echo "$(BLUE)Running unit tests with coverage analysis (90% required)...$(NC)"
	./scripts/run-coverage-test.sh

.PHONY: test-coverage-quick
test-coverage-quick:
	@echo "$(BLUE)Running coverage check (quick mode)...$(NC)"
	cd go && go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | grep total
	@echo "$(GREEN)Coverage summary complete$(NC)"

.PHONY: coverage-html
coverage-html: test-coverage-quick
	@echo "$(BLUE)Generating HTML coverage report...$(NC)"
	@mkdir -p build/coverage
	@cd go && go tool cover -html=coverage.out -o ../build/coverage/coverage.html
	@echo "$(GREEN)✅ Report generated: build/coverage/coverage.html$(NC)"
	@echo "Opening report..."
	@open build/coverage/coverage.html || xdg-open build/coverage/coverage.html || echo "Please open build/coverage/coverage.html manually"

.PHONY: setup-precommit
setup-precommit:
	@echo "$(BLUE)Setting up pre-commit hooks...$(NC)"
	./scripts/setup-precommit.sh

.PHONY: pre-commit-run
pre-commit-run:
	@echo "$(BLUE)Running all pre-commit hooks...$(NC)"
	pre-commit run --all-files

.PHONY: pre-commit-skip
pre-commit-skip:
	@echo "$(YELLOW)Skipping pre-commit hooks...$(NC)"
	@echo "Set environment: SKIP=go-test-coverage git commit -m 'message'"
