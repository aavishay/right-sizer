package aiops

import (
	"context"
	dashboardapi "right-sizer/dashboard-api"
	narrative "right-sizer/internal/aiops/narratives"
	"right-sizer/logger"
	"right-sizer/metrics"
	"time"
)

// HealthSnapshot represents a high-level overview of cluster health.
type HealthSnapshot struct {
	Timestamp       time.Time          `json:"timestamp"`
	ClusterID       string             `json:"clusterId"`
	ActiveIncidents int                `json:"activeIncidents"`
	RecentIncidents int                `json:"recentIncidents"` // last 24h
	ResourceUsage   map[string]float64 `json:"resourceUsage"`
	Recommendations int                `json:"recommendations"`
	Summary         string             `json:"summary"`
	Score           int                `json:"score"` // 0-100
}

// HealthReporter periodically aggregates cluster state and generates an AI summary.
type HealthReporter struct {
	store           *IncidentStore
	metricsProvider metrics.Provider
	dashboardClient *dashboardapi.Client
	narrativeGen    *narrative.NarrativeGenerator
	clusterID       string
	interval        time.Duration
	stopCh          chan struct{}
}

// NewHealthReporter creates a new health reporter.
func NewHealthReporter(
	store *IncidentStore,
	mp metrics.Provider,
	dc *dashboardapi.Client,
	ng *narrative.NarrativeGenerator,
	clusterID string,
) *HealthReporter {
	return &HealthReporter{
		store:           store,
		metricsProvider: mp,
		dashboardClient: dc,
		narrativeGen:    ng,
		clusterID:       clusterID,
		interval:        1 * time.Hour, // Default to hourly
		stopCh:          make(chan struct{}),
	}
}

// Start begins the periodic health reporting loop.
func (r *HealthReporter) Start(ctx context.Context) {
	logger.Info("[AIOPS] HealthReporter started with interval %v", r.interval)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Initial report
	r.ReportHealth(ctx)

	for {
		select {
		case <-ticker.C:
			r.ReportHealth(ctx)
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the reporter.
func (r *HealthReporter) Stop() {
	close(r.stopCh)
}

// ReportHealth performs the aggregation and AI summary generation.
func (r *HealthReporter) ReportHealth(ctx context.Context) {
	snapshot := r.collectSnapshot()

	// Generate AI Summary
	summary, score, err := r.narrativeGen.GenerateHealthSnapshotSummary(ctx, snapshot.ActiveIncidents, snapshot.RecentIncidents, snapshot.ResourceUsage)
	if err != nil {
		logger.Error("[AIOPS] Failed to generate health summary: %v", err)
		snapshot.Summary = "Unable to generate health summary at this time."
		snapshot.Score = 70 // Default "okay" score
	} else {
		snapshot.Summary = summary
		snapshot.Score = score
	}

	// Send to dashboard
	if r.dashboardClient != nil {
		status := dashboardapi.Status{
			ClusterID:       r.clusterID,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
			HealthSnapshot:  snapshot.Summary,
			HealthScore:     snapshot.Score,
			ActiveIncidents: snapshot.ActiveIncidents,
		}
		if err := r.dashboardClient.SendStatus(status); err != nil {
			logger.Error("[AIOPS] Failed to send health status to dashboard: %v", err)
		} else {
			logger.Info("[AIOPS] Health report sent to dashboard. Score: %d", snapshot.Score)
		}
	}
}

func (r *HealthReporter) collectSnapshot() HealthSnapshot {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	// Collect Incidents
	active := r.store.List(IncidentFilter{
		Statuses: []IncidentStatus{StatusDetected, StatusAnalyzing, StatusCorrelating, StatusExplained},
	})

	recent := r.store.List(IncidentFilter{
		UpdatedSince: &dayAgo,
	})

	// Collect Metrics (Simplified aggregation)
	// In a real scenario, we'd iterate over nodes/pods
	// For now, we use the metricsProvider to get an idea of the cluster load if possible
	resourceUsage := make(map[string]float64)
	// Placeholder: r.metricsProvider.GetClusterUsage() is not in the current interface
	// We'll rely on the count of incidents and recommendations primarily for the AI summary

	return HealthSnapshot{
		Timestamp:       now,
		ClusterID:       r.clusterID,
		ActiveIncidents: len(active),
		RecentIncidents: len(recent),
		ResourceUsage:   resourceUsage,
		Recommendations: 0, // Would be fetched from recommendation manager
	}
}
