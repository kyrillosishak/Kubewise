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

// NewInMemoryAnomalyStore creates a new in-memory anomaly store with demo data
func NewInMemoryAnomalyStore() *InMemoryAnomalyStore {
	store := &InMemoryAnomalyStore{
		anomalies: make(map[string]*rest.AnomalyDetail),
	}

	// Add demo anomalies
	now := time.Now()
	demoAnomalies := []*rest.AnomalyDetail{
		{
			Anomaly: rest.Anomaly{
				ID:         "anomaly-001",
				Type:       rest.AnomalyTypeMemoryLeak,
				Severity:   rest.AnomalySeverityWarning,
				Namespace:  "default",
				Deployment: "api-server",
				Container:  "api",
				DetectedAt: now.Add(-2 * time.Hour),
				Status:     "active",
			},
			Metrics: []rest.AnomalyMetric{
				{Timestamp: now.Add(-2 * time.Hour), Value: 512, Threshold: 450},
				{Timestamp: now.Add(-1 * time.Hour), Value: 580, Threshold: 450},
				{Timestamp: now, Value: 620, Threshold: 450},
			},
			RelatedRecommendations: []string{"rec-001"},
		},
		{
			Anomaly: rest.Anomaly{
				ID:         "anomaly-002",
				Type:       rest.AnomalyTypeCPUSpike,
				Severity:   rest.AnomalySeverityCritical,
				Namespace:  "production",
				Deployment: "worker",
				Container:  "processor",
				DetectedAt: now.Add(-30 * time.Minute),
				Status:     "active",
			},
			Metrics: []rest.AnomalyMetric{
				{Timestamp: now.Add(-30 * time.Minute), Value: 95, Threshold: 80},
				{Timestamp: now.Add(-15 * time.Minute), Value: 98, Threshold: 80},
				{Timestamp: now, Value: 92, Threshold: 80},
			},
			RelatedRecommendations: []string{"rec-002"},
		},
		{
			Anomaly: rest.Anomaly{
				ID:         "anomaly-003",
				Type:       rest.AnomalyTypeOOMRisk,
				Severity:   rest.AnomalySeverityCritical,
				Namespace:  "kubewise-test",
				Deployment: "memory-hog",
				Container:  "memory-hog",
				DetectedAt: now.Add(-10 * time.Minute),
				Status:     "active",
			},
			Metrics: []rest.AnomalyMetric{
				{Timestamp: now.Add(-10 * time.Minute), Value: 890, Threshold: 800},
				{Timestamp: now.Add(-5 * time.Minute), Value: 920, Threshold: 800},
				{Timestamp: now, Value: 950, Threshold: 800},
			},
			RelatedRecommendations: []string{},
		},
	}

	for _, a := range demoAnomalies {
		store.anomalies[a.ID] = a
	}

	return store
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
