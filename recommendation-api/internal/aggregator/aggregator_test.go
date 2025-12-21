// Package aggregator provides recommendation aggregation logic
package aggregator

import (
	"testing"
)

func TestGetWindowDuration(t *testing.T) {
	tests := []struct {
		name     string
		window   string
		expected string
	}{
		{"peak window", "peak", "24 hours"},
		{"off_peak window", "off_peak", "24 hours"},
		{"weekly window", "weekly", "7 days"},
		{"unknown window", "unknown", "24 hours"},
		{"empty window", "", "24 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWindowDuration(tt.window)
			if result != tt.expected {
				t.Errorf("getWindowDuration(%q) = %q, want %q", tt.window, result, tt.expected)
			}
		})
	}
}

func TestAggregatedRecommendationStruct(t *testing.T) {
	rec := AggregatedRecommendation{
		Namespace:            "production",
		Deployment:           "api-server",
		CpuRequestMillicores: 100,
		CpuLimitMillicores:   500,
		MemoryRequestBytes:   134217728,
		MemoryLimitBytes:     268435456,
		Confidence:           0.92,
		ModelVersion:         "v1.2.0",
		TimeWindow:           "peak",
		PredictionCount:      50,
	}

	if rec.Namespace != "production" {
		t.Errorf("Expected namespace 'production', got '%s'", rec.Namespace)
	}
	if rec.Deployment != "api-server" {
		t.Errorf("Expected deployment 'api-server', got '%s'", rec.Deployment)
	}
	if rec.Confidence < 0 || rec.Confidence > 1 {
		t.Errorf("Confidence should be between 0 and 1, got %f", rec.Confidence)
	}
}

func TestDeploymentStatsStruct(t *testing.T) {
	stats := DeploymentStats{
		Namespace:       "default",
		Deployment:      "web-app",
		PredictionCount: 100,
		AvgConfidence:   0.85,
	}

	if stats.PredictionCount != 100 {
		t.Errorf("Expected prediction count 100, got %d", stats.PredictionCount)
	}
}

func TestNamespaceStatsStruct(t *testing.T) {
	stats := NamespaceStats{
		Namespace:        "production",
		DeploymentCount:  10,
		TotalPredictions: 500,
		AvgConfidence:    0.88,
		PotentialSavings: 450.00,
	}

	if stats.DeploymentCount != 10 {
		t.Errorf("Expected deployment count 10, got %d", stats.DeploymentCount)
	}
	if stats.PotentialSavings != 450.00 {
		t.Errorf("Expected potential savings 450.00, got %f", stats.PotentialSavings)
	}
}

func TestClusterStatsStruct(t *testing.T) {
	stats := ClusterStats{
		TotalNamespaces:  5,
		TotalDeployments: 25,
		TotalPredictions: 1000,
		AvgConfidence:    0.87,
		TotalSavings:     2500.00,
	}

	if stats.TotalNamespaces != 5 {
		t.Errorf("Expected total namespaces 5, got %d", stats.TotalNamespaces)
	}
	if stats.TotalDeployments != 25 {
		t.Errorf("Expected total deployments 25, got %d", stats.TotalDeployments)
	}
}
