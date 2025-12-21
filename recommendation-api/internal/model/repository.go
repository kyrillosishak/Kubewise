// Package model provides model storage and management functionality
package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Repository handles model version database operations
type Repository struct {
	db      *sql.DB
	storage *Storage
}

// NewRepository creates a new model repository
func NewRepository(db *sql.DB, storage *Storage) *Repository {
	return &Repository{
		db:      db,
		storage: storage,
	}
}

// ModelVersion represents a model version in the database
type ModelVersion struct {
	Version                 string                 `json:"version"`
	CreatedAt               time.Time              `json:"created_at"`
	Description             string                 `json:"description,omitempty"`
	WeightsPath             string                 `json:"weights_path"`
	StoragePath             string                 `json:"storage_path"`
	StorageBackend          string                 `json:"storage_backend"`
	Checksum                string                 `json:"checksum"`
	ValidationAccuracy      float32                `json:"validation_accuracy"`
	SizeBytes               int64                  `json:"size_bytes"`
	IsActive                bool                   `json:"is_active"`
	RollbackCount           int                    `json:"rollback_count"`
	TrainingSamples         int64                  `json:"training_samples,omitempty"`
	TrainingDurationSeconds int                    `json:"training_duration_seconds,omitempty"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}

// CreateModelInput contains input for creating a new model version
type CreateModelInput struct {
	Version                 string
	Description             string
	Weights                 []byte
	ValidationAccuracy      float32
	TrainingSamples         int64
	TrainingDurationSeconds int
	Metadata                map[string]interface{}
}

// CreateModel creates a new model version
func (r *Repository) CreateModel(ctx context.Context, input *CreateModelInput) (*ModelVersion, error) {
	// Store weights in storage backend
	storagePath, checksum, err := r.storage.StoreModel(ctx, input.Version, input.Weights)
	if err != nil {
		return nil, fmt.Errorf("failed to store model weights: %w", err)
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// Insert into database
	query := `
		INSERT INTO model_versions (
			version, description, weights_path, storage_path, storage_backend,
			checksum, validation_accuracy, size_bytes, is_active, rollback_count,
			training_samples, training_duration_seconds, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, 0, $9, $10, $11)
		RETURNING created_at
	`

	var createdAt time.Time
	err = r.db.QueryRowContext(ctx, query,
		input.Version,
		input.Description,
		storagePath, // weights_path for backward compatibility
		storagePath,
		string(r.storage.config.Backend),
		checksum,
		input.ValidationAccuracy,
		len(input.Weights),
		input.TrainingSamples,
		input.TrainingDurationSeconds,
		metadataJSON,
	).Scan(&createdAt)
	if err != nil {
		// Clean up stored weights on failure
		_ = r.storage.DeleteModel(ctx, storagePath)
		return nil, fmt.Errorf("failed to insert model version: %w", err)
	}

	slog.Info("Created model version",
		"version", input.Version,
		"storage_path", storagePath,
		"size_bytes", len(input.Weights),
		"validation_accuracy", input.ValidationAccuracy,
	)

	return &ModelVersion{
		Version:                 input.Version,
		CreatedAt:               createdAt,
		Description:             input.Description,
		WeightsPath:             storagePath,
		StoragePath:             storagePath,
		StorageBackend:          string(r.storage.config.Backend),
		Checksum:                checksum,
		ValidationAccuracy:      input.ValidationAccuracy,
		SizeBytes:               int64(len(input.Weights)),
		IsActive:                false,
		RollbackCount:           0,
		TrainingSamples:         input.TrainingSamples,
		TrainingDurationSeconds: input.TrainingDurationSeconds,
		Metadata:                input.Metadata,
	}, nil
}

// GetModel retrieves a model version by version string
func (r *Repository) GetModel(ctx context.Context, version string) (*ModelVersion, error) {
	query := `
		SELECT version, created_at, COALESCE(description, ''), weights_path,
		       COALESCE(storage_path, weights_path), COALESCE(storage_backend, 'local'),
		       COALESCE(checksum, ''), validation_accuracy, size_bytes, is_active,
		       rollback_count, COALESCE(training_samples, 0),
		       COALESCE(training_duration_seconds, 0), COALESCE(metadata, '{}')
		FROM model_versions
		WHERE version = $1
	`

	var m ModelVersion
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, version).Scan(
		&m.Version, &m.CreatedAt, &m.Description, &m.WeightsPath,
		&m.StoragePath, &m.StorageBackend, &m.Checksum,
		&m.ValidationAccuracy, &m.SizeBytes, &m.IsActive,
		&m.RollbackCount, &m.TrainingSamples,
		&m.TrainingDurationSeconds, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	// Parse metadata
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &m.Metadata)
	}

	return &m, nil
}

// GetActiveModel retrieves the currently active model version
func (r *Repository) GetActiveModel(ctx context.Context) (*ModelVersion, error) {
	query := `
		SELECT version, created_at, COALESCE(description, ''), weights_path,
		       COALESCE(storage_path, weights_path), COALESCE(storage_backend, 'local'),
		       COALESCE(checksum, ''), validation_accuracy, size_bytes, is_active,
		       rollback_count, COALESCE(training_samples, 0),
		       COALESCE(training_duration_seconds, 0), COALESCE(metadata, '{}')
		FROM model_versions
		WHERE is_active = TRUE
		LIMIT 1
	`

	var m ModelVersion
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query).Scan(
		&m.Version, &m.CreatedAt, &m.Description, &m.WeightsPath,
		&m.StoragePath, &m.StorageBackend, &m.Checksum,
		&m.ValidationAccuracy, &m.SizeBytes, &m.IsActive,
		&m.RollbackCount, &m.TrainingSamples,
		&m.TrainingDurationSeconds, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active model: %w", err)
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &m.Metadata)
	}

	return &m, nil
}

// ListModels retrieves all model versions ordered by creation date
func (r *Repository) ListModels(ctx context.Context, limit int) ([]ModelVersion, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT version, created_at, COALESCE(description, ''), weights_path,
		       COALESCE(storage_path, weights_path), COALESCE(storage_backend, 'local'),
		       COALESCE(checksum, ''), validation_accuracy, size_bytes, is_active,
		       rollback_count, COALESCE(training_samples, 0),
		       COALESCE(training_duration_seconds, 0), COALESCE(metadata, '{}')
		FROM model_versions
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var models []ModelVersion
	for rows.Next() {
		var m ModelVersion
		var metadataJSON []byte

		err := rows.Scan(
			&m.Version, &m.CreatedAt, &m.Description, &m.WeightsPath,
			&m.StoragePath, &m.StorageBackend, &m.Checksum,
			&m.ValidationAccuracy, &m.SizeBytes, &m.IsActive,
			&m.RollbackCount, &m.TrainingSamples,
			&m.TrainingDurationSeconds, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}

		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &m.Metadata)
		}

		models = append(models, m)
	}

	return models, rows.Err()
}

// GetModelWeights retrieves model weights from storage
func (r *Repository) GetModelWeights(ctx context.Context, version string) ([]byte, string, error) {
	model, err := r.GetModel(ctx, version)
	if err != nil {
		return nil, "", err
	}
	if model == nil {
		return nil, "", fmt.Errorf("model version not found: %s", version)
	}

	weights, err := r.storage.GetModel(ctx, model.StoragePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get model weights: %w", err)
	}

	// Verify checksum
	if model.Checksum != "" && !r.storage.VerifyChecksum(weights, model.Checksum) {
		return nil, "", fmt.Errorf("model checksum mismatch")
	}

	return weights, model.Checksum, nil
}

// ActivateModel activates a model version (deactivates all others)
func (r *Repository) ActivateModel(ctx context.Context, version string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all models
	_, err = tx.ExecContext(ctx, "UPDATE model_versions SET is_active = FALSE")
	if err != nil {
		return fmt.Errorf("failed to deactivate models: %w", err)
	}

	// Activate target version
	result, err := tx.ExecContext(ctx,
		"UPDATE model_versions SET is_active = TRUE WHERE version = $1",
		version,
	)
	if err != nil {
		return fmt.Errorf("failed to activate model: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("model version not found: %s", version)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Activated model version", "version", version)
	return nil
}

// UpdateValidationScore updates the validation accuracy for a model
func (r *Repository) UpdateValidationScore(ctx context.Context, version string, accuracy float32) error {
	query := `
		UPDATE model_versions
		SET validation_accuracy = $2
		WHERE version = $1
	`

	result, err := r.db.ExecContext(ctx, query, version, accuracy)
	if err != nil {
		return fmt.Errorf("failed to update validation score: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("model version not found: %s", version)
	}

	return nil
}

// DeleteModel deletes a model version and its weights
func (r *Repository) DeleteModel(ctx context.Context, version string) error {
	// Get model to find storage path
	model, err := r.GetModel(ctx, version)
	if err != nil {
		return err
	}
	if model == nil {
		return fmt.Errorf("model version not found: %s", version)
	}

	if model.IsActive {
		return fmt.Errorf("cannot delete active model version")
	}

	// Delete from database first
	_, err = r.db.ExecContext(ctx, "DELETE FROM model_versions WHERE version = $1", version)
	if err != nil {
		return fmt.Errorf("failed to delete model from database: %w", err)
	}

	// Delete from storage
	if err := r.storage.DeleteModel(ctx, model.StoragePath); err != nil {
		slog.Warn("Failed to delete model from storage",
			"version", version,
			"path", model.StoragePath,
			"error", err,
		)
		// Don't fail the operation if storage deletion fails
	}

	slog.Info("Deleted model version", "version", version)
	return nil
}

// RecordValidation records a validation result for a model
func (r *Repository) RecordValidation(ctx context.Context, input *ValidationInput) error {
	query := `
		INSERT INTO model_validations (
			model_version, agent_id, accuracy, precision_score, recall_score,
			f1_score, sample_count, validation_type, passed, details
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	detailsJSON, _ := json.Marshal(input.Details)

	_, err := r.db.ExecContext(ctx, query,
		input.ModelVersion,
		input.AgentID,
		input.Accuracy,
		input.Precision,
		input.Recall,
		input.F1Score,
		input.SampleCount,
		input.ValidationType,
		input.Passed,
		detailsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to record validation: %w", err)
	}

	return nil
}

// ValidationInput contains input for recording a validation result
type ValidationInput struct {
	ModelVersion   string
	AgentID        *string
	Accuracy       float32
	Precision      *float32
	Recall         *float32
	F1Score        *float32
	SampleCount    int64
	ValidationType string
	Passed         bool
	Details        map[string]interface{}
}

// GetValidationHistory retrieves validation history for a model
func (r *Repository) GetValidationHistory(ctx context.Context, version string) ([]ValidationResult, error) {
	query := `
		SELECT id, model_version, agent_id, validated_at, accuracy,
		       precision_score, recall_score, f1_score, sample_count,
		       validation_type, passed, details
		FROM model_validations
		WHERE model_version = $1
		ORDER BY validated_at DESC
		LIMIT 50
	`

	rows, err := r.db.QueryContext(ctx, query, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get validation history: %w", err)
	}
	defer rows.Close()

	var results []ValidationResult
	for rows.Next() {
		var v ValidationResult
		var agentID sql.NullString
		var precision, recall, f1 sql.NullFloat64
		var detailsJSON []byte

		err := rows.Scan(
			&v.ID, &v.ModelVersion, &agentID, &v.ValidatedAt, &v.Accuracy,
			&precision, &recall, &f1, &v.SampleCount,
			&v.ValidationType, &v.Passed, &detailsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan validation: %w", err)
		}

		if agentID.Valid {
			v.AgentID = &agentID.String
		}
		if precision.Valid {
			p := float32(precision.Float64)
			v.Precision = &p
		}
		if recall.Valid {
			r := float32(recall.Float64)
			v.Recall = &r
		}
		if f1.Valid {
			f := float32(f1.Float64)
			v.F1Score = &f
		}
		if len(detailsJSON) > 0 {
			_ = json.Unmarshal(detailsJSON, &v.Details)
		}

		results = append(results, v)
	}

	return results, rows.Err()
}

// ValidationResult represents a validation result
type ValidationResult struct {
	ID             string                 `json:"id"`
	ModelVersion   string                 `json:"model_version"`
	AgentID        *string                `json:"agent_id,omitempty"`
	ValidatedAt    time.Time              `json:"validated_at"`
	Accuracy       float32                `json:"accuracy"`
	Precision      *float32               `json:"precision,omitempty"`
	Recall         *float32               `json:"recall,omitempty"`
	F1Score        *float32               `json:"f1_score,omitempty"`
	SampleCount    int64                  `json:"sample_count"`
	ValidationType string                 `json:"validation_type"`
	Passed         bool                   `json:"passed"`
	Details        map[string]interface{} `json:"details,omitempty"`
}
