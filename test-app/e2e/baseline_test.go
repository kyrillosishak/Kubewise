// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testBaselinePredictionAccuracy tests that Kubewise predictions are within 20% of actual usage.
// Requirements: 6.6, 7.3
func testBaselinePredictionAccuracy(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing baseline prediction accuracy...")

	// Configure steady-worker with known resource usage
	// CPU: 100m (100 millicores), Memory: 128Mi
	steadyConfig := ComponentConfig{
		"mode":        "steady",
		"cpuWorkMs":   100,  // CPU work per request
		"memoryAllocKB": 128, // Memory allocation per request
		"baseMemoryMB": 128, // Baseline memory
	}
	
	if err := helper.ConfigureComponent(ctx, "steady-worker", steadyConfig); err != nil {
		t.Fatalf("Failed to configure steady-worker: %v", err)
	}
	t.Log("Configured steady-worker with known resource usage")

	// Start load generator to create consistent traffic
	loadConfig := ComponentConfig{
		"mode":      "constant",
		"targetURL": "http://steady-worker-svc:8084/work",
		"rps":       10, // 10 requests per second
	}
	if err := helper.ConfigureComponent(ctx, "load-generator", loadConfig); err != nil {
		t.Fatalf("Failed to configure load-generator: %v", err)
	}
	
	if err := helper.StartLoadGenerator(ctx); err != nil {
		t.Fatalf("Failed to start load-generator: %v", err)
	}
	defer helper.StopLoadGenerator(ctx)
	t.Log("Started load generator")

	// Wait for Kubewise to collect data
	t.Logf("Waiting %v for Kubewise data collection...", dataCollectionWait)
	select {
	case <-ctx.Done():
		t.Fatal("Context cancelled while waiting for data collection")
	case <-time.After(dataCollectionWait):
	}

	// Get Kubewise recommendation for steady-worker
	rec, err := helper.GetRecommendation(ctx, helper.config.TestNamespace, "steady-worker")
	if err != nil {
		t.Fatalf("Failed to get recommendation: %v", err)
	}
	
	if rec == nil {
		t.Fatal("No recommendation found for steady-worker")
	}
	t.Logf("Got recommendation: CPU=%dm, Memory=%dMi, Confidence=%.2f",
		rec.CPURequestMillicores, rec.MemoryRequestBytes/(1024*1024), rec.Confidence)

	// Get actual resource usage from component status
	status, err := helper.GetComponentStatus(ctx, "steady-worker")
	if err != nil {
		t.Fatalf("Failed to get steady-worker status: %v", err)
	}
	t.Logf("Actual usage: CPU=%.1f%%, Memory=%dMB", status.CPUPercent, status.MemoryMB)

	// Validate predictions are within 20% of actual
	// Note: We compare against expected values since we configured known usage
	expectedCPUMillicores := float64(100) // 100m target
	expectedMemoryMB := float64(128)      // 128Mi target

	// CPU prediction accuracy
	predictedCPU := float64(rec.CPURequestMillicores)
	AssertWithinPercent(t, predictedCPU, expectedCPUMillicores, 20,
		"CPU prediction should be within 20% of expected")

	// Memory prediction accuracy
	predictedMemoryMB := float64(rec.MemoryRequestBytes) / (1024 * 1024)
	AssertWithinPercent(t, predictedMemoryMB, expectedMemoryMB, 20,
		"Memory prediction should be within 20% of expected")

	t.Log("Baseline prediction accuracy test passed")
}
