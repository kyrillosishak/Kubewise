// Package storage provides data persistence implementations
package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
)

// InMemoryClusterStore provides an in-memory cluster store
type InMemoryClusterStore struct {
	clusters map[string]*rest.Cluster
	mu       sync.RWMutex
}

// NewInMemoryClusterStore creates a new in-memory cluster store
func NewInMemoryClusterStore() *InMemoryClusterStore {
	return &InMemoryClusterStore{
		clusters: make(map[string]*rest.Cluster),
	}
}

// ListClusters returns all clusters
func (s *InMemoryClusterStore) ListClusters(ctx context.Context) ([]rest.Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clusters := make([]rest.Cluster, 0, len(s.clusters))
	for _, c := range s.clusters {
		clusters = append(clusters, *c)
	}
	return clusters, nil
}

// GetClusterHealth returns health details for a cluster
func (s *InMemoryClusterStore) GetClusterHealth(ctx context.Context, clusterID string) (*rest.ClusterHealth, error) {
	s.mu.RLock()
	cluster, exists := s.clusters[clusterID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("cluster not found")
	}

	return &rest.ClusterHealth{
		Cluster: *cluster,
		Agents:  []rest.AgentStatus{},
		Metrics: rest.ClusterMetrics{
			CPUUtilization:    0,
			MemoryUtilization: 0,
			PodCount:          0,
			NodeCount:         0,
		},
	}, nil
}

// RegisterCluster adds or updates a cluster
func (s *InMemoryClusterStore) RegisterCluster(ctx context.Context, cluster *rest.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster.LastSeen = time.Now()
	s.clusters[cluster.ID] = cluster
	return nil
}

// UpdateClusterStatus updates a cluster's status
func (s *InMemoryClusterStore) UpdateClusterStatus(ctx context.Context, clusterID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cluster, exists := s.clusters[clusterID]; exists {
		cluster.Status = status
		cluster.LastSeen = time.Now()
	}
	return nil
}
