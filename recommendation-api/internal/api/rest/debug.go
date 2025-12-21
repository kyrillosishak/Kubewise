// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// getPredictionHistoryHandler returns prediction history for a deployment
func getPredictionHistoryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	deployment := c.Param("deployment")

	// Parse namespace from deployment if provided as namespace/deployment
	namespace := ""
	if strings.Contains(deployment, "/") {
		parts := strings.SplitN(deployment, "/", 2)
		namespace = parts[0]
		deployment = parts[1]
	}

	if store == nil {
		// Return mock data
		now := time.Now()
		c.JSON(http.StatusOK, PredictionHistory{
			Deployment: deployment,
			Namespace:  namespace,
			Predictions: []Prediction{
				{
					Timestamp:            now.Add(-5 * time.Minute),
					CpuRequestMillicores: 100,
					CpuLimitMillicores:   500,
					MemoryRequestBytes:   134217728,
					MemoryLimitBytes:     268435456,
					Confidence:           0.92,
					ModelVersion:         "v1.2.0",
				},
				{
					Timestamp:            now.Add(-10 * time.Minute),
					CpuRequestMillicores: 95,
					CpuLimitMillicores:   480,
					MemoryRequestBytes:   130000000,
					MemoryLimitBytes:     260000000,
					Confidence:           0.90,
					ModelVersion:         "v1.2.0",
				},
				{
					Timestamp:            now.Add(-15 * time.Minute),
					CpuRequestMillicores: 105,
					CpuLimitMillicores:   520,
					MemoryRequestBytes:   138000000,
					MemoryLimitBytes:     275000000,
					Confidence:           0.88,
					ModelVersion:         "v1.2.0",
				},
			},
		})
		return
	}

	history, err := store.GetPredictionHistory(ctx, namespace, deployment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get prediction history",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	if history == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Deployment not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, history)
}
