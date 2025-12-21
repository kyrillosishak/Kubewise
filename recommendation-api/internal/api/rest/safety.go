// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// getNamespaceConfigHandler returns the safety configuration for a namespace
func getNamespaceConfigHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	if safetyStore == nil {
		// Return default config
		c.JSON(http.StatusOK, NamespaceConfig{
			Namespace:                        namespace,
			DryRunEnabled:                    false,
			AutoApproveEnabled:               false,
			HighRiskThresholdMemoryReduction: 0.30,
			HighRiskThresholdCPUReduction:    0.50,
			CreatedAt:                        time.Now(),
			UpdatedAt:                        time.Now(),
		})
		return
	}

	config, err := safetyStore.GetNamespaceConfig(ctx, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get namespace config",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// updateNamespaceConfigHandler updates the safety configuration for a namespace
func updateNamespaceConfigHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	var req NamespaceConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Code:  "INVALID_REQUEST",
		})
		return
	}

	if safetyStore == nil {
		c.JSON(http.StatusOK, NamespaceConfig{
			Namespace:                        namespace,
			DryRunEnabled:                    req.DryRunEnabled != nil && *req.DryRunEnabled,
			AutoApproveEnabled:               req.AutoApproveEnabled != nil && *req.AutoApproveEnabled,
			HighRiskThresholdMemoryReduction: 0.30,
			HighRiskThresholdCPUReduction:    0.50,
			UpdatedAt:                        time.Now(),
		})
		return
	}

	// Get existing config
	config, err := safetyStore.GetNamespaceConfig(ctx, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get namespace config",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	// Apply updates
	config.Namespace = namespace
	if req.DryRunEnabled != nil {
		config.DryRunEnabled = *req.DryRunEnabled
	}
	if req.AutoApproveEnabled != nil {
		config.AutoApproveEnabled = *req.AutoApproveEnabled
	}
	if req.HighRiskThresholdMemoryReduction != nil {
		config.HighRiskThresholdMemoryReduction = *req.HighRiskThresholdMemoryReduction
	}
	if req.HighRiskThresholdCPUReduction != nil {
		config.HighRiskThresholdCPUReduction = *req.HighRiskThresholdCPUReduction
	}

	if err := safetyStore.SetNamespaceConfig(ctx, config); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to update namespace config",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// listNamespaceConfigsHandler returns all namespace configurations
func listNamespaceConfigsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	if safetyStore == nil {
		c.JSON(http.StatusOK, []NamespaceConfig{
			{
				Namespace:                        "_global",
				DryRunEnabled:                    false,
				AutoApproveEnabled:               false,
				HighRiskThresholdMemoryReduction: 0.30,
				HighRiskThresholdCPUReduction:    0.50,
				CreatedAt:                        time.Now(),
				UpdatedAt:                        time.Now(),
			},
		})
		return
	}

	configs, err := safetyStore.ListNamespaceConfigs(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list namespace configs",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// dryRunRecommendationHandler evaluates a recommendation in dry-run mode
func dryRunRecommendationHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	if safetyStore == nil {
		// Return mock dry-run result
		c.JSON(http.StatusOK, DryRunResult{
			RecommendationID: id,
			Namespace:        "default",
			Deployment:       "example",
			WouldApply:       true,
			Changes: []ResourceChange{
				{
					Resource:      "memory_limit",
					CurrentValue:  "512Mi",
					NewValue:      "256Mi",
					ChangePercent: -50.0,
					IsReduction:   true,
				},
			},
			Warnings: []string{
				"Memory limit reduction of 50.0% may increase OOM risk",
			},
			YamlPatch: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: main
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "256Mi"`,
			EvaluatedAt: time.Now(),
		})
		return
	}

	result, err := safetyStore.EvaluateDryRun(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to evaluate dry-run",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getApprovalHistoryHandler returns approval history for a recommendation
func getApprovalHistoryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	if safetyStore == nil {
		c.JSON(http.StatusOK, []ApprovalHistory{})
		return
	}

	history, err := safetyStore.GetApprovalHistory(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get approval history",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, history)
}

// getRecommendationOutcomeHandler returns the outcome tracking for a recommendation
func getRecommendationOutcomeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	if safetyStore == nil {
		c.JSON(http.StatusOK, RecommendationOutcome{
			ID:                "mock-outcome-id",
			RecommendationID:  id,
			Namespace:         "default",
			Deployment:        "example",
			AppliedAt:         time.Now().Add(-1 * time.Hour),
			CheckTime:         time.Now(),
			OOMKillsBefore:    0,
			OOMKillsAfter:     0,
			CPUThrottleBefore: 0.0,
			CPUThrottleAfter:  0.0,
			OutcomeStatus:     "monitoring",
			RollbackTriggered: false,
		})
		return
	}

	outcome, err := safetyStore.GetRecommendationOutcome(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get recommendation outcome",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	if outcome == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Outcome not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, outcome)
}

// listRollbackEventsHandler returns rollback events
func listRollbackEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Query("namespace")

	if safetyStore == nil {
		c.JSON(http.StatusOK, []RollbackEvent{})
		return
	}

	events, err := safetyStore.ListRollbackEvents(ctx, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list rollback events",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, events)
}
