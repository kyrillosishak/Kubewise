// Package rest provides REST API handlers
package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthzHandler returns health status
func healthzHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// readyzHandler returns readiness status
func readyzHandler(c *gin.Context) {
	// TODO: Add actual readiness checks (database, etc.)
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
