// Package api provides HTTP handlers for the cpu-burster component.
package api

import (
	"net/http"
	"time"

	"github.com/container-resource-predictor/test-app/internal/cpuburster/worker"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for cpu-burster API.
type Handler struct {
	worker *worker.Worker
}

// NewHandler creates a new Handler with the given worker.
func NewHandler(w *worker.Worker) *Handler {
	return &Handler{worker: w}
}

// ConfigRequest represents a configuration update request.
type ConfigRequest struct {
	Mode          string `json:"mode"`
	TargetPercent int    `json:"targetPercent,omitempty"`
	SpikePercent  int    `json:"spikePercent,omitempty"`
	SpikeInterval string `json:"spikeInterval,omitempty"`
	SpikeDuration string `json:"spikeDuration,omitempty"`
	WavePeriod    string `json:"wavePeriod,omitempty"`
	WaveMin       int    `json:"waveMin,omitempty"`
	WaveMax       int    `json:"waveMax,omitempty"`
	Workers       int    `json:"workers,omitempty"`
}

// ConfigResponse represents the current configuration.
type ConfigResponse struct {
	Mode          string `json:"mode"`
	TargetPercent int    `json:"targetPercent"`
	SpikePercent  int    `json:"spikePercent"`
	SpikeInterval string `json:"spikeInterval"`
	SpikeDuration string `json:"spikeDuration"`
	WavePeriod    string `json:"wavePeriod"`
	WaveMin       int    `json:"waveMin"`
	WaveMax       int    `json:"waveMax"`
	Workers       int    `json:"workers"`
}

// StatusResponse represents the current status.
type StatusResponse struct {
	Running            bool    `json:"running"`
	Mode               string  `json:"mode"`
	CurrentUsagePercent float64 `json:"currentUsagePercent"`
	TargetPercent      int     `json:"targetPercent"`
	Timestamp          string  `json:"timestamp"`
}


// GetConfig handles GET /api/v1/config
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.worker.GetConfig()
	c.JSON(http.StatusOK, ConfigResponse{
		Mode:          string(cfg.Mode),
		TargetPercent: cfg.TargetPercent,
		SpikePercent:  cfg.SpikePercent,
		SpikeInterval: cfg.SpikeInterval.String(),
		SpikeDuration: cfg.SpikeDuration.String(),
		WavePeriod:    cfg.WavePeriod.String(),
		WaveMin:       cfg.WaveMin,
		WaveMax:       cfg.WaveMax,
		Workers:       cfg.Workers,
	})
}

// PostConfig handles POST /api/v1/config
func (h *Handler) PostConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.worker.GetConfig()

	// Update mode if provided
	if req.Mode != "" {
		switch req.Mode {
		case "steady":
			cfg.Mode = worker.ModeSteady
		case "spike":
			cfg.Mode = worker.ModeSpike
			h.worker.ResetSpikeTimer()
		case "wave":
			cfg.Mode = worker.ModeWave
			h.worker.ResetWaveTimer()
		case "random":
			cfg.Mode = worker.ModeRandom
			h.worker.ResetRandomTimer()
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be: steady, spike, wave, or random"})
			return
		}
	}

	// Update other fields if provided
	if req.TargetPercent > 0 {
		cfg.TargetPercent = req.TargetPercent
	}
	if req.SpikePercent > 0 {
		cfg.SpikePercent = req.SpikePercent
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
	if req.WavePeriod != "" {
		if d, err := time.ParseDuration(req.WavePeriod); err == nil {
			cfg.WavePeriod = d
		}
	}
	if req.WaveMin > 0 {
		cfg.WaveMin = req.WaveMin
	}
	if req.WaveMax > 0 {
		cfg.WaveMax = req.WaveMax
	}
	if req.Workers > 0 {
		cfg.Workers = req.Workers
	}

	if err := h.worker.SetConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "configuration updated",
		"config": ConfigResponse{
			Mode:          string(cfg.Mode),
			TargetPercent: cfg.TargetPercent,
			SpikePercent:  cfg.SpikePercent,
			SpikeInterval: cfg.SpikeInterval.String(),
			SpikeDuration: cfg.SpikeDuration.String(),
			WavePeriod:    cfg.WavePeriod.String(),
			WaveMin:       cfg.WaveMin,
			WaveMax:       cfg.WaveMax,
			Workers:       cfg.Workers,
		},
	})
}

// GetStatus handles GET /api/v1/status
func (h *Handler) GetStatus(c *gin.Context) {
	cfg := h.worker.GetConfig()
	c.JSON(http.StatusOK, StatusResponse{
		Running:            h.worker.IsRunning(),
		Mode:               string(cfg.Mode),
		CurrentUsagePercent: h.worker.CurrentUsagePercent(),
		TargetPercent:      cfg.TargetPercent,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
	})
}

// PostTrigger handles POST /api/v1/trigger - triggers an immediate CPU spike
func (h *Handler) PostTrigger(c *gin.Context) {
	h.worker.TriggerSpike()
	c.JSON(http.StatusOK, gin.H{
		"message":   "spike triggered",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// RegisterRoutes registers all cpu-burster API routes.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		api.GET("/config", h.GetConfig)
		api.POST("/config", h.PostConfig)
		api.GET("/status", h.GetStatus)
		api.POST("/trigger", h.PostTrigger)
	}
}
