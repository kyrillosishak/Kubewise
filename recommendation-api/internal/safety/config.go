// Package safety provides safety and rollout features for recommendations
package safety

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// NamespaceConfig holds safety configuration for a namespace
type NamespaceConfig struct {
	Namespace                       string    `json:"namespace"`
	DryRunEnabled                   bool      `json:"dry_run_enabled"`
	AutoApproveEnabled              bool      `json:"auto_approve_enabled"`
	HighRiskThresholdMemoryReduction float64   `json:"high_risk_threshold_memory_reduction"`
	HighRiskThresholdCPUReduction   float64   `json:"high_risk_threshold_cpu_reduction"`
	CreatedAt                       time.Time `json:"created_at"`
	UpdatedAt                       time.Time `json:"updated_at"`
}

// ConfigStore handles namespace configuration persistence
type ConfigStore struct {
	db *sql.DB
}

// NewConfigStore creates a new config store
func NewConfigStore(db *sql.DB) *ConfigStore {
	return &ConfigStore{db: db}
}

// GetNamespaceConfig returns the configuration for a namespace
// Falls back to global config if namespace-specific config doesn't exist
func (s *ConfigStore) GetNamespaceConfig(ctx context.Context, namespace string) (*NamespaceConfig, error) {
	query := `
		SELECT namespace, dry_run_enabled, auto_approve_enabled,
		       high_risk_threshold_memory_reduction, high_risk_threshold_cpu_reduction,
		       created_at, updated_at
		FROM namespace_config
		WHERE namespace = $1
	`

	var config NamespaceConfig
	err := s.db.QueryRowContext(ctx, query, namespace).Scan(
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
		return s.GetNamespaceConfig(ctx, "_global")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace config: %w", err)
	}

	return &config, nil
}

// SetNamespaceConfig creates or updates namespace configuration
func (s *ConfigStore) SetNamespaceConfig(ctx context.Context, config *NamespaceConfig) error {
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

	_, err := s.db.ExecContext(ctx, query,
		config.Namespace,
		config.DryRunEnabled,
		config.AutoApproveEnabled,
		config.HighRiskThresholdMemoryReduction,
		config.HighRiskThresholdCPUReduction,
	)
	if err != nil {
		return fmt.Errorf("failed to set namespace config: %w", err)
	}

	return nil
}

// ListNamespaceConfigs returns all namespace configurations
func (s *ConfigStore) ListNamespaceConfigs(ctx context.Context) ([]NamespaceConfig, error) {
	query := `
		SELECT namespace, dry_run_enabled, auto_approve_enabled,
		       high_risk_threshold_memory_reduction, high_risk_threshold_cpu_reduction,
		       created_at, updated_at
		FROM namespace_config
		ORDER BY namespace
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespace configs: %w", err)
	}
	defer rows.Close()

	var configs []NamespaceConfig
	for rows.Next() {
		var config NamespaceConfig
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

// IsDryRunEnabled checks if dry-run mode is enabled for a namespace
func (s *ConfigStore) IsDryRunEnabled(ctx context.Context, namespace string) (bool, error) {
	config, err := s.GetNamespaceConfig(ctx, namespace)
	if err != nil {
		return false, err
	}
	return config.DryRunEnabled, nil
}
