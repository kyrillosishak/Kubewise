// Package rest provides REST API handlers
package rest

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up all REST API routes
func RegisterRoutes(r *gin.Engine) {
	// Health endpoints
	r.GET("/healthz", healthzHandler)
	r.GET("/readyz", readyzHandler)

	// Auth endpoints (public)
	auth := r.Group("/api/auth")
	{
		auth.POST("/login", loginHandler)
	}

	// API v1 routes (protected)
	v1 := r.Group("/api/v1")
	v1.Use(AuthMiddleware())
	{
		// Auth - get current user
		v1.GET("/auth/me", meHandler)

		// Recommendations - list and namespace-scoped
		recommendations := v1.Group("/recommendations")
		{
			recommendations.GET("", listRecommendationsHandler)
			recommendations.GET("/:namespace", listNamespaceRecommendationsHandler)
			recommendations.GET("/:namespace/:name", getRecommendationHandler)
		}

		// Recommendation actions - by ID
		recActions := v1.Group("/recommendation")
		{
			recActions.POST("/:id/apply", applyRecommendationHandler)
			recActions.POST("/:id/approve", approveRecommendationHandler)
			recActions.POST("/:id/dry-run", dryRunRecommendationHandler)
			recActions.GET("/:id/approval-history", getApprovalHistoryHandler)
			recActions.GET("/:id/outcome", getRecommendationOutcomeHandler)
		}

		// Costs
		costs := v1.Group("/costs")
		{
			costs.GET("", getClusterCostsHandler)
			costs.GET("/:namespace", getNamespaceCostsHandler)
		}

		// Savings
		v1.GET("/savings", getSavingsHandler)

		// Clusters
		clusters := v1.Group("/clusters")
		{
			clusters.GET("", listClustersHandler)
			clusters.GET("/:id/health", getClusterHealthHandler)
		}

		// Anomalies
		anomalies := v1.Group("/anomalies")
		{
			anomalies.GET("", listAnomaliesHandler)
			anomalies.GET("/:id", getAnomalyDetailHandler)
		}

		// Models
		models := v1.Group("/models")
		{
			models.GET("", listModelsHandler)
			models.GET("/:version", getModelHandler)
			models.POST("/rollback/:version", rollbackModelHandler)
		}

		// Safety configuration
		safety := v1.Group("/safety")
		{
			safety.GET("/config", listNamespaceConfigsHandler)
			safety.GET("/config/:namespace", getNamespaceConfigHandler)
			safety.PUT("/config/:namespace", updateNamespaceConfigHandler)
			safety.GET("/rollbacks", listRollbackEventsHandler)
		}

		// Debug
		debug := v1.Group("/debug")
		{
			debug.GET("/predictions/:deployment", getPredictionHistoryHandler)
		}

		// Audit (requires admin)
		audit := v1.Group("/audit")
		{
			audit.GET("/logs", getAuditLogsHandler)
		}
	}
}
