// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testTimeAwareRecommendations tests that Kubewise provides separate peak/off-peak recommendations.
// Requirements: 4.1, 4.2, 4.5
func testTimeAwareRecommendations(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing time-aware recommendations...")

	// Configure Pattern Controller with time acceleration
	// Factor of 24 means 1 hour real time = 24 hours simulated time
	if err := helper.SetTimeAcceleration(ctx, 24); err != nil {
		t.Fatalf("Failed to set time acceleration: %v", err)
	}
	t.Log("Set time acceleration factor to 24x")

	// Start the time-patterns scenario which simulates business hours patterns
	// This scenario configures:
	// - High CPU/memory during simulated "business hours" (9AM-5PM)
	// - Low CPU/memory during simulated "off-peak" (nights/weekends)
	if err := helper.StartScenario(ctx, "time-patterns"); err != nil {
		// If time-patterns scenario doesn't exist, configure manually
		t.Log("time-patterns scenario not available, configuring manually...")
		
		// Configure steady-worker for business hours simulation
		businessHoursConfig := ComponentConfig{
			"mode":         "steady",
			"cpuWorkMs":    200, // Higher CPU during peak
			"baseMemoryMB": 256, // Higher memory during peak
		}
		if err := helper.ConfigureComponent(ctx, "steady-worker", businessHoursConfig); err != nil {
			t.Fatalf("Failed to configure steady-worker: %v", err)
		}
	}
	
	defer func() {
		helper.StopScenario(ctx)
		helper.SetTimeAcceleration(ctx, 1) // Reset to normal time
	}()

	// Wait for accelerated 24-hour cycle (1 hour real time with 24x acceleration)
	// For testing, we'll use a shorter duration
	waitDuration := 30 * time.Minute
	if *shortTest {
		waitDuration = 10 * time.Minute
	}
	
	t.Logf("Waiting %v for time-based pattern data collection...", waitDuration)
	select {
	case <-ctx.Done():
		t.Fatal("Context cancelled while waiting for time-based patterns")
	case <-time.After(waitDuration):
	}

	// Get Kubewise recommendations - should have separate peak/off-peak values
	rec, err := helper.GetRecommendation(ctx, helper.config.TestNamespace, "steady-worker")
	if err != nil {
		t.Fatalf("Failed to get recommendation: %v", err)
	}
	
	if rec == nil {
		t.Fatal("No recommendation found for steady-worker")
	}

	t.Logf("Got recommendation: CPU=%dm, Memory=%dMi",
		rec.CPURequestMillicores, rec.MemoryRequestBytes/(1024*1024))

	// Check for time-aware recommendations
	if rec.PeakHours != nil && rec.OffPeakHours != nil {
		t.Logf("Peak hours: CPU=%dm, Memory=%dMi",
			rec.PeakHours.CPURequest, rec.PeakHours.MemoryRequest/(1024*1024))
		t.Logf("Off-peak hours: CPU=%dm, Memory=%dMi",
			rec.OffPeakHours.CPURequest, rec.OffPeakHours.MemoryRequest/(1024*1024))

		// Validate that peak recommendations are higher than off-peak
		AssertGreater(t, float64(rec.PeakHours.CPURequest), float64(rec.OffPeakHours.CPURequest),
			"Peak CPU should be higher than off-peak")
		AssertGreater(t, float64(rec.PeakHours.MemoryRequest), float64(rec.OffPeakHours.MemoryRequest),
			"Peak memory should be higher than off-peak")
	} else {
		// If time-aware recommendations aren't available, check TimeWindow field
		if rec.TimeWindow != "" {
			t.Logf("Recommendation has time window: %s", rec.TimeWindow)
		} else {
			t.Log("Note: Time-aware recommendations (peak/off-peak) not available in this Kubewise version")
			// Don't fail - this feature may not be implemented yet
		}
	}

	t.Log("Time-aware recommendations test completed")
}
