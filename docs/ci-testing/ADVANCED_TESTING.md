# Advanced Testing Scenarios for Right-Sizer

## Table of Contents
- [Chaos Testing](#chaos-testing)
- [Performance Regression Testing](#performance-regression-testing)
- [Multi-Cluster Testing](#multi-cluster-testing)
- [Compliance Validation](#compliance-validation)
- [Load and Stress Testing](#load-and-stress-testing)
- [Security Testing](#security-testing)
- [Mutation Testing](#mutation-testing)
- [Contract Testing](#contract-testing)
- [Canary Testing](#canary-testing)
- [Test Data Management](#test-data-management)

## Chaos Testing

### Overview
Chaos testing helps ensure the Right-Sizer operator remains resilient under adverse conditions.

### Implementing Chaos Tests

#### 1. Pod Failure Scenarios

```bash
#!/bin/bash
# tests/chaos/pod-failure.sh

echo "Testing Right-Sizer resilience to pod failures..."

# Deploy test workload
kubectl apply -f tests/workloads/chaos-test-deployment.yaml

# Wait for right-sizer to resize
sleep 30

# Randomly delete pods
for i in {1..10}; do
  POD=$(kubectl get pods -l app=chaos-test -o jsonpath='{.items[0].metadata.name}')
  kubectl delete pod $POD --grace-period=0 --force
  sleep 10

  # Verify right-sizer continues to function
  kubectl logs -n right-sizer deployment/right-sizer --tail=10
done

# Verify final state
kubectl get pods -l app=chaos-test -o wide
```

#### 2. Network Partition Testing

```yaml
# tests/chaos/network-partition.yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: network-partition
spec:
  action: partition
  mode: all
  selector:
    namespaces:
      - right-sizer
  direction: both
  target:
    selector:
      namespaces:
        - default
  duration: "60s"
```

#### 3. Resource Exhaustion

```go
// tests/chaos/resource_exhaustion_test.go
func TestResourceExhaustion(t *testing.T) {
    // Create many pods simultaneously
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()
            pod := createTestPod(fmt.Sprintf("stress-pod-%d", index))
            _, err := k8sClient.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
            if err != nil {
                t.Logf("Expected error creating pod %d: %v", index, err)
            }
        }(i)
    }
    wg.Wait()

    // Verify right-sizer handles the load
    time.Sleep(30 * time.Second)

    // Check operator health
    health := checkOperatorHealth()
    assert.True(t, health.Ready, "Operator should remain healthy under load")
}
```

### Chaos Testing Framework

```go
// tests/chaos/framework.go
package chaos

import (
    "context"
    "math/rand"
    "time"
)

type ChaosTest struct {
    Name     string
    Duration time.Duration
    Actions  []ChaosAction
}

type ChaosAction interface {
    Execute(ctx context.Context) error
    Verify(ctx context.Context) error
}

type PodChaos struct {
    Namespace string
    Selector  map[string]string
}

func (p *PodChaos) Execute(ctx context.Context) error {
    // Randomly delete pods matching selector
    pods := listPods(p.Namespace, p.Selector)
    if len(pods) > 0 {
        victim := pods[rand.Intn(len(pods))]
        return deletePod(victim)
    }
    return nil
}

func RunChaosTest(test ChaosTest) error {
    ctx, cancel := context.WithTimeout(context.Background(), test.Duration)
    defer cancel()

    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            action := test.Actions[rand.Intn(len(test.Actions))]
            if err := action.Execute(ctx); err != nil {
                return err
            }
            if err := action.Verify(ctx); err != nil {
                return err
            }
        }
    }
}
```

## Performance Regression Testing

### Benchmark Suite

```go
// tests/performance/regression_test.go
package performance

import (
    "testing"
    "time"
)

type PerformanceBaseline struct {
    ResizeLatencyP50 time.Duration
    ResizeLatencyP99 time.Duration
    MemoryUsageMB    float64
    CPUUsageMillis   float64
}

var baseline = PerformanceBaseline{
    ResizeLatencyP50: 100 * time.Millisecond,
    ResizeLatencyP99: 500 * time.Millisecond,
    MemoryUsageMB:    100,
    CPUUsageMillis:   500,
}

func BenchmarkResizeDecision(b *testing.B) {
    controller := setupController()
    pod := createTestPod()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        start := time.Now()
        _, _ = controller.MakeResizeDecision(pod)
        elapsed := time.Since(start)

        if elapsed > baseline.ResizeLatencyP99 {
            b.Errorf("Resize decision took %v, exceeds P99 baseline %v",
                elapsed, baseline.ResizeLatencyP99)
        }
    }
}

func TestMemoryRegression(t *testing.T) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    initialMem := m.Alloc

    // Run workload
    runStandardWorkload()

    runtime.GC()
    runtime.ReadMemStats(&m)
    finalMem := m.Alloc

    memUsageMB := float64(finalMem-initialMem) / 1024 / 1024
    if memUsageMB > baseline.MemoryUsageMB*1.1 { // 10% tolerance
        t.Errorf("Memory usage %.2f MB exceeds baseline %.2f MB",
            memUsageMB, baseline.MemoryUsageMB)
    }
}
```

### Continuous Performance Monitoring

```yaml
# .github/workflows/performance.yml
name: Performance Regression Tests

on:
  pull_request:
    paths:
      - 'go/**'
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run benchmarks
        run: |
          cd go
          go test -bench=. -benchmem -benchtime=10s ./... > bench_new.txt

      - name: Compare with baseline
        run: |
          # Fetch baseline from main branch
          git fetch origin main
          git checkout origin/main -- benchmarks/baseline.txt

          # Compare results
          go install golang.org/x/perf/cmd/benchstat@latest
          benchstat benchmarks/baseline.txt bench_new.txt > comparison.txt

          # Check for regressions
          if grep -E "[-+][0-9]+\.[0-9]+%" comparison.txt | grep "+"; then
            echo "Performance regression detected!"
            cat comparison.txt
            exit 1
          fi
```

## Multi-Cluster Testing

### Cross-Cluster Test Setup

```bash
#!/bin/bash
# tests/multi-cluster/setup.sh

# Create test clusters
kind create cluster --name cluster1 --config tests/multi-cluster/cluster1.yaml
kind create cluster --name cluster2 --config tests/multi-cluster/cluster2.yaml

# Deploy Right-Sizer to both clusters
for cluster in cluster1 cluster2; do
  kubectl --context kind-$cluster apply -f deploy/
  kubectl --context kind-$cluster wait --for=condition=Ready \
    -n right-sizer deployment/right-sizer --timeout=60s
done

# Run cross-cluster tests
go test -tags=multicluster ./tests/multi-cluster/...
```

### Multi-Cluster Test Cases

```go
// tests/multi-cluster/multi_cluster_test.go
func TestCrossClusterConsistency(t *testing.T) {
    cluster1 := connectToCluster("kind-cluster1")
    cluster2 := connectToCluster("kind-cluster2")

    // Deploy identical workloads
    workload := createTestWorkload()
    deployToCluster(cluster1, workload)
    deployToCluster(cluster2, workload)

    // Wait for resize decisions
    time.Sleep(60 * time.Second)

    // Compare resize decisions
    pods1 := getPodsFromCluster(cluster1)
    pods2 := getPodsFromCluster(cluster2)

    for i := range pods1 {
        assert.Equal(t,
            pods1[i].Spec.Containers[0].Resources,
            pods2[i].Spec.Containers[0].Resources,
            "Resize decisions should be consistent across clusters")
    }
}
```

## Compliance Validation

### Kubernetes API Compliance

```go
// tests/compliance/k8s_api_test.go
func TestInPlaceResizeCompliance(t *testing.T) {
    tests := []struct {
        name     string
        validate func(*testing.T)
    }{
        {
            name: "ResizePolicy_RestartContainer",
            validate: validateRestartContainerPolicy,
        },
        {
            name: "ResizePolicy_NotRequired",
            validate: validateNotRequiredPolicy,
        },
        {
            name: "AllocatedResources_Tracking",
            validate: validateAllocatedResources,
        },
        {
            name: "ResizeStatus_Conditions",
            validate: validateResizeStatusConditions,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, tt.validate)
    }
}

func validateRestartContainerPolicy(t *testing.T) {
    pod := &v1.Pod{
        Spec: v1.PodSpec{
            Containers: []v1.Container{{
                Name: "test",
                ResizePolicy: []v1.ContainerResizePolicy{{
                    ResourceName:  v1.ResourceCPU,
                    RestartPolicy: v1.RestartContainer,
                }},
            }},
        },
    }

    // Apply resize
    resizer := NewRightSizer()
    result := resizer.Resize(pod)

    // Validate container restart occurred
    assert.True(t, result.ContainerRestarted)
}
```

### CRD Validation

```yaml
# tests/compliance/crd-validation.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: resizepolicies.rightsizer.io
spec:
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        type: object
        required:
        - spec
        properties:
          spec:
            type: object
            required:
            - targetRef
            - policy
            properties:
              targetRef:
                type: object
                required:
                - kind
                - name
              policy:
                type: object
                x-kubernetes-validations:
                - rule: "self.minReplicas <= self.maxReplicas"
                  message: "minReplicas must be less than maxReplicas"
```

## Load and Stress Testing

### Load Generator

```go
// tests/load/generator.go
package load

type LoadGenerator struct {
    Rate      int           // Requests per second
    Duration  time.Duration
    Scenario  LoadScenario
}

type LoadScenario interface {
    GenerateLoad(ctx context.Context) error
}

type PodCreationLoad struct {
    Namespace string
    Template  *v1.Pod
}

func (p *PodCreationLoad) GenerateLoad(ctx context.Context) error {
    ticker := time.NewTicker(time.Second / time.Duration(rate))
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            pod := p.Template.DeepCopy()
            pod.Name = fmt.Sprintf("%s-%d", pod.Name, time.Now().UnixNano())
            _, err := k8sClient.CoreV1().Pods(p.Namespace).Create(ctx, pod, metav1.CreateOptions{})
            if err != nil {
                return err
            }
        }
    }
}

func RunLoadTest(t *testing.T) {
    gen := &LoadGenerator{
        Rate:     100, // 100 pods per second
        Duration: 5 * time.Minute,
        Scenario: &PodCreationLoad{
            Namespace: "load-test",
            Template:  createLoadTestPod(),
        },
    }

    ctx, cancel := context.WithTimeout(context.Background(), gen.Duration)
    defer cancel()

    err := gen.Scenario.GenerateLoad(ctx)
    assert.NoError(t, err)

    // Verify system stability
    metrics := collectMetrics()
    assert.Less(t, metrics.ErrorRate, 0.01) // < 1% error rate
    assert.Less(t, metrics.P99Latency, 1*time.Second)
}
```

### Stress Test Scenarios

```yaml
# tests/stress/scenarios.yaml
scenarios:
  - name: "High Pod Churn"
    description: "Rapid pod creation and deletion"
    config:
      createRate: 50
      deleteRate: 45
      duration: 10m

  - name: "Resource Exhaustion"
    description: "Fill cluster to capacity"
    config:
      targetUtilization: 95
      resourceType: "memory"
      duration: 5m

  - name: "Burst Traffic"
    description: "Sudden spike in workload"
    config:
      normalRate: 10
      burstRate: 1000
      burstDuration: 30s
      totalDuration: 5m
```

## Security Testing

### Vulnerability Scanning Pipeline

```bash
#!/bin/bash
# tests/security/scan.sh

echo "Running comprehensive security scan..."

# 1. Dependency vulnerabilities
echo "Checking Go dependencies..."
govulncheck ./...

# 2. Container scanning
echo "Scanning container image..."
trivy image --severity HIGH,CRITICAL right-sizer:latest

# 3. Kubernetes manifests
echo "Scanning Kubernetes manifests..."
kubesec scan deploy/*.yaml

# 4. RBAC audit
echo "Auditing RBAC permissions..."
kubectl auth can-i --list --as=system:serviceaccount:right-sizer:right-sizer

# 5. Network policies
echo "Checking network policies..."
kubectl get networkpolicies -n right-sizer -o yaml

# 6. Secret scanning
echo "Scanning for secrets..."
trufflehog filesystem . --json > security-report.json

# Generate report
echo "Generating security report..."
python3 tests/security/generate_report.py security-report.json
```

### Penetration Testing

```go
// tests/security/pentest_test.go
func TestWebhookInjection(t *testing.T) {
    // Attempt SQL injection in webhook
    maliciousPod := &v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: "'; DROP TABLE pods; --",
            Annotations: map[string]string{
                "test": "<script>alert('xss')</script>",
            },
        },
    }

    resp, err := sendAdmissionRequest(maliciousPod)
    assert.NoError(t, err)
    assert.False(t, resp.Allowed, "Malicious pod should be rejected")
}

func TestRBACEscalation(t *testing.T) {
    // Try to escalate privileges
    sa := &v1.ServiceAccount{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "escalated",
            Namespace: "right-sizer",
        },
    }

    _, err := k8sClient.CoreV1().ServiceAccounts("right-sizer").Create(
        context.TODO(), sa, metav1.CreateOptions{})
    assert.Error(t, err, "Should not be able to create service accounts")
}
```

## Mutation Testing

### Mutation Test Framework

```go
// tests/mutation/mutator.go
package mutation

type Mutator interface {
    Mutate(code string) string
    Describe() string
}

type ConditionalMutator struct{}

func (c *ConditionalMutator) Mutate(code string) string {
    // Flip conditional operators
    replacements := map[string]string{
        "==": "!=",
        ">=": "<",
        "<=": ">",
        "&&": "||",
    }

    mutated := code
    for old, new := range replacements {
        mutated = strings.Replace(mutated, old, new, 1)
    }
    return mutated
}

func RunMutationTests(t *testing.T) {
    mutators := []Mutator{
        &ConditionalMutator{},
        &ArithmeticMutator{},
        &ReturnValueMutator{},
    }

    for _, mutator := range mutators {
        t.Run(mutator.Describe(), func(t *testing.T) {
            // Apply mutation
            originalCode := readSourceFile("controllers/rightsizer.go")
            mutatedCode := mutator.Mutate(originalCode)

            // Write mutated code
            writeSourceFile("controllers/rightsizer.go", mutatedCode)

            // Run tests
            output, err := exec.Command("go", "test", "./controllers/...").Output()

            // Restore original code
            writeSourceFile("controllers/rightsizer.go", originalCode)

            // Tests should fail with mutation
            assert.Error(t, err, "Tests should detect mutation: %s", mutator.Describe())
        })
    }
}
```

## Contract Testing

### API Contract Tests

```go
// tests/contract/api_contract_test.go
package contract

import (
    "github.com/pact-foundation/pact-go/dsl"
)

func TestMetricsAPIContract(t *testing.T) {
    pact := &dsl.Pact{
        Consumer: "right-sizer",
        Provider: "prometheus",
    }

    defer pact.Teardown()

    pact.
        AddInteraction().
        Given("Metrics are available").
        UponReceiving("A request for pod metrics").
        WithRequest(dsl.Request{
            Method: "GET",
            Path:   dsl.String("/api/v1/query"),
            Query: dsl.MapMatcher{
                "query": dsl.String("container_memory_usage_bytes{pod=\"test-pod\"}"),
            },
        }).
        WillRespondWith(dsl.Response{
            Status: 200,
            Body: dsl.Match(MetricsResponse{}),
        })

    err := pact.Verify(func() error {
        client := NewMetricsClient(fmt.Sprintf("http://localhost:%d", pact.Server.Port))
        _, err := client.GetPodMetrics("test-pod")
        return err
    })

    assert.NoError(t, err)
}
```

### Schema Validation

```yaml
# tests/contract/schemas/resize-request.json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["pod", "currentResources", "recommendedResources"],
  "properties": {
    "pod": {
      "type": "object",
      "required": ["name", "namespace"],
      "properties": {
        "name": {"type": "string"},
        "namespace": {"type": "string"}
      }
    },
    "currentResources": {
      "$ref": "#/definitions/resources"
    },
    "recommendedResources": {
      "$ref": "#/definitions/resources"
    }
  },
  "definitions": {
    "resources": {
      "type": "object",
      "properties": {
        "cpu": {"type": "string", "pattern": "^[0-9]+m?$"},
        "memory": {"type": "string", "pattern": "^[0-9]+[KMG]i?$"}
      }
    }
  }
}
```

## Canary Testing

### Canary Deployment Test

```bash
#!/bin/bash
# tests/canary/deploy.sh

# Deploy canary version
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: right-sizer-canary
  namespace: right-sizer
spec:
  selector:
    app: right-sizer
    version: canary
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-canary
  namespace: right-sizer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer
      version: canary
  template:
    metadata:
      labels:
        app: right-sizer
        version: canary
    spec:
      containers:
      - name: right-sizer
        image: right-sizer:canary
        env:
        - name: CANARY_MODE
          value: "true"
EOF

# Route 10% traffic to canary
kubectl apply -f tests/canary/traffic-split.yaml

# Monitor canary metrics
watch -n 5 'kubectl top pods -n right-sizer -l version=canary'
```

### Canary Analysis

```go
// tests/canary/analysis.go
func AnalyzeCanaryMetrics(t *testing.T) {
    stableMetrics := collectMetrics("version=stable")
    canaryMetrics := collectMetrics("version=canary")

    // Compare error rates
    if canaryMetrics.ErrorRate > stableMetrics.ErrorRate*1.05 {
        t.Errorf("Canary error rate %.2f%% exceeds stable %.2f%%",
            canaryMetrics.ErrorRate*100, stableMetrics.ErrorRate*100)
    }

    // Compare latency
    if canaryMetrics.P99Latency > stableMetrics.P99Latency*1.1 {
        t.Errorf("Canary P99 latency %v exceeds stable %v",
            canaryMetrics.P99Latency, stableMetrics.P99Latency)
    }

    // Compare resource usage
    if canaryMetrics.MemoryUsage > stableMetrics.MemoryUsage*1.2 {
        t.Errorf("Canary memory usage %.2f MB exceeds stable %.2f MB",
            canaryMetrics.MemoryUsage, stableMetrics.MemoryUsage)
    }
}
```

## Test Data Management

### Test Data Generation

```go
// tests/testdata/generator.go
package testdata

type TestDataGenerator struct {
    Seed int64
}

func (g *TestDataGenerator) GeneratePod(opts PodOptions) *v1.Pod {
    return &v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      g.generateName(),
            Namespace: opts.Namespace,
            Labels:    g.generateLabels(opts.LabelCount),
        },
        Spec: v1.PodSpec{
            Containers: g.generateContainers(opts.ContainerCount),
        },
    }
}

func (g *TestDataGenerator) GenerateWorkload(size WorkloadSize) []runtime.Object {
    switch size {
    case Small:
        return g.generateObjects(10, 1, 1)
    case Medium:
        return g.generateObjects(100, 5, 3)
    case Large:
        return g.generateObjects(1000, 10, 5)
    case XLarge:
        return g.generateObjects(10000, 20, 10)
    }
}

// Snapshot testing
func (g *TestDataGenerator) SaveSnapshot(name string, objects []runtime.Object) error {
    data, err := json.MarshalIndent(objects, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(fmt.Sprintf("testdata/snapshots/%s.json", name), data, 0644)
}

func (g *TestDataGenerator) LoadSnapshot(name string) ([]runtime.Object, error) {
    data, err := os.ReadFile(fmt.Sprintf("testdata/snapshots/%s.json", name))
    if err != nil {
        return nil, err
    }
    var objects []runtime.Object
    return objects, json.Unmarshal(data, &objects)
}
```

### Test Fixtures

```yaml
# tests/testdata/fixtures/standard-workload.yaml
apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: test-deployment
    spec:
      replicas: 3
      template:
        spec:
          containers:
          - name: app
            image: nginx
            resources:
              requests:
                cpu: 100m
                memory: 128Mi
              limits:
                cpu: 500m
                memory: 512Mi

  - apiVersion: v1
    kind: Service
    metadata:
      name: test-service
    spec:
      selector:
        app: test
      ports:
      - port: 80
```

### Golden Files

```go
// tests/golden/golden_test.go
func TestGoldenFiles(t *testing.T) {
    testCases := []struct {
        name   string
        input  string
        golden string
    }{
        {
            name:   "ResizeDecision",
            input:  "testdata/input/pod.yaml",
            golden: "testdata/golden/resize_decision.json",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            input := loadYAML(tc.input)
            actual := processInput(input)

            if *update {
                // Update golden file
                saveGolden(tc.golden, actual)
            }

            expected := loadGolden(tc.golden)
            assert.Equal(t, expected, actual)
        })
    }
}
```

---

*This advanced testing guide covers sophisticated testing scenarios for the Right-Sizer project. For basic testing, see the [Quick Start Guide](QUICK_START.md).*
