// Package collector provides clients for collecting data from external sources.
package collector

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
	TimeWindow           string    `json:"time_window"`
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

// SavingsReport represents savings over time.
type SavingsReport struct {
	TotalSavings   float64         `json:"total_savings"`
	Currency       string          `json:"currency"`
	Period         string          `json:"period"`
	SavingsByMonth []MonthlySaving `json:"savings_by_month"`
}

// MonthlySaving represents savings for a specific month.
type MonthlySaving struct {
	Month   string  `json:"month"`
	Savings float64 `json:"savings"`
}


// KubewiseClient is a client for the Kubewise API.
type KubewiseClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewKubewiseClient creates a new Kubewise API client.
func NewKubewiseClient(baseURL string) *KubewiseClient {
	return &KubewiseClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetRecommendations retrieves all recommendations from Kubewise.
func (c *KubewiseClient) GetRecommendations(ctx context.Context) ([]Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var list RecommendationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return list.Recommendations, nil
}

// GetRecommendationsByNamespace retrieves recommendations for a specific namespace.
func (c *KubewiseClient) GetRecommendationsByNamespace(ctx context.Context, namespace string) ([]Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations/%s", c.baseURL, namespace)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var list RecommendationList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return list.Recommendations, nil
}

// GetRecommendation retrieves a specific recommendation by namespace and deployment.
func (c *KubewiseClient) GetRecommendation(ctx context.Context, namespace, deployment string) (*Recommendation, error) {
	url := fmt.Sprintf("%s/api/v1/recommendations/%s/%s", c.baseURL, namespace, deployment)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
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
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &rec, nil
}

// GetCosts retrieves cluster-wide cost analysis.
func (c *KubewiseClient) GetCosts(ctx context.Context) (*CostAnalysis, error) {
	url := fmt.Sprintf("%s/api/v1/costs", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var costs CostAnalysis
	if err := json.NewDecoder(resp.Body).Decode(&costs); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &costs, nil
}

// GetCostsByNamespace retrieves cost analysis for a specific namespace.
func (c *KubewiseClient) GetCostsByNamespace(ctx context.Context, namespace string) (*CostAnalysis, error) {
	url := fmt.Sprintf("%s/api/v1/costs/%s", c.baseURL, namespace)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var costs CostAnalysis
	if err := json.NewDecoder(resp.Body).Decode(&costs); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &costs, nil
}

// GetSavings retrieves the savings report.
func (c *KubewiseClient) GetSavings(ctx context.Context, since string) (*SavingsReport, error) {
	url := fmt.Sprintf("%s/api/v1/savings?since=%s", c.baseURL, since)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var savings SavingsReport
	if err := json.NewDecoder(resp.Body).Decode(&savings); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &savings, nil
}

// HealthCheck checks if the Kubewise API is healthy.
func (c *KubewiseClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
