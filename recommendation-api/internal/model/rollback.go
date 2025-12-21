// Package model provides model storage and management functionality
package model

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// RollbackManager handles model rollback operations
type RollbackManager struct {
	db          *sql.DB
	repository  *Repository
	distributor *Distributor

	// Configuration
	maxVersionsToKeep int
}

// RollbackConfig holds configuration for rollback management
type RollbackConfig struct {
	// Maximum number of model versions to keep (default: 5)
	MaxVersionsToKeep int
}

// DefaultRollbackConfig returns default rollback configuration
func DefaultRollbackConfig() *RollbackConfig {
	return &RollbackConfig{
		MaxVersionsToKeep: 5,
	}
}

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(db *sql.DB, repo *Repository, dist *Distributor, cfg *RollbackConfig) *RollbackManager {
	if cfg == nil {
		cfg = DefaultRollbackConfig()
	}

	return &RollbackManager{
		db:                db,
		repository:        repo,
		distributor:       dist,
		maxVersionsToKeep: cfg.MaxVersionsToKeep,
	}
}

// RollbackResult contains the result of a rollback operation
type RollbackResult struct {
	PreviousVersion string    `json:"previous_version"`
	RolledBackTo    string    `json:"rolled_back_to"`
	RollbackTime    time.Time `json:"rollback_time"`
	Reason          string    `json:"reason"`
	AgentsNotified  int       `json:"agents_notified"`
}

// Rollback rolls back to a specific model version
func (r *RollbackManager) Rollback(ctx context.Context, targetVersion, reason string) (*RollbackResult, error) {
	// Get current active model
	currentModel, err := r.repository.GetActiveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current active model: %w", err)
	}

	var previousVersion string
	if currentModel != nil {
		previousVersion = currentModel.Version
	}

	// Verify target version exists
	targetModel, err := r.repository.GetModel(ctx, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get target model: %w", err)
	}
	if targetModel == nil {
		return nil, fmt.Errorf("target model version not found: %s", targetVersion)
	}

	// Perform rollback in transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all models
	_, err = tx.ExecContext(ctx, "UPDATE model_versions SET is_active = FALSE")
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate models: %w", err)
	}

	// Activate target version and increment rollback count
	_, err = tx.ExecContext(ctx,
		"UPDATE model_versions SET is_active = TRUE, rollback_count = rollback_count + 1 WHERE version = $1",
		targetVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to activate target model: %w", err)
	}

	// Record rollback event
	_, err = tx.ExecContext(ctx, `
		INSERT INTO model_rollbacks (from_version, to_version, reason)
		VALUES ($1, $2, $3)
	`, previousVersion, targetVersion, reason)
	if err != nil {
		// Table might not exist, log and continue
		slog.Debug("Failed to record rollback event", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit rollback: %w", err)
	}

	// Notify agents of the rollback
	agentsNotified := 0
	if r.distributor != nil {
		if err := r.distributor.NotifyAgentsOfUpdate(ctx, targetVersion); err != nil {
			slog.Warn("Failed to notify agents of rollback", "error", err)
		} else {
			// Get count of notified agents
			summary, _ := r.distributor.GetDeploymentSummary(ctx, targetVersion)
			if summary != nil {
				agentsNotified = summary.Total
			}
		}
	}

	slog.Info("Model rollback completed",
		"from_version", previousVersion,
		"to_version", targetVersion,
		"reason", reason,
		"agents_notified", agentsNotified,
	)

	return &RollbackResult{
		PreviousVersion: previousVersion,
		RolledBackTo:    targetVersion,
		RollbackTime:    time.Now(),
		Reason:          reason,
		AgentsNotified:  agentsNotified,
	}, nil
}

// RollbackToPrevious rolls back to the previous model version
func (r *RollbackManager) RollbackToPrevious(ctx context.Context, reason string) (*RollbackResult, error) {
	// Get the previous version
	previousVersion, err := r.GetPreviousVersion(ctx)
	if err != nil {
		return nil, err
	}
	if previousVersion == "" {
		return nil, fmt.Errorf("no previous version available for rollback")
	}

	return r.Rollback(ctx, previousVersion, reason)
}

// GetPreviousVersion returns the version before the current active version
func (r *RollbackManager) GetPreviousVersion(ctx context.Context) (string, error) {
	query := `
		SELECT version
		FROM model_versions
		WHERE is_active = FALSE
		ORDER BY created_at DESC
		LIMIT 1
	`

	var version string
	err := r.db.QueryRowContext(ctx, query).Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get previous version: %w", err)
	}

	return version, nil
}

// GetRollbackHistory returns the history of rollback operations
func (r *RollbackManager) GetRollbackHistory(ctx context.Context, limit int) ([]RollbackEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, from_version, to_version, reason, rolled_back_at
		FROM model_rollbacks
		ORDER BY rolled_back_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		// Table might not exist
		return nil, nil
	}
	defer rows.Close()

	var events []RollbackEvent
	for rows.Next() {
		var e RollbackEvent
		err := rows.Scan(&e.ID, &e.FromVersion, &e.ToVersion, &e.Reason, &e.RolledBackAt)
		if err != nil {
			continue
		}
		events = append(events, e)
	}

	return events, nil
}

// RollbackEvent represents a rollback event
type RollbackEvent struct {
	ID           string    `json:"id"`
	FromVersion  string    `json:"from_version"`
	ToVersion    string    `json:"to_version"`
	Reason       string    `json:"reason"`
	RolledBackAt time.Time `json:"rolled_back_at"`
}

// AutoRollbackOnValidationFailure checks if auto-rollback should be triggered
// based on validation failures
func (r *RollbackManager) AutoRollbackOnValidationFailure(ctx context.Context, modelVersion string, threshold float32) (*RollbackResult, error) {
	// Get validation results for the model
	validations, err := r.repository.GetValidationHistory(ctx, modelVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get validation history: %w", err)
	}

	if len(validations) == 0 {
		return nil, nil // No validations yet
	}

	// Check recent validations
	failedCount := 0
	totalCount := 0
	for _, v := range validations {
		if time.Since(v.ValidatedAt) > 24*time.Hour {
			break // Only consider last 24 hours
		}
		totalCount++
		if !v.Passed {
			failedCount++
		}
	}

	if totalCount == 0 {
		return nil, nil
	}

	failureRate := float32(failedCount) / float32(totalCount)
	if failureRate < threshold {
		return nil, nil // Below threshold
	}

	slog.Warn("Auto-rollback triggered due to validation failures",
		"model_version", modelVersion,
		"failure_rate", failureRate,
		"threshold", threshold,
	)

	reason := fmt.Sprintf("Auto-rollback: validation failure rate %.2f%% exceeds threshold %.2f%%",
		failureRate*100, threshold*100)

	return r.RollbackToPrevious(ctx, reason)
}

// CleanupOldVersions removes old model versions, keeping only the most recent N
func (r *RollbackManager) CleanupOldVersions(ctx context.Context) (int, error) {
	// Get versions to keep (most recent N + active)
	query := `
		SELECT version FROM model_versions
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, r.maxVersionsToKeep)
	if err != nil {
		return 0, fmt.Errorf("failed to get versions to keep: %w", err)
	}
	defer rows.Close()

	versionsToKeep := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			continue
		}
		versionsToKeep[version] = true
	}

	// Get all versions
	allVersions, err := r.repository.ListModels(ctx, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to list all models: %w", err)
	}

	// Delete old versions
	deletedCount := 0
	for _, model := range allVersions {
		if versionsToKeep[model.Version] || model.IsActive {
			continue
		}

		if err := r.repository.DeleteModel(ctx, model.Version); err != nil {
			slog.Warn("Failed to delete old model version",
				"version", model.Version,
				"error", err,
			)
			continue
		}
		deletedCount++
	}

	if deletedCount > 0 {
		slog.Info("Cleaned up old model versions", "deleted_count", deletedCount)
	}

	return deletedCount, nil
}

// GetAvailableRollbackVersions returns versions available for rollback
func (r *RollbackManager) GetAvailableRollbackVersions(ctx context.Context) ([]ModelVersion, error) {
	query := `
		SELECT version, created_at, COALESCE(description, ''), weights_path,
		       COALESCE(storage_path, weights_path), COALESCE(storage_backend, 'local'),
		       COALESCE(checksum, ''), validation_accuracy, size_bytes, is_active,
		       rollback_count, COALESCE(training_samples, 0),
		       COALESCE(training_duration_seconds, 0)
		FROM model_versions
		WHERE is_active = FALSE
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, r.maxVersionsToKeep)
	if err != nil {
		return nil, fmt.Errorf("failed to get rollback versions: %w", err)
	}
	defer rows.Close()

	var versions []ModelVersion
	for rows.Next() {
		var m ModelVersion
		err := rows.Scan(
			&m.Version, &m.CreatedAt, &m.Description, &m.WeightsPath,
			&m.StoragePath, &m.StorageBackend, &m.Checksum,
			&m.ValidationAccuracy, &m.SizeBytes, &m.IsActive,
			&m.RollbackCount, &m.TrainingSamples, &m.TrainingDurationSeconds,
		)
		if err != nil {
			continue
		}
		versions = append(versions, m)
	}

	return versions, nil
}

// ValidateBeforeActivation validates a model before activating it
// Returns error if validation fails
func (r *RollbackManager) ValidateBeforeActivation(ctx context.Context, version string) error {
	model, err := r.repository.GetModel(ctx, version)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return fmt.Errorf("model not found: %s", version)
	}

	// Check validation accuracy threshold
	const minAccuracy = 0.7
	if model.ValidationAccuracy < minAccuracy {
		return fmt.Errorf("model validation accuracy %.2f is below minimum threshold %.2f",
			model.ValidationAccuracy, minAccuracy)
	}

	// Check model size
	const maxSizeBytes = 100 * 1024 // 100KB
	if model.SizeBytes > maxSizeBytes {
		return fmt.Errorf("model size %d bytes exceeds maximum %d bytes",
			model.SizeBytes, maxSizeBytes)
	}

	// Verify weights can be loaded
	_, _, err = r.repository.GetModelWeights(ctx, version)
	if err != nil {
		return fmt.Errorf("failed to load model weights: %w", err)
	}

	return nil
}

// ActivateWithValidation activates a model version after validation
func (r *RollbackManager) ActivateWithValidation(ctx context.Context, version string) error {
	// Validate first
	if err := r.ValidateBeforeActivation(ctx, version); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Activate
	if err := r.repository.ActivateModel(ctx, version); err != nil {
		return fmt.Errorf("failed to activate model: %w", err)
	}

	// Notify agents
	if r.distributor != nil {
		if err := r.distributor.NotifyAgentsOfUpdate(ctx, version); err != nil {
			slog.Warn("Failed to notify agents of activation", "error", err)
		}
	}

	return nil
}
