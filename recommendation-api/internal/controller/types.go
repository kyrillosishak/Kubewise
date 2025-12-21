// Package controller implements the Kubernetes controller for ResourceRecommendation CRD
package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceRecommendation represents the CRD for resource recommendations
type ResourceRecommendation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceRecommendationSpec   `json:"spec,omitempty"`
	Status ResourceRecommendationStatus `json:"status,omitempty"`
}

// ResourceRecommendationSpec defines the desired state
type ResourceRecommendationSpec struct {
	TargetRef        TargetRef        `json:"targetRef"`
	Recommendation   Recommendation   `json:"recommendation"`
	CostImpact       *CostImpact      `json:"costImpact,omitempty"`
	AutoApply        bool             `json:"autoApply,omitempty"`
	RequiresApproval bool             `json:"requiresApproval,omitempty"`
	RiskLevel        string           `json:"riskLevel,omitempty"`
}

// TargetRef identifies the target workload
type TargetRef struct {
	APIVersion    string `json:"apiVersion,omitempty"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	ContainerName string `json:"containerName,omitempty"`
}

// Recommendation contains the recommended resource values
type Recommendation struct {
	CPURequest   string    `json:"cpuRequest,omitempty"`
	CPULimit     string    `json:"cpuLimit,omitempty"`
	MemoryRequest string   `json:"memoryRequest,omitempty"`
	MemoryLimit  string    `json:"memoryLimit,omitempty"`
	Confidence   float64   `json:"confidence,omitempty"`
	ModelVersion string    `json:"modelVersion,omitempty"`
	GeneratedAt  time.Time `json:"generatedAt,omitempty"`
	TimeWindow   string    `json:"timeWindow,omitempty"`
}

// CostImpact contains cost analysis
type CostImpact struct {
	CurrentMonthlyCost   string `json:"currentMonthlyCost,omitempty"`
	ProjectedMonthlyCost string `json:"projectedMonthlyCost,omitempty"`
	MonthlySavings       string `json:"monthlySavings,omitempty"`
	Currency             string `json:"currency,omitempty"`
}

// ResourceRecommendationStatus defines the observed state
type ResourceRecommendationStatus struct {
	Phase             string             `json:"phase,omitempty"`
	Conditions        []Condition        `json:"conditions,omitempty"`
	AppliedAt         *time.Time         `json:"appliedAt,omitempty"`
	AppliedBy         string             `json:"appliedBy,omitempty"`
	ApprovedAt        *time.Time         `json:"approvedAt,omitempty"`
	ApprovedBy        string             `json:"approvedBy,omitempty"`
	PreviousResources *PreviousResources `json:"previousResources,omitempty"`
	Outcome           *Outcome           `json:"outcome,omitempty"`
	LastUpdated       *time.Time         `json:"lastUpdated,omitempty"`
	Message           string             `json:"message,omitempty"`
	GeneratedPatch    string             `json:"generatedPatch,omitempty"`
}

// Condition represents a status condition
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// PreviousResources stores the original resource values
type PreviousResources struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// Outcome tracks the result of applying a recommendation
type Outcome struct {
	OOMKills           int     `json:"oomKills,omitempty"`
	CPUThrottleIncrease float64 `json:"cpuThrottleIncrease,omitempty"`
	ObservationPeriod  string  `json:"observationPeriod,omitempty"`
	Healthy            bool    `json:"healthy,omitempty"`
}

// ResourceRecommendationList contains a list of ResourceRecommendation
type ResourceRecommendationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceRecommendation `json:"items"`
}

// Phase constants
const (
	PhasePending    = "Pending"
	PhaseApproved   = "Approved"
	PhaseApplied    = "Applied"
	PhaseRolledBack = "RolledBack"
	PhaseFailed     = "Failed"
	PhaseRejected   = "Rejected"
)

// RiskLevel constants
const (
	RiskLevelLow    = "low"
	RiskLevelMedium = "medium"
	RiskLevelHigh   = "high"
)

// Condition types
const (
	ConditionTypeReady     = "Ready"
	ConditionTypeApproved  = "Approved"
	ConditionTypeApplied   = "Applied"
	ConditionTypeHealthy   = "Healthy"
)
