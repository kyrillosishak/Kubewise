// Package safety provides safety and rollout features for recommendations
package safety

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// ApprovalStatus represents the status of an approval
type ApprovalStatus string

const (
	ApprovalStatusPending      ApprovalStatus = "pending"
	ApprovalStatusApproved     ApprovalStatus = "approved"
	ApprovalStatusRejected     ApprovalStatus = "rejected"
	ApprovalStatusAutoApproved ApprovalStatus = "auto_approved"
)

// ApprovalAction represents an action taken on a recommendation
type ApprovalAction string

const (
	ActionApproved     ApprovalAction = "approved"
	ActionRejected     ApprovalAction = "rejected"
	ActionAutoApproved ApprovalAction = "auto_approved"
)

// ApprovalHistory represents an approval action record
type ApprovalHistory struct {
	ID               string         `json:"id"`
	RecommendationID string         `json:"recommendation_id"`
	Action           ApprovalAction `json:"action"`
	Approver         string         `json:"approver"`
	Reason           string         `json:"reason,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

// ApprovalRequest represents a request to approve or reject a recommendation
type ApprovalRequest struct {
	RecommendationID string `json:"recommendation_id"`
	Approver         string `json:"approver"`
	Reason           string `json:"reason,omitempty"`
}

// ApprovalService handles approval workflow operations
type ApprovalService struct {
	db          *sql.DB
	configStore *ConfigStore
	logger      *slog.Logger
}

// NewApprovalService creates a new approval service
func NewApprovalService(db *sql.DB, configStore *ConfigStore, logger *slog.Logger) *ApprovalService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ApprovalService{
		db:          db,
		configStore: configStore,
		logger:      logger,
	}
}

// CheckRequiresApproval determines if a recommendation requires approval
func (s *ApprovalService) CheckRequiresApproval(ctx context.Context, recommendationID string) (bool, string, error) {
	// Get the recommendation
	rec, err := s.getRecommendation(ctx, recommendationID)
	if err != nil {
		return false, "", err
	}

	// Get namespace config
	config, err := s.configStore.GetNamespaceConfig(ctx, rec.Namespace)
	if err != nil {
		return false, "", fmt.Errorf("failed to get namespace config: %w", err)
	}

	// If auto-approve is enabled, no approval required
	if config.AutoApproveEnabled {
		return false, "", nil
	}

	// Check if this is a high-risk change
	isHighRisk, riskReason := s.evaluateRisk(rec, config)
	if isHighRisk {
		// Mark recommendation as requiring approval
		if err := s.markRequiresApproval(ctx, recommendationID, true, riskReason); err != nil {
			s.logger.Warn("failed to mark recommendation as requiring approval", "error", err)
		}
		return true, riskReason, nil
	}

	return false, "", nil
}

// recommendation holds recommendation data for approval evaluation
type approvalRecommendation struct {
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

func (s *ApprovalService) getRecommendation(ctx context.Context, id string) (*approvalRecommendation, error) {
	query := `
		SELECT id, namespace, deployment,
		       cpu_request_millicores, cpu_limit_millicores,
		       memory_request_bytes, memory_limit_bytes,
		       current_cpu_request_millicores, current_cpu_limit_millicores,
		       current_memory_request_bytes, current_memory_limit_bytes
		FROM recommendations
		WHERE id = $1
	`

	var rec approvalRecommendation
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

func (s *ApprovalService) evaluateRisk(rec *approvalRecommendation, config *NamespaceConfig) (bool, string) {
	// Check memory reduction
	if rec.CurrentMemoryLimit.Valid && rec.CurrentMemoryLimit.Int64 > 0 {
		memoryReduction := float64(rec.CurrentMemoryLimit.Int64-rec.MemoryLimitBytes) / float64(rec.CurrentMemoryLimit.Int64)
		if memoryReduction > config.HighRiskThresholdMemoryReduction {
			return true, fmt.Sprintf("Memory limit reduction of %.1f%% exceeds threshold of %.1f%%",
				memoryReduction*100, config.HighRiskThresholdMemoryReduction*100)
		}
	}

	// Check CPU reduction
	if rec.CurrentCpuLimit.Valid && rec.CurrentCpuLimit.Int32 > 0 {
		cpuReduction := float64(rec.CurrentCpuLimit.Int32-rec.CpuLimitMillicores) / float64(rec.CurrentCpuLimit.Int32)
		if cpuReduction > config.HighRiskThresholdCPUReduction {
			return true, fmt.Sprintf("CPU limit reduction of %.1f%% exceeds threshold of %.1f%%",
				cpuReduction*100, config.HighRiskThresholdCPUReduction*100)
		}
	}

	return false, ""
}

func (s *ApprovalService) markRequiresApproval(ctx context.Context, recommendationID string, requiresApproval bool, riskReason string) error {
	query := `
		UPDATE recommendations
		SET requires_approval = $2, is_high_risk = $3, risk_reason = $4
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, recommendationID, requiresApproval, requiresApproval, riskReason)
	return err
}

// Approve approves a recommendation
func (s *ApprovalService) Approve(ctx context.Context, req *ApprovalRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update recommendation status
	query := `
		UPDATE recommendations
		SET status = 'approved', approved_at = NOW(), approved_by = $2
		WHERE id = $1 AND status = 'pending'
	`

	result, err := tx.ExecContext(ctx, query, req.RecommendationID, req.Approver)
	if err != nil {
		return fmt.Errorf("failed to approve recommendation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("recommendation not found or already processed")
	}

	// Record approval history
	historyQuery := `
		INSERT INTO approval_history (recommendation_id, action, approver, reason)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, historyQuery, req.RecommendationID, ActionApproved, req.Approver, req.Reason)
	if err != nil {
		return fmt.Errorf("failed to record approval history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("recommendation approved",
		"recommendation_id", req.RecommendationID,
		"approver", req.Approver,
	)

	return nil
}

// Reject rejects a recommendation
func (s *ApprovalService) Reject(ctx context.Context, req *ApprovalRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
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

	result, err := tx.ExecContext(ctx, query, req.RecommendationID)
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
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, historyQuery, req.RecommendationID, ActionRejected, req.Approver, req.Reason)
	if err != nil {
		return fmt.Errorf("failed to record rejection history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("recommendation rejected",
		"recommendation_id", req.RecommendationID,
		"approver", req.Approver,
		"reason", req.Reason,
	)

	return nil
}

// AutoApprove auto-approves a recommendation (for low-risk changes when auto-approve is enabled)
func (s *ApprovalService) AutoApprove(ctx context.Context, recommendationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update recommendation status
	query := `
		UPDATE recommendations
		SET status = 'approved', approved_at = NOW(), approved_by = 'system'
		WHERE id = $1 AND status = 'pending'
	`

	result, err := tx.ExecContext(ctx, query, recommendationID)
	if err != nil {
		return fmt.Errorf("failed to auto-approve recommendation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("recommendation not found or already processed")
	}

	// Record auto-approval history
	historyQuery := `
		INSERT INTO approval_history (recommendation_id, action, approver, reason)
		VALUES ($1, $2, 'system', 'Auto-approved based on namespace configuration')
	`
	_, err = tx.ExecContext(ctx, historyQuery, recommendationID, ActionAutoApproved)
	if err != nil {
		return fmt.Errorf("failed to record auto-approval history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("recommendation auto-approved",
		"recommendation_id", recommendationID,
	)

	return nil
}

// GetApprovalHistory returns the approval history for a recommendation
func (s *ApprovalService) GetApprovalHistory(ctx context.Context, recommendationID string) ([]ApprovalHistory, error) {
	query := `
		SELECT id, recommendation_id, action, approver, reason, created_at
		FROM approval_history
		WHERE recommendation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, recommendationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approval history: %w", err)
	}
	defer rows.Close()

	var history []ApprovalHistory
	for rows.Next() {
		var h ApprovalHistory
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
func (s *ApprovalService) GetPendingApprovals(ctx context.Context, namespace string) ([]string, error) {
	query := `
		SELECT id
		FROM recommendations
		WHERE status = 'pending' AND requires_approval = TRUE
		  AND ($1 = '' OR namespace = $1)
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending approvals: %w", err)
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
