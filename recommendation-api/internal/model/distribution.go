// Package model provides model storage and management functionality
package model

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DeploymentStatus represents the status of a model deployment to an agent
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusDeployed   DeploymentStatus = "deployed"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

// Distributor handles model distribution to agents
type Distributor struct {
	db         *sql.DB
	repository *Repository
	mu         sync.RWMutex

	// Cache of agent model versions
	agentVersions map[string]string
}

// NewDistributor creates a new model distributor
func NewDistributor(db *sql.DB, repo *Repository) *Distributor {
	return &Distributor{
		db:            db,
		repository:    repo,
		agentVersions: make(map[string]string),
	}
}

// ModelDeployment represents a model deployment to an agent
type ModelDeployment struct {
	ID               string           `json:"id"`
	ModelVersion     string           `json:"model_version"`
	AgentID          string           `json:"agent_id"`
	DeployedAt       time.Time        `json:"deployed_at"`
	Status           DeploymentStatus `json:"status"`
	ValidationPassed *bool            `json:"validation_passed,omitempty"`
	ValidationError  *string          `json:"validation_error,omitempty"`
	PreviousVersion  *string          `json:"previous_version,omitempty"`
}

// GetModelForAgent returns the model weights for an agent
// Returns nil if the agent already has the latest version
func (d *Distributor) GetModelForAgent(ctx context.Context, agentID, currentVersion string) (*ModelUpdate, error) {
	// Get active model
	activeModel, err := d.repository.GetActiveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active model: %w", err)
	}

	if activeModel == nil {
		return nil, nil // No active model
	}

	// Check if agent already has the latest version
	if currentVersion == activeModel.Version {
		return nil, nil // Already up to date
	}

	// Get model weights
	weights, checksum, err := d.repository.GetModelWeights(ctx, activeModel.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get model weights: %w", err)
	}

	// Record pending deployment
	if err := d.recordDeployment(ctx, activeModel.Version, agentID, currentVersion); err != nil {
		slog.Warn("Failed to record deployment", "error", err)
		// Don't fail the request
	}

	return &ModelUpdate{
		Version:            activeModel.Version,
		Weights:            weights,
		Checksum:           checksum,
		SizeBytes:          activeModel.SizeBytes,
		ValidationAccuracy: activeModel.ValidationAccuracy,
		CreatedAt:          activeModel.CreatedAt,
	}, nil
}

// ModelUpdate contains model update data for an agent
type ModelUpdate struct {
	Version            string    `json:"version"`
	Weights            []byte    `json:"-"`
	Checksum           string    `json:"checksum"`
	SizeBytes          int64     `json:"size_bytes"`
	ValidationAccuracy float32   `json:"validation_accuracy"`
	CreatedAt          time.Time `json:"created_at"`
}

// GetIncrementalUpdate returns an incremental model update (delta) if available
// This is more efficient for small model changes
func (d *Distributor) GetIncrementalUpdate(ctx context.Context, agentID, currentVersion string) (*IncrementalUpdate, error) {
	// Get active model
	activeModel, err := d.repository.GetActiveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active model: %w", err)
	}

	if activeModel == nil || currentVersion == activeModel.Version {
		return nil, nil // No update needed
	}

	// Get current model weights
	currentWeights, _, err := d.repository.GetModelWeights(ctx, currentVersion)
	if err != nil {
		// If we can't get current weights, fall back to full update
		slog.Debug("Cannot get current weights for incremental update, will use full update",
			"current_version", currentVersion,
			"error", err,
		)
		return nil, nil
	}

	// Get new model weights
	newWeights, newChecksum, err := d.repository.GetModelWeights(ctx, activeModel.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get new model weights: %w", err)
	}

	// Calculate delta
	delta, err := calculateDelta(currentWeights, newWeights)
	if err != nil {
		slog.Debug("Cannot calculate delta, will use full update", "error", err)
		return nil, nil
	}

	// Only use incremental if delta is significantly smaller
	if len(delta) >= len(newWeights)*80/100 {
		// Delta is not much smaller, use full update
		return nil, nil
	}

	return &IncrementalUpdate{
		FromVersion: currentVersion,
		ToVersion:   activeModel.Version,
		Delta:       delta,
		Checksum:    newChecksum,
	}, nil
}

// IncrementalUpdate contains an incremental model update
type IncrementalUpdate struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Delta       []byte `json:"-"`
	Checksum    string `json:"checksum"`
}

// calculateDelta calculates the difference between two model weight sets
func calculateDelta(oldWeights, newWeights []byte) ([]byte, error) {
	if len(oldWeights) != len(newWeights) {
		return nil, fmt.Errorf("weight size mismatch: %d vs %d", len(oldWeights), len(newWeights))
	}

	// Simple XOR delta - in production, use a proper diff algorithm
	delta := make([]byte, len(newWeights))
	for i := range newWeights {
		delta[i] = oldWeights[i] ^ newWeights[i]
	}

	return delta, nil
}

// recordDeployment records a model deployment to an agent
func (d *Distributor) recordDeployment(ctx context.Context, modelVersion, agentID, previousVersion string) error {
	query := `
		INSERT INTO model_deployments (model_version, agent_id, status, previous_version)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (model_version, agent_id) DO UPDATE
		SET status = $3, deployed_at = NOW()
	`

	var prevVer *string
	if previousVersion != "" {
		prevVer = &previousVersion
	}

	_, err := d.db.ExecContext(ctx, query, modelVersion, agentID, DeploymentStatusPending, prevVer)
	return err
}

// UpdateDeploymentStatus updates the status of a model deployment
func (d *Distributor) UpdateDeploymentStatus(ctx context.Context, modelVersion, agentID string, status DeploymentStatus, validationPassed *bool, validationError *string) error {
	query := `
		UPDATE model_deployments
		SET status = $3, validation_passed = $4, validation_error = $5
		WHERE model_version = $1 AND agent_id = $2
	`

	result, err := d.db.ExecContext(ctx, query, modelVersion, agentID, status, validationPassed, validationError)
	if err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("deployment not found")
	}

	// Update cache
	d.mu.Lock()
	if status == DeploymentStatusDeployed {
		d.agentVersions[agentID] = modelVersion
	}
	d.mu.Unlock()

	slog.Info("Updated deployment status",
		"model_version", modelVersion,
		"agent_id", agentID,
		"status", status,
	)

	return nil
}

// GetDeploymentStatus returns the deployment status for a model version
func (d *Distributor) GetDeploymentStatus(ctx context.Context, modelVersion string) ([]ModelDeployment, error) {
	query := `
		SELECT id, model_version, agent_id, deployed_at, status,
		       validation_passed, validation_error, previous_version
		FROM model_deployments
		WHERE model_version = $1
		ORDER BY deployed_at DESC
	`

	rows, err := d.db.QueryContext(ctx, query, modelVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}
	defer rows.Close()

	var deployments []ModelDeployment
	for rows.Next() {
		var dep ModelDeployment
		var validationPassed sql.NullBool
		var validationError, previousVersion sql.NullString

		err := rows.Scan(
			&dep.ID, &dep.ModelVersion, &dep.AgentID, &dep.DeployedAt, &dep.Status,
			&validationPassed, &validationError, &previousVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment: %w", err)
		}

		if validationPassed.Valid {
			dep.ValidationPassed = &validationPassed.Bool
		}
		if validationError.Valid {
			dep.ValidationError = &validationError.String
		}
		if previousVersion.Valid {
			dep.PreviousVersion = &previousVersion.String
		}

		deployments = append(deployments, dep)
	}

	return deployments, rows.Err()
}

// GetAgentDeployments returns all deployments for an agent
func (d *Distributor) GetAgentDeployments(ctx context.Context, agentID string) ([]ModelDeployment, error) {
	query := `
		SELECT id, model_version, agent_id, deployed_at, status,
		       validation_passed, validation_error, previous_version
		FROM model_deployments
		WHERE agent_id = $1
		ORDER BY deployed_at DESC
		LIMIT 10
	`

	rows, err := d.db.QueryContext(ctx, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent deployments: %w", err)
	}
	defer rows.Close()

	var deployments []ModelDeployment
	for rows.Next() {
		var dep ModelDeployment
		var validationPassed sql.NullBool
		var validationError, previousVersion sql.NullString

		err := rows.Scan(
			&dep.ID, &dep.ModelVersion, &dep.AgentID, &dep.DeployedAt, &dep.Status,
			&validationPassed, &validationError, &previousVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment: %w", err)
		}

		if validationPassed.Valid {
			dep.ValidationPassed = &validationPassed.Bool
		}
		if validationError.Valid {
			dep.ValidationError = &validationError.String
		}
		if previousVersion.Valid {
			dep.PreviousVersion = &previousVersion.String
		}

		deployments = append(deployments, dep)
	}

	return deployments, rows.Err()
}

// GetDeploymentSummary returns a summary of deployments for a model version
func (d *Distributor) GetDeploymentSummary(ctx context.Context, modelVersion string) (*DeploymentSummary, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'deployed' THEN 1 ELSE 0 END) as deployed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'rolled_back' THEN 1 ELSE 0 END) as rolled_back,
			SUM(CASE WHEN validation_passed = TRUE THEN 1 ELSE 0 END) as validation_passed,
			SUM(CASE WHEN validation_passed = FALSE THEN 1 ELSE 0 END) as validation_failed
		FROM model_deployments
		WHERE model_version = $1
	`

	var summary DeploymentSummary
	err := d.db.QueryRowContext(ctx, query, modelVersion).Scan(
		&summary.Total,
		&summary.Pending,
		&summary.Deployed,
		&summary.Failed,
		&summary.RolledBack,
		&summary.ValidationPassed,
		&summary.ValidationFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment summary: %w", err)
	}

	summary.ModelVersion = modelVersion
	return &summary, nil
}

// DeploymentSummary contains a summary of model deployments
type DeploymentSummary struct {
	ModelVersion     string `json:"model_version"`
	Total            int    `json:"total"`
	Pending          int    `json:"pending"`
	Deployed         int    `json:"deployed"`
	Failed           int    `json:"failed"`
	RolledBack       int    `json:"rolled_back"`
	ValidationPassed int    `json:"validation_passed"`
	ValidationFailed int    `json:"validation_failed"`
}

// GetCurrentAgentVersion returns the current model version for an agent
func (d *Distributor) GetCurrentAgentVersion(ctx context.Context, agentID string) (string, error) {
	// Check cache first
	d.mu.RLock()
	if version, ok := d.agentVersions[agentID]; ok {
		d.mu.RUnlock()
		return version, nil
	}
	d.mu.RUnlock()

	// Query database
	query := `
		SELECT model_version
		FROM model_deployments
		WHERE agent_id = $1 AND status = 'deployed'
		ORDER BY deployed_at DESC
		LIMIT 1
	`

	var version string
	err := d.db.QueryRowContext(ctx, query, agentID).Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get agent version: %w", err)
	}

	// Update cache
	d.mu.Lock()
	d.agentVersions[agentID] = version
	d.mu.Unlock()

	return version, nil
}

// NotifyAgentsOfUpdate marks all agents as needing an update
// This is called when a new model version is activated
func (d *Distributor) NotifyAgentsOfUpdate(ctx context.Context, modelVersion string) error {
	// Get all registered agents
	query := `
		SELECT agent_id FROM agents WHERE last_seen_at > NOW() - INTERVAL '1 hour'
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get agents: %w", err)
	}
	defer rows.Close()

	var agentIDs []string
	for rows.Next() {
		var agentID string
		if err := rows.Scan(&agentID); err != nil {
			continue
		}
		agentIDs = append(agentIDs, agentID)
	}

	// Create pending deployments for all agents
	for _, agentID := range agentIDs {
		currentVersion, _ := d.GetCurrentAgentVersion(ctx, agentID)
		if currentVersion != modelVersion {
			_ = d.recordDeployment(ctx, modelVersion, agentID, currentVersion)
		}
	}

	slog.Info("Notified agents of model update",
		"model_version", modelVersion,
		"agent_count", len(agentIDs),
	)

	return nil
}
