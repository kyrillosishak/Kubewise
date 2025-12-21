// Package collector provides clients for collecting data from external sources.
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PrometheusClient is a client for querying Prometheus.
type PrometheusClient struct {
	baseURL    string
	httpClient *http.Client
}

// PrometheusResponse represents a Prometheus API response.
type PrometheusResponse struct {
	Status string         `json:"status"`
	Data   PrometheusData `json:"data"`
}

// PrometheusData contains the result data from Prometheus.
type PrometheusData struct {
	ResultType string             `json:"resultType"`
	Result     []PrometheusResult `json:"result"`
}

// PrometheusResult represents a single result from Prometheus.
type PrometheusResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// ResourceUsage represents actual resource usage for a container.
type ResourceUsage struct {
	Namespace     string
	Pod           string
	Container     string
	CPUCores      float64 // CPU usage in cores
	MemoryBytes   uint64  // Memory usage in bytes
	Timestamp     time.Time
}

// NewPrometheusClient creates a new Prometheus client.
func NewPrometheusClient(baseURL string) *PrometheusClient {
	return &PrometheusClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Query executes a PromQL query and returns the results.
func (c *PrometheusClient) Query(ctx context.Context, query string) (*PrometheusResponse, error) {
	u := fmt.Sprintf("%s/api/v1/query", c.baseURL)
	
	params := url.Values{}
	params.Set("query", query)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u+"?"+params.Encode(), nil)
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

	var promResp PrometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: status=%s", promResp.Status)
	}

	return &promResp, nil
}


// GetCPUUsage queries CPU usage for containers in a namespace.
func (c *PrometheusClient) GetCPUUsage(ctx context.Context, namespace string) ([]ResourceUsage, error) {
	// Query for CPU usage rate over 5 minutes
	query := fmt.Sprintf(
		`sum(rate(container_cpu_usage_seconds_total{namespace="%s",container!="",container!="POD"}[5m])) by (namespace, pod, container)`,
		namespace,
	)

	resp, err := c.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying CPU usage: %w", err)
	}

	var results []ResourceUsage
	for _, r := range resp.Data.Result {
		cpuCores, err := parseValue(r.Value)
		if err != nil {
			continue
		}

		results = append(results, ResourceUsage{
			Namespace: r.Metric["namespace"],
			Pod:       r.Metric["pod"],
			Container: r.Metric["container"],
			CPUCores:  cpuCores,
			Timestamp: time.Now(),
		})
	}

	return results, nil
}

// GetMemoryUsage queries memory usage for containers in a namespace.
func (c *PrometheusClient) GetMemoryUsage(ctx context.Context, namespace string) ([]ResourceUsage, error) {
	// Query for current memory usage
	query := fmt.Sprintf(
		`sum(container_memory_working_set_bytes{namespace="%s",container!="",container!="POD"}) by (namespace, pod, container)`,
		namespace,
	)

	resp, err := c.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying memory usage: %w", err)
	}

	var results []ResourceUsage
	for _, r := range resp.Data.Result {
		memBytes, err := parseValue(r.Value)
		if err != nil {
			continue
		}

		results = append(results, ResourceUsage{
			Namespace:   r.Metric["namespace"],
			Pod:         r.Metric["pod"],
			Container:   r.Metric["container"],
			MemoryBytes: uint64(memBytes),
			Timestamp:   time.Now(),
		})
	}

	return results, nil
}

// GetCPUUsageByDeployment queries average CPU usage for a deployment.
func (c *PrometheusClient) GetCPUUsageByDeployment(ctx context.Context, namespace, deployment string) (float64, error) {
	// Query for average CPU usage across all pods of a deployment
	query := fmt.Sprintf(
		`avg(sum(rate(container_cpu_usage_seconds_total{namespace="%s",pod=~"%s-.*",container!="",container!="POD"}[5m])) by (pod))`,
		namespace, deployment,
	)

	resp, err := c.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("querying CPU usage: %w", err)
	}

	if len(resp.Data.Result) == 0 {
		return 0, nil
	}

	return parseValue(resp.Data.Result[0].Value)
}

// GetMemoryUsageByDeployment queries average memory usage for a deployment.
func (c *PrometheusClient) GetMemoryUsageByDeployment(ctx context.Context, namespace, deployment string) (uint64, error) {
	// Query for average memory usage across all pods of a deployment
	query := fmt.Sprintf(
		`avg(sum(container_memory_working_set_bytes{namespace="%s",pod=~"%s-.*",container!="",container!="POD"}) by (pod))`,
		namespace, deployment,
	)

	resp, err := c.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("querying memory usage: %w", err)
	}

	if len(resp.Data.Result) == 0 {
		return 0, nil
	}

	val, err := parseValue(resp.Data.Result[0].Value)
	if err != nil {
		return 0, err
	}

	return uint64(val), nil
}

// GetResourceUsageByDeployment queries both CPU and memory usage for a deployment.
func (c *PrometheusClient) GetResourceUsageByDeployment(ctx context.Context, namespace, deployment string) (*ResourceUsage, error) {
	cpu, err := c.GetCPUUsageByDeployment(ctx, namespace, deployment)
	if err != nil {
		return nil, fmt.Errorf("getting CPU usage: %w", err)
	}

	mem, err := c.GetMemoryUsageByDeployment(ctx, namespace, deployment)
	if err != nil {
		return nil, fmt.Errorf("getting memory usage: %w", err)
	}

	return &ResourceUsage{
		Namespace:   namespace,
		CPUCores:    cpu,
		MemoryBytes: mem,
		Timestamp:   time.Now(),
	}, nil
}

// HealthCheck checks if Prometheus is healthy.
func (c *PrometheusClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/-/healthy", c.baseURL)
	
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

// parseValue extracts the float64 value from a Prometheus result value.
func parseValue(value []interface{}) (float64, error) {
	if len(value) < 2 {
		return 0, fmt.Errorf("invalid value format")
	}

	strVal, ok := value[1].(string)
	if !ok {
		return 0, fmt.Errorf("value is not a string")
	}

	return strconv.ParseFloat(strVal, 64)
}
