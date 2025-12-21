// Package client provides HTTP clients for communicating with test components.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ComponentEndpoints defines the endpoints for each component.
type ComponentEndpoints struct {
	BaseURL string
}

// ComponentClient manages HTTP communication with test components.
type ComponentClient struct {
	httpClient *http.Client
	endpoints  map[string]ComponentEndpoints
}

// Config holds the client configuration.
type Config struct {
	Timeout    time.Duration
	Components map[string]string // component name -> base URL
}

// DefaultConfig returns a default configuration for local development.
func DefaultConfig() Config {
	return Config{
		Timeout: 10 * time.Second,
		Components: map[string]string{
			"memory-hog":     "http://memory-hog-svc:8082",
			"cpu-burster":    "http://cpu-burster-svc:8083",
			"steady-worker":  "http://steady-worker-svc:8084",
			"load-generator": "http://load-generator-svc:8081",
		},
	}
}

// New creates a new ComponentClient.
func New(cfg Config) *ComponentClient {
	endpoints := make(map[string]ComponentEndpoints)
	for name, url := range cfg.Components {
		endpoints[name] = ComponentEndpoints{BaseURL: url}
	}

	return &ComponentClient{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		endpoints: endpoints,
	}
}

// SetEndpoint sets or updates an endpoint for a component.
func (c *ComponentClient) SetEndpoint(name, baseURL string) {
	c.endpoints[name] = ComponentEndpoints{BaseURL: baseURL}
}

// ListComponents returns the list of known component names.
func (c *ComponentClient) ListComponents() []string {
	names := make([]string, 0, len(c.endpoints))
	for name := range c.endpoints {
		names = append(names, name)
	}
	return names
}

// Configure sends a configuration update to a component.
func (c *ComponentClient) Configure(ctx context.Context, name string, config map[string]interface{}) error {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/config", endpoint.BaseURL)
	return c.postJSON(ctx, url, config)
}

// Start sends a start command to a component (for load-generator).
func (c *ComponentClient) Start(ctx context.Context, name string) error {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/start", endpoint.BaseURL)
	return c.postJSON(ctx, url, nil)
}

// Stop sends a stop command to a component (for load-generator).
func (c *ComponentClient) Stop(ctx context.Context, name string) error {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/stop", endpoint.BaseURL)
	return c.postJSON(ctx, url, nil)
}

// GetStatus retrieves the status of a component.
func (c *ComponentClient) GetStatus(ctx context.Context, name string) (map[string]interface{}, error) {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return nil, fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/status", endpoint.BaseURL)
	return c.getJSON(ctx, url)
}

// GetConfig retrieves the configuration of a component.
func (c *ComponentClient) GetConfig(ctx context.Context, name string) (map[string]interface{}, error) {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return nil, fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/config", endpoint.BaseURL)
	return c.getJSON(ctx, url)
}

// TriggerSpike triggers an immediate spike on a component.
func (c *ComponentClient) TriggerSpike(ctx context.Context, name string) error {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/api/v1/trigger", endpoint.BaseURL)
	return c.postJSON(ctx, url, nil)
}

// Health checks the health of a component.
func (c *ComponentClient) Health(ctx context.Context, name string) error {
	endpoint, ok := c.endpoints[name]
	if !ok {
		return fmt.Errorf("unknown component: %s", name)
	}

	url := fmt.Sprintf("%s/health", endpoint.BaseURL)
	_, err := c.getJSON(ctx, url)
	return err
}

// postJSON sends a POST request with JSON body.
func (c *ComponentClient) postJSON(ctx context.Context, url string, body interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// getJSON sends a GET request and returns the JSON response.
func (c *ComponentClient) getJSON(ctx context.Context, url string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}
