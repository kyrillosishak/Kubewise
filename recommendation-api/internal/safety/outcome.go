// Package safety provides safety and rollout features for recommendations
package safety

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// OutcomeStatus represents the status of a recommendation outcome
type OutcomeStatus string

const (
	OutcomeStatusMonitoring OutcomeStatus = "monitoring"
	OutcomeStatusSuccess    OutcomeStatus = "success"
	OutcomeStatusDegraded   OutcomeStatus = "degraded"
	OutcomeStatusRolledBack OutcomeStatus = "rolled_back"
)

// RecommendationOutcome tracks the outcome of an applied recommendation
type RecommendationOutcome struct {
	ID                       string        `json:"id"`
	RecommendationID         string        `json:"recommendation_id"`
	Namespace                string        `json:"namespace"`
	Deployment               string        `json:"deployment"`
	AppliedAt                time.Time     `json:"applied_at"`
	CheckTime                time.Time     `json:"check_time"`
	OOMKillsBefore           int           `json:"oom_kills_before"`
	OOMKillsAfter            int           `json:"oom_kills_after"`
	CPUThrottleBefore        float64       `json:"cpu_throttle_before"`
	CPUThrottleAfter         float64       `json:"cpu_throttle_after"`
	MemoryUsageP95Before     int64         `json:"memory_usage_p95_before,omitempty"`
	MemoryUsageP95After      int64         `json:"memory_usage_p95_after,omitempty"`
	CPUUsageP95Before        float64       `json:"cpu_usage_p95_before,omitempty"`
	CPUUsageP95After         float64       `json:"cpu_usage_p95_after,omitempty"`
	OutcomeStatus            OutcomeStatus `json:"outcome_status"`
	RollbackTriggered        bool          `json:"rollback_triggered"`
	RollbackRecommendationID string        `json:"rollback_recommendation_id,omitempty"`
}

// OutcomeMetrics represents metrics collected for outcome tracking
type OutcomeMetrics struct {
	Namespace      string    `json:"namespace"`
	Deployment     string    `json:"deployment"`
	OOMKills       int       `json:"oom_kills"`
	CPUThrottle    float64   `json:"cpu_throttle"`
	MemoryUsageP95 int64     `json:"memory_usage_p95"`
	CPUUsageP95    float64   `json:"cpu_usage_p95"`
	CollectedAt    time.Time `json:"collected_at"`
}

// OutcomeService handles outcome tracking operations
type OutcomeService struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewOutcomeService creates a new outcome service
func NewOutcomeService(db *sql.DB, logger *slog.Logger) *OutcomeService {
	if logger == nil {
		logger = slog.Default()
	}
	return &OutcomeService{
		db:     db,
		logger: logger,
	}
}

// StartTracking starts tracking the outcome of an applied recommendation
func (s *OutcomeService) StartTracking(ctx context.Context, recommendationID string, baselineMetrics *OutcomeMetrics) error {
	// Get recommendation details
	var namespace, deployment string
	var appliedAt sql.NullTime

	query := `
		SELECT namespace, deployment, applied_at
		FROM recommendations
		WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, recommendationID).Scan(&namespace, &deployment, &appliedAt)
	if err != nil {
		return fmt.Errorf("failed to get recommendation: %w", err)
	}

	if !appliedAt.Valid {
		return fmt.Errorf("recommendation has not been applied")
	}

	// Create outcome record
	insertQuery := `
		INSERT INTO recommendation_outcomes (
			recommendation_id, namespace, deployment, applied_at,
			oom_kills_before, cpu_throttle_before,
			memory_usage_p95_before, cpu_usage_p95_before,
			outcome_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (recommendation_id) DO UPDATE SET
			oom_kills_before = EXCLUDED.oom_kills_before,
			cpu_throttle_before = EXCLUDED.cpu_throttle_before,
			memory_usage_p95_before = EXCLUDED.memory_usage_p95_before,
			cpu_usage_p95_before = EXCLUDED.cpu_usage_p95_before
	`

	_, err = s.db.ExecContext(ctx, insertQuery,
		recommendationID, namespace, deployment, appliedAt.Time,
		baselineMetrics.OOMKills, baselineMetrics.CPUThrottle,
		baselineMetrics.MemoryUsageP95, baselineMetrics.CPUUsageP95,
		OutcomeStatusMonitoring,
	)
	if err != nil {
		return fmt.Errorf("failed to create outcome record: %w", err)
	}

	s.logger.Info("started outcome tracking",
		"recommendation_id", recommendationID,
		"namespace", namespace,
		"deployment", deployment,
	)

	return nil
}

// UpdateOutcome updates the outcome with current metrics
func (s *OutcomeService) UpdateOutcome(ctx context.Context, recommendationID string, currentMetrics *OutcomeMetrics) (*RecommendationOutcome, error) {
	// Get existing outcome
	outcome, err := s.GetOutcome(ctx, recommendationID)
	if err != nil {
		return nil, err
	}
	if outcome == nil {
		return nil, fmt.Errorf("outcome not found for recommendation")
	}

	// Update with current metrics
	outcome.OOMKillsAfter = currentMetrics.OOMKills
	outcome.CPUThrottleAfter = currentMetrics.CPUThrottle
	outcome.MemoryUsageP95After = currentMetrics.MemoryUsageP95
	outcome.CPUUsageP95After = currentMetrics.CPUUsageP95
	outcome.CheckTime = time.Now()

	// Evaluate outcome status
	outcome.OutcomeStatus = s.evaluateOutcomeStatus(outcome)

	// Update database
	updateQuery := `
		UPDATE recommendation_outcomes
		SET oom_kills_after = $2, cpu_throttle_after = $3,
		    memory_usage_p95_after = $4, cpu_usage_p95_after = $5,
		    check_time = $6, outcome_status = $7
		WHERE recommendation_id = $1
	`

	_, err = s.db.ExecContext(ctx, updateQuery,
		recommendationID,
		outcome.OOMKillsAfter, outcome.CPUThrottleAfter,
		outcome.MemoryUsageP95After, outcome.CPUUsageP95After,
		outcome.CheckTime, outcome.OutcomeStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update outcome: %w", err)
	}

	// Also update the recommendation's outcome fields
	recUpdateQuery := `
		UPDATE recommendations
		SET outcome_oom_kills = $2, outcome_throttle_increase = $3
		WHERE id = $1
	`
	throttleIncrease := outcome.CPUThrottleAfter - outcome.CPUThrottleBefore
	_, err = s.db.ExecContext(ctx, recUpdateQuery, recommendationID, outcome.OOMKillsAfter, throttleIncrease)
	if err != nil {
		s.logger.Warn("failed to update recommendation outcome fields", "error", err)
	}

	s.logger.Info("outcome updated",
		"recommendation_id", recommendationID,
		"status", outcome.OutcomeStatus,
		"oom_kills_delta", outcome.OOMKillsAfter-outcome.OOMKillsBefore,
	)

	return outcome, nil
}

func (s *OutcomeService) evaluateOutcomeStatus(outcome *RecommendationOutcome) OutcomeStatus {
	// Check for OOM kill increase
	oomDelta := outcome.OOMKillsAfter - outcome.OOMKillsBefore
	if oomDelta > 0 {
		return OutcomeStatusDegraded
	}

	// Check for significant throttle increase (>10%)
	if outcome.CPUThrottleBefore > 0 {
		throttleIncrease := (outcome.CPUThrottleAfter - outcome.CPUThrottleBefore) / outcome.CPUThrottleBefore
		if throttleIncrease > 0.1 {
			return OutcomeStatusDegraded
		}
	} else if outcome.CPUThrottleAfter > 0.05 { // New throttling above 5%
		return OutcomeStatusDegraded
	}

	// If monitoring period is complete (1 hour after application), mark as success
	if time.Since(outcome.AppliedAt) > time.Hour {
		return OutcomeStatusSuccess
	}

	return OutcomeStatusMonitoring
}

// GetOutcome returns the outcome for a recommendation
func (s *OutcomeService) GetOutcome(ctx context.Context, recommendationID string) (*RecommendationOutcome, error) {
	query := `
		SELECT id, recommendation_id, namespace, deployment, applied_at, check_time,
		       oom_kills_before, oom_kills_after, cpu_throttle_before, cpu_throttle_after,
		       memory_usage_p95_before, memory_usage_p95_after,
		       cpu_usage_p95_before, cpu_usage_p95_after,
		       outcome_status, rollback_triggered, rollback_recommendation_id
		FROM recommendation_outcomes
		WHERE recommendation_id = $1
	`

	var outcome RecommendationOutcome
	var rollbackRecID sql.NullString
	var memBefore, memAfter sql.NullInt64
	var cpuBefore, cpuAfter sql.NullFloat64

	err := s.db.QueryRowContext(ctx, query, recommendationID).Scan(
		&outcome.ID, &outcome.RecommendationID, &outcome.Namespace, &outcome.Deployment,
		&outcome.AppliedAt, &outcome.CheckTime,
		&outcome.OOMKillsBefore, &outcome.OOMKillsAfter,
		&outcome.CPUThrottleBefore, &outcome.CPUThrottleAfter,
		&memBefore, &memAfter, &cpuBefore, &cpuAfter,
		&outcome.OutcomeStatus, &outcome.RollbackTriggered, &rollbackRecID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get outcome: %w", err)
	}

	if rollbackRecID.Valid {
		outcome.RollbackRecommendationID = rollbackRecID.String
	}
	if memBefore.Valid {
		outcome.MemoryUsageP95Before = memBefore.Int64
	}
	if memAfter.Valid {
		outcome.MemoryUsageP95After = memAfter.Int64
	}
	if cpuBefore.Valid {
		outcome.CPUUsageP95Before = cpuBefore.Float64
	}
	if cpuAfter.Valid {
		outcome.CPUUsageP95After = cpuAfter.Float64
	}

	return &outcome, nil
}

// ListDegradedOutcomes returns outcomes that are in degraded status
func (s *OutcomeService) ListDegradedOutcomes(ctx context.Context) ([]RecommendationOutcome, error) {
	query := `
		SELECT id, recommendation_id, namespace, deployment, applied_at, check_time,
		       oom_kills_before, oom_kills_after, cpu_throttle_before, cpu_throttle_after,
		       outcome_status, rollback_triggered
		FROM recommendation_outcomes
		WHERE outcome_status = $1 AND rollback_triggered = FALSE
		ORDER BY check_time DESC
	`

	rows, err := s.db.QueryContext(ctx, query, OutcomeStatusDegraded)
	if err != nil {
		return nil, fmt.Errorf("failed to list degraded outcomes: %w", err)
	}
	defer rows.Close()

	var outcomes []RecommendationOutcome
	for rows.Next() {
		var outcome RecommendationOutcome
		err := rows.Scan(
			&outcome.ID, &outcome.RecommendationID, &outcome.Namespace, &outcome.Deployment,
			&outcome.AppliedAt, &outcome.CheckTime,
			&outcome.OOMKillsBefore, &outcome.OOMKillsAfter,
			&outcome.CPUThrottleBefore, &outcome.CPUThrottleAfter,
			&outcome.OutcomeStatus, &outcome.RollbackTriggered,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outcome: %w", err)
		}
		outcomes = append(outcomes, outcome)
	}

	return outcomes, rows.Err()
}

// GetRecentlyAppliedRecommendations returns recommendations applied within the monitoring window
func (s *OutcomeService) GetRecentlyAppliedRecommendations(ctx context.Context, monitoringWindow time.Duration) ([]string, error) {
	query := `
		SELECT id
		FROM recommendations
		WHERE status = 'applied' AND applied_at > $1
		ORDER BY applied_at DESC
	`

	cutoff := time.Now().Add(-monitoringWindow)
	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get recently applied recommendations: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan recommendation id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// MarkRollbackTriggered marks an outcome as having triggered a rollback
func (s *OutcomeService) MarkRollbackTriggered(ctx context.Context, recommendationID string, rollbackRecommendationID string) error {
	query := `
		UPDATE recommendation_outcomes
		SET rollback_triggered = TRUE, rollback_recommendation_id = $2, outcome_status = $3
		WHERE recommendation_id = $1
	`

	_, err := s.db.ExecContext(ctx, query, recommendationID, rollbackRecommendationID, OutcomeStatusRolledBack)
	if err != nil {
		return fmt.Errorf("failed to mark rollback triggered: %w", err)
	}

	return nil
}
