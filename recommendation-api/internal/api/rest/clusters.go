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
	Cluster Cluster       `json:"cluster"`
	Agents  []AgentStatus `json:"agents"`
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
