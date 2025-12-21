// Package cost provides cost estimation and pricing functionality
package cost

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// CostSnapshot represents a point-in-time cost snapshot
type CostSnapshot struct {
	Time                   time.Time `json:"time"`
	Namespace              string    `json:"namespace"`
	CurrentCostMonthly     float64   `json:"current_cost_monthly"`
	RecommendedCostMonthly float64   `json:"recommended_cost_monthly"`
	SavingsMonthly         float64   `json:"savings_monthly"`
	DeploymentCount        int       `json:"deployment_count"`
}

// SavingsRecord represents actual savings after recommendation application
type SavingsRecord struct {
	ID                string    `json:"id"`
	RecommendationID  string    `json:"recommendation_id"`
	Namespace         string    `json:"namespace"`
	Deployment        string    `json:"deployment"`
	AppliedAt         time.Time `json:"applied_at"`
	CostBeforeMonthly float64   `json:"cost_before_monthly"`
	CostAfterMonthly  float64   `json:"cost_after_monthly"`
	ActualSavings     float64   `json:"actual_savings"`
	MeasuredAt        time.Time `json:"measured_at"`
	MeasurementPeriod int       `json:"measurement_period_days"`
}

// NamespaceSavingsReport represents savings report for a namespace
type NamespaceSavingsReport struct {
	Namespace        string    `json:"namespace"`
	TotalSavings     float64   `json:"total_savings"`
	ProjectedSavings float64   `json:"projected_savings"`
	AppliedCount     int       `json:"applied_count"`
	PendingCount     int       `json:"pending_count"`
	Period           string    `json:"period"`
	Currency         string    `json:"currency"`
	LastUpdated      time.Time `json:"last_updated"`
}

// TeamSavingsReport represents savings report for a team
type TeamSavingsReport struct {
	Team             string   `json:"team"`
	Namespaces       []string `json:"namespaces"`
	TotalSavings     float64  `json:"total_savings"`
	ProjectedSavings float64  `json:"projected_savings"`
	AppliedCount     int      `json:"applied_count"`
	Period           string   `json:"period"`
	Currency         string   `json:"currency"`
}

// SavingsTracker tracks cost savings over time
type SavingsTracker struct {
	db         *sql.DB
	calculator *Calculator
}

// NewSavingsTracker creates a new savings tracker
func NewSavingsTracker(db *sql.DB, calculator *Calculator) *SavingsTracker {
	return &SavingsTracker{
		db:         db,
		calculator: calculator,
	}
}

// RecordDailySnapshot records a daily cost snapshot for a namespace
func (t *SavingsTracker) RecordDailySnapshot(ctx context.Context, snapshot CostSnapshot) error {
	query := `
		INSERT INTO cost_snapshots (time, namespace, current_cost_monthly, recommended_cost_monthly, savings_monthly, deployment_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (time, namespace) DO UPDATE SET
			current_cost_monthly = EXCLUDED.current_cost_monthly,
			recommended_cost_monthly = EXCLUDED.recommended_cost_monthly,
			savings_monthly = EXCLUDED.savings_monthly,
			deployment_count = EXCLUDED.deployment_count
	`

	_, err := t.db.ExecContext(ctx, query,
		snapshot.Time,
		snapshot.Namespace,
		snapshot.CurrentCostMonthly,
		snapshot.RecommendedCostMonthly,
		snapshot.SavingsMonthly,
		snapshot.DeploymentCount,
	)
	if err != nil {
		return fmt.Errorf("failed to record cost snapshot: %w", err)
	}

	return nil
}

// RecordAllNamespaceSnapshots records daily snapshots for all namespaces
func (t *SavingsTracker) RecordAllNamespaceSnapshots(ctx context.Context) error {
	slog.Info("Recording daily cost snapshots")

	// Get all namespaces with recommendations
	query := `
		SELECT DISTINCT namespace FROM recommendations WHERE status IN ('pending', 'approved', 'applied')
	`

	rows, err := t.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
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

	now := time.Now().Truncate(24 * time.Hour)

	for _, namespace := range namespaces {
		snapshot, err := t.calculateNamespaceSnapshot(ctx, namespace, now)
		if err != nil {
			slog.Error("Failed to calculate snapshot", "namespace", namespace, "error", err)
			continue
		}

		if err := t.RecordDailySnapshot(ctx, *snapshot); err != nil {
			slog.Error("Failed to record snapshot", "namespace", namespace, "error", err)
			continue
		}
	}

	slog.Info("Completed recording cost snapshots", "namespaces", len(namespaces))
	return nil
}

// calculateNamespaceSnapshot calculates cost snapshot for a namespace
func (t *SavingsTracker) calculateNamespaceSnapshot(ctx context.Context, namespace string, snapshotTime time.Time) (*CostSnapshot, error) {
	query := `
		SELECT 
			COUNT(*) as deployment_count,
			COALESCE(SUM(current_cpu_request_millicores), 0) as current_cpu,
			COALESCE(SUM(current_memory_request_bytes), 0) as current_memory,
			COALESCE(SUM(cpu_request_millicores), 0) as recommended_cpu,
			COALESCE(SUM(memory_request_bytes), 0) as recommended_memory
		FROM recommendations
		WHERE namespace = $1 AND status IN ('pending', 'approved', 'applied')
	`

	var deploymentCount int
	var currentCPU, currentMemory, recommendedCPU, recommendedMemory int64

	err := t.db.QueryRowContext(ctx, query, namespace).Scan(
		&deploymentCount,
		&currentCPU,
		&currentMemory,
		&recommendedCPU,
		&recommendedMemory,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recommendations: %w", err)
	}

	currentUsage := ResourceUsage{
		Namespace:            namespace,
		CPURequestMillicores: uint32(currentCPU),
		MemoryRequestBytes:   uint64(currentMemory),
		ReplicaCount:         1,
	}

	recommendedUsage := ResourceUsage{
		Namespace:            namespace,
		CPURequestMillicores: uint32(recommendedCPU),
		MemoryRequestBytes:   uint64(recommendedMemory),
		ReplicaCount:         1,
	}

	currentCost := t.calculator.CalculateCost(currentUsage)
	recommendedCost := t.calculator.CalculateCost(recommendedUsage)

	return &CostSnapshot{
		Time:                   snapshotTime,
		Namespace:              namespace,
		CurrentCostMonthly:     currentCost.TotalCostMonthly,
		RecommendedCostMonthly: recommendedCost.TotalCostMonthly,
		SavingsMonthly:         currentCost.TotalCostMonthly - recommendedCost.TotalCostMonthly,
		DeploymentCount:        deploymentCount,
	}, nil
}

// TrackActualSavings tracks actual savings after a recommendation is applied
func (t *SavingsTracker) TrackActualSavings(ctx context.Context, recommendationID string) error {
	// Get the recommendation details
	query := `
		SELECT namespace, deployment, applied_at,
			   current_cpu_request_millicores, current_memory_request_bytes,
			   cpu_request_millicores, memory_request_bytes
		FROM recommendations
		WHERE id = $1 AND status = 'applied' AND applied_at IS NOT NULL
	`

	var namespace, deployment string
	var appliedAt time.Time
	var currentCPU, recommendedCPU sql.NullInt32
	var currentMemory, recommendedMemory sql.NullInt64

	err := t.db.QueryRowContext(ctx, query, recommendationID).Scan(
		&namespace, &deployment, &appliedAt,
		&currentCPU, &currentMemory,
		&recommendedCPU, &recommendedMemory,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("recommendation not found or not applied")
	}
	if err != nil {
		return fmt.Errorf("failed to get recommendation: %w", err)
	}

	// Calculate costs
	costBefore := t.calculator.CalculateCost(ResourceUsage{
		Namespace:            namespace,
		Deployment:           deployment,
		CPURequestMillicores: uint32(currentCPU.Int32),
		MemoryRequestBytes:   uint64(currentMemory.Int64),
		ReplicaCount:         1,
	})

	costAfter := t.calculator.CalculateCost(ResourceUsage{
		Namespace:            namespace,
		Deployment:           deployment,
		CPURequestMillicores: uint32(recommendedCPU.Int32),
		MemoryRequestBytes:   uint64(recommendedMemory.Int64),
		ReplicaCount:         1,
	})

	// Calculate measurement period
	measurementPeriod := int(time.Since(appliedAt).Hours() / 24)
	if measurementPeriod < 1 {
		measurementPeriod = 1
	}

	// Record the savings
	insertQuery := `
		INSERT INTO savings_records (recommendation_id, namespace, deployment, applied_at,
			cost_before_monthly, cost_after_monthly, actual_savings, measured_at, measurement_period_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8)
		ON CONFLICT (recommendation_id) DO UPDATE SET
			actual_savings = EXCLUDED.actual_savings,
			measured_at = EXCLUDED.measured_at,
			measurement_period_days = EXCLUDED.measurement_period_days
	`

	actualSavings := costBefore.TotalCostMonthly - costAfter.TotalCostMonthly

	_, err = t.db.ExecContext(ctx, insertQuery,
		recommendationID, namespace, deployment, appliedAt,
		costBefore.TotalCostMonthly, costAfter.TotalCostMonthly,
		actualSavings, measurementPeriod,
	)
	if err != nil {
		return fmt.Errorf("failed to record savings: %w", err)
	}

	return nil
}

// GetNamespaceSavingsReport generates a savings report for a namespace
func (t *SavingsTracker) GetNamespaceSavingsReport(ctx context.Context, namespace, period string) (*NamespaceSavingsReport, error) {
	interval := parsePeriodToInterval(period)

	query := `
		SELECT 
			COALESCE(SUM(savings_monthly), 0) as total_savings,
			COUNT(DISTINCT time) as snapshot_count
		FROM cost_snapshots
		WHERE namespace = $1 AND time > NOW() - $2::interval
	`

	var totalSavings float64
	var snapshotCount int

	err := t.db.QueryRowContext(ctx, query, namespace, interval).Scan(&totalSavings, &snapshotCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get savings: %w", err)
	}

	// Get recommendation counts
	countQuery := `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'applied') as applied_count,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count
		FROM recommendations
		WHERE namespace = $1
	`

	var appliedCount, pendingCount int
	err = t.db.QueryRowContext(ctx, countQuery, namespace).Scan(&appliedCount, &pendingCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get recommendation counts: %w", err)
	}

	// Get projected savings from pending recommendations
	projectedQuery := `
		SELECT COALESCE(SUM(
			(COALESCE(current_cpu_request_millicores, 0) - cpu_request_millicores) * 0.0425 * 730 / 1000 +
			(COALESCE(current_memory_request_bytes, 0) - memory_request_bytes) * 0.00533 * 730 / 1073741824
		), 0)
		FROM recommendations
		WHERE namespace = $1 AND status = 'pending'
	`

	var projectedSavings float64
	t.db.QueryRowContext(ctx, projectedQuery, namespace).Scan(&projectedSavings)

	return &NamespaceSavingsReport{
		Namespace:        namespace,
		TotalSavings:     roundToTwoDecimals(totalSavings),
		ProjectedSavings: roundToTwoDecimals(projectedSavings),
		AppliedCount:     appliedCount,
		PendingCount:     pendingCount,
		Period:           period,
		Currency:         t.calculator.pricing.Currency,
		LastUpdated:      time.Now(),
	}, nil
}

// GetTeamSavingsReport generates a savings report for a team (multiple namespaces)
func (t *SavingsTracker) GetTeamSavingsReport(ctx context.Context, teamLabel, period string) (*TeamSavingsReport, error) {
	interval := parsePeriodToInterval(period)

	// Get namespaces for the team (assuming team label is stored in namespace metadata)
	// For simplicity, we'll use namespace prefix matching
	query := `
		SELECT DISTINCT namespace
		FROM cost_snapshots
		WHERE namespace LIKE $1 || '%' AND time > NOW() - $2::interval
	`

	rows, err := t.db.QueryContext(ctx, query, teamLabel, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get team namespaces: %w", err)
	}
	defer rows.Close()

	var namespaces []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, fmt.Errorf("failed to scan namespace: %w", err)
		}
		namespaces = append(namespaces, ns)
	}

	// Aggregate savings across namespaces
	var totalSavings, projectedSavings float64
	var appliedCount int

	for _, ns := range namespaces {
		report, err := t.GetNamespaceSavingsReport(ctx, ns, period)
		if err != nil {
			continue
		}
		totalSavings += report.TotalSavings
		projectedSavings += report.ProjectedSavings
		appliedCount += report.AppliedCount
	}

	return &TeamSavingsReport{
		Team:             teamLabel,
		Namespaces:       namespaces,
		TotalSavings:     roundToTwoDecimals(totalSavings),
		ProjectedSavings: roundToTwoDecimals(projectedSavings),
		AppliedCount:     appliedCount,
		Period:           period,
		Currency:         t.calculator.pricing.Currency,
	}, nil
}

// GetClusterSavingsReport generates a cluster-wide savings report
func (t *SavingsTracker) GetClusterSavingsReport(ctx context.Context, period string) ([]NamespaceSavingsReport, error) {
	interval := parsePeriodToInterval(period)

	query := `
		SELECT DISTINCT namespace
		FROM cost_snapshots
		WHERE time > NOW() - $1::interval
	`

	rows, err := t.db.QueryContext(ctx, query, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespaces: %w", err)
	}
	defer rows.Close()

	var reports []NamespaceSavingsReport
	for rows.Next() {
		var namespace string
		if err := rows.Scan(&namespace); err != nil {
			return nil, fmt.Errorf("failed to scan namespace: %w", err)
		}

		report, err := t.GetNamespaceSavingsReport(ctx, namespace, period)
		if err != nil {
			slog.Error("Failed to get namespace report", "namespace", namespace, "error", err)
			continue
		}
		reports = append(reports, *report)
	}

	return reports, nil
}

// GetSavingsHistory returns historical savings data
func (t *SavingsTracker) GetSavingsHistory(ctx context.Context, namespace, period string) ([]CostSnapshot, error) {
	interval := parsePeriodToInterval(period)

	query := `
		SELECT time, namespace, current_cost_monthly, recommended_cost_monthly, savings_monthly, deployment_count
		FROM cost_snapshots
		WHERE ($1 = '' OR namespace = $1) AND time > NOW() - $2::interval
		ORDER BY time DESC
	`

	rows, err := t.db.QueryContext(ctx, query, namespace, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get savings history: %w", err)
	}
	defer rows.Close()

	var snapshots []CostSnapshot
	for rows.Next() {
		var s CostSnapshot
		if err := rows.Scan(&s.Time, &s.Namespace, &s.CurrentCostMonthly, &s.RecommendedCostMonthly, &s.SavingsMonthly, &s.DeploymentCount); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}

	return snapshots, nil
}

// parsePeriodToInterval converts period string to PostgreSQL interval
func parsePeriodToInterval(period string) string {
	switch period {
	case "7d":
		return "7 days"
	case "14d":
		return "14 days"
	case "30d":
		return "30 days"
	case "90d":
		return "90 days"
	case "1y":
		return "365 days"
	default:
		return "30 days"
	}
}
