// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// listModelsHandler returns all model versions
func listModelsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	if store == nil {
		// Return mock data
		c.JSON(http.StatusOK, ModelList{
			Models: []ModelVersion{
				{
					Version:            "v1.2.0",
					CreatedAt:          time.Now().Add(-24 * time.Hour),
					ValidationAccuracy: 0.92,
					SizeBytes:          98304,
					IsActive:           true,
					RollbackCount:      0,
				},
				{
					Version:            "v1.1.0",
					CreatedAt:          time.Now().Add(-7 * 24 * time.Hour),
					ValidationAccuracy: 0.89,
					SizeBytes:          95232,
					IsActive:           false,
					RollbackCount:      1,
				},
				{
					Version:            "v1.0.0",
					CreatedAt:          time.Now().Add(-30 * 24 * time.Hour),
					ValidationAccuracy: 0.85,
					SizeBytes:          92160,
					IsActive:           false,
					RollbackCount:      0,
				},
			},
			Total: 3,
		})
		return
	}

	models, err := store.ListModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list models",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, ModelList{
		Models: models,
		Total:  len(models),
	})
}

// getModelHandler returns a specific model version
func getModelHandler(c *gin.Context) {
	ctx := c.Request.Context()
	version := c.Param("version")

	if store == nil {
		// Return mock data
		c.JSON(http.StatusOK, ModelVersion{
			Version:            version,
			CreatedAt:          time.Now().Add(-24 * time.Hour),
			ValidationAccuracy: 0.92,
			SizeBytes:          98304,
			IsActive:           true,
			RollbackCount:      0,
		})
		return
	}

	model, err := store.GetModel(ctx, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get model",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	if model == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Model not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, model)
}

// rollbackModelHandler rolls back to a specific model version
func rollbackModelHandler(c *gin.Context) {
	ctx := c.Request.Context()
	version := c.Param("version")

	if store == nil {
		c.JSON(http.StatusOK, RollbackResponse{
			Version:        version,
			Status:         "rolled_back",
			Message:        "Model rolled back successfully",
			PreviousActive: "v1.2.0",
		})
		return
	}

	response, err := store.RollbackModel(ctx, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to rollback model",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
