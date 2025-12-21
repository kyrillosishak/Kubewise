// Package model provides model storage and management functionality
package model

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// FederatedAggregator handles federated learning gradient aggregation
type FederatedAggregator struct {
	db         *sql.DB
	repository *Repository
	mu         sync.Mutex

	// Configuration
	minAgentsForAggregation int
	aggregationInterval     time.Duration
}

// FederatedConfig holds configuration for federated learning
type FederatedConfig struct {
	// Minimum number of agents required before aggregation
	MinAgentsForAggregation int
	// How often to check for and perform aggregation
	AggregationInterval time.Duration
}

// DefaultFederatedConfig returns default federated learning configuration
func DefaultFederatedConfig() *FederatedConfig {
	return &FederatedConfig{
		MinAgentsForAggregation: 3,
		AggregationInterval:     time.Hour,
	}
}

// NewFederatedAggregator creates a new federated learning aggregator
func NewFederatedAggregator(db *sql.DB, repo *Repository, cfg *FederatedConfig) *FederatedAggregator {
	if cfg == nil {
		cfg = DefaultFederatedConfig()
	}

	return &FederatedAggregator{
		db:                      db,
		repository:              repo,
		minAgentsForAggregation: cfg.MinAgentsForAggregation,
		aggregationInterval:     cfg.AggregationInterval,
	}
}

// GradientUpdate represents a gradient update from an agent
type GradientUpdate struct {
	ID           string
	AgentID      string
	ModelVersion string
	Gradients    []byte
	SampleCount  int64
	CreatedAt    time.Time
	Aggregated   bool
}

// StoreGradients stores gradient updates from an agent
func (f *FederatedAggregator) StoreGradients(ctx context.Context, agentID, modelVersion string, gradients []byte, sampleCount int64) error {
	query := `
		INSERT INTO model_gradients (agent_id, model_version, gradients, sample_count, aggregated)
		VALUES ($1, $2, $3, $4, FALSE)
	`

	_, err := f.db.ExecContext(ctx, query, agentID, modelVersion, gradients, sampleCount)
	if err != nil {
		return fmt.Errorf("failed to store gradients: %w", err)
	}

	slog.Info("Stored gradient update",
		"agent_id", agentID,
		"model_version", modelVersion,
		"sample_count", sampleCount,
		"gradients_size", len(gradients),
	)

	return nil
}

// GetPendingGradients retrieves all non-aggregated gradients for a model version
func (f *FederatedAggregator) GetPendingGradients(ctx context.Context, modelVersion string) ([]GradientUpdate, error) {
	query := `
		SELECT id, agent_id, model_version, gradients, sample_count, created_at, aggregated
		FROM model_gradients
		WHERE model_version = $1 AND aggregated = FALSE
		ORDER BY created_at ASC
	`

	rows, err := f.db.QueryContext(ctx, query, modelVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending gradients: %w", err)
	}
	defer rows.Close()

	var updates []GradientUpdate
	for rows.Next() {
		var g GradientUpdate
		err := rows.Scan(&g.ID, &g.AgentID, &g.ModelVersion, &g.Gradients, &g.SampleCount, &g.CreatedAt, &g.Aggregated)
		if err != nil {
			return nil, fmt.Errorf("failed to scan gradient: %w", err)
		}
		updates = append(updates, g)
	}

	return updates, rows.Err()
}

// AggregateGradients performs FedAvg aggregation on pending gradients
func (f *FederatedAggregator) AggregateGradients(ctx context.Context, modelVersion string) (*AggregationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Get pending gradients
	updates, err := f.GetPendingGradients(ctx, modelVersion)
	if err != nil {
		return nil, err
	}

	if len(updates) < f.minAgentsForAggregation {
		return nil, fmt.Errorf("insufficient agents for aggregation: have %d, need %d",
			len(updates), f.minAgentsForAggregation)
	}

	slog.Info("Starting gradient aggregation",
		"model_version", modelVersion,
		"num_updates", len(updates),
	)

	// Perform FedAvg aggregation
	aggregatedGradients, totalSamples, err := f.fedAvg(updates)
	if err != nil {
		return nil, fmt.Errorf("fedavg aggregation failed: %w", err)
	}

	// Mark gradients as aggregated
	if err := f.markGradientsAggregated(ctx, updates); err != nil {
		return nil, fmt.Errorf("failed to mark gradients as aggregated: %w", err)
	}

	result := &AggregationResult{
		ModelVersion:        modelVersion,
		AggregatedGradients: aggregatedGradients,
		TotalSamples:        totalSamples,
		NumAgents:           len(updates),
		AggregatedAt:        time.Now(),
	}

	slog.Info("Gradient aggregation completed",
		"model_version", modelVersion,
		"num_agents", len(updates),
		"total_samples", totalSamples,
	)

	return result, nil
}

// fedAvg implements the Federated Averaging algorithm
// Weights each gradient by its sample count
func (f *FederatedAggregator) fedAvg(updates []GradientUpdate) ([]byte, int64, error) {
	if len(updates) == 0 {
		return nil, 0, fmt.Errorf("no updates to aggregate")
	}

	// Calculate total samples for weighting
	var totalSamples int64
	for _, u := range updates {
		totalSamples += u.SampleCount
	}

	if totalSamples == 0 {
		return nil, 0, fmt.Errorf("total sample count is zero")
	}

	// Parse first gradient to get dimensions
	firstGradients, err := parseGradients(updates[0].Gradients)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse first gradient: %w", err)
	}

	// Initialize aggregated gradients
	aggregated := make([]float32, len(firstGradients))

	// Weighted average: sum(weight_i * gradient_i) where weight_i = samples_i / total_samples
	for _, update := range updates {
		gradients, err := parseGradients(update.Gradients)
		if err != nil {
			slog.Warn("Failed to parse gradient, skipping",
				"agent_id", update.AgentID,
				"error", err,
			)
			continue
		}

		if len(gradients) != len(aggregated) {
			slog.Warn("Gradient dimension mismatch, skipping",
				"agent_id", update.AgentID,
				"expected", len(aggregated),
				"got", len(gradients),
			)
			continue
		}

		weight := float32(update.SampleCount) / float32(totalSamples)
		for i, g := range gradients {
			aggregated[i] += weight * g
		}
	}

	// Serialize aggregated gradients
	result := serializeGradients(aggregated)

	return result, totalSamples, nil
}

// parseGradients deserializes gradient bytes to float32 slice
func parseGradients(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid gradient data length: %d", len(data))
	}

	numFloats := len(data) / 4
	gradients := make([]float32, numFloats)

	for i := 0; i < numFloats; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		gradients[i] = math.Float32frombits(bits)
	}

	return gradients, nil
}

// serializeGradients serializes float32 slice to bytes
func serializeGradients(gradients []float32) []byte {
	data := make([]byte, len(gradients)*4)

	for i, g := range gradients {
		bits := math.Float32bits(g)
		binary.LittleEndian.PutUint32(data[i*4:(i+1)*4], bits)
	}

	return data
}

// markGradientsAggregated marks gradient updates as aggregated
func (f *FederatedAggregator) markGradientsAggregated(ctx context.Context, updates []GradientUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE model_gradients SET aggregated = TRUE WHERE id = $1")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range updates {
		if _, err := stmt.ExecContext(ctx, u.ID); err != nil {
			return fmt.Errorf("failed to mark gradient as aggregated: %w", err)
		}
	}

	return tx.Commit()
}

// AggregationResult contains the result of gradient aggregation
type AggregationResult struct {
	ModelVersion        string
	AggregatedGradients []byte
	TotalSamples        int64
	NumAgents           int
	AggregatedAt        time.Time
}

// GenerateNewModelVersion creates a new model version from aggregated gradients
func (f *FederatedAggregator) GenerateNewModelVersion(ctx context.Context, baseVersion string, aggregation *AggregationResult) (*ModelVersion, error) {
	// Get base model
	baseModel, err := f.repository.GetModel(ctx, baseVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get base model: %w", err)
	}
	if baseModel == nil {
		return nil, fmt.Errorf("base model not found: %s", baseVersion)
	}

	// Get base model weights
	baseWeights, _, err := f.repository.GetModelWeights(ctx, baseVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get base model weights: %w", err)
	}

	// Apply gradients to base weights
	newWeights, err := applyGradients(baseWeights, aggregation.AggregatedGradients)
	if err != nil {
		return nil, fmt.Errorf("failed to apply gradients: %w", err)
	}

	// Generate new version string
	newVersion := generateVersionString(baseVersion)

	// Create new model version
	input := &CreateModelInput{
		Version:         newVersion,
		Description:     fmt.Sprintf("Federated learning update from %s with %d agents", baseVersion, aggregation.NumAgents),
		Weights:         newWeights,
		TrainingSamples: aggregation.TotalSamples,
		Metadata: map[string]interface{}{
			"base_version":       baseVersion,
			"aggregation_agents": aggregation.NumAgents,
			"aggregation_time":   aggregation.AggregatedAt,
		},
	}

	newModel, err := f.repository.CreateModel(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create new model version: %w", err)
	}

	slog.Info("Generated new model version from federated learning",
		"base_version", baseVersion,
		"new_version", newVersion,
		"num_agents", aggregation.NumAgents,
		"total_samples", aggregation.TotalSamples,
	)

	return newModel, nil
}

// applyGradients applies gradient updates to model weights
// Uses simple gradient descent: new_weights = old_weights - learning_rate * gradients
func applyGradients(weights []byte, gradients []byte) ([]byte, error) {
	const learningRate = 0.01

	weightFloats, err := parseGradients(weights)
	if err != nil {
		return nil, fmt.Errorf("failed to parse weights: %w", err)
	}

	gradientFloats, err := parseGradients(gradients)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gradients: %w", err)
	}

	if len(weightFloats) != len(gradientFloats) {
		return nil, fmt.Errorf("weight and gradient dimension mismatch: %d vs %d",
			len(weightFloats), len(gradientFloats))
	}

	// Apply gradient descent
	for i := range weightFloats {
		weightFloats[i] -= learningRate * gradientFloats[i]
	}

	return serializeGradients(weightFloats), nil
}

// generateVersionString generates a new version string based on the base version
func generateVersionString(baseVersion string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("v%s-fed", timestamp)
}

// GetAggregationStats returns statistics about gradient aggregation
func (f *FederatedAggregator) GetAggregationStats(ctx context.Context, modelVersion string) (*AggregationStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_updates,
			COUNT(DISTINCT agent_id) as unique_agents,
			SUM(sample_count) as total_samples,
			SUM(CASE WHEN aggregated THEN 1 ELSE 0 END) as aggregated_count,
			SUM(CASE WHEN NOT aggregated THEN 1 ELSE 0 END) as pending_count
		FROM model_gradients
		WHERE model_version = $1
	`

	var stats AggregationStats
	err := f.db.QueryRowContext(ctx, query, modelVersion).Scan(
		&stats.TotalUpdates,
		&stats.UniqueAgents,
		&stats.TotalSamples,
		&stats.AggregatedCount,
		&stats.PendingCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregation stats: %w", err)
	}

	stats.ModelVersion = modelVersion
	stats.MinAgentsRequired = f.minAgentsForAggregation
	stats.ReadyForAggregation = stats.PendingCount >= f.minAgentsForAggregation

	return &stats, nil
}

// AggregationStats contains statistics about gradient aggregation
type AggregationStats struct {
	ModelVersion        string `json:"model_version"`
	TotalUpdates        int    `json:"total_updates"`
	UniqueAgents        int    `json:"unique_agents"`
	TotalSamples        int64  `json:"total_samples"`
	AggregatedCount     int    `json:"aggregated_count"`
	PendingCount        int    `json:"pending_count"`
	MinAgentsRequired   int    `json:"min_agents_required"`
	ReadyForAggregation bool   `json:"ready_for_aggregation"`
}

// CleanupOldGradients removes old aggregated gradients
func (f *FederatedAggregator) CleanupOldGradients(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM model_gradients
		WHERE aggregated = TRUE AND created_at < $1
	`

	cutoff := time.Now().Add(-olderThan)
	result, err := f.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old gradients: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		slog.Info("Cleaned up old gradients", "count", rowsAffected, "older_than", olderThan)
	}

	return rowsAffected, nil
}
