// Package safety provides safety and rollout features for recommendations
package safety

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// DryRunResult contains the result of a dry-run evaluation
type DryRunResult struct {
	RecommendationID string           `json:"recommendation_id"`
	Namespace        string           `json:"namespace"`
	Deployment       string           `json:"deployment"`
	WouldApply       bool             `json:"would_apply"`
	Changes          []ResourceChange `json:"changes"`
	Warnings         []string         `json:"warnings,omitempty"`
	YamlPatch        string           `json:"yaml_patch"`
	EvaluatedAt      time.Time        `json:"evaluated_at"`
}

// ResourceChange describes a single resource change
type ResourceChange struct {
	Resource      string  `json:"resource"` // cpu_request, cpu_limit, memory_request, memory_limit
	CurrentValue  string  `json:"current_value"`
	NewValue      string  `json:"new_value"`
	ChangePercent float64 `json:"change_percent"`
	IsReduction   bool    `json:"is_reduction"`
}

// DryRunService handles dry-run mode operations
type DryRunService struct {
	db          *sql.DB
	configStore *ConfigStore
	logger      *slog.Logger
}

// NewDryRunService creates a new dry-run service
func NewDryRunService(db *sql.DB, configStore *ConfigStore, logger *slog.Logger) *DryRunService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DryRunService{
		db:          db,
		configStore: configStore,
		logger:      logger,
	}
}

// EvaluateDryRun evaluates what would happen if a recommendation were applied
func (s *DryRunService) EvaluateDryRun(ctx context.Context, recommendationID string) (*DryRunResult, error) {
	// Get the recommendation
	rec, err := s.getRecommendation(ctx, recommendationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation: %w", err)
	}

	result := &DryRunResult{
		RecommendationID: recommendationID,
		Namespace:        rec.Namespace,
		Deployment:       rec.Deployment,
		WouldApply:       true,
		EvaluatedAt:      time.Now(),
	}

	// Calculate changes
	result.Changes = s.calculateChanges(rec)

	// Check for warnings
	result.Warnings = s.checkWarnings(rec, result.Changes)

	// Generate YAML patch
	result.YamlPatch = s.generateYamlPatch(rec)

	// Log the dry-run evaluation
	s.logger.Info("dry-run evaluation completed",
		"recommendation_id", recommendationID,
		"namespace", rec.Namespace,
		"deployment", rec.Deployment,
		"changes_count", len(result.Changes),
		"warnings_count", len(result.Warnings),
	)

	// Store the dry-run result
	if err := s.storeDryRunResult(ctx, recommendationID, result); err != nil {
		s.logger.Warn("failed to store dry-run result", "error", err)
	}

	return result, nil
}

// recommendation holds recommendation data for dry-run evaluation
type recommendation struct {
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

func (s *DryRunService) getRecommendation(ctx context.Context, id string) (*recommendation, error) {
	query := `
		SELECT id, namespace, deployment,
		       cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes,
		       current_cpu_request_millicores, current_cpu_limit_millicores,
		       current_memory_request_bytes, current_memory_limit_bytes
		FROM recommendations
		WHERE id = $1
	`

	var rec recommendation
	err := s.db.QueryRowContext(ctx, query, id).Scan(
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
		return nil, err
	}

	return &rec, nil
}

func (s *DryRunService) calculateChanges(rec *recommendation) []ResourceChange {
	var changes []ResourceChange

	// CPU Request
	if rec.CurrentCpuRequest.Valid {
		current := rec.CurrentCpuRequest.Int32
		new := rec.CpuRequestMillicores
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, ResourceChange{
				Resource:      "cpu_request",
				CurrentValue:  fmt.Sprintf("%dm", current),
				NewValue:      fmt.Sprintf("%dm", new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// CPU Limit
	if rec.CurrentCpuLimit.Valid {
		current := rec.CurrentCpuLimit.Int32
		new := rec.CpuLimitMillicores
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, ResourceChange{
				Resource:      "cpu_limit",
				CurrentValue:  fmt.Sprintf("%dm", current),
				NewValue:      fmt.Sprintf("%dm", new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// Memory Request
	if rec.CurrentMemoryRequest.Valid {
		current := rec.CurrentMemoryRequest.Int64
		new := rec.MemoryRequestBytes
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, ResourceChange{
				Resource:      "memory_request",
				CurrentValue:  formatBytes(current),
				NewValue:      formatBytes(new),
				ChangePercent: changePercent,
				IsReduction:   new < current,
			})
		}
	}

	// Memory Limit
	if rec.CurrentMemoryLimit.Valid {
		current := rec.CurrentMemoryLimit.Int64
		new := rec.MemoryLimitBytes
		if current != new {
			changePercent := float64(new-current) / float64(current) * 100
			changes = append(changes, ResourceChange{
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

func (s *DryRunService) checkWarnings(rec *recommendation, changes []ResourceChange) []string {
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

func (s *DryRunService) generateYamlPatch(rec *recommendation) string {
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

func (s *DryRunService) storeDryRunResult(ctx context.Context, recommendationID string, result *DryRunResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal dry-run result: %w", err)
	}

	query := `
		UPDATE recommendations
		SET dry_run_result = $1
		WHERE id = $2
	`

	_, err = s.db.ExecContext(ctx, query, resultJSON, recommendationID)
	return err
}

// Helper function to format bytes
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
