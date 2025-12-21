// Package api provides HTTP API handlers for the metrics validator.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/reporter"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/storage"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/validator"
)

// Handlers provides HTTP handlers for the metrics validator API.
type Handlers struct {
	validator *validator.Validator
	reporter  *reporter.Reporter
	history   *storage.ValidationHistory
	webhook   *reporter.WebhookNotifier
}

// NewHandlers creates new API handlers.
func NewHandlers(v *validator.Validator, r *reporter.Reporter, h *storage.ValidationHistory, w *reporter.WebhookNotifier) *Handlers {
	return &Handlers{
		validator: v,
		reporter:  r,
		history:   h,
		webhook:   w,
	}
}

// RegisterRoutes registers all API routes.
func (h *Handlers) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Validation results
		api.GET("/results", h.getResults)
		api.GET("/results/:component", h.getResultsByComponent)

		// Reports
		api.GET("/reports", h.getReports)
		api.GET("/reports/latest", h.getLatestReport)
		api.POST("/reports/generate", h.generateReport)

		// Anomaly validation
		api.GET("/anomalies", h.getAnomalyValidations)
		api.POST("/anomalies/register", h.registerAnomaly)
		api.POST("/anomalies/:id/detected", h.markAnomalyDetected)
		api.POST("/anomalies/:id/false-positive", h.markFalsePositive)
		api.GET("/anomalies/stats", h.getAnomalyStats)

		// Cost validation
		api.GET("/costs", h.getCostValidations)
		api.GET("/costs/stats", h.getCostStats)

		// History and trends
		api.GET("/history/trend", h.getTrend)

		// Status
		api.GET("/status", h.getStatus)
	}
}

func (h *Handlers) getResults(c *gin.Context) {
	results := h.validator.GetResults()
	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

func (h *Handlers) getResultsByComponent(c *gin.Context) {
	component := c.Param("component")
	results := h.validator.GetResultsByComponent(component)
	c.JSON(http.StatusOK, gin.H{
		"component": component,
		"results":   results,
		"total":     len(results),
	})
}

func (h *Handlers) getReports(c *gin.Context) {
	reports := h.reporter.GetHistory()
	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

func (h *Handlers) getLatestReport(c *gin.Context) {
	report := h.reporter.GetLatestReport()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no reports available"})
		return
	}
	c.JSON(http.StatusOK, report)
}


func (h *Handlers) generateReport(c *gin.Context) {
	report := h.reporter.GenerateReport()
	h.history.AddReport(report)

	// Send webhook notification
	if h.webhook != nil && h.webhook.IsConfigured() {
		go h.webhook.NotifyTestComplete(c.Request.Context(), report)
	}

	c.JSON(http.StatusOK, report)
}

func (h *Handlers) getAnomalyValidations(c *gin.Context) {
	validations := h.validator.GetAnomalyValidations()
	c.JSON(http.StatusOK, gin.H{
		"anomalies": validations,
		"total":     len(validations),
	})
}

type registerAnomalyRequest struct {
	AnomalyType string `json:"anomaly_type" binding:"required"`
	Component   string `json:"component" binding:"required"`
}

func (h *Handlers) registerAnomaly(c *gin.Context) {
	var req registerAnomalyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := h.validator.RegisterAnomaly(req.AnomalyType, req.Component)
	c.JSON(http.StatusOK, gin.H{
		"id":           id,
		"anomaly_type": req.AnomalyType,
		"component":    req.Component,
		"registered":   true,
	})
}

func (h *Handlers) markAnomalyDetected(c *gin.Context) {
	id := c.Param("id")
	h.validator.MarkAnomalyDetected(id)

	validation := h.validator.GetAnomalyValidation(id)
	if validation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "anomaly not found"})
		return
	}

	// Send webhook notification
	if h.webhook != nil && h.webhook.IsConfigured() && validation.WasDetected {
		go h.webhook.NotifyAnomalyDetected(c.Request.Context(), validation.AnomalyType, validation.Component, validation.TimeToDetect)
	}

	c.JSON(http.StatusOK, validation)
}

func (h *Handlers) markFalsePositive(c *gin.Context) {
	id := c.Param("id")
	h.validator.MarkFalsePositive(id)

	validation := h.validator.GetAnomalyValidation(id)
	if validation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "anomaly not found"})
		return
	}

	c.JSON(http.StatusOK, validation)
}

func (h *Handlers) getAnomalyStats(c *gin.Context) {
	stats := h.validator.CalculateAnomalyStats()
	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) getCostValidations(c *gin.Context) {
	validations := h.validator.GetCostValidations()
	c.JSON(http.StatusOK, gin.H{
		"validations": validations,
		"total":       len(validations),
	})
}

func (h *Handlers) getCostStats(c *gin.Context) {
	stats := h.validator.CalculateCostStats()
	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) getTrend(c *gin.Context) {
	durationStr := c.DefaultQuery("duration", "1h")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid duration"})
		return
	}

	trend := h.history.GetTrend(duration)
	c.JSON(http.StatusOK, trend)
}

func (h *Handlers) getStatus(c *gin.Context) {
	results := h.validator.GetResults()
	anomalyStats := h.validator.CalculateAnomalyStats()
	costStats := h.validator.CalculateCostStats()

	var avgCPUAccuracy, avgMemAccuracy float64
	if len(results) > 0 {
		for _, r := range results {
			avgCPUAccuracy += r.CPUAccuracy
			avgMemAccuracy += r.MemoryAccuracy
		}
		avgCPUAccuracy /= float64(len(results))
		avgMemAccuracy /= float64(len(results))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":              "running",
		"total_validations":   len(results),
		"avg_cpu_accuracy":    avgCPUAccuracy * 100,
		"avg_memory_accuracy": avgMemAccuracy * 100,
		"anomaly_stats":       anomalyStats,
		"cost_stats":          costStats,
	})
}
