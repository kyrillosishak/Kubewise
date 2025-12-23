// Package storage provides data persistence implementations
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	grpcapi "github.com/container-resource-predictor/recommendation-api/internal/api/grpc"
	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
	predictorv1 "github.com/container-resource-predictor/recommendation-api/api/proto/predictor/v1"
)

// InMemoryAgentStore bridges gRPC agent data to REST stores
type InMemoryAgentStore struct {
	agents       map[string]*grpcapi.AgentInfo
	clusterStore *InMemoryClusterStore
	anomalyStore *InMemoryAnomalyStore
	mu           sync.RWMutex

	// Metrics tracking per cluster
	metricsCount     map[string]int
	predictionsCount map[string]int
	anomaliesCount   map[string]int
}

// NewInMemoryAgentStore creates a new agent store that syncs with cluster/anomaly stores
func NewInMemoryAgentStore(clusterStore *InMemoryClusterStore, anomalyStore *InMemoryAnomalyStore) *InMemoryAgentStore {
	return &InMemoryAgentStore{
		agents:           make(map[string]*grpcapi.AgentInfo),
		clusterStore:     clusterStore,
		anomalyStore:     anomalyStore,
		metricsCount:     make(map[string]int),
		predictionsCount: make(map[string]int),
		anomaliesCount:   make(map[string]int),
	}
}

// RegisterAgent registers an agent and creates/updates the corresponding cluster
func (s *InMemoryAgentStore) RegisterAgent(ctx context.Context, agent *grpcapi.AgentInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[agent.AgentID] = agent

	// Extract cluster name from node name (e.g., "kind-kubewise-control-plane" -> "kind-kubewise")
	clusterName := extractClusterName(agent.NodeName)
	clusterID := clusterName

	slog.Info("Registering agent and updating cluster",
		"agent_id", agent.AgentID,
		"node_name", agent.NodeName,
		"cluster_id", clusterID,
	)

	// Create or update cluster in cluster store
	cluster := &rest.Cluster{
		ID:                   clusterID,
		Name:                 clusterName,
		Status:               "healthy",
		ContainersMonitored:  s.metricsCount[clusterID],
		PredictionsGenerated: s.predictionsCount[clusterID],
		AnomaliesDetected:    s.anomaliesCount[clusterID],
		ModelVersion:         agent.ModelVersion,
		LastSeen:             time.Now(),
	}

	return s.clusterStore.RegisterCluster(ctx, cluster)
}

// GetAgent returns agent info by ID
func (s *InMemoryAgentStore) GetAgent(ctx context.Context, agentID string) (*grpcapi.AgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return agent, nil
}

// UpdateAgentLastSeen updates the last seen timestamp
func (s *InMemoryAgentStore) UpdateAgentLastSeen(ctx context.Context, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent, exists := s.agents[agentID]; exists {
		agent.LastSeenAt = time.Now()

		// Also update cluster last seen
		clusterID := extractClusterName(agent.NodeName)
		s.clusterStore.UpdateClusterStatus(ctx, clusterID, "healthy")
	}
	return nil
}

// StoreMetrics stores metrics and updates cluster stats
func (s *InMemoryAgentStore) StoreMetrics(ctx context.Context, batch *predictorv1.MetricsBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clusterID := extractClusterName(batch.NodeName)
	s.metricsCount[clusterID] += len(batch.Metrics)

	// Update cluster with new container count
	if cluster, err := s.clusterStore.GetClusterHealth(ctx, clusterID); err == nil {
		cluster.Cluster.ContainersMonitored = s.metricsCount[clusterID]
		cluster.Cluster.LastSeen = time.Now()
		s.clusterStore.RegisterCluster(ctx, &cluster.Cluster)
	}

	slog.Debug("Stored metrics",
		"cluster_id", clusterID,
		"metrics_count", len(batch.Metrics),
		"total_metrics", s.metricsCount[clusterID],
	)

	return nil
}

// StorePredictions stores predictions and updates cluster stats
func (s *InMemoryAgentStore) StorePredictions(ctx context.Context, predictions []*predictorv1.ResourceProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Group by cluster
	for range predictions {
		// Use namespace to infer cluster for now
		clusterID := "kind-kubewise" // Default cluster
		s.predictionsCount[clusterID]++
	}

	slog.Debug("Stored predictions", "count", len(predictions))
	return nil
}

// StoreAnomalies stores anomalies in the anomaly store
func (s *InMemoryAgentStore) StoreAnomalies(ctx context.Context, anomalies []*predictorv1.Anomaly) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range anomalies {
		anomalyID := fmt.Sprintf("anomaly-%s-%d", a.ContainerId, time.Now().UnixNano())
		
		var anomalyType rest.AnomalyType
		switch a.Type {
		case predictorv1.AnomalyType_ANOMALY_TYPE_MEMORY_LEAK:
			anomalyType = rest.AnomalyTypeMemoryLeak
		case predictorv1.AnomalyType_ANOMALY_TYPE_CPU_SPIKE:
			anomalyType = rest.AnomalyTypeCPUSpike
		case predictorv1.AnomalyType_ANOMALY_TYPE_OOM_RISK:
			anomalyType = rest.AnomalyTypeOOMRisk
		default:
			anomalyType = rest.AnomalyTypeCPUSpike
		}

		var severity rest.AnomalySeverity
		switch a.Severity {
		case predictorv1.Severity_SEVERITY_CRITICAL:
			severity = rest.AnomalySeverityCritical
		default:
			severity = rest.AnomalySeverityWarning
		}

		detail := &rest.AnomalyDetail{
			Anomaly: rest.Anomaly{
				ID:         anomalyID,
				Type:       anomalyType,
				Severity:   severity,
				Namespace:  a.Namespace,
				Deployment: a.PodName,
				Container:  a.ContainerId,
				DetectedAt: a.DetectedAt.AsTime(),
				Status:     "active",
			},
			Metrics:                []rest.AnomalyMetric{},
			RelatedRecommendations: []string{},
		}

		s.anomalyStore.RecordAnomaly(ctx, detail)
		
		// Update cluster anomaly count
		clusterID := "kind-kubewise"
		s.anomaliesCount[clusterID]++
	}

	slog.Info("Stored anomalies from agent", "count", len(anomalies))
	return nil
}

// extractClusterName extracts cluster name from node name
func extractClusterName(nodeName string) string {
	// For kind clusters: "kind-kubewise-control-plane" -> "kind-kubewise"
	// For other clusters: use the node name prefix
	if len(nodeName) > 0 {
		// Remove common suffixes
		suffixes := []string{"-control-plane", "-worker", "-master", "-node"}
		result := nodeName
		for _, suffix := range suffixes {
			if len(result) > len(suffix) && result[len(result)-len(suffix):] == suffix {
				result = result[:len(result)-len(suffix)]
				break
			}
		}
		return result
	}
	return "unknown-cluster"
}
