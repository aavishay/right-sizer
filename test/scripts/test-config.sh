#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later


# Test script to verify environment variable configuration for right-sizer operator

echo "========================================="
echo "Testing Right-Sizer Configuration"
echo "========================================="
echo ""

# Set test environment variables
export CPU_REQUEST_MULTIPLIER=1.5
export MEMORY_REQUEST_MULTIPLIER=1.3
export CPU_LIMIT_MULTIPLIER=3.0
export MEMORY_LIMIT_MULTIPLIER=2.5
export MAX_CPU_LIMIT=8000
export MAX_MEMORY_LIMIT=16384
export MIN_CPU_REQUEST=20
export MIN_MEMORY_REQUEST=128
export METRICS_PROVIDER=prometheus
export PROMETHEUS_URL=http://prometheus-test:9090
export ENABLE_INPLACE_RESIZE=true
export DRY_RUN=true

echo "Environment variables set:"
echo "  CPU_REQUEST_MULTIPLIER=$CPU_REQUEST_MULTIPLIER"
echo "  MEMORY_REQUEST_MULTIPLIER=$MEMORY_REQUEST_MULTIPLIER"
echo "  CPU_LIMIT_MULTIPLIER=$CPU_LIMIT_MULTIPLIER"
echo "  MEMORY_LIMIT_MULTIPLIER=$MEMORY_LIMIT_MULTIPLIER"
echo "  MAX_CPU_LIMIT=$MAX_CPU_LIMIT"
echo "  MAX_MEMORY_LIMIT=$MAX_MEMORY_LIMIT"
echo "  MIN_CPU_REQUEST=$MIN_CPU_REQUEST"
echo "  MIN_MEMORY_REQUEST=$MIN_MEMORY_REQUEST"
echo "  METRICS_PROVIDER=$METRICS_PROVIDER"
echo "  PROMETHEUS_URL=$PROMETHEUS_URL"
echo "  ENABLE_INPLACE_RESIZE=$ENABLE_INPLACE_RESIZE"
echo "  DRY_RUN=$DRY_RUN"
echo ""
echo "========================================="
echo "Starting operator (will exit after showing config)..."
echo "========================================="
echo ""

# Run the operator briefly to see config loading
# We'll use timeout to stop it after a few seconds since it will try to connect to k8s
timeout 3 ./right-sizer 2>&1 | head -30

echo ""
echo "========================================="
echo "Test completed!"
echo "========================================="
echo ""
echo "Configuration loading test finished."
echo "Check the output above to verify that the environment variables were loaded correctly."
echo ""
echo "Expected to see lines like:"
echo "  CPU_REQUEST_MULTIPLIER set to: 1.50"
echo "  MEMORY_REQUEST_MULTIPLIER set to: 1.30"
echo "  CPU_LIMIT_MULTIPLIER set to: 3.00"
echo "  MEMORY_LIMIT_MULTIPLIER set to: 2.50"
echo "  MAX_CPU_LIMIT set to: 8000 millicores"
echo "  MAX_MEMORY_LIMIT set to: 16384 MB"
echo "  MIN_CPU_REQUEST set to: 20 millicores"
echo "  MIN_MEMORY_REQUEST set to: 128 MB"
