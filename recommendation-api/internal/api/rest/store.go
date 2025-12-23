// Package rest provides REST API handlers
package rest

import (
	"context"
)

// Store defines the interface for data persistence
type Store interface {
	RecommendationStore
	CostStore
	ModelStore
	PredictionStore
}

// RecommendationStore handles recommendation data
type RecommendationStore interface {
	ListRecommendations(ctx context.Context, namespace string) ([]Recommendation, error)
	GetRecommendation(ctx context.Context, namespace, name string) (*Recommendation, error)
	GetRecommendationByID(ctx context.Context, id string) (*Recommendation, error)
	ApplyRecommendation(ctx context.Context, id string, dryRun bool) (*ApplyRecommendationResponse, error)
	ApproveRecommendation(ctx context.Context, id string) (*ApproveRecommendationResponse, error)
}

// CostStore handles cost analysis data
type CostStore interface {
	GetClusterCosts(ctx context.Context) (*CostAnalysis, error)
	GetNamespaceCosts(ctx context.Context, namespace string) (*CostAnalysis, error)
	GetSavingsReport(ctx context.Context, since string) (*SavingsReport, error)
}

// ModelStore handles model version data
type ModelStore interface {
	ListModels(ctx context.Context) ([]ModelVersion, error)
	GetModel(ctx context.Context, version string) (*ModelVersion, error)
	RollbackModel(ctx context.Context, version string) (*RollbackResponse, error)
}

// PredictionStore handles prediction history data
type PredictionStore interface {
	GetPredictionHistory(ctx context.Context, namespace, deployment string) (*PredictionHistory, error)
}

// SafetyStore handles safety and rollout features
type SafetyStore interface {
	// Namespace configuration
	GetNamespaceConfig(ctx context.Context, namespace string) (*NamespaceConfig, error)
	SetNamespaceConfig(ctx context.Context, config *NamespaceConfig) error
	ListNamespaceConfigs(ctx context.Context) ([]NamespaceConfig, error)

	// Dry-run
	EvaluateDryRun(ctx context.Context, recommendationID string) (*DryRunResult, error)

	// Approval workflow
	ApproveRecommendationWithDetails(ctx context.Context, id string, approver string, reason string) (*ApproveRecommendationResponse, error)
	RejectRecommendation(ctx context.Context, id string, approver string, reason string) error
	GetApprovalHistory(ctx context.Context, recommendationID string) ([]ApprovalHistory, error)
	GetPendingApprovals(ctx context.Context, namespace string) ([]Recommendation, error)

	// Outcome tracking
	GetRecommendationOutcome(ctx context.Context, recommendationID string) (*RecommendationOutcome, error)
	RecordOutcome(ctx context.Context, outcome *RecommendationOutcome) error
	UpdateOutcomeStatus(ctx context.Context, recommendationID string, status string) error

	// Rollback
	ListRollbackEvents(ctx context.Context, namespace string) ([]RollbackEvent, error)
	TriggerRollback(ctx context.Context, recommendationID string, reason string, autoTriggered bool) (*RollbackEvent, error)
}

// ClusterStore handles cluster data
type ClusterStore interface {
	ListClusters(ctx context.Context) ([]Cluster, error)
	GetClusterHealth(ctx context.Context, clusterID string) (*ClusterHealth, error)
}

// AnomalyStore handles anomaly data
type AnomalyStore interface {
	ListAnomalies(ctx context.Context, filters AnomalyFilters) ([]Anomaly, error)
	GetAnomalyDetail(ctx context.Context, anomalyID string) (*AnomalyDetail, error)
}

// AuthStore handles authentication
type AuthStore interface {
	Authenticate(ctx context.Context, email, password string) (*User, string, error)
	ValidateToken(ctx context.Context, token string) (string, error)
	GetUser(ctx context.Context, userID string) (*User, error)
	GetPermissions(ctx context.Context, userID string) ([]Permission, error)
}

// store is the global store instance
var store Store

// safetyStore is the global safety store instance
var safetyStore SafetyStore

// clusterStore is the global cluster store instance
var clusterStore ClusterStore

// anomalyStore is the global anomaly store instance
var anomalyStore AnomalyStore

// authStore is the global auth store instance
var authStore AuthStore

// SetStore sets the global store instance
func SetStore(s Store) {
	store = s
}

// GetStore returns the global store instance
func GetStore() Store {
	return store
}

// SetSafetyStore sets the global safety store instance
func SetSafetyStore(s SafetyStore) {
	safetyStore = s
}

// GetSafetyStore returns the global safety store instance
func GetSafetyStore() SafetyStore {
	return safetyStore
}

// SetClusterStore sets the global cluster store instance
func SetClusterStore(s ClusterStore) {
	clusterStore = s
}

// getClusterStore returns the global cluster store instance
func getClusterStore() ClusterStore {
	return clusterStore
}

// SetAnomalyStore sets the global anomaly store instance
func SetAnomalyStore(s AnomalyStore) {
	anomalyStore = s
}

// getAnomalyStore returns the global anomaly store instance
func getAnomalyStore() AnomalyStore {
	return anomalyStore
}

// SetAuthStore sets the global auth store instance
func SetAuthStore(s AuthStore) {
	authStore = s
}

// getAuthStore returns the global auth store instance
func getAuthStore() AuthStore {
	return authStore
}
