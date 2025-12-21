// Package api provides HTTP handlers for the load-generator component.
package api

import (
	"net/http"
	"time"

	"github.com/container-resource-predictor/test-app/internal/loadgen/generator"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for load-generator API.
type Handler struct {
	generator *generator.Generator
}

// NewHandler creates a new Handler with the given generator.
func NewHandler(g *generator.Generator) *Handler {
	return &Handler{generator: g}
}

// ConfigRequest represents a configuration update request.
type ConfigRequest struct {
	Mode          string `json:"mode"`
	TargetURL     string `json:"targetURL,omitempty"`
	RPS           int    `json:"rps,omitempty"`
	RampStartRPS  int    `json:"rampStartRPS,omitempty"`
	RampEndRPS    int    `json:"rampEndRPS,omitempty"`
	RampDuration  string `json:"rampDuration,omitempty"`
	BurstRPS      int    `json:"burstRPS,omitempty"`
	BurstInterval string `json:"burstInterval,omitempty"`
	BurstDuration string `json:"burstDuration,omitempty"`
	PayloadSizeKB int    `json:"payloadSizeKB,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
	Concurrency   int    `json:"concurrency,omitempty"`
}

// ConfigResponse represents the current configuration.
type ConfigResponse struct {
	Mode          string `json:"mode"`
	TargetURL     string `json:"targetURL"`
	RPS           int    `json:"rps"`
	RampStartRPS  int    `json:"rampStartRPS"`
	RampEndRPS    int    `json:"rampEndRPS"`
	RampDuration  string `json:"rampDuration"`
	BurstRPS      int    `json:"burstRPS"`
	BurstInterval string `json:"burstInterval"`
	BurstDuration string `json:"burstDuration"`
	PayloadSizeKB int    `json:"payloadSizeKB"`
	Timeout       string `json:"timeout"`
	Concurrency   int    `json:"concurrency"`
}


// StatusResponse represents the current status.
type StatusResponse struct {
	Running         bool    `json:"running"`
	Mode            string  `json:"mode"`
	TargetURL       string  `json:"targetURL"`
	CurrentRPS      float64 `json:"currentRPS"`
	TargetRPS       int     `json:"targetRPS"`
	TotalRequests   int64   `json:"totalRequests"`
	SuccessRequests int64   `json:"successRequests"`
	FailedRequests  int64   `json:"failedRequests"`
	SuccessRate     float64 `json:"successRate"`
	AvgLatencyMs    float64 `json:"avgLatencyMs"`
	P50LatencyMs    float64 `json:"p50LatencyMs"`
	P95LatencyMs    float64 `json:"p95LatencyMs"`
	P99LatencyMs    float64 `json:"p99LatencyMs"`
	Timestamp       string  `json:"timestamp"`
}

// GetConfig handles GET /api/v1/config
func (h *Handler) GetConfig(c *gin.Context) {
	cfg := h.generator.GetConfig()
	c.JSON(http.StatusOK, ConfigResponse{
		Mode:          string(cfg.Mode),
		TargetURL:     cfg.TargetURL,
		RPS:           cfg.RPS,
		RampStartRPS:  cfg.RampStartRPS,
		RampEndRPS:    cfg.RampEndRPS,
		RampDuration:  cfg.RampDuration.String(),
		BurstRPS:      cfg.BurstRPS,
		BurstInterval: cfg.BurstInterval.String(),
		BurstDuration: cfg.BurstDuration.String(),
		PayloadSizeKB: cfg.PayloadSizeKB,
		Timeout:       cfg.Timeout.String(),
		Concurrency:   cfg.Concurrency,
	})
}

// PostConfig handles POST /api/v1/config
func (h *Handler) PostConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.generator.GetConfig()

	// Update mode if provided
	if req.Mode != "" {
		switch req.Mode {
		case "constant":
			cfg.Mode = generator.ModeConstant
		case "ramp-up":
			cfg.Mode = generator.ModeRampUp
			h.generator.ResetRampTimer()
		case "ramp-down":
			cfg.Mode = generator.ModeRampDown
			h.generator.ResetRampTimer()
		case "burst":
			cfg.Mode = generator.ModeBurst
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be: constant, ramp-up, ramp-down, or burst"})
			return
		}
	}

	// Update other fields if provided
	if req.TargetURL != "" {
		cfg.TargetURL = req.TargetURL
	}
	if req.RPS > 0 {
		cfg.RPS = req.RPS
	}
	if req.RampStartRPS > 0 {
		cfg.RampStartRPS = req.RampStartRPS
	}
	if req.RampEndRPS > 0 {
		cfg.RampEndRPS = req.RampEndRPS
	}
	if req.RampDuration != "" {
		if d, err := time.ParseDuration(req.RampDuration); err == nil {
			cfg.RampDuration = d
		}
	}
	if req.BurstRPS > 0 {
		cfg.BurstRPS = req.BurstRPS
	}
	if req.BurstInterval != "" {
		if d, err := time.ParseDuration(req.BurstInterval); err == nil {
			cfg.BurstInterval = d
		}
	}
	if req.BurstDuration != "" {
		if d, err := time.ParseDuration(req.BurstDuration); err == nil {
			cfg.BurstDuration = d
		}
	}
	if req.PayloadSizeKB > 0 {
		cfg.PayloadSizeKB = req.PayloadSizeKB
	}
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			cfg.Timeout = d
		}
	}
	if req.Concurrency > 0 {
		cfg.Concurrency = req.Concurrency
	}

	if err := h.generator.SetConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "configuration updated",
		"config": ConfigResponse{
			Mode:          string(cfg.Mode),
			TargetURL:     cfg.TargetURL,
			RPS:           cfg.RPS,
			RampStartRPS:  cfg.RampStartRPS,
			RampEndRPS:    cfg.RampEndRPS,
			RampDuration:  cfg.RampDuration.String(),
			BurstRPS:      cfg.BurstRPS,
			BurstInterval: cfg.BurstInterval.String(),
			BurstDuration: cfg.BurstDuration.String(),
			PayloadSizeKB: cfg.PayloadSizeKB,
			Timeout:       cfg.Timeout.String(),
			Concurrency:   cfg.Concurrency,
		},
	})
}


// GetStatus handles GET /api/v1/status
func (h *Handler) GetStatus(c *gin.Context) {
	cfg := h.generator.GetConfig()
	stats := h.generator.GetStats()

	successRate := float64(0)
	if stats.TotalRequests > 0 {
		successRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
	}

	c.JSON(http.StatusOK, StatusResponse{
		Running:         h.generator.IsRunning(),
		Mode:            string(cfg.Mode),
		TargetURL:       cfg.TargetURL,
		CurrentRPS:      stats.CurrentRPS,
		TargetRPS:       cfg.RPS,
		TotalRequests:   stats.TotalRequests,
		SuccessRequests: stats.SuccessRequests,
		FailedRequests:  stats.FailedRequests,
		SuccessRate:     successRate,
		AvgLatencyMs:    stats.AvgLatencyMs,
		P50LatencyMs:    stats.P50LatencyMs,
		P95LatencyMs:    stats.P95LatencyMs,
		P99LatencyMs:    stats.P99LatencyMs,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	})
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.generator.GetStats()
	c.JSON(http.StatusOK, stats)
}

// PostStart handles POST /api/v1/start - starts load generation
func (h *Handler) PostStart(c *gin.Context) {
	if h.generator.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message":   "generator already running",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if err := h.generator.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "generator started",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// PostStop handles POST /api/v1/stop - stops load generation
func (h *Handler) PostStop(c *gin.Context) {
	if !h.generator.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message":   "generator not running",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if err := h.generator.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "generator stopped",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// PostReset handles POST /api/v1/reset - resets statistics
func (h *Handler) PostReset(c *gin.Context) {
	h.generator.ResetStats()
	c.JSON(http.StatusOK, gin.H{
		"message":   "statistics reset",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// RegisterRoutes registers all load-generator API routes.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		api.GET("/config", h.GetConfig)
		api.POST("/config", h.PostConfig)
		api.GET("/status", h.GetStatus)
		api.GET("/stats", h.GetStats)
		api.POST("/start", h.PostStart)
		api.POST("/stop", h.PostStop)
		api.POST("/reset", h.PostReset)
	}
}
