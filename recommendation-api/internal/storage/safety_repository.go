// Package storage provides database access for the recommendation API
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
)

// SafetyRepository implements the safety store interface
type SafetyRepository struct {
	db     *DB
	logger *slog.Logger
}

// NewSafetyRepository creates a new safety repository
func NewSafetyRepository(db *DB, logger *slog.Logger) *SafetyRepository {
	if logger == nil {
		logger = slog.Default()
	}
	return &SafetyRepository{
		db:     db,
		logger: logger,
	}
}

// GetNamespaceConfig returns the configuration for a namespace
func (r *SafetyRepository) GetNamespaceConfig(ctx context.Context, namespace string) (*rest.NamespaceConfig, error) {
	query := `
		SELECT namespace, dry_run_enabled, auto_approve_enabled,
		       high_risk_threshold_memory_reduction, high_risk_threshold_cpu_reduction,
		       created_at, updated_at
		FROM namespace_config
		WHERE namespace = $1
	`

	var config rest.NamespaceConfig
	err := r.db.QueryRowContext(ctx, query, namespace).Scan(
		&config.Namespace,
		&config.DryRunEnabled,
		&config.AutoApproveEnabled,
		&config.HighRiskThresholdMemoryReduction,
		&config.HighRiskThresholdCPUReduction,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Fall back to global config
		if namespace != "_global" {
			return r.GetNamespaceConfig(ctx, "_global")
		}
		// Return default config
		return &rest.NamespaceConfig{
			Namespace:                        namespace,
			DryRunEnabled:                    false,
			AutoApproveEnabled:               false,
			HighRiskThresholdMemoryReduction: 0.30,
			HighRiskThresholdCPUReduction:    0.50,
			CreatedAt:                        time.Now(),
			UpdatedAt:                        time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace config: %w", err)
	}

	return &config, nil
}

// SetNamespaceConfig creates or updates namespace configuration
func (r *SafetyRepository) SetNamespaceConfig(ctx context.Context, config *rest.NamespaceConfig) error {
	query := `
		INSERT INTO namespace_config (
			namespace, dry_run_enabled, auto_approve_enabled,
			high_risk_threshold_memory_reduction, high_risk_threshold_cpu_reduction,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (namespace) DO UPDATE SET
			dry_run_enabled = EXCLUDED.dry_run_enabled,
			auto_approve_enabled = EXCLUDED.auto_approve_enabled,
			high_risk_threshold_memory_reduction = EXCLUDED.high_risk_threshold_memory_reduction,
			high_risk_threshold_cpu_reduction = EXCLUDED.high_risk_threshold_cpu_reduction,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query,
		config.Namespace,
		config.DryRunEnabled,
		config.AutoApproveEnabled,
		config.HighRiskThresholdMemoryReduction,
		config.HighRiskThresholdCPUReduction,
	)
	if err != nil {
		return fmt.Errorf("failed to set namespace config: %w", err)
	}

	r.logger.Info("namespace config updated",
		"namespace", config.Namespace,
		"dry_run_enabled", config.DryRunEnabled,
	)

	return nil
}

// ListNamespaceConfigs returns all namespace configurations
func (r *SafetyRepository) ListNamespaceConfigs(ctx context.Context) ([]rest.NamespaceConfig, error) {
	query := `
		SELECT namespace, dry_run_enabled, auto_approve_enabled,
		       high_risk_threshold_memory_reduction, high_risk_threshold_cpu_reduction,
		       created_at, updated_at
		FROM namespace_config
		ORDER BY namespace
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespace configs: %w", err)
	}
	defer rows.Close()

	var configs []rest.NamespaceConfig
	for rows.Next() {
		var config rest.NamespaceConfig
		if err := rows.Scan(
			&config.Namespace,
			&config.DryRunEnabled,
			&config.AutoApproveEnabled,
			&config.HighRiskThresholdMemoryReduction,
			&config.HighRiskThresholdCPUReduction,
			&config.CreatedAt,
			&config.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan namespace config: %w", err)
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// EvaluateDryRun evaluates what would happen if a recommendation were applied
func (r *SafetyRepository) EvaluateDryRun(ctx context.Context, recommendationID string) (*rest.DryRunResult, error) {
	// Get the recommendation
	query := `
		SELECT id, namespace, deployment,
		       cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes,
		       current_cpu_request_millicores, current_cpu_limit_millicores,
		       current_memory_request_bytes, current_memory_limit_bytes
		FROM recommendations
		WHERE id = $1
	`

	var rec struct {
		ID                   string
		Namespace            string
		Deployment           string
		CpuRequestMillicores int32
		CpuLimitMillicores   int32
		MemoryRequestBytes   int64
		MemoryLimitBytes     int64
		CurrentCpuRequest    sql.NullInt32
		CurrentCpuLimit      sql.NullInt32
		CurrentMemoryRequest sql.NullInt64
		CurrentMemoryLimit   sql.NullInt64
	}

	err := r.db.QueryRowContext(ctx, query, recommendationID).Scan(
		&rec.ID, &rec.Namespace, &rec.Deployment,
		&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
		&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
		&rec.CurrentCpuRequest, &rec.CurrentCpuLimit,
		&rec.CurrentMemoryRequest, &rec.CurrentMemoryLimit,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("recommendation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation: %w", err)
	}

	result := &rest.DryRunResult{
		RecommendationID: recommendationID,
		Namespace:        rec.Namespace,
		Deployment:       rec.Deployment,
		WouldApply:       true,
		EvaluatedAt:      time.Now(),
	}

	// Calculate changes
	result.Changes = r.calculateChanges(&rec)

	// Check for warnings
	result.Warnings = r.checkWarnings(result.Changes)

	// Generate YAML patch
	result.YamlPatch = r.generateYamlPatch(&rec)

	// Log the dry-run evaluation
	r.logger.Info("dry-run evaluation completed",
		"recommendation_id", recommendationID,
		"namespace", rec.Namespace,
		"deployment", rec.Deployment,
		"changes_count", len(result.Changes),
		"warnings_count", len(result.Warnings),
	)

	// Store the dry-run result
	if err := r.storeDryRunResult(ctx, recommendationID, result); err != nil {
		r.logger.Warn("failed to store dry-run result", "error", err)
	}

	return result, nil
}

func (r *SafetyRepository) calculateChanges(rec *struct {
	ID                   string
	Namespace            string
	Deployment           string
	CpuRequestMillicores int32
	CpuLimitMillicores   int32
	MemoryRequestBytes   int64
	MemoryLimitBytes     int64
	CurrentCpuRequest    sql.NullInt32
	CurrentCpuLimit      sql.NullInt32
	CurrentMemoryRequest sql.NullInt64
	CurrentMemoryLimit   sql.NullInt64
}) []rest.ResourceChange {
	var changes []rest.ResourceChange

	// CPU Request
	if rec.CurrentCpuRequest.Valid && rec.CurrentCpuRequest.Int32 > 0 {
		current := rec.CurrentCpuRequest.Int32
		new := rec.CpuRequestMillicores
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, rest.ResourceChange{
				Resource:      "cpu_request",
				CurrentValue:  fmt.Sprintf("%dm", current),
				NewValue:      fmt.Sprintf("%dm", new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// CPU Limit
	if rec.CurrentCpuLimit.Valid && rec.CurrentCpuLimit.Int32 > 0 {
		current := rec.CurrentCpuLimit.Int32
		new := rec.CpuLimitMillicores
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, rest.ResourceChange{
				Resource:      "cpu_limit",
				CurrentValue:  fmt.Sprintf("%dm", current),
				NewValue:      fmt.Sprintf("%dm", new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// Memory Request
	if rec.CurrentMemoryRequest.Valid && rec.CurrentMemoryRequest.Int64 > 0 {
		current := rec.CurrentMemoryRequest.Int64
		new := rec.MemoryRequestBytes
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, rest.ResourceChange{
				Resource:      "memory_request",
				CurrentValue:  formatBytes(current),
				NewValue:      formatBytes(new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// Memory Limit
	if rec.CurrentMemoryLimit.Valid && rec.CurrentMemoryLimit.Int64 > 0 {
		current := rec.CurrentMemoryLimit.Int64
		new := rec.MemoryLimitBytes
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, rest.ResourceChange{
				Resource:      "memory_limit",
				CurrentValue:  formatBytes(current),
				NewValue:      formatBytes(new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	return changes
}

func (r *SafetyRepository) checkWarnings(changes []rest.ResourceChange) []string {
	var warnings []string

	for _, change := range changes {
		// Warn on significant memory reductions (>30%)
		if change.Resource == "memory_limit" && change.IsReduction && change.ChangePercent < -30 {
			warnings = append(warnings, fmt.Sprintf(
				"Memory limit reduction of %.1f%% may increase OOM risk",
				-change.ChangePercent,
			))
		}

		// Warn on significant CPU reductions (>50%)
		if change.Resource == "cpu_limit" && change.IsReduction && change.ChangePercent < -50 {
			warnings = append(warnings, fmt.Sprintf(
				"CPU limit reduction of %.1f%% may increase throttling",
				-change.ChangePercent,
			))
		}
	}

	return warnings
}

func (r *SafetyRepository) generateYamlPatch(rec *struct {
	ID                   string
	Namespace            string
	Deployment           string
	CpuRequestMillicores int32
	CpuLimitMillicores   int32
	MemoryRequestBytes   int64
	MemoryLimitBytes     int64
	CurrentCpuRequest    sql.NullInt32
	CurrentCpuLimit      sql.NullInt32
	CurrentMemoryRequest sql.NullInt64
	CurrentMemoryLimit   sql.NullInt64
}) string {
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
		rec.CpuRequestMillicores, formatBytes(rec.MemoryRequestBytes),
		rec.CpuLimitMillicores, formatBytes(rec.MemoryLimitBytes),
	)
}

func (r *SafetyRepository) storeDryRunResult(ctx context.Context, recommendationID string, result *rest.DryRunResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal dry-run result: %w", err)
	}

	query := `
		UPDATE recommendations
		SET dry_run_result = $1
		WHERE id = $2
	`

	_, err = r.db.ExecContext(ctx, query, resultJSON, recommendationID)
	return err
}

// Ensure SafetyRepository implements rest.SafetyStore interface
var _ rest.SafetyStore = (*SafetyRepository)(nil)


// ApproveRecommendationWithDetails approves a recommendation with approver details
func (r *SafetyRepository) ApproveRecommendationWithDetails(ctx context.Context, id string, approver string, reason string) (*rest.ApproveRecommendationResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update recommendation status
	query := `
		UPDATE recommendations
		SET status = 'approved', approved_at = NOW(), approved_by = $2
		WHERE id = $1 AND status = 'pending'
		RETURNING id
	`

	var returnedID string
	err = tx.QueryRowContext(ctx, query, id, approver).Scan(&returnedID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("recommendation not found or already processed")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to approve recommendation: %w", err)
	}

	// Record approval history
	historyQuery := `
		INSERT INTO approval_history (recommendation_id, action, approver, reason)
		VALUES ($1, 'approved', $2, $3)
	`
	_, err = tx.ExecContext(ctx, historyQuery, id, approver, reason)
	if err != nil {
		return nil, fmt.Errorf("failed to record approval history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("recommendation approved",
		"recommendation_id", id,
		"approver", approver,
	)

	return &rest.ApproveRecommendationResponse{
		ID:       id,
		Status:   "approved",
		Message:  "Recommendation approved",
		Approver: approver,
	}, nil
}

// RejectRecommendation rejects a recommendation
func (r *SafetyRepository) RejectRecommendation(ctx context.Context, id string, approver string, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update recommendation status
	query := `
		UPDATE recommendations
		SET status = 'rejected'
		WHERE id = $1 AND status = 'pending'
	`

	result, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to reject recommendation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("recommendation not found or already processed")
	}

	// Record rejection history
	historyQuery := `
		INSERT INTO approval_history (recommendation_id, action, approver, reason)
		VALUES ($1, 'rejected', $2, $3)
	`
	_, err = tx.ExecContext(ctx, historyQuery, id, approver, reason)
	if err != nil {
		return fmt.Errorf("failed to record rejection history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("recommendation rejected",
		"recommendation_id", id,
		"approver", approver,
		"reason", reason,
	)

	return nil
}

// GetApprovalHistory returns approval history for a recommendation
func (r *SafetyRepository) GetApprovalHistory(ctx context.Context, recommendationID string) ([]rest.ApprovalHistory, error) {
	query := `
		SELECT id, recommendation_id, action, approver, reason, created_at
		FROM approval_history
		WHERE recommendation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, recommendationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approval history: %w", err)
	}
	defer rows.Close()

	var history []rest.ApprovalHistory
	for rows.Next() {
		var h rest.ApprovalHistory
		var reason sql.NullString
		if err := rows.Scan(&h.ID, &h.RecommendationID, &h.Action, &h.Approver, &reason, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan approval history: %w", err)
		}
		if reason.Valid {
			h.Reason = reason.String
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

// GetPendingApprovals returns recommendations pending approval
func (r *SafetyRepository) GetPendingApprovals(ctx context.Context, namespace string) ([]rest.Recommendation, error) {
	query := `
		SELECT id, namespace, deployment, cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes, confidence, model_version,
		       status, created_at, time_window, is_high_risk, risk_reason
		FROM recommendations
		WHERE status = 'pending' AND requires_approval = TRUE
		  AND ($1 = '' OR namespace = $1)
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending approvals: %w", err)
	}
	defer rows.Close()

	var recommendations []rest.Recommendation
	for rows.Next() {
		var rec rest.Recommendation
		var isHighRisk sql.NullBool
		var riskReason sql.NullString

		err := rows.Scan(
			&rec.ID, &rec.Namespace, &rec.Deployment,
			&rec.CpuRequestMillicores, &rec.CpuLimitMillicores,
			&rec.MemoryRequestBytes, &rec.MemoryLimitBytes,
			&rec.Confidence, &rec.ModelVersion, &rec.Status,
			&rec.CreatedAt, &rec.TimeWindow, &isHighRisk, &riskReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recommendation: %w", err)
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations, rows.Err()
}

// GetRecommendationOutcome returns the outcome tracking for a recommendation
func (r *SafetyRepository) GetRecommendationOutcome(ctx context.Context, recommendationID string) (*rest.RecommendationOutcome, error) {
	query := `
		SELECT id, recommendation_id, namespace, deployment, applied_at, check_time,
		       oom_kills_before, oom_kills_after, cpu_throttle_before, cpu_throttle_after,
		       outcome_status, rollback_triggered, rollback_recommendation_id
		FROM recommendation_outcomes
		WHERE recommendation_id = $1
		ORDER BY check_time DESC
		LIMIT 1
	`

	var outcome rest.RecommendationOutcome
	var rollbackRecID sql.NullString

	err := r.db.QueryRowContext(ctx, query, recommendationID).Scan(
		&outcome.ID, &outcome.RecommendationID, &outcome.Namespace, &outcome.Deployment,
		&outcome.AppliedAt, &outcome.CheckTime,
		&outcome.OOMKillsBefore, &outcome.OOMKillsAfter,
		&outcome.CPUThrottleBefore, &outcome.CPUThrottleAfter,
		&outcome.OutcomeStatus, &outcome.RollbackTriggered, &rollbackRecID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation outcome: %w", err)
	}

	if rollbackRecID.Valid {
		outcome.RollbackRecommendationID = rollbackRecID.String
	}

	return &outcome, nil
}

// RecordOutcome records an outcome for a recommendation
func (r *SafetyRepository) RecordOutcome(ctx context.Context, outcome *rest.RecommendationOutcome) error {
	query := `
		INSERT INTO recommendation_outcomes (
			recommendation_id, namespace, deployment, applied_at,
			oom_kills_before, oom_kills_after, cpu_throttle_before, cpu_throttle_after,
			outcome_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		outcome.RecommendationID, outcome.Namespace, outcome.Deployment, outcome.AppliedAt,
		outcome.OOMKillsBefore, outcome.OOMKillsAfter,
		outcome.CPUThrottleBefore, outcome.CPUThrottleAfter,
		outcome.OutcomeStatus,
	)
	if err != nil {
		return fmt.Errorf("failed to record outcome: %w", err)
	}

	r.logger.Info("outcome recorded",
		"recommendation_id", outcome.RecommendationID,
		"status", outcome.OutcomeStatus,
	)

	return nil
}

// UpdateOutcomeStatus updates the status of an outcome
func (r *SafetyRepository) UpdateOutcomeStatus(ctx context.Context, recommendationID string, status string) error {
	query := `
		UPDATE recommendation_outcomes
		SET outcome_status = $2, check_time = NOW()
		WHERE recommendation_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, recommendationID, status)
	if err != nil {
		return fmt.Errorf("failed to update outcome status: %w", err)
	}

	return nil
}

// ListRollbackEvents returns rollback events
func (r *SafetyRepository) ListRollbackEvents(ctx context.Context, namespace string) ([]rest.RollbackEvent, error) {
	query := `
		SELECT re.id, re.original_recommendation_id, re.rollback_recommendation_id,
		       re.trigger_reason, re.oom_kills_detected, re.throttle_increase_percent,
		       re.auto_triggered, re.created_at, re.alert_sent
		FROM rollback_events re
		JOIN recommendations r ON re.original_recommendation_id = r.id
		WHERE ($1 = '' OR r.namespace = $1)
		ORDER BY re.created_at DESC
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list rollback events: %w", err)
	}
	defer rows.Close()

	var events []rest.RollbackEvent
	for rows.Next() {
		var event rest.RollbackEvent
		var rollbackRecID sql.NullString
		var oomKills sql.NullInt32
		var throttleIncrease sql.NullFloat64

		err := rows.Scan(
			&event.ID, &event.OriginalRecommendationID, &rollbackRecID,
			&event.TriggerReason, &oomKills, &throttleIncrease,
			&event.AutoTriggered, &event.CreatedAt, &event.AlertSent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rollback event: %w", err)
		}

		if rollbackRecID.Valid {
			event.RollbackRecommendationID = rollbackRecID.String
		}
		if oomKills.Valid {
			event.OOMKillsDetected = int(oomKills.Int32)
		}
		if throttleIncrease.Valid {
			event.ThrottleIncreasePercent = throttleIncrease.Float64
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// TriggerRollback triggers a rollback for a recommendation
func (r *SafetyRepository) TriggerRollback(ctx context.Context, recommendationID string, reason string, autoTriggered bool) (*rest.RollbackEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the original recommendation
	var namespace, deployment string
	var currentCpuReq, currentCpuLim sql.NullInt32
	var currentMemReq, currentMemLim sql.NullInt64

	query := `
		SELECT namespace, deployment,
		       current_cpu_request_millicores, current_cpu_limit_millicores,
		       current_memory_request_bytes, current_memory_limit_bytes
		FROM recommendations
		WHERE id = $1
	`
	err = tx.QueryRowContext(ctx, query, recommendationID).Scan(
		&namespace, &deployment,
		&currentCpuReq, &currentCpuLim, &currentMemReq, &currentMemLim,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get original recommendation: %w", err)
	}

	// Create rollback recommendation (restore original values)
	var rollbackRecID string
	if currentCpuReq.Valid && currentMemReq.Valid {
		insertQuery := `
			INSERT INTO recommendations (
				namespace, deployment,
				cpu_request_millicores, cpu_limit_millicores,
				memory_request_bytes, memory_limit_bytes,
				confidence, model_version, status, time_window
			) VALUES ($1, $2, $3, $4, $5, $6, 1.0, 'rollback', 'pending', 'rollback')
			RETURNING id
		`
		err = tx.QueryRowContext(ctx, insertQuery,
			namespace, deployment,
			currentCpuReq.Int32, currentCpuLim.Int32,
			currentMemReq.Int64, currentMemLim.Int64,
		).Scan(&rollbackRecID)
		if err != nil {
			return nil, fmt.Errorf("failed to create rollback recommendation: %w", err)
		}
	}

	// Update original recommendation status
	_, err = tx.ExecContext(ctx,
		"UPDATE recommendations SET status = 'rolled_back' WHERE id = $1",
		recommendationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update recommendation status: %w", err)
	}

	// Update outcome
	_, err = tx.ExecContext(ctx,
		`UPDATE recommendation_outcomes 
		 SET rollback_triggered = TRUE, rollback_recommendation_id = $2, outcome_status = 'rolled_back'
		 WHERE recommendation_id = $1`,
		recommendationID, rollbackRecID,
	)
	if err != nil {
		r.logger.Warn("failed to update outcome for rollback", "error", err)
	}

	// Create rollback event
	var eventID string
	eventQuery := `
		INSERT INTO rollback_events (
			original_recommendation_id, rollback_recommendation_id,
			trigger_reason, auto_triggered
		) VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, eventQuery,
		recommendationID, rollbackRecID, reason, autoTriggered,
	).Scan(&eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to create rollback event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("rollback triggered",
		"original_recommendation_id", recommendationID,
		"rollback_recommendation_id", rollbackRecID,
		"reason", reason,
		"auto_triggered", autoTriggered,
	)

	return &rest.RollbackEvent{
		ID:                       eventID,
		OriginalRecommendationID: recommendationID,
		RollbackRecommendationID: rollbackRecID,
		TriggerReason:            reason,
		AutoTriggered:            autoTriggered,
		CreatedAt:                time.Now(),
	}, nil
}
