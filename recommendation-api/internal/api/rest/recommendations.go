// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// listRecommendationsHandler returns all recommendations
func listRecommendationsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	if store == nil {
		// Return mock data when store is not configured
		c.JSON(http.StatusOK, RecommendationList{
			Recommendations: []Recommendation{},
			Total:           0,
		})
		return
	}

	recommendations, err := store.ListRecommendations(ctx, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list recommendations",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, RecommendationList{
		Recommendations: recommendations,
		Total:           len(recommendations),
	})
}

// listNamespaceRecommendationsHandler returns recommendations for a namespace
func listNamespaceRecommendationsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	if store == nil {
		c.JSON(http.StatusOK, RecommendationList{
			Recommendations: []Recommendation{},
			Total:           0,
		})
		return
	}

	recommendations, err := store.ListRecommendations(ctx, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list recommendations",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, RecommendationList{
		Recommendations: recommendations,
		Total:           len(recommendations),
	})
}

// getRecommendationHandler returns a specific recommendation
func getRecommendationHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	name := c.Param("name")

	if store == nil {
		// Return mock recommendation
		c.JSON(http.StatusOK, Recommendation{
			ID:                   "mock-id",
			Namespace:            namespace,
			Deployment:           name,
			CpuRequestMillicores: 100,
			CpuLimitMillicores:   500,
			MemoryRequestBytes:   134217728, // 128Mi
			MemoryLimitBytes:     268435456, // 256Mi
			Confidence:           0.85,
			ModelVersion:         "v1.0.0",
			Status:               "pending",
			CreatedAt:            time.Now(),
			TimeWindow:           "peak",
		})
		return
	}

	recommendation, err := store.GetRecommendation(ctx, namespace, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get recommendation",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	if recommendation == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Recommendation not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, recommendation)
}

// applyRecommendationHandler applies a recommendation
func applyRecommendationHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var req ApplyRecommendationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to non-dry-run if no body provided
		req.DryRun = false
	}

	if store == nil {
		// Return mock response
		c.JSON(http.StatusOK, ApplyRecommendationResponse{
			ID:      id,
			Status:  "applied",
			Message: "Recommendation applied successfully",
			YamlPatch: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
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
		})
		return
	}

	response, err := store.ApplyRecommendation(ctx, id, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to apply recommendation",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// approveRecommendationHandler approves a recommendation
func approveRecommendationHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default approver if not provided
		req.Approver = "anonymous"
	}

	if store == nil {
		c.JSON(http.StatusOK, ApproveRecommendationResponse{
			ID:       id,
			Status:   "approved",
			Message:  "Recommendation approved",
			Approver: req.Approver,
		})
		return
	}

	// Use safety store if available for detailed approval tracking
	if safetyStore != nil {
		response, err := safetyStore.ApproveRecommendationWithDetails(ctx, id, req.Approver, req.Reason)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error: "Failed to approve recommendation",
				Code:  "INTERNAL_ERROR",
			})
			return
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response, err := store.ApproveRecommendation(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to approve recommendation",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
