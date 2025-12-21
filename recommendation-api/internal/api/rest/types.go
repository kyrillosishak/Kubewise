// Package rest provides REST API handlers
package rest

import "time"

// Recommendation represents a resource recommendation for a deployment
type Recommendation struct {
	ID                   string    `json:"id"`
	Namespace            string    `json:"namespace"`
	Deployment           string    `json:"deployment"`
	CpuRequestMillicores uint32    `json:"cpu_request_millicores"`
	CpuLimitMillicores   uint32    `json:"cpu_limit_millicores"`
	MemoryRequestBytes   uint64    `json:"memory_request_bytes"`
	MemoryLimitBytes     uint64    `json:"memory_limit_bytes"`
	Confidence           float32   `json:"confidence"`
	ModelVersion         string    `json:"model_version"`
	Status               string    `json:"status"` // pending, approved, applied, rolled_back
	CreatedAt            time.Time `json:"created_at"`
	AppliedAt            *time.Time `json:"applied_at,omitempty"`
	TimeWindow           string    `json:"time_window"` // peak, off_peak, weekly
	CurrentResources     *ResourceSpec `json:"current_resources,omitempty"`
	RecommendedResources *ResourceSpec `json:"recommended_resources,omitempty"`
}

// ResourceSpec represents current or recommended resource specifications
type ResourceSpec struct {
	CpuRequest    string `json:"cpu_request"`
	CpuLimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
}

// RecommendationList is a list of recommendations
type RecommendationList struct {
	Recommendations []Recommendation `json:"recommendations"`
	Total           int              `json:"total"`
}

// ApplyRecommendationRequest is the request body for applying a recommendation
type ApplyRecommendationRequest struct {
	DryRun bool `json:"dry_run"`
}

// ApplyRecommendationResponse is the response for applying a recommendation
type ApplyRecommendationResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	YamlPatch string `json:"yaml_patch,omitempty"`
}

// ApproveRecommendationResponse is the response for approving a recommendation
type ApproveRecommendationResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Approver string `json:"approver,omitempty"`
}


// CostAnalysis represents cost analysis for a namespace or cluster
type CostAnalysis struct {
	Namespace              string  `json:"namespace,omitempty"`
	CurrentMonthlyCost     float64 `json:"current_monthly_cost"`
	RecommendedMonthlyCost float64 `json:"recommended_monthly_cost"`
	PotentialSavings       float64 `json:"potential_savings"`
	Currency               string  `json:"currency"`
	DeploymentCount        int     `json:"deployment_count"`
	LastUpdated            time.Time `json:"last_updated"`
}

// SavingsReport represents savings over time
type SavingsReport struct {
	TotalSavings    float64         `json:"total_savings"`
	Currency        string          `json:"currency"`
	Period          string          `json:"period"`
	SavingsByMonth  []MonthlySaving `json:"savings_by_month"`
	SavingsByTeam   []TeamSaving    `json:"savings_by_team,omitempty"`
}

// MonthlySaving represents savings for a specific month
type MonthlySaving struct {
	Month   string  `json:"month"`
	Savings float64 `json:"savings"`
}

// TeamSaving represents savings for a specific team/namespace
type TeamSaving struct {
	Team    string  `json:"team"`
	Savings float64 `json:"savings"`
}

// ModelVersion represents a model version
type ModelVersion struct {
	Version            string    `json:"version"`
	CreatedAt          time.Time `json:"created_at"`
	ValidationAccuracy float32   `json:"validation_accuracy"`
	SizeBytes          int64     `json:"size_bytes"`
	IsActive           bool      `json:"is_active"`
	RollbackCount      int       `json:"rollback_count"`
}

// ModelList is a list of model versions
type ModelList struct {
	Models []ModelVersion `json:"models"`
	Total  int            `json:"total"`
}

// RollbackResponse is the response for model rollback
type RollbackResponse struct {
	Version        string `json:"version"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	PreviousActive string `json:"previous_active,omitempty"`
}

// PredictionHistory represents prediction history for a deployment
type PredictionHistory struct {
	Deployment  string       `json:"deployment"`
	Namespace   string       `json:"namespace"`
	Predictions []Prediction `json:"predictions"`
}

// Prediction represents a single prediction entry
type Prediction struct {
	Timestamp            time.Time `json:"timestamp"`
	CpuRequestMillicores uint32    `json:"cpu_request_millicores"`
	CpuLimitMillicores   uint32    `json:"cpu_limit_millicores"`
	MemoryRequestBytes   uint64    `json:"memory_request_bytes"`
	MemoryLimitBytes     uint64    `json:"memory_limit_bytes"`
	Confidence           float32   `json:"confidence"`
	ModelVersion         string    `json:"model_version"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

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
	Resource      string  `json:"resource"`
	CurrentValue  string  `json:"current_value"`
	NewValue      string  `json:"new_value"`
	ChangePercent float64 `json:"change_percent"`
	IsReduction   bool    `json:"is_reduction"`
}

// NamespaceConfig holds safety configuration for a namespace
type NamespaceConfig struct {
	Namespace                        string    `json:"namespace"`
	DryRunEnabled                    bool      `json:"dry_run_enabled"`
	AutoApproveEnabled               bool      `json:"auto_approve_enabled"`
	HighRiskThresholdMemoryReduction float64   `json:"high_risk_threshold_memory_reduction"`
	HighRiskThresholdCPUReduction    float64   `json:"high_risk_threshold_cpu_reduction"`
	CreatedAt                        time.Time `json:"created_at"`
	UpdatedAt                        time.Time `json:"updated_at"`
}

// NamespaceConfigRequest is the request body for updating namespace config
type NamespaceConfigRequest struct {
	DryRunEnabled                    *bool    `json:"dry_run_enabled,omitempty"`
	AutoApproveEnabled               *bool    `json:"auto_approve_enabled,omitempty"`
	HighRiskThresholdMemoryReduction *float64 `json:"high_risk_threshold_memory_reduction,omitempty"`
	HighRiskThresholdCPUReduction    *float64 `json:"high_risk_threshold_cpu_reduction,omitempty"`
}

// ApprovalHistory represents an approval action
type ApprovalHistory struct {
	ID               string    `json:"id"`
	RecommendationID string    `json:"recommendation_id"`
	Action           string    `json:"action"` // approved, rejected, auto_approved
	Approver         string    `json:"approver"`
	Reason           string    `json:"reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ApproveRequest is the request body for approving a recommendation
type ApproveRequest struct {
	Approver string `json:"approver"`
	Reason   string `json:"reason,omitempty"`
}

// RecommendationOutcome tracks the outcome of an applied recommendation
type RecommendationOutcome struct {
	ID                       string    `json:"id"`
	RecommendationID         string    `json:"recommendation_id"`
	Namespace                string    `json:"namespace"`
	Deployment               string    `json:"deployment"`
	AppliedAt                time.Time `json:"applied_at"`
	CheckTime                time.Time `json:"check_time"`
	OOMKillsBefore           int       `json:"oom_kills_before"`
	OOMKillsAfter            int       `json:"oom_kills_after"`
	CPUThrottleBefore        float64   `json:"cpu_throttle_before"`
	CPUThrottleAfter         float64   `json:"cpu_throttle_after"`
	OutcomeStatus            string    `json:"outcome_status"` // monitoring, success, degraded, rolled_back
	RollbackTriggered        bool      `json:"rollback_triggered"`
	RollbackRecommendationID string    `json:"rollback_recommendation_id,omitempty"`
}

// RollbackEvent represents an automatic or manual rollback event
type RollbackEvent struct {
	ID                       string    `json:"id"`
	OriginalRecommendationID string    `json:"original_recommendation_id"`
	RollbackRecommendationID string    `json:"rollback_recommendation_id,omitempty"`
	TriggerReason            string    `json:"trigger_reason"` // oom_increase, throttle_increase, manual
	OOMKillsDetected         int       `json:"oom_kills_detected,omitempty"`
	ThrottleIncreasePercent  float64   `json:"throttle_increase_percent,omitempty"`
	AutoTriggered            bool      `json:"auto_triggered"`
	CreatedAt                time.Time `json:"created_at"`
	AlertSent                bool      `json:"alert_sent"`
}
