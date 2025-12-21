// Package api provides HTTP handlers for the memory-hog component.
package api

import (
	"net/http"
	"time"

	"github.com/container-resource-predictor/test-app/internal/memoryhog/allocator"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for memory-hog API.
type Handler struct {
	allocator *allocator.Allocator
}

// NewHandler creates a new Handler with the given allocator.
func NewHandler(alloc *allocator.Allocator) *Handler {
	return &Handler{allocator: alloc}
}

// ConfigRequest represents a configuration update request.
type ConfigRequest struct {
	Mode          string `json:"mode"`
	TargetMB      int    `json:"targetMB,omitempty"`
	LeakRateMBMin int    `json:"leakRateMBMin,omitempty"`
	SpikeSizeMB   int    `json:"spikeSizeMB,omitempty"`
	SpikeInterval string `json:"spikeInterval,omitempty"`
	SpikeDuration string `json:"spikeDuration,omitempty"`
}

// ConfigResponse represents the current configuration.
type ConfigResponse struct {
	Mode          string `json:"mode"`
	TargetMB      int    `json:"targetMB"`
	LeakRateMBMin int    `json:"leakRateMBMin"`
	SpikeSizeMB   int    `json:"spikeSizeMB"`
	SpikeInterval string `json:"spikeInterval"`
	SpikeDuration string `json:"spikeDuration"`
}

// StatusResponse represents the current status.
type StatusResponse struct {
	Running       bool   `json:"running"`
	Paused        bool   `json:"paused"`
	Mode          string `json:"mode"`
	CurrentUsageMB int   `json:"currentUsageMB"`
	CurrentUsageBytes int64 `json:"currentUsageBytes"`
	TargetMB      int    `json:"targetMB"`
	Timestamp     string `json:"timestamp"`
}

// GetConfig handles GET /api/v1/config
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.allocator.GetConfig()
	c.JSON(http.StatusOK, ConfigResponse{
		Mode:          string(cfg.Mode),
		TargetMB:      cfg.TargetMB,
		LeakRateMBMin: cfg.LeakRateMBMin,
		SpikeSizeMB:   cfg.SpikeSizeMB,
		SpikeInterval: cfg.SpikeInterval.String(),
		SpikeDuration: cfg.SpikeDuration.String(),
	})
}


// PostConfig handles POST /api/v1/config
func (h *Handler) PostConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.allocator.GetConfig()

	// Update mode if provided
	if req.Mode != "" {
		switch req.Mode {
		case "steady":
			cfg.Mode = allocator.ModeSteady
		case "leak":
			cfg.Mode = allocator.ModeLeak
			h.allocator.ResetLeakTimer()
		case "spike":
			cfg.Mode = allocator.ModeSpike
			h.allocator.ResetSpikeTimer()
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be: steady, leak, or spike"})
			return
		}
	}

	// Update other fields if provided
	if req.TargetMB > 0 {
		cfg.TargetMB = req.TargetMB
	}
	if req.LeakRateMBMin > 0 {
		cfg.LeakRateMBMin = req.LeakRateMBMin
	}
	if req.SpikeSizeMB > 0 {
		cfg.SpikeSizeMB = req.SpikeSizeMB
	}
	if req.SpikeInterval != "" {
		if d, err := time.ParseDuration(req.SpikeInterval); err == nil {
			cfg.SpikeInterval = d
		}
	}
	if req.SpikeDuration != "" {
		if d, err := time.ParseDuration(req.SpikeDuration); err == nil {
			cfg.SpikeDuration = d
		}
	}

	if err := h.allocator.SetConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "configuration updated",
		"config": ConfigResponse{
			Mode:          string(cfg.Mode),
			TargetMB:      cfg.TargetMB,
			LeakRateMBMin: cfg.LeakRateMBMin,
			SpikeSizeMB:   cfg.SpikeSizeMB,
			SpikeInterval: cfg.SpikeInterval.String(),
			SpikeDuration: cfg.SpikeDuration.String(),
		},
	})
}

// GetStatus handles GET /api/v1/status
func (h *Handler) GetStatus(c *gin.Context) {
	cfg := h.allocator.GetConfig()
	c.JSON(http.StatusOK, StatusResponse{
		Running:          h.allocator.IsRunning(),
		Paused:           h.allocator.IsPaused(),
		Mode:             string(cfg.Mode),
		CurrentUsageMB:   h.allocator.CurrentUsageMB(),
		CurrentUsageBytes: h.allocator.CurrentUsageBytes(),
		TargetMB:         cfg.TargetMB,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	})
}

// PostTrigger handles POST /api/v1/trigger - triggers an immediate spike
func (h *Handler) PostTrigger(c *gin.Context) {
	h.allocator.TriggerSpike()
	c.JSON(http.StatusOK, gin.H{
		"message":   "spike triggered",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// RegisterRoutes registers all memory-hog API routes.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		api.GET("/config", h.GetConfig)
		api.POST("/config", h.PostConfig)
		api.GET("/status", h.GetStatus)
		api.POST("/trigger", h.PostTrigger)
	}
}
