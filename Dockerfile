# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Build stage
# Using golang base image - alternatives if Docker Hub is down:
# - gcr.io/distroless/base:latest (Google)
# - public.ecr.aws/docker/library/golang:1.25-alpine (AWS)
# - quay.io/projectquay/golang:1.25-alpine (Red Hat)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_DATE
ARG GIT_COMMIT

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go/go.mod go/go.sum ./

# Download dependencies - this layer is cached as long as go.mod/go.sum don't change
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download && \
  go mod verify

# Copy source code
COPY go/ .

# Build the binary with caching
# Use cache mounts for faster builds
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
  go build -a -installsuffix cgo \
  -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
  -o right-sizer main.go && \
  chmod +x right-sizer

# Final stage - use distroless for minimal size and security
# Alternative base images if gcr.io is unavailable:
# - alpine:3.18 (requires adding ca-certificates)
# - scratch (most minimal, requires copying ca-certificates from builder)
FROM gcr.io/distroless/static-debian12:nonroot

# Re-declare build args for final stage label expansion
ARG VERSION=dev
ARG BUILD_DATE
ARG GIT_COMMIT

# Labels for OCI compliance
LABEL org.opencontainers.image.title="Right-Sizer" \
  org.opencontainers.image.description="Kubernetes operator for automatic pod resource right-sizing" \
  org.opencontainers.image.vendor="Right-Sizer Contributors" \
  org.opencontainers.image.licenses="AGPL-3.0-or-later" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.revision="${GIT_COMMIT}" \
  org.opencontainers.image.created="${BUILD_DATE}"

# Copy the binary from builder
COPY --from=builder /build/right-sizer /app/right-sizer

# Use nonroot user (already set in base image)
USER nonroot:nonroot

WORKDIR /app

# Expose metrics and health check port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/right-sizer", "--health-check"]

# Run the binary
ENTRYPOINT ["/app/right-sizer"]
