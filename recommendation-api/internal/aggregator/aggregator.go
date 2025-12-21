// Package aggregator provides recommendation aggregation logic
package aggregator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Aggregator handles prediction aggregation into recommendations
type Aggregator struct {
	db *sql.DB
}

// New creates a new aggregator
func New(db *sql.DB) *Aggregator {
	return &Aggregator{db: db}
}

// AggregatedRecommendation represents an aggregated recommendation
type AggregatedRecommendation struct {
	Namespace            string
	Deployment           string
	CpuRequestMillicores uint32
	CpuLimitMillicores   uint32
	MemoryRequestBytes   uint64
	MemoryLimitBytes     uint64
	Confidence           float32
	ModelVersion         string
	TimeWindow           string
	PredictionCount      int
}

// DeploymentStats holds statistics for a deployment
type DeploymentStats struct {
	Namespace       string
	Deployment      string
	PredictionCount int
	AvgConfidence   float32
	LastPrediction  time.Time
}

// NamespaceStats holds statistics for a namespace
type NamespaceStats struct {
	Namespace        string
	DeploymentCount  int
	TotalPredictions int
	AvgConfidence    float32
	PotentialSavings float64
}

// ClusterStats holds cluster-wide statistics
type ClusterStats struct {
	TotalNamespaces  int
	TotalDeployments int
	TotalPredictions int
	AvgConfidence    float32
	TotalSavings     float64
}

// AggregateByDeployment aggregates predictions by deployment
func (a *Aggregator) AggregateByDeployment(ctx context.Context, namespace, deployment string, timeWindow string) (*AggregatedRecommendation, error) {
	// Get predictions for the deployment within the time window
	windowDuration := getWindowDuration(timeWindow)

	query := `
		SELECT 
			namespace,
			deployment,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY cpu_request_millicores) as cpu_request_p95,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY cpu_limit_millicores) as cpu_limit_p95,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY memory_request_bytes) as memory_request_p95,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY memory_limit_bytes) as memory_limit_p95,
			AVG(confidence) as avg_confidence,
			MAX(model_version) as model_version,
			COUNT(*) as prediction_count
		FROM predictions
		WHERE namespace = $1 
		  AND deployment = $2
		  AND created_at > NOW() - $3::interval
		GROUP BY namespace, deployment
	`

	var rec AggregatedRecommendation
	var cpuReqP95, cpuLimP95, memReqP95, memLimP95 float64

	err := a.db.QueryRowContext(ctx, query, namespace, deployment, windowDuration).Scan(
		&rec.Namespace,
		&rec.Deployment,
		&cpuReqP95,
		&cpuLimP95,
		&memReqP95,
		&memLimP95,
		&rec.Confidence,
		&rec.ModelVersion,
		&rec.PredictionCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate predictions: %w", err)
	}

	// Apply 20% buffer for memory as per requirements
	rec.CpuRequestMillicores = uint32(cpuReqP95)
	rec.CpuLimitMillicores = uint32(cpuLimP95)
	rec.MemoryRequestBytes = uint64(memReqP95 * 1.2) // 20% buffer
	rec.MemoryLimitBytes = uint64(memLimP95 * 1.2)   // 20% buffer
	rec.TimeWindow = timeWindow

	return &rec, nil
}

// AggregateNamespace aggregates all deployments in a namespace
func (a *Aggregator) AggregateNamespace(ctx context.Context, namespace string) ([]AggregatedRecommendation, error) {
	query := `
		SELECT DISTINCT deployment
		FROM predictions
		WHERE namespace = $1
		  AND created_at > NOW() - INTERVAL '7 days'
	`

	rows, err := a.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	defer rows.Close()

	var recommendations []AggregatedRecommendation
	for rows.Next() {
		var deployment string
		if err := rows.Scan(&deployment); err != nil {
			return nil, fmt.Errorf("failed to scan deployment: %w", err)
		}

		// Aggregate for each time window
		for _, window := range []string{"peak", "off_peak", "weekly"} {
			rec, err := a.AggregateByDeployment(ctx, namespace, deployment, window)
			if err != nil {
				slog.Error("Failed to aggregate deployment", "namespace", namespace, "deployment", deployment, "error", err)
				continue
			}
			if rec != nil {
				recommendations = append(recommendations, *rec)
			}
		}
	}

	return recommendations, rows.Err()
}

// GetNamespaceStats returns statistics for a namespace
func (a *Aggregator) GetNamespaceStats(ctx context.Context, namespace string) (*NamespaceStats, error) {
	query := `
		SELECT 
			namespace,
			COUNT(DISTINCT deployment) as deployment_count,
			COUNT(*) as total_predictions,
			AVG(confidence) as avg_confidence
		FROM predictions
		WHERE namespace = $1
		  AND created_at > NOW() - INTERVAL '7 days'
		GROUP BY namespace
	`

	var stats NamespaceStats
	err := a.db.QueryRowContext(ctx, query, namespace).Scan(
		&stats.Namespace,
		&stats.DeploymentCount,
		&stats.TotalPredictions,
		&stats.AvgConfidence,
	)
	if err == sql.ErrNoRows {
		return &NamespaceStats{Namespace: namespace}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace stats: %w", err)
	}

	// Calculate potential savings from cost snapshots
	savingsQuery := `
		SELECT COALESCE(savings_monthly, 0)
		FROM cost_snapshots
		WHERE namespace = $1
		ORDER BY time DESC
		LIMIT 1
	`
	a.db.QueryRowContext(ctx, savingsQuery, namespace).Scan(&stats.PotentialSavings)

	return &stats, nil
}

// GetClusterStats returns cluster-wide statistics
func (a *Aggregator) GetClusterStats(ctx context.Context) (*ClusterStats, error) {
	query := `
		SELECT 
			COUNT(DISTINCT namespace) as namespace_count,
			COUNT(DISTINCT deployment) as deployment_count,
			COUNT(*) as total_predictions,
			AVG(confidence) as avg_confidence
		FROM predictions
		WHERE created_at > NOW() - INTERVAL '7 days'
	`

	var stats ClusterStats
	err := a.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalNamespaces,
		&stats.TotalDeployments,
		&stats.TotalPredictions,
		&stats.AvgConfidence,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster stats: %w", err)
	}

	// Calculate total savings
	savingsQuery := `
		SELECT COALESCE(SUM(savings_monthly), 0)
		FROM cost_snapshots
		WHERE time > NOW() - INTERVAL '1 day'
	`
	a.db.QueryRowContext(ctx, savingsQuery).Scan(&stats.TotalSavings)

	return &stats, nil
}

// GenerateRecommendations generates recommendations from aggregated predictions
func (a *Aggregator) GenerateRecommendations(ctx context.Context) error {
	slog.Info("Starting recommendation generation")

	// Get all namespaces with recent predictions
	query := `
		SELECT DISTINCT namespace
		FROM predictions
		WHERE created_at > NOW() - INTERVAL '7 days'
	`

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	defer rows.Close()

	var namespaces []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return fmt.Errorf("failed to scan namespace: %w", err)
		}
		namespaces = append(namespaces, ns)
	}

	// Generate recommendations for each namespace
	for _, namespace := range namespaces {
		recommendations, err := a.AggregateNamespace(ctx, namespace)
		if err != nil {
			slog.Error("Failed to aggregate namespace", "namespace", namespace, "error", err)
			continue
		}

		for _, rec := range recommendations {
			if err := a.upsertRecommendation(ctx, &rec); err != nil {
				slog.Error("Failed to upsert recommendation",
					"namespace", rec.Namespace,
					"deployment", rec.Deployment,
					"error", err)
			}
		}
	}

	slog.Info("Recommendation generation completed", "namespaces", len(namespaces))
	return nil
}

// upsertRecommendation inserts or updates a recommendation
func (a *Aggregator) upsertRecommendation(ctx context.Context, rec *AggregatedRecommendation) error {
	query := `
		INSERT INTO recommendations (
			namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
			memory_request_bytes, memory_limit_bytes, confidence, model_version,
			time_window, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
		ON CONFLICT (namespace, deployment, time_window) 
		DO UPDATE SET
			cpu_request_millicores = EXCLUDED.cpu_request_millicores,
			cpu_limit_millicores = EXCLUDED.cpu_limit_millicores,
			memory_request_bytes = EXCLUDED.memory_request_bytes,
			memory_limit_bytes = EXCLUDED.memory_limit_bytes,
			confidence = EXCLUDED.confidence,
			model_version = EXCLUDED.model_version,
			created_at = NOW()
		WHERE recommendations.status = 'pending'
	`

	_, err := a.db.ExecContext(ctx, query,
		rec.Namespace, rec.Deployment,
		rec.CpuRequestMillicores, rec.CpuLimitMillicores,
		rec.MemoryRequestBytes, rec.MemoryLimitBytes,
		rec.Confidence, rec.ModelVersion, rec.TimeWindow,
	)
	return err
}

// GetTimeWindowRecommendations returns recommendations for a specific time window
func (a *Aggregator) GetTimeWindowRecommendations(ctx context.Context, timeWindow string) ([]AggregatedRecommendation, error) {
	query := `
		SELECT 
			namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
			memory_request_bytes, memory_limit_bytes, confidence, model_version, time_window
		FROM recommendations
		WHERE time_window = $1 AND status = 'pending'
		ORDER BY confidence DESC
	`

	rows, err := a.db.QueryContext(ctx, query, timeWindow)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}
	defer rows.Close()

	var recommendations []AggregatedRecommendation
	for rows.Next() {
		var rec AggregatedRecommendation
		if err := rows.Scan(
			&rec.Namespace, &rec.Deployment,
			&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
			&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
			&rec.Confidence, &rec.ModelVersion, &rec.TimeWindow,
		); err != nil {
			return nil, fmt.Errorf("failed to scan recommendation: %w", err)
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations, rows.Err()
}

// Helper function to get window duration
func getWindowDuration(timeWindow string) string {
	switch timeWindow {
	case "peak":
		return "24 hours"
	case "off_peak":
		return "24 hours"
	case "weekly":
		return "7 days"
	default:
		return "24 hours"
	}
}
