# 🗺️ Right-Sizer Roadmap

This document outlines the development trajectory for the Right-Sizer project. It serves as a guide for contributors and users to understand the upcoming features, improvements, and long-term vision.

> **Note:** This roadmap is a living document and subject to change based on community feedback and project priorities.

## 🌟 Vision

To provide the most intelligent, safe, and zero-downtime resource optimization platform for Kubernetes, enabling organizations to maximize efficiency without compromising application stability.

---

## 📅 Release Plan

### ✅ Current Status: v0.2.x (Beta)
- **Core Functionality:** In-place pod resizing for CPU and Memory.
- **Modes:** Adaptive, Conservative, Aggressive, Balanced strategies.
- **Safety:** Respects LimitRanges, Quotas, and PodDisruptionBudgets.
- **Observability:** Basic Prometheus metrics and Health endpoints.

### ✅ v0.3.0: Reliability & Observability (COMPLETE)
*Focus: Technical debt reduction, stability, and enhanced insights.*

- **Technical Debt & Code Quality**
  - [x] **Context Propagation:** Eliminate overuse of `context.TODO()` throughout the controller logic to ensure proper timeout and cancellation handling.
  - [x] **Structured Logging:** Complete migration from standard `log` to structured logging (Zap/Logr) across all modules.
  - [x] **Error Handling:** Standardize error wrapping and reporting.

- **Metrics & Monitoring**
  - [x] **Granular Metrics:** Implement `RecordDeferredResize` and `RecordRetryProcessing` metrics.
  - [x] **Health API:** Connect `/readyz/detailed` to real-time internal component status.
  - [x] **Latency Goals:** Optimize query latency to <100ms (achieved 61.9ns with caching).

- **Remediation Engine**
  - [x] **Test Coverage:** Increase unit test coverage for the Remediation Engine to >50% (achieved 63.8%).

### ✅ v0.4.0: Intelligence & ML (COMPLETE)
*Focus: Smarter decision making and predictive capabilities.*

- **Advanced Analytics**
  - [x] **Memory Store** (Package: `go/memstore/`): Enhanced internal state management for tracking long-term pod behavior. Features sliding windows, statistical analysis (mean, stddev, min, max), trend detection, and percentile calculations. Test coverage: 78.7%. Performance: 100k+ ops/sec. PR #41.
  - [x] **ML-based Anomaly Detection** (Package: `go/anomaly/`): Z-score based detection to distinguish abnormal usage patterns from legitimate load spikes. Configurable sensitivity (default 2.5σ = 98.8% confidence). Test coverage: 26.5%. PR #41.
  - [x] **Enhanced Predictor: Seasonal Patterns** (Package: `go/predictor/seasonal.go`): Linear regression-based prediction using daily/weekly patterns. Extracts hour-of-day and day-of-week seasonality. Confidence scaling with 95% CI bounds. Burst vs anomaly discrimination. Test coverage: 78.9%. PR #43.
  - [ ] **Predictive Scaling:** Move beyond reactive resizing to proactive resource adjustment based on historical patterns (future v0.5.0).

- **Dashboard Integration**
  - [ ] **Visual Recommendations:** Deeper integration with the Right-Sizer Dashboard for visualizing "What-If" scenarios and predictions (future v0.5.0).

### � v0.5.0: Predictive Intelligence (Short Term)
*Focus: Proactive resource optimization and dashboard integration.*

- **Predictive Scaling & Optimization**
  - [ ] **Predictive Scaling:** Use seasonal patterns to proactively adjust resources before demand spikes
  - [ ] **What-If Analysis:** Model resource changes to predict impact on workload performance
  - [ ] **Capacity Planning:** Forecast future resource requirements based on growth trends

- **Dashboard Integration**
  - [ ] **Visual Predictions:** Display seasonal patterns and confidence intervals in dashboard UI
  - [ ] **Recommendation Engine:** Suggest optimal resource assignments based on historical patterns
  - [ ] **Alert Integration:** Notify teams of anomalies and predicted resource shortfalls

### �🚀 v1.0.0: General Availability (Long Term)
*Focus: Enterprise readiness, full feature support, and stability.*

- **Extended Kubernetes Support**
  - [ ] **Init Containers:** Support for resizing Init Containers.
  - [ ] **Ephemeral Containers:** Support for or exclusion logic handling Ephemeral (debug) containers.
  - [ ] **Sidecars:** Intelligent handling of sidecar containers (e.g., service mesh proxies).

- **Enterprise Features**
  - [ ] **Multi-Tenancy:** Advanced RBAC and quota management for multi-tenant clusters.
  - [ ] **Compliance Reporting:** Automated PDF/CSV reports on savings and efficiency gains.
  - [ ] **Plugin System:** Webhooks for external approval flows before applying aggressive resizes.

---

## 📥 Backlog & Future Ideas

- **Cost Analysis:** Direct integration with cloud provider billing APIs (AWS, GCP, Azure) to show dollar-amount savings.
- **VPA Integration:** Conflict resolution or cooperation mode with Kubernetes Vertical Pod Autoscaler.
- **GitOps Integration:** Automatically generate Pull Requests for `RightSizerConfig` changes instead of applying them directly.
- **Energy Efficiency:** "Green Mode" to optimize for carbon footprint reduction during off-peak hours.

---

## 🤝 Contributing

We welcome contributions to any of these roadmap items! Please check the [Issues](https://github.com/aavishay/right-sizer/issues) tab to see if work has already started on a specific item, or open a new RFC to discuss your ideas.
