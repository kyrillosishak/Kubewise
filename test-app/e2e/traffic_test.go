// Package e2e provides end-to-end tests for validating Kubewise functionality.
package e2e

import (
	"context"
	"testing"
	"time"
)

// testTrafficDrivenWorkloads tests that Kubewise recommendations scale with traffic.
// Requirements: 5.1, 5.2, 5.5
func testTrafficDrivenWorkloads(t *testing.T, ctx context.Context, helper *TestHelper) {
	t.Log("Testing traffic-driven workloads...")

	// Configure steady-worker for traffic testing
	workerConfig := ComponentConfig{
		"mode":          "steady",
		"cpuWorkMs":     50,  // CPU work per request
		"memoryAllocKB": 64,  // Memory per request
		"baseMemoryMB":  64,  // Baseline memory
	}
	if err := helper.ConfigureComponent(ctx, "steady-worker", workerConfig); err != nil {
		t.Fatalf("Failed to configure steady-worker: %v", err)
	}

	// Phase 1: Low traffic baseline
	t.Log("Phase 1: Establishing low traffic baseline...")
	lowTrafficConfig := ComponentConfig{
		"mode":      "constant",
		"targetURL": "http://steady-worker-svc:8084/work",
		"rps":       10, // 10 requests per second
	}
	if err := helper.ConfigureComponent(ctx, "load-generator", lowTrafficConfig); err != nil {
		t.Fatalf("Failed to configure load-generator: %v", err)
	}
	
	if err := helper.StartLoadGenerator(ctx); err != nil {
		t.Fatalf("Failed to start load-generator: %v", err)
	}
	defer helper.StopLoadGenerator(ctx)

	// Wait for baseline data collection
	baselineWait := 5 * time.Minute
	if *shortTest {
		baselineWait = 2 * time.Minute
	}
	t.Logf("Waiting %v for baseline data collection...", baselineWait)
	time.Sleep(baselineWait)

	// Get baseline recommendation
	recBefore, err := helper.GetRecommendation(ctx, helper.config.TestNamespace, "steady-worker")
	if err != nil {
		t.Fatalf("Failed to get baseline recommendation: %v", err)
	}
	
	var baselineCPU uint32
	if recBefore != nil {
		baselineCPU = recBefore.CPURequestMillicores
		t.Logf("Baseline recommendation: CPU=%dm, Memory=%dMi",
			recBefore.CPURequestMillicores, recBefore.MemoryRequestBytes/(1024*1024))
	} else {
		t.Log("No baseline recommendation available yet")
	}

	// Phase 2: Ramp up traffic
	t.Log("Phase 2: Ramping up traffic...")
	rampConfig := ComponentConfig{
		"mode":         "ramp-up",
		"targetURL":    "http://steady-worker-svc:8084/work",
		"rampStartRPS": 10,
		"rampEndRPS":   100,
		"rampDuration": "5m",
	}
	if err := helper.ConfigureComponent(ctx, "load-generator", rampConfig); err != nil {
		t.Fatalf("Failed to configure load-generator for ramp: %v", err)
	}

	// Wait for ramp and data collection
	rampWait := 10 * time.Minute
	if *shortTest {
		rampWait = 5 * time.Minute
	}
	t.Logf("Waiting %v for traffic ramp and data collection...", rampWait)
	time.Sleep(rampWait)

	// Check load stats
	stats, err := helper.GetLoadStats(ctx)
	if err == nil {
		t.Logf("Load stats: RPS=%.1f, Total=%d, Success=%d, AvgLatency=%.1fms",
			stats.CurrentRPS, stats.TotalRequests, stats.SuccessRequests, stats.AvgLatencyMs)
	}

	// Phase 3: High constant traffic
	t.Log("Phase 3: Maintaining high traffic...")
	highTrafficConfig := ComponentConfig{
		"mode":      "constant",
		"targetURL": "http://steady-worker-svc:8084/work",
		"rps":       100, // 100 requests per second
	}
	if err := helper.ConfigureComponent(ctx, "load-generator", highTrafficConfig); err != nil {
		t.Fatalf("Failed to configure load-generator for high traffic: %v", err)
	}

	// Wait for high traffic data collection
	highTrafficWait := 5 * time.Minute
	if *shortTest {
		highTrafficWait = 2 * time.Minute
	}
	t.Logf("Waiting %v for high traffic data collection...", highTrafficWait)
	time.Sleep(highTrafficWait)

	// Get updated recommendation
	recAfter, err := helper.GetRecommendation(ctx, helper.config.TestNamespace, "steady-worker")
	if err != nil {
		t.Fatalf("Failed to get updated recommendation: %v", err)
	}
	
	if recAfter == nil {
		t.Fatal("No recommendation found after traffic increase")
	}
	
	t.Logf("Updated recommendation: CPU=%dm, Memory=%dMi",
		recAfter.CPURequestMillicores, recAfter.MemoryRequestBytes/(1024*1024))

	// Validate that recommendations increased with traffic
	if baselineCPU > 0 {
		AssertGreater(t, float64(recAfter.CPURequestMillicores), float64(baselineCPU),
			"CPU recommendation should increase with traffic")
	}

	// Verify steady-worker is handling the load
	status, err := helper.GetComponentStatus(ctx, "steady-worker")
	if err == nil {
		t.Logf("Steady-worker status: CPU=%.1f%%, Memory=%dMB, RPS=%.1f",
			status.CPUPercent, status.MemoryMB, status.RequestsPerSecond)
	}

	t.Log("Traffic-driven workloads test passed")
}
