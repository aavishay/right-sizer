# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git (needed for go mod)
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o right-sizer main.go

# Final stage
FROM alpine:3.18

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/right-sizer .

# Create non-root user
RUN adduser -D -s /bin/sh appuser
USER appuser

# Expose health check port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./right-sizer"]
