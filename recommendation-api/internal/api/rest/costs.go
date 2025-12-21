// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// getClusterCostsHandler returns cluster-wide cost analysis
func getClusterCostsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	if store == nil {
		// Return mock data
		c.JSON(http.StatusOK, CostAnalysis{
			CurrentMonthlyCost:     1500.00,
			RecommendedMonthlyCost: 1050.00,
			PotentialSavings:       450.00,
			Currency:               "USD",
			DeploymentCount:        25,
			LastUpdated:            time.Now(),
		})
		return
	}

	costs, err := store.GetClusterCosts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get cluster costs",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, costs)
}

// getNamespaceCostsHandler returns namespace cost analysis
func getNamespaceCostsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	if store == nil {
		// Return mock data
		c.JSON(http.StatusOK, CostAnalysis{
			Namespace:              namespace,
			CurrentMonthlyCost:     300.00,
			RecommendedMonthlyCost: 210.00,
			PotentialSavings:       90.00,
			Currency:               "USD",
			DeploymentCount:        5,
			LastUpdated:            time.Now(),
		})
		return
	}

	costs, err := store.GetNamespaceCosts(ctx, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get namespace costs",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, costs)
}

// getSavingsHandler returns savings report
func getSavingsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	since := c.DefaultQuery("since", "30d")

	if store == nil {
		// Return mock data
		c.JSON(http.StatusOK, SavingsReport{
			TotalSavings: 1350.00,
			Currency:     "USD",
			Period:       since,
			SavingsByMonth: []MonthlySaving{
				{Month: "2025-12", Savings: 450.00},
				{Month: "2025-11", Savings: 450.00},
				{Month: "2025-10", Savings: 450.00},
			},
			SavingsByTeam: []TeamSaving{
				{Team: "platform", Savings: 500.00},
				{Team: "backend", Savings: 450.00},
				{Team: "frontend", Savings: 400.00},
			},
		})
		return
	}

	report, err := store.GetSavingsReport(ctx, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to get savings report",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, report)
}
