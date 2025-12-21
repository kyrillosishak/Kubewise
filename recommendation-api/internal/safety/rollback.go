// Package safety provides safety and rollout features for recommendations
package safety

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// RollbackTriggerReason represents the reason for a rollback
type RollbackTriggerReason string

const (
	RollbackReasonOOMIncrease      RollbackTriggerReason = "oom_increase"
	RollbackReasonThrottleIncrease RollbackTriggerReason = "throttle_increase"
	RollbackReasonManual           RollbackTriggerReason = "manual"
)

// RollbackEvent represents a rollback event
type RollbackEvent struct {
	ID                       string                `json:"id"`
	OriginalRecommendationID string                `json:"original_recommendation_id"`
	RollbackRecommendationID string                `json:"rollback_recommendation_id,omitempty"`
	TriggerReason            RollbackTriggerReason `json:"trigger_reason"`
	OOMKillsDetected         int                   `json:"oom_kills_detected,omitempty"`
	ThrottleIncreasePercent  float64               `json:"throttle_increase_percent,omitempty"`
	AutoTriggered            bool                  `json:"auto_triggered"`
	CreatedAt                time.Time             `json:"created_at"`
	AlertSent                bool                  `json:"alert_sent"`
	AlertSentAt              *time.Time            `json:"alert_sent_at,omitempty"`
}

// RollbackConfig holds configuration for automatic rollback
type RollbackConfig struct {
	MonitoringWindow          time.Duration `json:"monitoring_window"`           // How long to monitor after application
	OOMKillThreshold          int           `json:"oom_kill_threshold"`          // Number of OOM kills to trigger rollback
	ThrottleIncreaseThreshold float64       `json:"throttle_increase_threshold"` // Percentage increase to trigger rollback
	AutoRollbackEnabled       bool          `json:"auto_rollback_enabled"`
}

// DefaultRollbackConfig returns the default rollback configuration
func DefaultRollbackConfig() *RollbackConfig {
	return &RollbackConfig{
		MonitoringWindow:          time.Hour,
		OOMKillThreshold:          1,    // Any OOM kill triggers rollback
		ThrottleIncreaseThreshold: 0.25, // 25% increase in throttling
		AutoRollbackEnabled:       true,
	}
}

// RollbackService handles automatic rollback operations
type RollbackService struct {
	db             *sql.DB
	outcomeService *OutcomeService
	config         *RollbackConfig
	logger         *slog.Logger
	alertCallback  func(event *RollbackEvent) error
}

// NewRollbackService creates a new rollback service
func NewRollbackService(db *sql.DB, outcomeService *OutcomeService, config *RollbackConfig, logger *slog.Logger) *RollbackService {
	if config == nil {
		config = DefaultRollbackConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RollbackService{
		db:             db,
		outcomeService: outcomeService,
		config:         config,
		logger:         logger,
	}
}

// SetAlertCallback sets the callback function for sending alerts
func (s *RollbackService) SetAlertCallback(callback func(event *RollbackEvent) error) {
	s.alertCallback = callback
}

// CheckAndTriggerRollbacks checks all recently applied recommendations and triggers rollbacks if needed
func (s *RollbackService) CheckAndTriggerRollbacks(ctx context.Context) ([]RollbackEvent, error) {
	if !s.config.AutoRollbackEnabled {
		return nil, nil
	}

	// Get degraded outcomes
	degradedOutcomes, err := s.outcomeService.ListDegradedOutcomes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list degraded outcomes: %w", err)
	}

	var rollbackEvents []RollbackEvent

	for _, outcome := range degradedOutcomes {
		// Check if within monitoring window
		if time.Since(outcome.AppliedAt) > s.config.MonitoringWindow {
			continue
		}

		// Determine if rollback is needed
		shouldRollback, reason := s.shouldTriggerRollback(&outcome)
		if !shouldRollback {
			continue
		}

		// Trigger rollback
		event, err := s.TriggerRollback(ctx, outcome.RecommendationID, reason, true)
		if err != nil {
			s.logger.Error("failed to trigger rollback",
				"recommendation_id", outcome.RecommendationID,
				"error", err,
			)
			continue
		}

		rollbackEvents = append(rollbackEvents, *event)
	}

	return rollbackEvents, nil
}

func (s *RollbackService) shouldTriggerRollback(outcome *RecommendationOutcome) (bool, RollbackTriggerReason) {
	// Check OOM kills
	oomDelta := outcome.OOMKillsAfter - outcome.OOMKillsBefore
	if oomDelta >= s.config.OOMKillThreshold {
		return true, RollbackReasonOOMIncrease
	}

	// Check throttle increase
	if outcome.CPUThrottleBefore > 0 {
		throttleIncrease := (outcome.CPUThrottleAfter - outcome.CPUThrottleBefore) / outcome.CPUThrottleBefore
		if throttleIncrease >= s.config.ThrottleIncreaseThreshold {
			return true, RollbackReasonThrottleIncrease
		}
	} else if outcome.CPUThrottleAfter >= s.config.ThrottleIncreaseThreshold {
		// New throttling appeared
		return true, RollbackReasonThrottleIncrease
	}

	return false, ""
}

// TriggerRollback triggers a rollback for a recommendation
func (s *RollbackService) TriggerRollback(ctx context.Context, recommendationID string, reason RollbackTriggerReason, autoTriggered bool) (*RollbackEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
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

	// Get outcome metrics for the event
	var oomKillsDetected int
	var throttleIncrease float64
	outcomeQuery := `
		SELECT oom_kills_after - oom_kills_before, cpu_throttle_after - cpu_throttle_before
		FROM recommendation_outcomes
		WHERE recommendation_id = $1
	`
	err = tx.QueryRowContext(ctx, outcomeQuery, recommendationID).Scan(&oomKillsDetected, &throttleIncrease)
	if err != nil && err != sql.ErrNoRows {
		s.logger.Warn("failed to get outcome metrics for rollback event", "error", err)
	}

	// Update outcome
	_, err = tx.ExecContext(ctx,
		`UPDATE recommendation_outcomes 
		 SET rollback_triggered = TRUE, rollback_recommendation_id = $2, outcome_status = 'rolled_back'
		 WHERE recommendation_id = $1`,
		recommendationID, rollbackRecID,
	)
	if err != nil {
		s.logger.Warn("failed to update outcome for rollback", "error", err)
	}

	// Create rollback event
	var eventID string
	eventQuery := `
		INSERT INTO rollback_events (
			original_recommendation_id, rollback_recommendation_id,
			trigger_reason, oom_kills_detected, throttle_increase_percent, auto_triggered
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, eventQuery,
		recommendationID, rollbackRecID, reason, oomKillsDetected, throttleIncrease*100, autoTriggered,
	).Scan(&eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to create rollback event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	event := &RollbackEvent{
		ID:                       eventID,
		OriginalRecommendationID: recommendationID,
		RollbackRecommendationID: rollbackRecID,
		TriggerReason:            reason,
		OOMKillsDetected:         oomKillsDetected,
		ThrottleIncreasePercent:  throttleIncrease * 100,
		AutoTriggered:            autoTriggered,
		CreatedAt:                time.Now(),
	}

	s.logger.Info("rollback triggered",
		"original_recommendation_id", recommendationID,
		"rollback_recommendation_id", rollbackRecID,
		"reason", reason,
		"auto_triggered", autoTriggered,
		"oom_kills_detected", oomKillsDetected,
	)

	// Send alert
	if err := s.sendAlert(ctx, event); err != nil {
		s.logger.Warn("failed to send rollback alert", "error", err)
	}

	return event, nil
}

func (s *RollbackService) sendAlert(ctx context.Context, event *RollbackEvent) error {
	if s.alertCallback == nil {
		return nil
	}

	if err := s.alertCallback(event); err != nil {
		return err
	}

	// Mark alert as sent
	now := time.Now()
	event.AlertSent = true
	event.AlertSentAt = &now

	query := `
		UPDATE rollback_events
		SET alert_sent = TRUE, alert_sent_at = $2
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, event.ID, now)
	return err
}

// ListRollbackEvents returns rollback events, optionally filtered by namespace
func (s *RollbackService) ListRollbackEvents(ctx context.Context, namespace string) ([]RollbackEvent, error) {
	query := `
		SELECT re.id, re.original_recommendation_id, re.rollback_recommendation_id,
		       re.trigger_reason, re.oom_kills_detected, re.throttle_increase_percent,
		       re.auto_triggered, re.created_at, re.alert_sent, re.alert_sent_at
		FROM rollback_events re
		JOIN recommendations r ON re.original_recommendation_id = r.id
		WHERE ($1 = '' OR r.namespace = $1)
		ORDER BY re.created_at DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list rollback events: %w", err)
	}
	defer rows.Close()

	var events []RollbackEvent
	for rows.Next() {
		var event RollbackEvent
		var rollbackRecID sql.NullString
		var oomKills sql.NullInt32
		var throttleIncrease sql.NullFloat64
		var alertSentAt sql.NullTime

		err := rows.Scan(
			&event.ID, &event.OriginalRecommendationID, &rollbackRecID,
			&event.TriggerReason, &oomKills, &throttleIncrease,
			&event.AutoTriggered, &event.CreatedAt, &event.AlertSent, &alertSentAt,
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
		if alertSentAt.Valid {
			event.AlertSentAt = &alertSentAt.Time
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetRollbackEvent returns a specific rollback event
func (s *RollbackService) GetRollbackEvent(ctx context.Context, eventID string) (*RollbackEvent, error) {
	query := `
		SELECT id, original_recommendation_id, rollback_recommendation_id,
		       trigger_reason, oom_kills_detected, throttle_increase_percent,
		       auto_triggered, created_at, alert_sent, alert_sent_at
		FROM rollback_events
		WHERE id = $1
	`

	var event RollbackEvent
	var rollbackRecID sql.NullString
	var oomKills sql.NullInt32
	var throttleIncrease sql.NullFloat64
	var alertSentAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, eventID).Scan(
		&event.ID, &event.OriginalRecommendationID, &rollbackRecID,
		&event.TriggerReason, &oomKills, &throttleIncrease,
		&event.AutoTriggered, &event.CreatedAt, &event.AlertSent, &alertSentAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get rollback event: %w", err)
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
	if alertSentAt.Valid {
		event.AlertSentAt = &alertSentAt.Time
	}

	return &event, nil
}
