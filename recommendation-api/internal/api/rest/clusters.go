// Package rest provides REST API handlers
package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Cluster represents a connected Kubernetes cluster
type Cluster struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Status               string    `json:"status"` // healthy, degraded, disconnected
	ContainersMonitored  int       `json:"containersMonitored"`
	PredictionsGenerated int       `json:"predictionsGenerated"`
	AnomaliesDetected    int       `json:"anomaliesDetected"`
	ModelVersion         string    `json:"modelVersion"`
	LastSeen             time.Time `json:"lastSeen"`
}

// ClusterHealth represents detailed health information for a cluster
type ClusterHealth struct {
	Cluster Cluster        `json:"cluster"`
	Agents  []AgentStatus  `json:"agents"`
	Metrics ClusterMetrics `json:"metrics"`
}

// AgentStatus represents the status of a resource agent on a node
type AgentStatus struct {
	NodeID        string    `json:"nodeId"`
	NodeName      string    `json:"nodeName"`
	Status        string    `json:"status"` // running, stopped, error
	LastHeartbeat time.Time `json:"lastHeartbeat"`
	Version       string    `json:"version"`
}

// ClusterMetrics represents cluster-wide metrics
type ClusterMetrics struct {
	CPUUtilization    float64 `json:"cpuUtilization"`
	MemoryUtilization float64 `json:"memoryUtilization"`
	PodCount          int     `json:"podCount"`
	NodeCount         int     `json:"nodeCount"`
}

// RegisterClusterRequest is the request body for manual cluster registration
type RegisterClusterRequest struct {
	ID           string `json:"id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	ModelVersion string `json:"modelVersion,omitempty"`
}

// listClustersHandler returns all connected clusters
func listClustersHandler(c *gin.Context) {
	clusters, err := getClusterStore().ListClusters(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list clusters",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, clusters)
}

// getClusterHealthHandler returns health details for a specific cluster
func getClusterHealthHandler(c *gin.Context) {
	clusterID := c.Param("id")

	health, err := getClusterStore().GetClusterHealth(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Cluster not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, health)
}

// registerClusterHandler manually registers a cluster (for quick-start without agents)
func registerClusterHandler(c *gin.Context) {
	var req RegisterClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body: id and name are required",
			Code:  "BAD_REQUEST",
		})
		return
	}

	modelVersion := req.ModelVersion
	if modelVersion == "" {
		modelVersion = "v1.0.0"
	}

	cluster := &Cluster{
		ID:                   req.ID,
		Name:                 req.Name,
		Status:               "pending", // Will become healthy when agent connects
		ContainersMonitored:  0,
		PredictionsGenerated: 0,
		AnomaliesDetected:    0,
		ModelVersion:         modelVersion,
		LastSeen:             time.Now(),
	}

	if err := getClusterStore().RegisterCluster(c.Request.Context(), cluster); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to register cluster",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

// deleteClusterHandler removes a cluster registration
func deleteClusterHandler(c *gin.Context) {
	clusterID := c.Param("id")

	if err := getClusterStore().DeleteCluster(c.Request.Context(), clusterID); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Cluster not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cluster deleted"})
}
