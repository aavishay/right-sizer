# v0.5.0 Quick Reference Guide

## What's New in v0.5.0?

### 🎯 Three New Dashboard API Endpoints

#### 1. Predict Pod Resource Usage (24 Hours)
```bash
POST /api/predictions
```
**Use Case**: See what resources a pod will need tomorrow
**Returns**: Forecast + trend + confidence score

**Example:**
```bash
curl -X POST http://localhost:3000/api/predictions \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "abc123",
    "podName": "api-server",
    "namespace": "production",
    "resourceType": "cpu",
    "hoursAhead": 24
  }'
```

**Response Highlights:**
- `predictedValue`: 280 (forecast for tomorrow)
- `trend`: "increasing" or "decreasing"
- `confidence`: 0.85 (85% confidence)
- `max`: 420 (historical peak)

---

#### 2. Get 30-Day Forecast
```bash
GET /api/forecasts?clusterId=abc123&daysAhead=30
```
**Use Case**: Plan capacity for the month ahead
**Returns**: Daily forecasts for all pods with confidence intervals

**Response Highlights:**
- Day-by-day predictions for 30 days
- Upper/lower bounds (confidence intervals)
- Average and standard deviation
- Confidence score per pod

---

#### 3. Simulate Resource Change Impact
```bash
POST /api/what-if
```
**Use Case**: "What if I scale this pod from 500m to 600m?"
**Returns**: Risk analysis + cost impact for each scenario

**Example:**
```bash
curl -X POST http://localhost:3000/api/what-if \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "abc123",
    "podName": "api-server",
    "resourceType": "cpu",
    "currentAllocation": 500,
    "proposedAllocations": "[400, 600, 800]"
  }'
```

**Response Highlights:**
- Risk level per scenario (low/medium/high)
- Monthly cost delta ($7300 in example)
- Utilization percentage
- Recommendation (risky/optimal/downscale)

---

## 🔧 Architecture

### How It Works

1. **Historical Data**: Metrics collected in TimescaleDB
2. **API Analysis**:
   - Trends via linear regression
   - Seasonality via pattern detection
   - Risk via peak utilization analysis
3. **Output**: Confidence-scored recommendations
4. **Dashboard**: React UI displays predictions + scenarios

### Data Flow
```
Metrics → Database → Predictions API → Frontend Charts
                  ↓
            What-If Analysis
                  ↓
            Scenario Recommendations
```

---

## 📊 Key Metrics

### Confidence Scoring
- **0.3-0.5**: Limited data (unreliable)
- **0.5-0.7**: Moderate confidence (use with caution)
- **0.7-0.95**: High confidence (trustworthy)

### Risk Levels
- **Low** (<70% utilization): Safe to downscale
- **Medium** (70-90%): Balanced risk/benefit
- **High** (>90%): Risk of throttling

### Cost Model
- **$0.01** per core per hour
- **730** hours per month
- Example: 100m CPU = $0.73/month

---

## 🚀 Getting Started

### Prerequisites
1. Backend server running
2. JWT authentication token
3. Historical metrics (≥24 hours)
4. Database with metrics table

### Basic Test

```bash
# 1. Set your token
export JWT="your-token-here"

# 2. Get a prediction
curl -X POST http://localhost:3000/api/predictions \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "test-cluster",
    "podName": "test-pod",
    "namespace": "default",
    "resourceType": "cpu"
  }'

# 3. Analyze scenarios
curl -X POST http://localhost:3000/api/what-if \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "clusterId": "test-cluster",
    "podName": "test-pod",
    "namespace": "default",
    "resourceType": "cpu",
    "currentAllocation": 500,
    "proposedAllocations": "[400, 600, 800]"
  }'
```

---

## 🛠️ Configuration

### Adjustable Parameters (in `go/scaling/engine.go`)

```go
bufferPercent: 0.15          // 15% safety margin
confidenceThreshold: 0.70    // Minimum acceptable confidence
maxGradualScale: 0.20        // Max 20% change per recommendation
```

### Cost Model (in `predictions.ts`)

```typescript
const CPUPricePerHour = 0.01;
const HoursPerMonth = 730;
const CostDelta = allocation_delta * CPUPricePerHour * HoursPerMonth;
```

---

## 📋 File Locations

### Backend
- `backend/src/api/predictions.ts` - All 3 endpoints (180 lines)
- `backend/src/server.ts` - Route mounting

### Go Packages (Frameworks - Ready for Implementation)
- `go/scaling/engine.go` - Scaling recommendations
- `go/whatif/simulator.go` - What-if analysis
- `go/recommendations/engine.go` - Optimization suggestions

### Documentation
- `/V05_IMPLEMENTATION_SUMMARY.md` - Full details
- This file - Quick reference

---

## ✅ Testing Checklist

- [ ] Backend builds without errors
- [ ] JWT middleware validates tokens
- [ ] Predictions endpoint returns forecast
- [ ] Forecasts endpoint returns 30-day trends
- [ ] What-if endpoint analyzes scenarios
- [ ] Confidence scores between 0-1
- [ ] Risk levels correctly assigned
- [ ] Cost deltas calculated accurately
- [ ] Error handling returns proper HTTP codes
- [ ] Response times <500ms

---

## 🐛 Troubleshooting

### "No historical data available"
**Cause**: Pod has less than 24 hours of metrics
**Fix**: Wait for metrics to accumulate or provide test data

### "Confidence too low"
**Cause**: Highly variable workload or insufficient data
**Fix**: Lower confidenceThreshold in config or wait for more data

### "Invalid JWT token"
**Cause**: Token expired or invalid
**Fix**: Get a fresh token from auth server

### Endpoint returns 500 error
**Cause**: Database query failed
**Fix**: Check database connection and metrics table schema

---

## 📞 Support

For issues or questions:
1. Check `/V05_IMPLEMENTATION_SUMMARY.md` for details
2. Review error response messages
3. Check database metrics table schema
4. Verify JWT token validity
5. Check backend logs: `npm run logs:backend`

---

## 🔄 Phase 2 (Coming Soon)

- [ ] Frontend UI components for forecasts
- [ ] Alert system for anomalies
- [ ] End-to-end integration tests
- [ ] Performance benchmarks (1000+ pods)
- [ ] Production deployment guide

---

**Status**: v0.5.0 Core Complete
**Last Updated**: January 2025
**Next Phase**: Testing & Integration
