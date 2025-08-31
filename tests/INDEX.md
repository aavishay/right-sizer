# Tests Directory Index

## Overview

This directory contains all test files, scripts, reports, and test-related documentation for the right-sizer project.

## Directory Structure

```
tests/
├── fixtures/           # Test fixtures and sample data
├── integration/        # Integration tests
├── logs/              # Test execution logs
├── reports/           # Test reports and coverage summaries
├── rbac/              # RBAC-specific tests
├── scripts/           # Test automation scripts
├── unit/              # Unit tests
└── yaml/              # Test YAML manifests
```

## Test Scripts

### Core Test Scripts

#### Logging Tests
- **`scripts/test-duplicate-logging.sh`** - Tests for duplicate logging reduction
- **`scripts/test-info-prefix-removal.sh`** - Verifies [INFO] prefix removal
- **`scripts/verify-logging-fix.sh`** - Quick verification of logging improvements
- **`scripts/demo-logging-improvements.sh`** - Interactive demo of logging improvements

#### Memory Implementation Tests
- **`scripts/test-memory-implementation.sh`** - Comprehensive memory metrics testing
- **`scripts/verify-memory-simple.sh`** - Simple memory verification test
- **`memory-metrics-minikube-test.sh`** - Minikube-specific memory tests
- **`quick-memory-test.sh`** - Quick memory functionality test

#### Integration Tests
- **`run-all-tests.sh`** - Runs all test suites
- **`interactive-test.sh`** - Interactive testing interface
- **`minikube-sanity-test.sh`** - Basic sanity checks for Minikube deployment
- **`simple-minikube-test.sh`** - Simple Minikube integration test
- **`test_additions.sh`** - Additional test scenarios

## Test YAML Files

Located in `yaml/`:
- **`test-deployment.yaml`** - Sample deployment for testing
- **`test-memory-pod.yaml`** - Pod configuration for memory testing
- **`test-skip-pod.yaml`** - Pod with skip annotation testing
- **`test-config.yaml`** - Test configuration settings
- **`simple-skip-test.yaml`** - Simple skip logic test
- **`skip-logic-test.yaml`** - Comprehensive skip logic test

## Test Reports

Located in `reports/`:
- **`TEST_COVERAGE_SUMMARY.md`** - Code coverage summary
- **`duplicate-logging-test-*.log`** - Logging test results
- Test execution reports are automatically generated here with timestamps

## Test Logs

Located in `logs/`:
- **`memory-metrics-test-*.log`** - Memory test execution logs
- **`memory-metrics-report-*.json`** - Memory metrics reports in JSON format
- Detailed test execution logs with timestamps

## Documentation

### Test Documentation
- **`README.md`** - Main test documentation
- **`COMPREHENSIVE_TEST_RESULTS.md`** - Comprehensive test results
- **`MEMORY_METRICS_TESTING.md`** - Memory metrics testing guide
- **`MEMORY_METRICS_TEST_RESULTS.md`** - Memory test results documentation
- **`TEST_REPORT.md`** - General test report
- **`MIGRATION-SUMMARY.md`** - Test migration summary
- **`scaling-thresholds-test-report.md`** - Scaling thresholds test report

## Running Tests

### Quick Start

```bash
# Run all tests
./tests/run-all-tests.sh

# Run specific test suites
./tests/scripts/test-duplicate-logging.sh
./tests/scripts/test-memory-implementation.sh

# Interactive testing
./tests/interactive-test.sh
```

### Test Categories

1. **Unit Tests**: Located in `unit/`
   - Logger tests: `unit/test_logger.go`
   - Run with: `go test ./tests/unit/...`

2. **Integration Tests**: Located in `integration/`
   - Full system integration tests
   - Requires Kubernetes cluster

3. **RBAC Tests**: Located in `rbac/`
   - Permission and security tests
   - Validates RBAC configurations

## Test Conventions

### Naming Conventions
- Test scripts: `test-<feature>.sh` or `verify-<feature>.sh`
- Test reports: `<test-name>-<timestamp>.log`
- Test YAML: `test-<component>.yaml`

### Output Locations
- Logs: `tests/logs/`
- Reports: `tests/reports/`
- Temporary files: `/tmp/rightsizer-*`

### Exit Codes
- `0`: Test passed
- `1`: Test failed
- `2`: Test skipped or environment issue

## CI/CD Integration

Tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Tests
  run: |
    ./tests/run-all-tests.sh
    
- name: Upload Test Reports
  uses: actions/upload-artifact@v2
  with:
    name: test-reports
    path: tests/reports/
```

## Contributing

When adding new tests:
1. Place test scripts in `scripts/`
2. Place test YAML in `yaml/`
3. Direct output to `logs/` or `reports/`
4. Update this INDEX.md file
5. Follow existing naming conventions

## Maintenance

### Cleanup Old Test Artifacts

```bash
# Remove logs older than 7 days
find tests/logs -name "*.log" -mtime +7 -delete

# Remove old reports
find tests/reports -name "*.json" -mtime +30 -delete
```

### Test Dependencies

- Kubernetes cluster (Minikube, Kind, or cloud)
- kubectl configured
- Helm 3.x
- Docker (for build tests)
- jq (for JSON processing)
- bc (for calculations)

## Troubleshooting

### Common Issues

1. **Tests failing with "command not found"**
   - Ensure all test scripts are executable: `chmod +x tests/scripts/*.sh`

2. **Log files not created**
   - Create required directories: `mkdir -p tests/{logs,reports}`

3. **Permission denied errors**
   - Check RBAC permissions
   - Verify ServiceAccount has necessary roles

### Debug Mode

Enable debug output in tests:
```bash
DEBUG=true ./tests/scripts/test-memory-implementation.sh
```

## Contact

For test-related issues or questions, please refer to the main project documentation or open an issue in the project repository.