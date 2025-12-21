// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TestHelperConfig holds configuration for the test helper.
type TestHelperConfig struct {
	KubewiseURL   string
	TestNamespace string
	Kubeconfig    string
}

// TestHelper provides utilities for E2E tests.
type TestHelper struct {
	config     TestHelperConfig
	k8sClient  kubernetes.Interface
	httpClient *http.Client
}

// NewTestHelper creates a new test helper.
func NewTestHelper(ctx context.Context, config TestHelperConfig) (*TestHelper, error) {
	k8sClient, err := createK8sClient(config.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}

	return &TestHelper{
		config:    config,
		k8sClient: k8sClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func createK8sClient(kubeconfigPath string) (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else if home := os.Getenv("HOME"); home != "" {
		defaultPath := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(defaultPath); err == nil {
			config, err = clientcmd.BuildConfigFromFlags("", defaultPath)
		} else {
			config, err = rest.InClusterConfig()
		}
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("building config: %w", err)
	}

	return kubernetes.NewForConfig(config)
}

// VerifyPrerequisites checks that all required components are running.
func (h *TestHelper) VerifyPrerequisites(ctx context.Context) error {
	// Check Kubewise API health
	if err := h.checkKubewiseHealth(ctx); err != nil {
		return fmt.Errorf("kubewise health check failed: %w", err)
	}

	// Check test components
	components := []string{
		"pattern-controller",
		"memory-hog",
		"cpu-burster",
		"steady-worker",
		"load-generator",
		"metrics-validator",
	}

	for _, component := range components {
		if err := h.checkComponentHealth(ctx, component); err != nil {
			return fmt.Errorf("component %s health check failed: %w", component, err)
		}
	}

	return nil
}

func (h *TestHelper) checkKubewiseHealth(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", h.config.KubewiseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func (h *TestHelper) checkComponentHealth(ctx context.Context, component string) error {
	url := h.getComponentURL(component, "/health")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func (h *TestHelper) getComponentURL(component, path string) string {
	ports := map[string]int{
		"pattern-controller": 8080,
		"memory-hog":         8082,
		"cpu-burster":        8083,
		"steady-worker":      8084,
		"load-generator":     8081,
		"metrics-validator":  8080,
	}
	port := ports[component]
	return fmt.Sprintf("http://%s-svc.%s:%d%s", component, h.config.TestNamespace, port, path)
}

// Cleanup performs cleanup after tests.
func (h *TestHelper) Cleanup(ctx context.Context) {
	// Reset components to default state
	h.resetComponent(ctx, "memory-hog", map[string]interface{}{
		"mode":     "steady",
		"targetMB": 128,
	})
	h.resetComponent(ctx, "cpu-burster", map[string]interface{}{
		"mode":          "steady",
		"targetPercent": 10,
	})
	h.stopLoadGenerator(ctx)
}

func (h *TestHelper) resetComponent(ctx context.Context, component string, config map[string]interface{}) {
	url := h.getComponentURL(component, "/api/v1/config")
	body, _ := json.Marshal(config)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.httpClient.Do(req)
}

func (h *TestHelper) stopLoadGenerator(ctx context.Context) {
	url := h.getComponentURL("load-generator", "/api/v1/stop")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	h.httpClient.Do(req)
}
