// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Recommendation represents a resource recommendation from Kubewise.
type Recommendation struct {
	ID                   string    `json:"id"`
	Namespace            string    `json:"namespace"`
	Deployment           string    `json:"deployment"`
	CPURequestMillicores uint32    `json:"cpu_request_millicores"`
	CPULimitMillicores   uint32    `json:"cpu_limit_millicores"`
	MemoryRequestBytes   uint64    `json:"memory_request_bytes"`
	MemoryLimitBytes     uint64    `json:"memory_limit_bytes"`
	Confidence           float32   `json:"confidence"`
	ModelVersion         string    `json:"model_version"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	TimeWindow           string    `json:"time_window,omitempty"`
	PeakHours            *ResourceValues `json:"peak_hours,omitempty"`
	OffPeakHours         *ResourceValues `json:"off_peak_hours,omitempty"`
}

// ResourceValues holds CPU and memory values.
type ResourceValues struct {
	CPURequest    uint32 `json:"cpu_request_millicores"`
	MemoryRequest uint64 `json:"memory_request_bytes"`
}

// RecommendationList is a list of recommendations.
type RecommendationList struct {
	Recommendations []Recommendation `json:"recommendations"`
	Total           int              `json:"total"`
}

// CostAnalysis represents cost analysis from Kubewise.
type CostAnalysis struct {
	Namespace              string    `json:"namespace,omitempty"`
	CurrentMonthlyCost     float64   `json:"current_monthly_cost"`
	RecommendedMonthlyCost float64   `json:"recommended_monthly_cost"`
	PotentialSavings       float64   `json:"potential_savings"`
	Currency               string    `json:"currency"`
	DeploymentCount        int       `json:"deployment_count"`
	LastUpdated            time.Time `json:"last_updated"`
}

// Anomaly represents an anomaly detected by Kubewise.
type Anomaly struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Namespace   string    `json:"namespace"`
	Deployment  string    `json:"deployment"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AnomalyList is a list of anomalies.
type AnomalyList struct {
	Anomalies []Anomaly `json:"anomalies"`
	Total     int       `json:"total"`
}

// GetRecommendations retrieves all recommendations from Kubewise.
func (h *TestHelper) GetRecommendations(ctx context.Context) ([]Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations", h.config.KubewiseURL)
	return h.getRecommendations(ctx, url)
}

// GetRecommendationsByNamespace retrieves recommendations for a specific namespace.
func (h *TestHelper) GetRecommendationsByNamespace(ctx context.Context, namespace string) ([]Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations/%s", h.config.KubewiseURL, namespace)
	return h.getRecommendations(ctx, url)
}

// GetRecommendation retrieves a specific recommendation.
func (h *TestHelper) GetRecommendation(ctx context.Context, namespace, deployment string) (*Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations/%s/%s", h.config.KubewiseURL, namespace, deployment)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var rec Recommendation
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (h *TestHelper) getRecommendations(ctx context.Context, url string) ([]Recommendation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var list RecommendationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list.Recommendations, nil
}

// GetCosts retrieves cost analysis from Kubewise.
func (h *TestHelper) GetCosts(ctx context.Context) (*CostAnalysis, error) {
	url := fmt.Sprintf("%s/api/v1/costs", h.config.KubewiseURL)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var costs CostAnalysis
	if err := json.NewDecoder(resp.Body).Decode(&costs); err != nil {
		return nil, err
	}
	return &costs, nil
}

// GetCostsByNamespace retrieves cost analysis for a specific namespace.
func (h *TestHelper) GetCostsByNamespace(ctx context.Context, namespace string) (*CostAnalysis, error) {
	url := fmt.Sprintf("%s/api/v1/costs/%s", h.config.KubewiseURL, namespace)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var costs CostAnalysis
	if err := json.NewDecoder(resp.Body).Decode(&costs); err != nil {
		return nil, err
	}
	return &costs, nil
}

// GetAnomalies retrieves anomalies from Kubewise.
func (h *TestHelper) GetAnomalies(ctx context.Context, namespace string) ([]Anomaly, error) {
	url := fmt.Sprintf("%s/api/v1/anomalies?namespace=%s", h.config.KubewiseURL, namespace)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var list AnomalyList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list.Anomalies, nil
}

// WaitForAnomaly waits for an anomaly of the specified type to be detected.
func (h *TestHelper) WaitForAnomaly(ctx context.Context, namespace, deployment, anomalyType string, timeout time.Duration) (*Anomaly, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for anomaly %s on %s/%s", anomalyType, namespace, deployment)
			}

			anomalies, err := h.GetAnomalies(ctx, namespace)
			if err != nil {
				continue // Retry on error
			}

			for _, a := range anomalies {
				if a.Deployment == deployment && a.Type == anomalyType {
					return &a, nil
				}
			}
		}
	}
}
