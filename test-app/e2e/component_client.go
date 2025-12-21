// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ComponentConfig represents configuration for a test component.
type ComponentConfig map[string]interface{}

// ComponentStatus represents the status of a test component.
type ComponentStatus struct {
	Name              string  `json:"name"`
	Ready             bool    `json:"ready"`
	Mode              string  `json:"mode"`
	CPUPercent        float64 `json:"cpu_percent,omitempty"`
	MemoryMB          int     `json:"memory_mb,omitempty"`
	AllocatedMB       int     `json:"allocated_mb,omitempty"`
	CurrentRPS        float64 `json:"current_rps,omitempty"`
	TotalRequests     int64   `json:"total_requests,omitempty"`
	RequestsPerSecond float64 `json:"requests_per_second,omitempty"`
}

// LoadStats represents load generator statistics.
type LoadStats struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	CurrentRPS      float64 `json:"current_rps"`
}

// ValidationReport represents a validation report from metrics-validator.
type ValidationReport struct {
	GeneratedAt           string  `json:"generated_at"`
	TestDuration          string  `json:"test_duration"`
	ScenarioName          string  `json:"scenario_name"`
	OverallCPUAccuracy    float64 `json:"overall_cpu_accuracy"`
	OverallMemAccuracy    float64 `json:"overall_mem_accuracy"`
	ConfidenceCorrelation float64 `json:"confidence_correlation"`
	TotalPredictions      int     `json:"total_predictions"`
	AccuratePredictions   int     `json:"accurate_predictions"`
	AnomalyStats          AnomalyStats `json:"anomaly_stats"`
	CostStats             CostStats    `json:"cost_stats"`
	PassCriteria          []PassCriterion `json:"pass_criteria"`
	OverallPass           bool    `json:"overall_pass"`
}

// AnomalyStats holds anomaly detection statistics.
type AnomalyStats struct {
	Total             int     `json:"total"`
	Detected          int     `json:"detected"`
	FalsePositives    int     `json:"false_positives"`
	FalseNegatives    int     `json:"false_negatives"`
	DetectionRate     float64 `json:"detection_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	FalseNegativeRate float64 `json:"false_negative_rate"`
}

// CostStats holds cost validation statistics.
type CostStats struct {
	TotalValidations int     `json:"total_validations"`
	AverageAccuracy  float64 `json:"average_accuracy"`
	AccurateCount    int     `json:"accurate_count"`
}

// PassCriterion represents a pass/fail criterion.
type PassCriterion struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Pass     bool   `json:"pass"`
}

// ConfigureComponent sends configuration to a component.
func (h *TestHelper) ConfigureComponent(ctx context.Context, component string, config ComponentConfig) error {
	url := h.getComponentURL(component, "/api/v1/config")
	return h.postJSON(ctx, url, config)
}

// GetComponentStatus retrieves the status of a component.
func (h *TestHelper) GetComponentStatus(ctx context.Context, component string) (*ComponentStatus, error) {
	url := h.getComponentURL(component, "/api/v1/status")
	
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

	var status ComponentStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// TriggerSpike triggers an immediate spike on a component.
func (h *TestHelper) TriggerSpike(ctx context.Context, component string, duration string) error {
	url := h.getComponentURL(component, "/api/v1/trigger")
	config := map[string]interface{}{}
	if duration != "" {
		config["duration"] = duration
	}
	return h.postJSON(ctx, url, config)
}

// StartLoadGenerator starts the load generator.
func (h *TestHelper) StartLoadGenerator(ctx context.Context) error {
	url := h.getComponentURL("load-generator", "/api/v1/start")
	return h.postJSON(ctx, url, nil)
}

// StopLoadGenerator stops the load generator.
func (h *TestHelper) StopLoadGenerator(ctx context.Context) error {
	url := h.getComponentURL("load-generator", "/api/v1/stop")
	return h.postJSON(ctx, url, nil)
}

// GetLoadStats retrieves load generator statistics.
func (h *TestHelper) GetLoadStats(ctx context.Context) (*LoadStats, error) {
	url := h.getComponentURL("load-generator", "/api/v1/stats")
	
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

	var stats LoadStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// StartScenario starts a test scenario via pattern controller.
func (h *TestHelper) StartScenario(ctx context.Context, scenarioName string) error {
	url := h.getComponentURL("pattern-controller", "/api/v1/scenarios/start")
	return h.postJSON(ctx, url, map[string]interface{}{"name": scenarioName})
}

// StopScenario stops the current scenario.
func (h *TestHelper) StopScenario(ctx context.Context) error {
	url := h.getComponentURL("pattern-controller", "/api/v1/scenarios/stop")
	return h.postJSON(ctx, url, nil)
}

// SetTimeAcceleration sets the time acceleration factor.
func (h *TestHelper) SetTimeAcceleration(ctx context.Context, factor int) error {
	url := h.getComponentURL("pattern-controller", "/api/v1/time/accelerate")
	return h.postJSON(ctx, url, map[string]interface{}{"factor": factor})
}

// TriggerValidation triggers a validation cycle on metrics-validator.
func (h *TestHelper) TriggerValidation(ctx context.Context) error {
	url := h.getComponentURL("metrics-validator", "/api/v1/validate")
	return h.postJSON(ctx, url, nil)
}

// GetValidationReport retrieves the latest validation report.
func (h *TestHelper) GetValidationReport(ctx context.Context) (*ValidationReport, error) {
	url := h.getComponentURL("metrics-validator", "/api/v1/reports/latest")
	
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

	var report ValidationReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

func (h *TestHelper) postJSON(ctx context.Context, url string, body interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
