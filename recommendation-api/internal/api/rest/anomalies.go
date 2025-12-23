// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AnomalyType represents the type of anomaly detected
type AnomalyType string

const (
	AnomalyTypeMemoryLeak AnomalyType = "memory_leak"
	AnomalyTypeCPUSpike   AnomalyType = "cpu_spike"
	AnomalyTypeOOMRisk    AnomalyType = "oom_risk"
)

// AnomalySeverity represents the severity level of an anomaly
type AnomalySeverity string

const (
	AnomalySeverityWarning  AnomalySeverity = "warning"
	AnomalySeverityCritical AnomalySeverity = "critical"
)

// Anomaly represents a detected resource anomaly
type Anomaly struct {
	ID         string          `json:"id"`
	Type       AnomalyType     `json:"type"`
	Severity   AnomalySeverity `json:"severity"`
	Namespace  string          `json:"namespace"`
	Deployment string          `json:"deployment"`
	Container  string          `json:"container"`
	DetectedAt time.Time       `json:"detectedAt"`
	ResolvedAt *time.Time      `json:"resolvedAt,omitempty"`
	Status     string          `json:"status"` // active, resolved, acknowledged
}

// AnomalyDetail provides detailed information about an anomaly
type AnomalyDetail struct {
	Anomaly
	Metrics                  []AnomalyMetric `json:"metrics"`
	RelatedRecommendations   []string        `json:"relatedRecommendations"`
}

// AnomalyMetric represents a metric data point for an anomaly
type AnomalyMetric struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
}

// AnomalyFilters represents query filters for anomalies
type AnomalyFilters struct {
	Type      AnomalyType     `form:"type"`
	Severity  AnomalySeverity `form:"severity"`
	Namespace string          `form:"namespace"`
	StartDate string          `form:"startDate"`
	EndDate   string          `form:"endDate"`
}

// listAnomaliesHandler returns anomalies with optional filters
func listAnomaliesHandler(c *gin.Context) {
	var filters AnomalyFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid query parameters",
			Code:  "BAD_REQUEST",
		})
		return
	}

	anomalies, err := getAnomalyStore().ListAnomalies(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list anomalies",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, anomalies)
}

// getAnomalyDetailHandler returns detailed information about a specific anomaly
func getAnomalyDetailHandler(c *gin.Context) {
	anomalyID := c.Param("id")

	detail, err := getAnomalyStore().GetAnomalyDetail(c.Request.Context(), anomalyID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Anomaly not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, detail)
}
