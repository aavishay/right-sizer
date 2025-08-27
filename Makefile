# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

.PHONY: all build clean test fmt lint docker help

# Variables
BINARY_NAME := right-sizer
IMAGE_NAME := right-sizer
IMAGE_TAG := latest
GO := go
DOCKER := docker
KUBECTL := kubectl

# Build variables
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.Version=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

all: build ## Build everything

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) main.go

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -rf bin/ dist/ vendor/

test: ## Run tests
	@echo "Running tests..."
	$(GO) test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...
	$(GO) mod tidy

lint: ## Run linters
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run

docker: ## Build Docker image
	@echo "Building Docker image..."
	$(DOCKER) build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-push: docker ## Push Docker image
	@echo "Pushing Docker image..."
	$(DOCKER) push $(IMAGE_NAME):$(IMAGE_TAG)

deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	$(KUBECTL) apply -f deploy/kubernetes/

undeploy: ## Remove from Kubernetes
	@echo "Removing from Kubernetes..."
	$(KUBECTL) delete -f deploy/kubernetes/

run: build ## Build and run locally
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

install: build ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install

vendor: ## Download dependencies to vendor/
	@echo "Vendoring dependencies..."
	$(GO) mod vendor

verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GO) mod verify

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
