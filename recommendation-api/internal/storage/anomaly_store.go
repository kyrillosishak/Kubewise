// Package storage provides data persistence implementations
package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
)

// InMemoryAnomalyStore provides an in-memory anomaly store
type InMemoryAnomalyStore struct {
	anomalies map[string]*rest.AnomalyDetail
	mu        sync.RWMutex
}

// NewInMemoryAnomalyStore creates a new in-memory anomaly store
func NewInMemoryAnomalyStore() *InMemoryAnomalyStore {
	return &InMemoryAnomalyStore{
		anomalies: make(map[string]*rest.AnomalyDetail),
	}
}

// ListAnomalies returns anomalies matching the filters
func (s *InMemoryAnomalyStore) ListAnomalies(ctx context.Context, filters rest.AnomalyFilters) ([]rest.Anomaly, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	anomalies := make([]rest.Anomaly, 0)
	for _, a := range s.anomalies {
		// Apply filters
		if filters.Type != "" && a.Type != filters.Type {
			continue
		}
		if filters.Severity != "" && a.Severity != filters.Severity {
			continue
		}
		if filters.Namespace != "" && a.Namespace != filters.Namespace {
			continue
		}
		anomalies = append(anomalies, a.Anomaly)
	}
	return anomalies, nil
}

// GetAnomalyDetail returns detailed information about an anomaly
func (s *InMemoryAnomalyStore) GetAnomalyDetail(ctx context.Context, anomalyID string) (*rest.AnomalyDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	anomaly, exists := s.anomalies[anomalyID]
	if !exists {
		return nil, errors.New("anomaly not found")
	}
	return anomaly, nil
}

// RecordAnomaly adds a new anomaly
func (s *InMemoryAnomalyStore) RecordAnomaly(ctx context.Context, anomaly *rest.AnomalyDetail) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	anomaly.DetectedAt = time.Now()
	s.anomalies[anomaly.ID] = anomaly
	return nil
}

// ResolveAnomaly marks an anomaly as resolved
func (s *InMemoryAnomalyStore) ResolveAnomaly(ctx context.Context, anomalyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if anomaly, exists := s.anomalies[anomalyID]; exists {
		now := time.Now()
		anomaly.ResolvedAt = &now
		anomaly.Status = "resolved"
	}
	return nil
}
