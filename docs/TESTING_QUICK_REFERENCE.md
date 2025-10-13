# Testing Quick Reference

## 🚀 Quick Commands

### Unit Testing
```bash
# Run all unit tests
cd go && go test ./...

# With verbose output
go test -v ./...

# With coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Linting
```bash
# Run golangci-lint
cd go && golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

### Integration Testing
```bash
# Run integration tests (requires build tag)
cd tests && go test -v -tags=integration ./integration

# With environment variable
RUN_INTEGRATION_TESTS=true go test -v -tags=integration ./integration
```

### E2E Testing
```bash
# Basic Minikube test
./tests/test-minikube-basic.sh

# Comprehensive E2E test
./tests/test-e2e-minikube.sh

# Custom namespace
NAMESPACE=my-ns TEST_NAMESPACE=test-ns ./tests/test-e2e-minikube.sh
```

### Minikube Setup
```bash
# Start Minikube cluster
minikube start --profile=right-sizer \
  --kubernetes-version=v1.34.0 \
  --memory=4096 --cpus=2

# Deploy right-sizer
make mk-deploy

# Check status
kubectl get all -n right-sizer
```

### GitHub Actions (Local)
```bash
# Test with act
act -j test --container-architecture linux/amd64

# List workflows
act -l

# Specific workflow
act -W .github/workflows/test.yml
```

### Performance Testing
```bash
# Run benchmarks
cd go && go test -bench=. -benchmem ./...

# CPU profiling
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory profiling
go test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

## 📊 Test Coverage Goals

| Package | Target | Current |
|---------|--------|---------|
| Overall | 80%+ | ~82% ✅ |
| Core | 90%+ | 85% ✅ |
| API | 95%+ | 82% 🔄 |
| Controllers | 85%+ | 75% 🔄 |

## 🔍 Common Test Patterns

### Table-Driven Tests
```go
func TestExample(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"test1", "input1", "expected1"},
        {"test2", "input2", "expected2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Skip Tests
```go
func TestRequiresCluster(t *testing.T) {
    if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
        t.Skip("Skipping integration test")
    }
    // test code
}
```

## 🛠️ Troubleshooting

### Clear test cache
```bash
go clean -testcache
```

### Update dependencies
```bash
go mod tidy
go mod download
```

### Check test names
```bash
go test -list . ./...
```

### Run specific test
```bash
go test -v -run TestName ./package
```

### Parallel execution
```bash
go test -parallel 4 ./...
```

## 📁 Test File Locations

```
right-sizer/
├── go/
│   ├── *_test.go           # Unit tests
│   └── testdata/           # Test fixtures
├── tests/
│   ├── integration/        # Integration tests
│   ├── e2e/               # End-to-end tests
│   ├── fixtures/          # Test data
│   └── *.sh              # Test scripts
└── .github/
    └── workflows/
        └── test.yml       # CI tests
```

## ✅ Pre-commit Checklist

- [ ] Run unit tests: `go test ./...`
- [ ] Run linting: `golangci-lint run`
- [ ] Check coverage: `go test -cover ./...`
- [ ] Update tests for new features
- [ ] Verify CI passes: `act -j test`

## 📚 Full Documentation

- [Comprehensive Testing Guide](./TESTING_GUIDE.md)
- [Runtime Testing Guide](./RUNTIME_TESTING_GUIDE.md)
- [GitHub Actions Testing](./github-actions-testing.md)
- [Troubleshooting Guide](./troubleshooting-k8s.md)

---
*Quick Reference v1.0 | Updated: October 2024*
