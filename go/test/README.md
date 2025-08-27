# Right-Sizer Tests

This directory contains test scripts and manifests for the right-sizer operator.

## Structure

- `scripts/` - Test scripts for various scenarios
- `manifests/` - Kubernetes YAML files for testing
- `minikube/` - Minikube-specific test scripts

## Running Tests

### Unit Tests
```bash
make test
```

### Integration Tests
```bash
cd test/scripts
./test-config.sh
```

### Minikube Tests
```bash
cd test/minikube
./minikube-full-test.sh
```

## Test Scenarios

1. **Configuration Tests** - Verify environment variable configuration
2. **Interval Tests** - Test different resize intervals
3. **Log Level Tests** - Verify logging at different levels
4. **Resource Sizing Tests** - Test various multiplier configurations
5. **In-Place Resize Tests** - Test Kubernetes 1.33+ features
