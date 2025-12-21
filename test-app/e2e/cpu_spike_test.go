// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testCPUSpikeDetection tests that Kubewise detects CPU spikes within 5 minutes.
// Requirements: 3.5, 8.3
func testCPUSpikeDetection(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing CPU spike detection...")

	// First, ensure cpu-burster is in steady state
	steadyConfig := ComponentConfig{
		"mode":          "steady",
		"targetPercent": 20, // 20% baseline CPU
	}
	if err := helper.ConfigureComponent(ctx, "cpu-burster", steadyConfig); err != nil {
		t.Fatalf("Failed to reset cpu-burster to steady mode: %v", err)
	}
	
	// Wait for steady state baseline
	t.Log("Establishing baseline CPU usage...")
	time.Sleep(60 * time.Second)

	// Record start time for spike
	spikeStartTime := time.Now()

	// Trigger CPU spike
	t.Log("Triggering CPU spike...")
	if err := helper.TriggerSpike(ctx, "cpu-burster", "60s"); err != nil {
		t.Fatalf("Failed to trigger CPU spike: %v", err)
	}

	// Verify spike is happening
	time.Sleep(5 * time.Second)
	status, err := helper.GetComponentStatus(ctx, "cpu-burster")
	if err == nil {
		t.Logf("CPU-burster status during spike: %.1f%% CPU", status.CPUPercent)
	}

	// Wait for Kubewise to detect the CPU spike
	t.Logf("Waiting up to %v for Kubewise to detect CPU spike...", spikeDetectionWait)
	
	anomaly, err := helper.WaitForAnomaly(ctx, helper.config.TestNamespace, "cpu-burster", "cpu_spike", spikeDetectionWait)
	
	// Reset cpu-burster to steady mode regardless of result
	defer func() {
		resetConfig := ComponentConfig{
			"mode":          "steady",
			"targetPercent": 20,
		}
		helper.ConfigureComponent(ctx, "cpu-burster", resetConfig)
		t.Log("Reset cpu-burster to steady mode")
	}()

	if err != nil {
		// Check current CPU status
		status, statusErr := helper.GetComponentStatus(ctx, "cpu-burster")
		if statusErr == nil {
			t.Logf("Current cpu-burster status: %.1f%% CPU", status.CPUPercent)
		}
		t.Fatalf("CPU spike not detected within timeout: %v", err)
	}

	// Validate detection time
	detectionTime := anomaly.DetectedAt.Sub(spikeStartTime)
	t.Logf("CPU spike detected in %v", detectionTime)

	if detectionTime > 5*time.Minute {
		t.Errorf("CPU spike detection took %v, expected <= 5 minutes", detectionTime)
	}

	AssertNotNil(t, anomaly, "Anomaly should be detected")
	AssertTrue(t, anomaly.Type == "cpu_spike", "Anomaly type should be cpu_spike")

	t.Log("CPU spike detection test passed")
}
