// Package api provides HTTP handlers for the steady-worker component.
package api

import (
	"net/http"
	"time"

	"github.com/container-resource-predictor/test-app/internal/steadyworker/workload"
	"github.com/container-resource-predictor/test-app/pkg/common"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for steady-worker API.
type Handler struct {
	workload *workload.Workload
	metrics  *common.Metrics
}

// NewHandler creates a new Handler with the given workload.
func NewHandler(w *workload.Workload) *Handler {
	return &Handler{workload: w}
}

// SetMetrics sets the metrics instance for tracking request latency and throughput.
func (h *Handler) SetMetrics(m *common.Metrics) {
	h.metrics = m
}

// ConfigRequest represents a configuration update request.
type ConfigRequest struct {
	CPUWorkMs       int `json:"cpuWorkMs,omitempty"`
	MemoryAllocKB   int `json:"memoryAllocKB,omitempty"`
	MemoryHoldMs    int `json:"memoryHoldMs,omitempty"`
	ResponseDelayMs int `json:"responseDelayMs,omitempty"`
	BaseMemoryMB    int `json:"baseMemoryMB,omitempty"`
}

// ConfigResponse represents the current configuration.
type ConfigResponse struct {
	CPUWorkMs       int `json:"cpuWorkMs"`
	MemoryAllocKB   int `json:"memoryAllocKB"`
	MemoryHoldMs    int `json:"memoryHoldMs"`
	ResponseDelayMs int `json:"responseDelayMs"`
	BaseMemoryMB    int `json:"baseMemoryMB"`
}

// StatusResponse represents the current status.
type StatusResponse struct {
	Ready           bool    `json:"ready"`
	TotalRequests   int64   `json:"totalRequests"`
	ActiveRequests  int32   `json:"activeRequests"`
	CurrentMemoryKB int64   `json:"currentMemoryKB"`
	AvgCPUTimeMs    float64 `json:"avgCpuTimeMs"`
	AvgMemoryKB     float64 `json:"avgMemoryKB"`
	Timestamp       string  `json:"timestamp"`
}

// WorkResponse represents the response from the /work endpoint.
type WorkResponse struct {
	Status     string `json:"status"`
	CPUTimeMs  int64  `json:"cpuTimeMs"`
	MemoryKB   int64  `json:"memoryKB"`
	Timestamp  string `json:"timestamp"`
}

// GetConfig handles GET /api/v1/config
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.workload.GetConfig()
	c.JSON(http.StatusOK, ConfigResponse{
		CPUWorkMs:       cfg.CPUWorkMs,
		MemoryAllocKB:   cfg.MemoryAllocKB,
		MemoryHoldMs:    cfg.MemoryHoldMs,
		ResponseDelayMs: cfg.ResponseDelayMs,
		BaseMemoryMB:    cfg.BaseMemoryMB,
	})
}


// PostConfig handles POST /api/v1/config
func (h *Handler) PostConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.workload.GetConfig()

	// Update fields if provided (use -1 to explicitly set to 0)
	if req.CPUWorkMs > 0 {
		cfg.CPUWorkMs = req.CPUWorkMs
	}
	if req.MemoryAllocKB > 0 {
		cfg.MemoryAllocKB = req.MemoryAllocKB
	}
	if req.MemoryHoldMs > 0 {
		cfg.MemoryHoldMs = req.MemoryHoldMs
	}
	if req.ResponseDelayMs >= 0 {
		cfg.ResponseDelayMs = req.ResponseDelayMs
	}
	if req.BaseMemoryMB > 0 {
		cfg.BaseMemoryMB = req.BaseMemoryMB
	}

	if err := h.workload.SetConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "configuration updated",
		"config": ConfigResponse{
			CPUWorkMs:       cfg.CPUWorkMs,
			MemoryAllocKB:   cfg.MemoryAllocKB,
			MemoryHoldMs:    cfg.MemoryHoldMs,
			ResponseDelayMs: cfg.ResponseDelayMs,
			BaseMemoryMB:    cfg.BaseMemoryMB,
		},
	})
}

// GetStatus handles GET /api/v1/status
func (h *Handler) GetStatus(c *gin.Context) {
	stats := h.workload.GetStats()
	c.JSON(http.StatusOK, StatusResponse{
		Ready:           true,
		TotalRequests:   stats.TotalRequests,
		ActiveRequests:  stats.ActiveRequests,
		CurrentMemoryKB: stats.CurrentMemoryKB,
		AvgCPUTimeMs:    stats.AvgCPUTimeMs,
		AvgMemoryKB:     stats.AvgMemoryKB,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleWork handles GET/POST /work - performs configurable CPU and memory work
func (h *Handler) HandleWork(c *gin.Context) {
	start := time.Now()

	cpuTimeMs, memoryKB := h.workload.DoWork()

	// Record metrics
	if h.metrics != nil {
		h.metrics.RequestsTotal.Inc()
		h.metrics.RequestLatency.Observe(time.Since(start).Seconds())
		h.metrics.MemoryUsage.Set(float64(h.workload.CurrentMemoryBytes()))
	}

	c.JSON(http.StatusOK, WorkResponse{
		Status:    "OK",
		CPUTimeMs: cpuTimeMs,
		MemoryKB:  memoryKB,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// RegisterRoutes registers all steady-worker API routes.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	// Work endpoint - the main endpoint for load testing
	router.GET("/work", h.HandleWork)
	router.POST("/work", h.HandleWork)

	// API endpoints for configuration and status
	api := router.Group("/api/v1")
	{
		api.GET("/config", h.GetConfig)
		api.POST("/config", h.PostConfig)
		api.GET("/status", h.GetStatus)
	}
}
