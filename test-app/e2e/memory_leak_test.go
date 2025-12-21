// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testMemoryLeakDetection tests that Kubewise detects memory leaks within 30 minutes.
// Requirements: 2.5, 8.2
func testMemoryLeakDetection(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing memory leak detection...")

	// First, ensure memory-hog is in steady state
	steadyConfig := ComponentConfig{
		"mode":     "steady",
		"targetMB": 128,
	}
	if err := helper.ConfigureComponent(ctx, "memory-hog", steadyConfig); err != nil {
		t.Fatalf("Failed to reset memory-hog to steady mode: %v", err)
	}
	
	// Wait for steady state
	time.Sleep(30 * time.Second)

	// Configure memory-hog for leak mode
	leakConfig := ComponentConfig{
		"mode":          "leak",
		"leakRateMBMin": 10, // 10MB per minute leak rate
	}
	
	if err := helper.ConfigureComponent(ctx, "memory-hog", leakConfig); err != nil {
		t.Fatalf("Failed to configure memory-hog for leak mode: %v", err)
	}
	t.Log("Configured memory-hog in leak mode (10MB/min)")

	// Record start time for leak
	leakStartTime := time.Now()

	// Wait for Kubewise to detect the memory leak
	t.Logf("Waiting up to %v for Kubewise to detect memory leak...", leakDetectionWait)
	
	anomaly, err := helper.WaitForAnomaly(ctx, helper.config.TestNamespace, "memory-hog", "memory_leak", leakDetectionWait)
	
	// Reset memory-hog to steady mode regardless of result
	defer func() {
		resetConfig := ComponentConfig{
			"mode":     "steady",
			"targetMB": 128,
		}
		helper.ConfigureComponent(ctx, "memory-hog", resetConfig)
		t.Log("Reset memory-hog to steady mode")
	}()

	if err != nil {
		// Check if we at least see memory increasing
		status, statusErr := helper.GetComponentStatus(ctx, "memory-hog")
		if statusErr == nil {
			t.Logf("Current memory-hog status: %dMB allocated", status.AllocatedMB)
		}
		t.Fatalf("Memory leak not detected within timeout: %v", err)
	}

	// Validate detection time
	detectionTime := anomaly.DetectedAt.Sub(leakStartTime)
	t.Logf("Memory leak detected in %v", detectionTime)

	if detectionTime > 30*time.Minute {
		t.Errorf("Memory leak detection took %v, expected <= 30 minutes", detectionTime)
	}

	AssertNotNil(t, anomaly, "Anomaly should be detected")
	AssertTrue(t, anomaly.Type == "memory_leak", "Anomaly type should be memory_leak")

	t.Log("Memory leak detection test passed")
}
