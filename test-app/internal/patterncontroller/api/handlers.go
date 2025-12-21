// Package api provides HTTP handlers for the pattern-controller component.
package api

import (
	"net/http"
	"time"

	"github.com/container-resource-predictor/test-app/internal/patterncontroller/client"
	"github.com/container-resource-predictor/test-app/internal/patterncontroller/scheduler"
	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for pattern-controller API.
type Handler struct {
	scheduler *scheduler.Scheduler
	client    *client.ComponentClient
}

// NewHandler creates a new Handler.
func NewHandler(sched *scheduler.Scheduler, cli *client.ComponentClient) *Handler {
	return &Handler{
		scheduler: sched,
		client:    cli,
	}
}

// --- Scenario Endpoints ---

// StartScenarioRequest represents a request to start a scenario.
type StartScenarioRequest struct {
	Name               string  `json:"name" binding:"required"`
	TimeAcceleration   float64 `json:"timeAcceleration,omitempty"`
}

// ScenarioResponse represents a scenario response.
type ScenarioResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Duration    string   `json:"duration"`
	Phases      int      `json:"phases"`
}

// PostScenariosStart handles POST /api/v1/scenarios/start
func (h *Handler) PostScenariosStart(c *gin.Context) {
	var req StartScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set time acceleration if provided
	if req.TimeAcceleration > 0 {
		if err := h.scheduler.SetTimeAcceleration(req.TimeAcceleration); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Start the scenario
	if err := h.scheduler.StartScenario(c.Request.Context(), scheduler.ScenarioName(req.Name)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "scenario started",
		"scenario":  req.Name,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// PostScenariosStop handles POST /api/v1/scenarios/stop
func (h *Handler) PostScenariosStop(c *gin.Context) {
	if err := h.scheduler.StopScenario(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "scenario stopped",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetScenariosStatus handles GET /api/v1/scenarios/status
func (h *Handler) GetScenariosStatus(c *gin.Context) {
	status := h.scheduler.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"name":          status.Name,
		"running":       status.Running,
		"startedAt":     status.StartedAt.Format(time.RFC3339),
		"elapsedTime":   status.ElapsedTime.String(),
		"simulatedTime": status.SimulatedTime.String(),
		"currentPhase":  status.CurrentPhase,
		"totalPhases":   status.TotalPhases,
		"progress":      status.Progress,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

// GetScenariosList handles GET /api/v1/scenarios/list
func (h *Handler) GetScenariosList(c *gin.Context) {
	scenarios := make([]ScenarioResponse, 0)
	for _, name := range scheduler.ListScenarios() {
		if s, ok := scheduler.GetScenario(name); ok {
			scenarios = append(scenarios, ScenarioResponse{
				Name:        string(s.Name),
				Description: s.Description,
				Duration:    s.Duration.String(),
				Phases:      len(s.Phases),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"scenarios": scenarios,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetScenariosEvents handles GET /api/v1/scenarios/events
func (h *Handler) GetScenariosEvents(c *gin.Context) {
	events := h.scheduler.GetEvents()
	c.JSON(http.StatusOK, gin.H{
		"events":    events,
		"count":     len(events),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Component Endpoints ---

// ComponentConfigRequest represents a component configuration request.
type ComponentConfigRequest struct {
	Mode   string                 `json:"mode,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// PostComponentConfig handles POST /api/v1/components/:name/config
func (h *Handler) PostComponentConfig(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component name is required"})
		return
	}

	var req ComponentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build config map
	config := make(map[string]interface{})
	if req.Mode != "" {
		config["mode"] = req.Mode
	}
	for k, v := range req.Config {
		config[k] = v
	}

	if err := h.client.Configure(c.Request.Context(), name, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "component configured",
		"component": name,
		"config":    config,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetComponentConfig handles GET /api/v1/components/:name/config
func (h *Handler) GetComponentConfig(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component name is required"})
		return
	}

	config, err := h.client.GetConfig(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"component": name,
		"config":    config,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetComponentStatus handles GET /api/v1/components/:name/status
func (h *Handler) GetComponentStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component name is required"})
		return
	}

	status, err := h.client.GetStatus(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"component": name,
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetComponents handles GET /api/v1/components
func (h *Handler) GetComponents(c *gin.Context) {
	components := h.client.ListComponents()
	statuses := make(map[string]interface{})

	for _, name := range components {
		status, err := h.client.GetStatus(c.Request.Context(), name)
		if err != nil {
			statuses[name] = map[string]interface{}{
				"error": err.Error(),
			}
		} else {
			statuses[name] = status
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"components": statuses,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Time Control Endpoints ---

// TimeAccelerateRequest represents a time acceleration request.
type TimeAccelerateRequest struct {
	Factor float64 `json:"factor" binding:"required,gt=0,lte=100"`
}

// PostTimeAccelerate handles POST /api/v1/time/accelerate
func (h *Handler) PostTimeAccelerate(c *gin.Context) {
	var req TimeAccelerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.scheduler.SetTimeAcceleration(req.Factor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "time acceleration updated",
		"factor":    req.Factor,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetTimeCurrent handles GET /api/v1/time/current
func (h *Handler) GetTimeCurrent(c *gin.Context) {
	timeInfo := h.scheduler.GetSimulatedTime()
	c.JSON(http.StatusOK, timeInfo)
}

// RegisterRoutes registers all pattern-controller API routes.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Scenario endpoints
		scenarios := api.Group("/scenarios")
		{
			scenarios.POST("/start", h.PostScenariosStart)
			scenarios.POST("/stop", h.PostScenariosStop)
			scenarios.GET("/status", h.GetScenariosStatus)
			scenarios.GET("/list", h.GetScenariosList)
			scenarios.GET("/events", h.GetScenariosEvents)
		}

		// Component endpoints
		components := api.Group("/components")
		{
			components.GET("", h.GetComponents)
			components.POST("/:name/config", h.PostComponentConfig)
			components.GET("/:name/config", h.GetComponentConfig)
			components.GET("/:name/status", h.GetComponentStatus)
		}

		// Time control endpoints
		timeCtrl := api.Group("/time")
		{
			timeCtrl.POST("/accelerate", h.PostTimeAccelerate)
			timeCtrl.GET("/current", h.GetTimeCurrent)
		}
	}
}
