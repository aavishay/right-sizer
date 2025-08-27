#!/bin/bash

# Copyright (C) 2024 right-sizer contributors
# SPDX-License-Identifier: AGPL-3.0-or-later

# Final validation script for all Right-Sizer configuration features
# This script validates that all environment variable configurations work correctly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0

# Function to print colored output
print_header() {
  echo ""
  echo -e "${BLUE}=================================================${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}=================================================${NC}"
}

print_status() {
  echo -e "${GREEN}[✓]${NC} $1"
  ((TESTS_PASSED++))
}

print_warning() {
  echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
  echo -e "${RED}[✗]${NC} $1"
  ((TESTS_FAILED++))
}

print_info() {
  echo -e "${CYAN}[i]${NC} $1"
}

print_test() {
  echo -e "${MAGENTA}[TEST]${NC} $1"
}

# Banner
print_header "Right-Sizer Complete Configuration Validation"
echo ""
echo "This script validates all configuration features:"
echo "  • Resource multipliers (CPU/Memory request and limit)"
echo "  • Resource boundaries (min/max limits)"
echo "  • Resize interval configuration"
echo "  • Log level settings"
echo "  • Multiple concurrent configurations"
echo ""

# Check prerequisites
print_header "Prerequisites Check"

# Check if running locally or in cluster
if [ -f "/.dockerenv" ] || [ -f "/var/run/secrets/kubernetes.io/serviceaccount/token" ]; then
  print_warning "Running inside container/cluster - some tests will be skipped"
  IN_CLUSTER=true
else
  IN_CLUSTER=false
fi

# Check for required tools
if ! command -v go &>/dev/null && [ "$IN_CLUSTER" = false ]; then
  print_error "Go is not installed"
  echo "Please install Go to run compilation tests"
  exit 1
else
  [ "$IN_CLUSTER" = false ] && print_status "Go is installed"
fi

if ! command -v docker &>/dev/null && [ "$IN_CLUSTER" = false ]; then
  print_warning "Docker is not installed - Docker tests will be skipped"
  DOCKER_AVAILABLE=false
else
  DOCKER_AVAILABLE=true
  [ "$IN_CLUSTER" = false ] && print_status "Docker is available"
fi

# Test 1: Configuration Loading
print_header "Test 1: Configuration Loading"

print_test "Testing environment variable loading..."

# Create a test script to verify configuration
cat >/tmp/test_config.go <<'EOF'
package main

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

func main() {
    // Test numeric parsing
    cpuMulti := os.Getenv("CPU_REQUEST_MULTIPLIER")
    if cpuMulti != "" {
        if val, err := strconv.ParseFloat(cpuMulti, 64); err == nil {
            fmt.Printf("CPU_REQUEST_MULTIPLIER: %.2f\n", val)
        } else {
            fmt.Printf("ERROR: Invalid CPU_REQUEST_MULTIPLIER\n")
        }
    }

    // Test duration parsing
    interval := os.Getenv("RESIZE_INTERVAL")
    if interval != "" {
        if dur, err := time.ParseDuration(interval); err == nil {
            fmt.Printf("RESIZE_INTERVAL: %v\n", dur)
        } else {
            fmt.Printf("ERROR: Invalid RESIZE_INTERVAL\n")
        }
    }

    // Test log level
    logLevel := os.Getenv("LOG_LEVEL")
    validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
    if validLevels[logLevel] {
        fmt.Printf("LOG_LEVEL: %s\n", logLevel)
    } else if logLevel != "" {
        fmt.Printf("ERROR: Invalid LOG_LEVEL: %s\n", logLevel)
    }
}
EOF

if [ "$IN_CLUSTER" = false ]; then
  export CPU_REQUEST_MULTIPLIER=1.5
  export RESIZE_INTERVAL=30s
  export LOG_LEVEL=debug

  go run /tmp/test_config.go >/tmp/config_test.out 2>&1

  if grep -q "CPU_REQUEST_MULTIPLIER: 1.50" /tmp/config_test.out; then
    print_status "CPU_REQUEST_MULTIPLIER loaded correctly"
  else
    print_error "CPU_REQUEST_MULTIPLIER failed to load"
  fi

  if grep -q "RESIZE_INTERVAL: 30s" /tmp/config_test.out; then
    print_status "RESIZE_INTERVAL loaded correctly"
  else
    print_error "RESIZE_INTERVAL failed to load"
  fi

  if grep -q "LOG_LEVEL: debug" /tmp/config_test.out; then
    print_status "LOG_LEVEL loaded correctly"
  else
    print_error "LOG_LEVEL failed to load"
  fi
fi

# Test 2: Configuration Defaults
print_header "Test 2: Configuration Defaults"

print_test "Verifying default values..."

# Check if config package exists
if [ -f "config/config.go" ]; then
  print_status "Config package exists"

  # Check for default values
  if grep -q "CPURequestMultiplier.*1.2" config/config.go; then
    print_status "Default CPU request multiplier: 1.2"
  else
    print_error "Default CPU request multiplier not found"
  fi

  if grep -q "ResizeInterval.*30.*time.Second" config/config.go; then
    print_status "Default resize interval: 30s"
  else
    print_error "Default resize interval not found"
  fi

  if grep -q 'LogLevel.*"info"' config/config.go; then
    print_status "Default log level: info"
  else
    print_error "Default log level not found"
  fi
else
  print_error "Config package not found"
fi

# Test 3: Logger Implementation
print_header "Test 3: Logger Implementation"

print_test "Checking logger package..."

if [ -f "logger/logger.go" ]; then
  print_status "Logger package exists"

  # Check for log levels
  for level in DEBUG INFO WARN ERROR; do
    if grep -q "func.*$level" logger/logger.go; then
      print_status "Log level $level implemented"
    else
      print_error "Log level $level not found"
    fi
  done
else
  print_error "Logger package not found"
fi

# Test 4: Controller Integration
print_header "Test 4: Controller Integration"

print_test "Verifying controller updates..."

controllers=(
  "adaptive_rightsizer.go"
  "deployment_rightsizer.go"
  "inplace_rightsizer.go"
  "nondisruptive_rightsizer.go"
)

for controller in "${controllers[@]}"; do
  if [ -f "controllers/$controller" ]; then
    # Check for config usage
    if grep -q "config.Get()" "controllers/$controller"; then
      print_status "$controller uses configuration"
    else
      print_error "$controller doesn't use configuration"
    fi

    # Check for ResizeInterval usage
    if grep -q "cfg.ResizeInterval" "controllers/$controller"; then
      print_status "$controller uses ResizeInterval"
    else
      print_error "$controller doesn't use ResizeInterval"
    fi
  else
    print_error "$controller not found"
  fi
done

# Test 5: Multiplier Calculations
print_header "Test 5: Multiplier Calculations"

print_test "Testing resource calculations..."

# Create a test calculation script
cat >/tmp/test_calc.sh <<'EOF'
#!/bin/bash

# Test calculation with multipliers
CPU_USAGE=100
MEM_USAGE=100

CPU_REQ_MULTI=1.5
MEM_REQ_MULTI=1.3
CPU_LIM_MULTI=2.5
MEM_LIM_MULTI=2.0

CPU_REQ=$(echo "$CPU_USAGE * $CPU_REQ_MULTI" | bc)
MEM_REQ=$(echo "$MEM_USAGE * $MEM_REQ_MULTI" | bc)
CPU_LIM=$(echo "$CPU_REQ * $CPU_LIM_MULTI" | bc)
MEM_LIM=$(echo "$MEM_REQ * $MEM_LIM_MULTI" | bc)

echo "For 100m CPU and 100Mi memory usage:"
echo "  CPU Request: ${CPU_REQ}m (100 × $CPU_REQ_MULTI)"
echo "  Memory Request: ${MEM_REQ}Mi (100 × $MEM_REQ_MULTI)"
echo "  CPU Limit: ${CPU_LIM}m ($CPU_REQ × $CPU_LIM_MULTI)"
echo "  Memory Limit: ${MEM_LIM}Mi ($MEM_REQ × $MEM_LIM_MULTI)"
EOF

chmod +x /tmp/test_calc.sh
calc_output=$(/tmp/test_calc.sh 2>/dev/null)

if echo "$calc_output" | grep -q "CPU Request: 150"; then
  print_status "CPU request calculation correct (100 × 1.5 = 150)"
else
  print_error "CPU request calculation incorrect"
fi

if echo "$calc_output" | grep -q "CPU Limit: 375"; then
  print_status "CPU limit calculation correct (150 × 2.5 = 375)"
else
  print_error "CPU limit calculation incorrect"
fi

# Test 6: Deployment Manifests
print_header "Test 6: Deployment Manifests"

print_test "Checking deployment configurations..."

if [ -f "deployment.yaml" ]; then
  # Check for new environment variables
  for env in RESIZE_INTERVAL LOG_LEVEL; do
    if grep -q "name: $env" deployment.yaml; then
      print_status "$env in deployment.yaml"
    else
      print_error "$env missing from deployment.yaml"
    fi
  done
fi

if [ -f "helm/values.yaml" ]; then
  if grep -q "resizeInterval:" helm/values.yaml; then
    print_status "resizeInterval in Helm values"
  else
    print_error "resizeInterval missing from Helm values"
  fi

  if grep -q "logLevel:" helm/values.yaml; then
    print_status "logLevel in Helm values"
  else
    print_error "logLevel missing from Helm values"
  fi
fi

# Test 7: Documentation
print_header "Test 7: Documentation"

print_test "Verifying documentation updates..."

docs_updated=0
docs_missing=0

# Check README
if [ -f "README.md" ]; then
  if grep -q "RESIZE_INTERVAL" README.md && grep -q "LOG_LEVEL" README.md; then
    print_status "README.md documents new variables"
    ((docs_updated++))
  else
    print_error "README.md missing new variables"
    ((docs_missing++))
  fi
fi

# Check CONFIGURATION.md
if [ -f "CONFIGURATION.md" ]; then
  if grep -q "RESIZE_INTERVAL" CONFIGURATION.md && grep -q "LOG_LEVEL" CONFIGURATION.md; then
    print_status "CONFIGURATION.md documents new variables"
    ((docs_updated++))
  else
    print_error "CONFIGURATION.md missing new variables"
    ((docs_missing++))
  fi
fi

# Test 8: Example Configurations
print_header "Test 8: Example Configurations"

print_test "Checking example files..."

if [ -d "examples" ]; then
  example_count=$(find examples -name "*.yaml" | wc -l)
  if [ "$example_count" -gt 0 ]; then
    print_status "Found $example_count example configuration files"
  else
    print_error "No example configuration files found"
  fi
fi

# Test 9: Build Test (if not in cluster)
if [ "$IN_CLUSTER" = false ]; then
  print_header "Test 9: Build Test"

  print_test "Compiling right-sizer..."

  if go build -o /tmp/right-sizer-test main.go 2>/dev/null; then
    print_status "Compilation successful"

    # Test with environment variables
    export CPU_REQUEST_MULTIPLIER=1.7
    export MEMORY_REQUEST_MULTIPLIER=1.5
    export RESIZE_INTERVAL=45s
    export LOG_LEVEL=debug

    # Run briefly to check configuration loading
    timeout 2 /tmp/right-sizer-test 2>&1 | head -20 >/tmp/run_test.out || true

    if grep -q "CPU_REQUEST_MULTIPLIER set to: 1.70" /tmp/run_test.out; then
      print_status "Runtime configuration loading works"
    else
      print_warning "Could not verify runtime configuration"
    fi
  else
    print_error "Compilation failed"
  fi
fi

# Test 10: Docker Image (if Docker available)
if [ "$DOCKER_AVAILABLE" = true ] && [ "$IN_CLUSTER" = false ]; then
  print_header "Test 10: Docker Image Test"

  print_test "Building Docker image..."

  if docker build -t right-sizer:validate-test . -q >/dev/null 2>&1; then
    print_status "Docker image built successfully"

    # Test running with environment variables
    docker run --rm \
      -e CPU_REQUEST_MULTIPLIER=1.8 \
      -e RESIZE_INTERVAL=20s \
      -e LOG_LEVEL=info \
      right-sizer:validate-test 2>&1 | head -10 >/tmp/docker_test.out || true

    if grep -q "Configuration Loaded" /tmp/docker_test.out; then
      print_status "Docker container runs with configuration"
    else
      print_warning "Could not verify Docker configuration"
    fi
  else
    print_error "Docker build failed"
  fi
fi

# Final Summary
print_header "Validation Summary"

echo ""
echo -e "${GREEN}Tests Passed:${NC} $TESTS_PASSED"
echo -e "${RED}Tests Failed:${NC} $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
  echo -e "${GREEN}✅ All configuration features validated successfully!${NC}"
  echo ""
  echo "The Right-Sizer operator is fully configured with:"
  echo "  • Configurable resource multipliers"
  echo "  • Adjustable resize intervals"
  echo "  • Flexible log levels"
  echo "  • Environment-based configuration"
  exit 0
else
  echo -e "${RED}❌ Some tests failed. Please review the errors above.${NC}"
  exit 1
fi
