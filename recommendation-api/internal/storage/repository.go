// Package storage provides database access for the recommendation API
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
)

// Repository implements the storage interfaces
type Repository struct {
	db *DB
}

// NewRepository creates a new repository
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// ListRecommendations returns recommendations, optionally filtered by namespace
func (r *Repository) ListRecommendations(ctx context.Context, namespace string) ([]rest.Recommendation, error) {
	query := `
		SELECT id, namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes, confidence, model_version,
		       status, created_at, applied_at, time_window,
		       current_cpu_request_millicores, current_cpu_limit_millicores,
		       current_memory_request_bytes, current_memory_limit_bytes
		FROM recommendations
		WHERE ($1 = '' OR namespace = $1)
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to query recommendations: %w", err)
	}
	defer rows.Close()

	var recommendations []rest.Recommendation
	for rows.Next() {
		var rec rest.Recommendation
		var appliedAt sql.NullTime
		var currentCpuReq, currentCpuLim sql.NullInt32
		var currentMemReq, currentMemLim sql.NullInt64

		err := rows.Scan(
			&rec.ID, &rec.Namespace, &rec.Deployment,
			&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
			&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
			&rec.Confidence, &rec.ModelVersion, &rec.Status,
			&rec.CreatedAt, &appliedAt, &rec.TimeWindow,
			&currentCpuReq, &currentCpuLim, &currentMemReq, &currentMemLim,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recommendation: %w", err)
		}

		if appliedAt.Valid {
			rec.AppliedAt = &appliedAt.Time
		}

		if currentCpuReq.Valid {
			rec.CurrentResources = &rest.ResourceSpec{
				CpuRequest:    fmt.Sprintf("%dm", currentCpuReq.Int32),
				CpuLimit:      fmt.Sprintf("%dm", currentCpuLim.Int32),
				MemoryRequest: formatBytes(currentMemReq.Int64),
				MemoryLimit:   formatBytes(currentMemLim.Int64),
			}
		}

		rec.RecommendedResources = &rest.ResourceSpec{
			CpuRequest:    fmt.Sprintf("%dm", rec.CpuRequestMillicores),
			CpuLimit:      fmt.Sprintf("%dm", rec.CpuLimitMillicores),
			MemoryRequest: formatBytes(int64(rec.MemoryRequestBytes)),
			MemoryLimit:   formatBytes(int64(rec.MemoryLimitBytes)),
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations, rows.Err()
}

// GetRecommendation returns a specific recommendation
func (r *Repository) GetRecommendation(ctx context.Context, namespace, name string) (*rest.Recommendation, error) {
	query := `
		SELECT id, namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes, confidence, model_version,
		       status, created_at, applied_at, time_window
		FROM recommendations
		WHERE namespace = $1 AND deployment = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var rec rest.Recommendation
	var appliedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, namespace, name).Scan(
		&rec.ID, &rec.Namespace, &rec.Deployment,
		&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
		&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
		&rec.Confidence, &rec.ModelVersion, &rec.Status,
		&rec.CreatedAt, &appliedAt, &rec.TimeWindow,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation: %w", err)
	}

	if appliedAt.Valid {
		rec.AppliedAt = &appliedAt.Time
	}

	return &rec, nil
}

// GetRecommendationByID returns a recommendation by ID
func (r *Repository) GetRecommendationByID(ctx context.Context, id string) (*rest.Recommendation, error) {
	query := `
		SELECT id, namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes, confidence, model_version,
		       status, created_at, applied_at, time_window
		FROM recommendations
		WHERE id = $1
	`

	var rec rest.Recommendation
	var appliedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rec.ID, &rec.Namespace, &rec.Deployment,
		&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
		&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
		&rec.Confidence, &rec.ModelVersion, &rec.Status,
		&rec.CreatedAt, &appliedAt, &rec.TimeWindow,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation: %w", err)
	}

	if appliedAt.Valid {
		rec.AppliedAt = &appliedAt.Time
	}

	return &rec, nil
}


// ApplyRecommendation applies a recommendation
func (r *Repository) ApplyRecommendation(ctx context.Context, id string, dryRun bool) (*rest.ApplyRecommendationResponse, error) {
	rec, err := r.GetRecommendationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("recommendation not found")
	}

	// Generate YAML patch
	yamlPatch := generateYamlPatch(rec)

	if dryRun {
		return &rest.ApplyRecommendationResponse{
			ID:        id,
			Status:    "dry_run",
			Message:   "Dry run completed - no changes applied",
			YamlPatch: yamlPatch,
		}, nil
	}

	// Update status to applied
	query := `
		UPDATE recommendations
		SET status = 'applied', applied_at = NOW()
		WHERE id = $1
	`
	_, err = r.db.ExecContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update recommendation status: %w", err)
	}

	return &rest.ApplyRecommendationResponse{
		ID:        id,
		Status:    "applied",
		Message:   "Recommendation applied successfully",
		YamlPatch: yamlPatch,
	}, nil
}

// ApproveRecommendation approves a recommendation
func (r *Repository) ApproveRecommendation(ctx context.Context, id string) (*rest.ApproveRecommendationResponse, error) {
	query := `
		UPDATE recommendations
		SET status = 'approved', approved_at = NOW()
		WHERE id = $1 AND status = 'pending'
		RETURNING id
	`

	var returnedID string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("recommendation not found or already processed")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to approve recommendation: %w", err)
	}

	return &rest.ApproveRecommendationResponse{
		ID:      id,
		Status:  "approved",
		Message: "Recommendation approved",
	}, nil
}

// GetClusterCosts returns cluster-wide cost analysis
func (r *Repository) GetClusterCosts(ctx context.Context) (*rest.CostAnalysis, error) {
	query := `
		SELECT 
			COALESCE(SUM(current_cost_monthly), 0) as current_cost,
			COALESCE(SUM(recommended_cost_monthly), 0) as recommended_cost,
			COALESCE(SUM(savings_monthly), 0) as savings,
			COALESCE(SUM(deployment_count), 0) as deployment_count,
			MAX(time) as last_updated
		FROM cost_snapshots
		WHERE time > NOW() - INTERVAL '1 day'
	`

	var costs rest.CostAnalysis
	var lastUpdated sql.NullTime

	err := r.db.QueryRowContext(ctx, query).Scan(
		&costs.CurrentMonthlyCost,
		&costs.RecommendedMonthlyCost,
		&costs.PotentialSavings,
		&costs.DeploymentCount,
		&lastUpdated,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster costs: %w", err)
	}

	costs.Currency = "USD"
	if lastUpdated.Valid {
		costs.LastUpdated = lastUpdated.Time
	} else {
		costs.LastUpdated = time.Now()
	}

	return &costs, nil
}

// GetNamespaceCosts returns namespace cost analysis
func (r *Repository) GetNamespaceCosts(ctx context.Context, namespace string) (*rest.CostAnalysis, error) {
	query := `
		SELECT 
			namespace,
			COALESCE(current_cost_monthly, 0) as current_cost,
			COALESCE(recommended_cost_monthly, 0) as recommended_cost,
			COALESCE(savings_monthly, 0) as savings,
			COALESCE(deployment_count, 0) as deployment_count,
			time as last_updated
		FROM cost_snapshots
		WHERE namespace = $1
		ORDER BY time DESC
		LIMIT 1
	`

	var costs rest.CostAnalysis
	err := r.db.QueryRowContext(ctx, query, namespace).Scan(
		&costs.Namespace,
		&costs.CurrentMonthlyCost,
		&costs.RecommendedMonthlyCost,
		&costs.PotentialSavings,
		&costs.DeploymentCount,
		&costs.LastUpdated,
	)
	if err == sql.ErrNoRows {
		return &rest.CostAnalysis{
			Namespace:    namespace,
			Currency:     "USD",
			LastUpdated:  time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace costs: %w", err)
	}

	costs.Currency = "USD"
	return &costs, nil
}

// GetSavingsReport returns savings report
func (r *Repository) GetSavingsReport(ctx context.Context, since string) (*rest.SavingsReport, error) {
	// Parse duration
	interval := "30 days"
	if since != "" {
		interval = since
	}

	query := `
		SELECT 
			to_char(time, 'YYYY-MM') as month,
			SUM(savings_monthly) as savings
		FROM cost_snapshots
		WHERE time > NOW() - $1::interval
		GROUP BY to_char(time, 'YYYY-MM')
		ORDER BY month DESC
	`

	rows, err := r.db.QueryContext(ctx, query, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get savings report: %w", err)
	}
	defer rows.Close()

	var report rest.SavingsReport
	report.Currency = "USD"
	report.Period = since

	for rows.Next() {
		var monthly rest.MonthlySaving
		if err := rows.Scan(&monthly.Month, &monthly.Savings); err != nil {
			return nil, fmt.Errorf("failed to scan monthly savings: %w", err)
		}
		report.SavingsByMonth = append(report.SavingsByMonth, monthly)
		report.TotalSavings += monthly.Savings
	}

	return &report, rows.Err()
}


// ListModels returns all model versions
func (r *Repository) ListModels(ctx context.Context) ([]rest.ModelVersion, error) {
	query := `
		SELECT version, created_at, validation_accuracy, size_bytes, is_active, rollback_count
		FROM model_versions
		ORDER BY created_at DESC
		LIMIT 10
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var models []rest.ModelVersion
	for rows.Next() {
		var m rest.ModelVersion
		if err := rows.Scan(&m.Version, &m.CreatedAt, &m.ValidationAccuracy, &m.SizeBytes, &m.IsActive, &m.RollbackCount); err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, m)
	}

	return models, rows.Err()
}

// GetModel returns a specific model version
func (r *Repository) GetModel(ctx context.Context, version string) (*rest.ModelVersion, error) {
	query := `
		SELECT version, created_at, validation_accuracy, size_bytes, is_active, rollback_count
		FROM model_versions
		WHERE version = $1
	`

	var m rest.ModelVersion
	err := r.db.QueryRowContext(ctx, query, version).Scan(
		&m.Version, &m.CreatedAt, &m.ValidationAccuracy, &m.SizeBytes, &m.IsActive, &m.RollbackCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return &m, nil
}

// RollbackModel rolls back to a specific model version
func (r *Repository) RollbackModel(ctx context.Context, version string) (*rest.RollbackResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current active version
	var previousActive string
	err = tx.QueryRowContext(ctx, "SELECT version FROM model_versions WHERE is_active = TRUE").Scan(&previousActive)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get current active model: %w", err)
	}

	// Deactivate all models
	_, err = tx.ExecContext(ctx, "UPDATE model_versions SET is_active = FALSE")
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate models: %w", err)
	}

	// Activate target version and increment rollback count
	result, err := tx.ExecContext(ctx,
		"UPDATE model_versions SET is_active = TRUE, rollback_count = rollback_count + 1 WHERE version = $1",
		version,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to activate model: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("model version not found")
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &rest.RollbackResponse{
		Version:        version,
		Status:         "rolled_back",
		Message:        "Model rolled back successfully",
		PreviousActive: previousActive,
	}, nil
}

// GetPredictionHistory returns prediction history for a deployment
func (r *Repository) GetPredictionHistory(ctx context.Context, namespace, deployment string) (*rest.PredictionHistory, error) {
	query := `
		SELECT created_at, cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes, confidence, model_version
		FROM predictions
		WHERE ($1 = '' OR namespace = $1) AND deployment = $2
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query, namespace, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction history: %w", err)
	}
	defer rows.Close()

	history := &rest.PredictionHistory{
		Deployment: deployment,
		Namespace:  namespace,
	}

	for rows.Next() {
		var p rest.Prediction
		if err := rows.Scan(
			&p.Timestamp, &p.CpuRequestMillicores, &p.CpuLimitMillicores,
			&p.MemoryRequestBytes, &p.MemoryLimitBytes, &p.Confidence, &p.ModelVersion,
		); err != nil {
			return nil, fmt.Errorf("failed to scan prediction: %w", err)
		}
		history.Predictions = append(history.Predictions, p)
	}

	if len(history.Predictions) == 0 {
		return nil, nil
	}

	return history, rows.Err()
}

// Helper functions

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%dGi", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%dMi", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%dKi", bytes/KB)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

func generateYamlPatch(rec *rest.Recommendation) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: main
        resources:
          requests:
            cpu: "%dm"
            memory: "%s"
          limits:
            cpu: "%dm"
            memory: "%s"`,
		rec.Deployment, rec.Namespace,
		rec.CpuRequestMillicores, formatBytes(int64(rec.MemoryRequestBytes)),
		rec.CpuLimitMillicores, formatBytes(int64(rec.MemoryLimitBytes)),
	)
}

// Ensure Repository implements rest.Store interface
var _ rest.Store = (*Repository)(nil)
