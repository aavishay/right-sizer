# 🗺️ Right-Sizer Unified Roadmap (Operator Focus)

> **Note**: This is the operator-focused view. For complete platform view, see the root-level [UNIFIED_ROADMAP.md](../UNIFIED_ROADMAP.md)

**Last Updated**: February 1, 2026  
**Operator Version**: v0.5.0 (In Development)  
**Dashboard Version**: v0.4.0 (In Development)

---

## 🎯 Operator v0.5.0: Predictive Intelligence

**Timeline**: Mar 1 - Apr 15, 2026 (6 weeks)  
**Team**: 3-4 Go engineers, 1 ML specialist  
**Status**: 🔄 Planning phase complete

### Core Features

#### 1. Proactive Predictive Scaling (Weeks 1-3)

**What It Does**: Moves from reactive to proactive resource resizing by forecasting demand spikes and adjusting resources before they're needed.

**Architecture**:
```
Historical Metrics → Seasonal Patterns → Demand Forecast → 
Proactive Resize → Validation → Apply with Gradual Rollback
```

**Implementation**:

```go
// go/controllers/proactive_rightsizer.go (NEW)
type ProactiveRightSizer struct {
    Config          *config.Config
    Predictor       *predictor.Engine
    MetricsProvider metrics.Provider
    Client          client.Client
}

// Main reconciliation loop
func (p *ProactiveRightSizer) Reconcile(ctx context.Context, pod *corev1.Pod) {
    // Get historical patterns
    patterns := p.getSeasonalPatterns(pod)
    
    // Forecast demand (3-7 day horizon)
    forecast := p.predictDemand(pod, patterns)
    
    // If confidence high and demand will spike, schedule resize
    if forecast.Confidence > p.Config.PredictionConfidenceThreshold {
        p.scheduleProactiveResize(pod, forecast)
    }
}

// Schedule resize before demand spike
func (p *ProactiveRightSizer) scheduleProactiveResize(
    pod *corev1.Pod,
    forecast *predictor.Forecast,
) error {
    // Calculate when to resize
    resizeTime := forecast.SpikeTime.Add(-p.Config.ResizeLeadTime)
    
    // Create scheduled resize event
    event := events.ProactiveResizeScheduled{
        Pod:        pod.Name,
        Namespace:  pod.Namespace,
        ResizeTime: resizeTime,
        Forecast:   forecast,
        Reason:     "Proactive scaling: predicted demand spike",
    }
    
    // Publish event
    p.EventBus.Publish(event)
    
    // Store in scheduler
    p.ResizeScheduler.Schedule(resizeTime, pod, forecast)
    
    return nil
}
```

**Key Files**:
- `go/controllers/proactive_rightsizer.go` (new, 300 lines)
- `go/predictor/scheduler.go` (new, 200 lines)
- `go/api/grpc/predictions.proto` (extend)
- `go/config/prediction_config.go` (new, 100 lines)

**Testing Strategy**:
```bash
# Unit tests
go test -v ./controllers -run TestProactiveResize

# Integration tests
go test -v ./controllers -integration -run TestProactiveWithOperator

# E2E (minikube)
make mk-test-proactive
```

**Dependencies**:
- ✅ Memory Store (v0.4.0) - provides historical context
- ✅ Predictor Engine (v0.4.0) - seasonal patterns
- ⏳ Dashboard UI (v0.4.0) - visualization
- ⏳ Recommendation engine (v0.4.0) - candidate sizing

**Success Criteria**:
- Prediction accuracy MAPE: <10%
- Proactive resize success rate: >90%
- False positive rate: <5%
- Resizes occur within ±30min of forecast spike
- Zero missed demand spikes

---

#### 2. What-If Analysis Engine (Weeks 2-4)

**What It Does**: Models the impact of resource changes on workload performance and cost, enabling data-driven decision making.

**Architecture**:
```
Current State → Apply Hypothetical Changes → Simulate Metrics →
Project Performance Impact → Estimate Cost Delta → Report
```

**Implementation**:

```go
// go/whatif/analyzer.go (NEW)
type WhatIfAnalyzer struct {
    Config          *config.Config
    MetricsProvider metrics.Provider
    CostCalculator  cost.Calculator
    Client          client.Client
}

// WhatIfScenario represents a hypothetical change
type WhatIfScenario struct {
    PodName         string
    Namespace       string
    NewRequests     *corev1.ResourceList
    NewLimits       *corev1.ResourceList
    HorizonDays     int
}

// Analyze runs what-if simulation
func (w *WhatIfAnalyzer) Analyze(
    ctx context.Context,
    scenario *WhatIfScenario,
) (*WhatIfResult, error) {
    // Get current metrics for pod
    currentMetrics := w.getHistoricalMetrics(
        scenario.PodName,
        scenario.Namespace,
        scenario.HorizonDays,
    )
    
    // Project metrics with new resources
    projectedMetrics := w.projectMetricsWithResources(
        currentMetrics,
        scenario.NewRequests,
        scenario.NewLimits,
    )
    
    // Calculate impact
    impact := w.calculateImpact(currentMetrics, projectedMetrics)
    
    // Estimate cost delta
    costDelta := w.estimateCostDelta(
        currentMetrics,
        projectedMetrics,
        scenario.HorizonDays,
    )
    
    return &WhatIfResult{
        Scenario:              scenario,
        PerformanceImpact:     impact,
        CostDelta:             costDelta,
        SuccessProbability:    w.estimateSuccessProbability(impact),
        RecommendedAction:     w.recommendAction(impact, costDelta),
        Confidence:            w.calculateConfidence(scenario),
    }, nil
}

// Impact represents projected changes
type WhatIfResult struct {
    Scenario              *WhatIfScenario
    PerformanceImpact     PerformanceMetrics
    CostDelta             CostImpact
    SuccessProbability    float64        // 0-1
    RecommendedAction     string         // "apply", "monitor", "reject"
    Confidence            float64        // 0-1
}
```

**gRPC Service Definition**:

```protobuf
// api/grpc/whatif.proto
service WhatIfAnalyzer {
    rpc AnalyzeResize(WhatIfRequest) returns (WhatIfResponse);
    rpc CompareScearios(CompareRequest) returns (ComparisonResponse);
    rpc SimulateImpact(SimulationRequest) returns (SimulationResponse);
}

message WhatIfRequest {
    string pod_name = 1;
    string namespace = 2;
    ResourceRequirements new_resources = 3;
    int32 horizon_days = 4;
}

message WhatIfResponse {
    PerformanceImpact performance = 1;
    CostImpact cost = 2;
    double success_probability = 3;
    double confidence = 4;
}
```

**Key Files**:
- `go/whatif/analyzer.go` (new, 400 lines)
- `go/whatif/impact_calculator.go` (new, 200 lines)
- `go/api/grpc/whatif.proto` (new, 100 lines)
- `go/tests/whatif_test.go` (new, 300 lines)

**Testing Strategy**:
```bash
# Unit tests for impact calculation
go test -v ./whatif -run TestCalculateImpact

# Integration with simulator
go test -v ./whatif -integration -run TestWhatIfWithMetrics

# Accuracy tests (compare projected vs actual)
go test -v ./whatif -accuracy -run TestAccuracy
```

**Dashboard Integration**:
```typescript
// frontend/src/pages/WhatIf.tsx
// Calls operator: POST /grpc/WhatIfAnalyzer/AnalyzeResize
const scenario = {
  pod_name: "web-app",
  namespace: "production",
  new_resources: { cpu: "500m", memory: "512Mi" },
  horizon_days: 7
};
const result = await operatorClient.whatif.analyzeResize(scenario);
```

**Success Criteria**:
- Simulation time: <100ms per scenario
- Projection accuracy: >85% (compared to actual)
- Support 1K+ concurrent analyses
- Cost delta accuracy: >90%

---

#### 3. Capacity Planning (Weeks 3-5)

**What It Does**: Forecasts future resource requirements based on growth trends and recommends cluster scaling actions.

**Architecture**:
```
Historical Trends → Growth Rate Analysis → Future Projection →
Headroom Calculation → Scaling Recommendations
```

**Implementation**:

```go
// go/predictions/capacity_planner.go (NEW)
type CapacityPlanner struct {
    Config          *config.Config
    MetricsProvider metrics.Provider
    MemStore        *memstore.Store
}

// CapacityForecast predicts future resource needs
type CapacityForecast struct {
    Namespace          string
    TimeHorizon        time.Duration  // 7, 14, 30 days
    CurrentUsage       resource.Quantity
    ProjectedUsage     resource.Quantity
    GrowthRate         float64
    HeadroomRecommended resource.Quantity
    ConfidenceInterval [2]resource.Quantity
}

// PlanCapacity forecasts namespace resource requirements
func (cp *CapacityPlanner) PlanCapacity(
    ctx context.Context,
    namespace string,
    horizon time.Duration,
) (*CapacityForecast, error) {
    // Get historical usage
    history := cp.getHistoricalUsage(namespace, 90*time.Day)
    
    // Calculate growth rate
    growthRate := cp.calculateGrowthRate(history)
    
    // Project forward
    projected := cp.projectUsage(
        history.Latest,
        growthRate,
        horizon,
    )
    
    // Calculate recommended headroom (e.g., 20% buffer)
    headroom := cp.calculateHeadroom(projected)
    
    // Estimate confidence interval
    ci := cp.estimateConfidenceInterval(history, growthRate)
    
    return &CapacityForecast{
        Namespace:           namespace,
        TimeHorizon:         horizon,
        CurrentUsage:        history.Latest,
        ProjectedUsage:      projected,
        GrowthRate:          growthRate,
        HeadroomRecommended: headroom,
        ConfidenceInterval:  ci,
    }, nil
}

// ClusterCapacityForecast aggregates across all namespaces
func (cp *CapacityPlanner) PlanClusterCapacity(
    ctx context.Context,
    cluster string,
) (*ClusterCapacityForecast, error) {
    namespaces := cp.getNamespaces(cluster)
    
    var forecasts []*CapacityForecast
    totalProjected := resource.Quantity{}
    
    for _, ns := range namespaces {
        forecast, err := cp.PlanCapacity(ctx, ns, 30*time.Day)
        if err != nil {
            continue
        }
        forecasts = append(forecasts, forecast)
        totalProjected.Add(forecast.ProjectedUsage)
    }
    
    // Recommend node scaling
    scaling := cp.recommendNodeScaling(cluster, totalProjected)
    
    return &ClusterCapacityForecast{
        Forecasts:           forecasts,
        TotalProjectedUsage: totalProjected,
        NodeScalingAdvice:   scaling,
    }, nil
}
```

**Key Files**:
- `go/predictions/capacity_planner.go` (new, 400 lines)
- `go/predictions/growth_analyzer.go` (new, 200 lines)
- `go/api/grpc/capacity.proto` (new, 100 lines)

**Success Criteria**:
- Forecast accuracy: >85% for 30-day horizon
- Planning time: <500ms per cluster
- Support 100+ clusters
- Capture seasonal variations

---

#### 4. Enhanced Event Stream (Weeks 1-2, then Embedded)

**What It Does**: Extends event taxonomy to include predictions, what-if scenarios, and capacity alerts, enabling full platform visibility.

**New Event Types**:

```go
// go/events/types.go (EXTEND)

// Prediction events
type PredictionGenerated struct {
    Pod              string
    Namespace        string
    Forecast         *predictor.Forecast
    Timestamp        time.Time
}

type ProactiveResizeScheduled struct {
    Pod              string
    Namespace        string
    ScheduledTime    time.Time
    Forecast         *predictor.Forecast
    Reason           string
}

// What-If events
type WhatIfAnalysisCompleted struct {
    Pod              string
    Namespace        string
    Scenario         *WhatIfScenario
    Result           *WhatIfResult
    RecommendedBy    string  // user or system
}

// Capacity events
type CapacityWarning struct {
    Namespace        string
    ResourceType     string  // cpu, memory
    ProjectedUsage   resource.Quantity
    ThresholdPercent float64
    TimeUntilFull    time.Duration
}

type NodeScalingRecommended struct {
    Cluster          string
    NodeCount        int32
    Reason           string
    CostImpact       float64
}
```

**Event Publishing**:

```go
// Example: Publish prediction event
event := events.PredictionGenerated{
    Pod:       "web-app-5c7d8f",
    Namespace: "production",
    Forecast: &predictor.Forecast{
        Value:      "750m",
        Confidence: 0.92,
        Type:       "seasonal",
    },
    Timestamp: time.Now(),
}
p.EventBus.Publish(event)
```

**Key Files**:
- `go/events/types.go` (extend, +100 lines)
- `go/api/grpc/events.proto` (extend, +50 lines)
- Event publishing throughout controllers

**Success Criteria**:
- Event throughput: 10K events/sec
- Event latency: <50ms
- Dashboard receives events: <100ms
- Event loss: 0%

---

### Implementation Timeline

```
Week 1-2: Setup & Groundwork
  ├─ Extend event taxonomy (2 days)
  ├─ Create gRPC service definitions (2 days)
  ├─ Set up test infrastructure (2 days)
  └─ Create base implementation files (2 days)

Week 2-3: Proactive Scaling
  ├─ Implement scheduling logic (3 days)
  ├─ Add rollback mechanism (2 days)
  ├─ Integration with Memory Store (2 days)
  └─ Testing & validation (2 days)

Week 3-4: What-If Engine
  ├─ Build impact calculator (3 days)
  ├─ Implement scenario simulator (3 days)
  ├─ Add cost projections (2 days)
  └─ Testing & benchmarking (2 days)

Week 4-5: Capacity Planning
  ├─ Trend analysis algorithms (2 days)
  ├─ Growth rate calculations (2 days)
  ├─ Cluster-level aggregation (2 days)
  ├─ Node scaling recommendations (2 days)
  └─ Testing (2 days)

Week 5-6: Integration & Polish
  ├─ Cross-feature integration (3 days)
  ├─ Performance optimization (2 days)
  ├─ Documentation (2 days)
  ├─ E2E testing (2 days)
  └─ Code review & cleanup (2 days)
```

---

### Integration with Dashboard

**Dashboard Benefits**:

1. **Predictive Dashboard** (v0.4.0 Feature 1)
   - Display forecasts with confidence intervals
   - Show scheduled proactive resizes
   - Timeline of upcoming changes
   - Manual override options

2. **What-If Scenarios** (v0.4.0 Feature 3)
   - Scenario builder UI
   - Comparison view
   - Impact visualization
   - One-click apply

3. **Capacity Planner** (v0.4.0 Feature 5)
   - Namespace capacity view
   - Cluster capacity dashboard
   - Growth trend charts
   - Scaling recommendations

4. **Alerts & Notifications** (v0.4.0)
   - Capacity warnings
   - Scaling recommendations
   - Prediction updates
   - What-If completions

---

### Testing & Validation

**Test Coverage Target**: 85%+

```bash
# Unit tests
go test -v ./... -run Test -cover

# Integration tests
go test -v ./... -tags=integration -run Integration

# E2E with dashboard
make mk-test-v0.5

# Performance benchmarks
go test -v ./... -bench=. -benchmem
```

**Key Test Scenarios**:

1. Proactive Scaling
   - ✅ Normal demand spike prediction
   - ✅ False positive handling
   - ✅ Rollback on failure
   - ✅ Dashboard updates in real-time

2. What-If Analysis
   - ✅ Accuracy vs actual outcomes
   - ✅ Edge cases (OOM, throttling)
   - ✅ Cost calculation correctness
   - ✅ Concurrent analyses

3. Capacity Planning
   - ✅ Growth rate calculation
   - ✅ Multi-namespace aggregation
   - ✅ Confidence intervals
   - ✅ Node scaling math

---

### Success Metrics (v0.5.0)

| Metric | Target | Measurement |
|--------|--------|-------------|  
| **Prediction MAPE** | <10% | Historical comparison |
| **Proactive Success** | >90% | Resize success rate |
| **What-If Accuracy** | >85% | Projected vs actual |
| **Forecast Speed** | <100ms | API latency p99 |
| **Event Throughput** | 10K/sec | Sustained load |
| **Uptime** | 99.95% | Production monitoring |
| **Code Coverage** | 85%+ | Go test coverage |

---

### Release Criteria

Before v0.5.0 production release:
- [ ] All unit tests passing (85%+ coverage)
- [ ] Integration tests passing
- [ ] E2E tests with dashboard passing
- [ ] Performance benchmarks met
- [ ] Security audit passed
- [ ] Documentation complete
- [ ] Dashboard UI for all features ready
- [ ] Zero critical bugs

---

### Rollback Plan

If v0.5.0 experiences issues in production:

1. **Disable Proactive Scaling**
   ```bash
   kubectl set env -n right-sizer deployment/right-sizer \
     PROACTIVE_RESIZING_ENABLED=false
   ```

2. **Revert to v0.4.0**
   ```bash
   helm rollback right-sizer 1
   ```

3. **Investigation**
   - Gather logs and metrics
   - Analyze what-if impact
   - Create bug tickets

---

## Related Documents

- [ROADMAP.md](./ROADMAP.md) - Original operator roadmap
- [UNIFIED_ROADMAP.md](../UNIFIED_ROADMAP.md) - Full platform roadmap
- [.github/copilot-instructions.md](./.github/copilot-instructions.md) - Developer guide
- [CODE_QUALITY_ANALYSIS.md](./CODE_QUALITY_ANALYSIS.md) - Architecture overview

---

Last Updated: February 1, 2026
