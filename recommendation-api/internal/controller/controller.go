package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Controller manages ResourceRecommendation CRD reconciliation
type Controller struct {
	mu              sync.RWMutex
	recommendations map[string]*ResourceRecommendation
	patchGenerator  *PatchGenerator
	logger          *slog.Logger

	// Callbacks for integration
	onApply    func(ctx context.Context, rec *ResourceRecommendation) error
	onRollback func(ctx context.Context, rec *ResourceRecommendation) error
}

// ControllerConfig holds controller configuration
type ControllerConfig struct {
	Logger     *slog.Logger
	OnApply    func(ctx context.Context, rec *ResourceRecommendation) error
	OnRollback func(ctx context.Context, rec *ResourceRecommendation) error
}

// NewController creates a new Controller
func NewController(cfg ControllerConfig) *Controller {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Controller{
		recommendations: make(map[string]*ResourceRecommendation),
		patchGenerator:  NewPatchGenerator(),
		logger:          logger,
		onApply:         cfg.OnApply,
		onRollback:      cfg.OnRollback,
	}
}

// key generates a unique key for a recommendation
func (c *Controller) key(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// Reconcile handles the reconciliation loop for a ResourceRecommendation
func (c *Controller) Reconcile(ctx context.Context, rec *ResourceRecommendation) error {
	c.logger.Info("reconciling resource recommendation",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"phase", rec.Status.Phase,
	)

	// Generate patch if not already generated
	if rec.Status.GeneratedPatch == "" {
		patch, err := c.patchGenerator.GenerateStrategicMergePatch(rec)
		if err != nil {
			return c.updateStatusFailed(rec, fmt.Sprintf("failed to generate patch: %v", err))
		}
		rec.Status.GeneratedPatch = patch
	}

	// Handle based on current phase
	switch rec.Status.Phase {
	case "", PhasePending:
		return c.reconcilePending(ctx, rec)
	case PhaseApproved:
		return c.reconcileApproved(ctx, rec)
	case PhaseApplied:
		return c.reconcileApplied(ctx, rec)
	case PhaseRolledBack:
		return c.reconcileRolledBack(ctx, rec)
	case PhaseFailed:
		// No action needed for failed state
		return nil
	case PhaseRejected:
		// No action needed for rejected state
		return nil
	default:
		c.logger.Warn("unknown phase", "phase", rec.Status.Phase)
		return nil
	}
}

func (c *Controller) reconcilePending(ctx context.Context, rec *ResourceRecommendation) error {
	// Check if auto-apply is enabled and approval is not required
	if rec.Spec.AutoApply && !rec.Spec.RequiresApproval {
		return c.approve(ctx, rec, "system")
	}

	// Check risk level for high-risk recommendations
	if rec.Spec.RiskLevel == RiskLevelHigh {
		rec.Spec.RequiresApproval = true
		c.setCondition(rec, ConditionTypeApproved, "False", "HighRisk",
			"High-risk recommendation requires manual approval")
	}

	// Update status
	now := time.Now()
	rec.Status.LastUpdated = &now
	rec.Status.Message = "Waiting for approval"

	c.store(rec)
	return nil
}

func (c *Controller) reconcileApproved(ctx context.Context, rec *ResourceRecommendation) error {
	// Apply the recommendation
	if c.onApply != nil {
		if err := c.onApply(ctx, rec); err != nil {
			return c.updateStatusFailed(rec, fmt.Sprintf("failed to apply: %v", err))
		}
	}

	// Update status to applied
	now := time.Now()
	rec.Status.Phase = PhaseApplied
	rec.Status.AppliedAt = &now
	rec.Status.LastUpdated = &now
	rec.Status.Message = "Recommendation applied successfully"

	c.setCondition(rec, ConditionTypeApplied, "True", "Applied",
		"Recommendation has been applied to the workload")

	c.logger.Info("recommendation applied",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"target", rec.Spec.TargetRef.Name,
	)

	c.store(rec)
	return nil
}

func (c *Controller) reconcileApplied(ctx context.Context, rec *ResourceRecommendation) error {
	// Check outcome if available
	if rec.Status.Outcome != nil {
		// Check for OOM kills indicating the recommendation was too aggressive
		if rec.Status.Outcome.OOMKills > 0 {
			c.logger.Warn("OOM kills detected after applying recommendation",
				"namespace", rec.Namespace,
				"name", rec.Name,
				"oomKills", rec.Status.Outcome.OOMKills,
			)

			// Auto-rollback if OOM kills detected
			return c.rollback(ctx, rec, "Auto-rollback due to OOM kills")
		}

		// Check for significant CPU throttle increase
		if rec.Status.Outcome.CPUThrottleIncrease > 20.0 {
			c.logger.Warn("significant CPU throttle increase detected",
				"namespace", rec.Namespace,
				"name", rec.Name,
				"throttleIncrease", rec.Status.Outcome.CPUThrottleIncrease,
			)
		}

		// Mark as healthy if no issues
		if !rec.Status.Outcome.Healthy && rec.Status.Outcome.OOMKills == 0 {
			rec.Status.Outcome.Healthy = true
			c.setCondition(rec, ConditionTypeHealthy, "True", "Healthy",
				"Workload is healthy after applying recommendation")
		}
	}

	c.store(rec)
	return nil
}

func (c *Controller) reconcileRolledBack(ctx context.Context, rec *ResourceRecommendation) error {
	// No further action needed after rollback
	now := time.Now()
	rec.Status.LastUpdated = &now
	c.store(rec)
	return nil
}

// Approve approves a recommendation for application
func (c *Controller) Approve(ctx context.Context, namespace, name, approver string) error {
	rec, err := c.Get(namespace, name)
	if err != nil {
		return err
	}
	return c.approve(ctx, rec, approver)
}

func (c *Controller) approve(ctx context.Context, rec *ResourceRecommendation, approver string) error {
	if rec.Status.Phase != PhasePending && rec.Status.Phase != "" {
		return fmt.Errorf("cannot approve recommendation in phase %s", rec.Status.Phase)
	}

	now := time.Now()
	rec.Status.Phase = PhaseApproved
	rec.Status.ApprovedAt = &now
	rec.Status.ApprovedBy = approver
	rec.Status.LastUpdated = &now
	rec.Status.Message = fmt.Sprintf("Approved by %s", approver)

	c.setCondition(rec, ConditionTypeApproved, "True", "Approved",
		fmt.Sprintf("Approved by %s", approver))

	c.logger.Info("recommendation approved",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"approver", approver,
	)

	c.store(rec)

	// Trigger reconciliation to apply
	return c.Reconcile(ctx, rec)
}

// Reject rejects a recommendation
func (c *Controller) Reject(ctx context.Context, namespace, name, reason string) error {
	rec, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if rec.Status.Phase != PhasePending && rec.Status.Phase != "" {
		return fmt.Errorf("cannot reject recommendation in phase %s", rec.Status.Phase)
	}

	now := time.Now()
	rec.Status.Phase = PhaseRejected
	rec.Status.LastUpdated = &now
	rec.Status.Message = fmt.Sprintf("Rejected: %s", reason)

	c.setCondition(rec, ConditionTypeApproved, "False", "Rejected", reason)

	c.logger.Info("recommendation rejected",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"reason", reason,
	)

	c.store(rec)
	return nil
}

// Rollback rolls back an applied recommendation
func (c *Controller) Rollback(ctx context.Context, namespace, name, reason string) error {
	rec, err := c.Get(namespace, name)
	if err != nil {
		return err
	}
	return c.rollback(ctx, rec, reason)
}

func (c *Controller) rollback(ctx context.Context, rec *ResourceRecommendation, reason string) error {
	if rec.Status.Phase != PhaseApplied {
		return fmt.Errorf("cannot rollback recommendation in phase %s", rec.Status.Phase)
	}

	if rec.Status.PreviousResources == nil {
		return fmt.Errorf("no previous resources stored for rollback")
	}

	// Execute rollback callback
	if c.onRollback != nil {
		if err := c.onRollback(ctx, rec); err != nil {
			return c.updateStatusFailed(rec, fmt.Sprintf("rollback failed: %v", err))
		}
	}

	now := time.Now()
	rec.Status.Phase = PhaseRolledBack
	rec.Status.LastUpdated = &now
	rec.Status.Message = fmt.Sprintf("Rolled back: %s", reason)

	c.setCondition(rec, ConditionTypeApplied, "False", "RolledBack", reason)

	c.logger.Info("recommendation rolled back",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"reason", reason,
	)

	c.store(rec)
	return nil
}

// UpdateOutcome updates the outcome tracking for an applied recommendation
func (c *Controller) UpdateOutcome(namespace, name string, outcome *Outcome) error {
	rec, err := c.Get(namespace, name)
	if err != nil {
		return err
	}

	if rec.Status.Phase != PhaseApplied {
		return fmt.Errorf("can only update outcome for applied recommendations")
	}

	rec.Status.Outcome = outcome
	now := time.Now()
	rec.Status.LastUpdated = &now

	c.store(rec)
	return nil
}

// Create creates a new ResourceRecommendation
func (c *Controller) Create(rec *ResourceRecommendation) error {
	key := c.key(rec.Namespace, rec.Name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.recommendations[key]; exists {
		return fmt.Errorf("recommendation %s already exists", key)
	}

	// Initialize status
	if rec.Status.Phase == "" {
		rec.Status.Phase = PhasePending
	}
	now := time.Now()
	rec.Status.LastUpdated = &now

	// Generate patch
	patch, err := c.patchGenerator.GenerateStrategicMergePatch(rec)
	if err != nil {
		return fmt.Errorf("failed to generate patch: %w", err)
	}
	rec.Status.GeneratedPatch = patch

	c.recommendations[key] = rec
	return nil
}

// Get retrieves a ResourceRecommendation
func (c *Controller) Get(namespace, name string) (*ResourceRecommendation, error) {
	key := c.key(namespace, name)

	c.mu.RLock()
	defer c.mu.RUnlock()

	rec, exists := c.recommendations[key]
	if !exists {
		return nil, fmt.Errorf("recommendation %s not found", key)
	}

	return rec, nil
}

// List lists all ResourceRecommendations, optionally filtered by namespace
func (c *Controller) List(namespace string) []*ResourceRecommendation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*ResourceRecommendation
	for _, rec := range c.recommendations {
		if namespace == "" || rec.Namespace == namespace {
			result = append(result, rec)
		}
	}
	return result
}

// Delete deletes a ResourceRecommendation
func (c *Controller) Delete(namespace, name string) error {
	key := c.key(namespace, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.recommendations[key]; !exists {
		return fmt.Errorf("recommendation %s not found", key)
	}

	delete(c.recommendations, key)
	return nil
}

// store stores a recommendation (internal, assumes lock is held or not needed)
func (c *Controller) store(rec *ResourceRecommendation) {
	key := c.key(rec.Namespace, rec.Name)
	c.mu.Lock()
	c.recommendations[key] = rec
	c.mu.Unlock()
}

// setCondition sets or updates a condition on the recommendation
func (c *Controller) setCondition(rec *ResourceRecommendation, condType, status, reason, message string) {
	now := time.Now()

	// Find existing condition
	for i, cond := range rec.Status.Conditions {
		if cond.Type == condType {
			rec.Status.Conditions[i].Status = status
			rec.Status.Conditions[i].Reason = reason
			rec.Status.Conditions[i].Message = message
			rec.Status.Conditions[i].LastTransitionTime = now
			return
		}
	}

	// Add new condition
	rec.Status.Conditions = append(rec.Status.Conditions, Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

func (c *Controller) updateStatusFailed(rec *ResourceRecommendation, message string) error {
	now := time.Now()
	rec.Status.Phase = PhaseFailed
	rec.Status.LastUpdated = &now
	rec.Status.Message = message

	c.setCondition(rec, ConditionTypeReady, "False", "Failed", message)

	c.logger.Error("recommendation failed",
		"namespace", rec.Namespace,
		"name", rec.Name,
		"message", message,
	)

	c.store(rec)
	return fmt.Errorf("%s", message)
}

// ToJSON serializes a recommendation to JSON
func (c *Controller) ToJSON(rec *ResourceRecommendation) ([]byte, error) {
	return json.MarshalIndent(rec, "", "  ")
}

// FromJSON deserializes a recommendation from JSON
func (c *Controller) FromJSON(data []byte) (*ResourceRecommendation, error) {
	var rec ResourceRecommendation
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
