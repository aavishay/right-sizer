# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (needed for go mod)
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go/go.mod go/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY go/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o right-sizer main.go

# Final stage
FROM alpine:3.18

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user first
RUN adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy the binary from builder stage and ensure it's executable
COPY --from=builder --chown=appuser:appuser /app/right-sizer /app/right-sizer
RUN chmod +x /app/right-sizer

# Switch to non-root user
USER appuser

# Expose health check port
EXPOSE 8081

# Run the binary
ENTRYPOINT ["/app/right-sizer"]
