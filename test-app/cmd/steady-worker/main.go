// Package main implements the steady-worker component.
// Steady-worker provides consistent baseline resource usage for testing
// Kubewise prediction accuracy against known resource patterns.
package main

import (
	"log"

	"github.com/container-resource-predictor/test-app/internal/steadyworker/api"
	"github.com/container-resource-predictor/test-app/internal/steadyworker/workload"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("Starting steady-worker...")

	// Load configuration from environment
	port := common.GetEnvInt("PORT", 8084)
	cpuWorkMs := common.GetEnvInt("CPU_WORK_MS", 10)
	memoryAllocKB := common.GetEnvInt("MEMORY_ALLOC_KB", 64)
	memoryHoldMs := common.GetEnvInt("MEMORY_HOLD_MS", 50)
	responseDelayMs := common.GetEnvInt("RESPONSE_DELAY_MS", 0)
	baseMemoryMB := common.GetEnvInt("BASE_MEMORY_MB", 64)

	// Create workload configuration
	cfg := workload.Config{
		CPUWorkMs:       cpuWorkMs,
		MemoryAllocKB:   memoryAllocKB,
		MemoryHoldMs:    memoryHoldMs,
		ResponseDelayMs: responseDelayMs,
		BaseMemoryMB:    baseMemoryMB,
	}

	// Create workload manager
	wl := workload.New(cfg)

	// Create HTTP server
	server := common.NewServer("steady-worker", port)

	// Set up metrics callback
	wl.SetOnMetricsChange(func(cpuPercent float64, memoryBytes int64) {
		server.Metrics().CPUUsage.Set(cpuPercent)
		server.Metrics().MemoryUsage.Set(float64(memoryBytes))
	})

	// Register API handlers
	handler := api.NewHandler(wl)
	handler.SetMetrics(server.Metrics())
	handler.RegisterRoutes(server.Router())

	// Start workload (allocates baseline memory)
	if err := wl.Start(); err != nil {
		log.Fatalf("Failed to start workload: %v", err)
	}

	// Update initial memory metric
	server.Metrics().MemoryUsage.Set(float64(wl.CurrentMemoryBytes()))

	log.Printf("Steady-worker configured: cpuWorkMs=%d, memoryAllocKB=%d, baseMemoryMB=%d",
		cpuWorkMs, memoryAllocKB, baseMemoryMB)

	// Run server with graceful shutdown
	server.RunWithGracefulShutdown()

	// Stop workload on shutdown
	if err := wl.Stop(); err != nil {
		log.Printf("Error stopping workload: %v", err)
	}
}
