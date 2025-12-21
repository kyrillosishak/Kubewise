// Package main implements the memory-hog component.
// Memory-hog creates predictable memory usage patterns for testing.
package main

import (
	"context"
	"log"
	"time"

	"github.com/container-resource-predictor/test-app/internal/memoryhog/allocator"
	"github.com/container-resource-predictor/test-app/internal/memoryhog/api"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("Starting memory-hog...")

	// Load configuration from environment
	port := common.GetEnvInt("PORT", 8082)
	mode := common.GetEnv("MODE", "steady")
	targetMB := common.GetEnvInt("TARGET_MB", 256)
	leakRateMBMin := common.GetEnvInt("LEAK_RATE_MB_MIN", 10)
	spikeSizeMB := common.GetEnvInt("SPIKE_SIZE_MB", 128)
	spikeInterval := common.GetEnvDuration("SPIKE_INTERVAL", 5*time.Minute)
	spikeDuration := common.GetEnvDuration("SPIKE_DURATION", 30*time.Second)
	containerLimitMB := common.GetEnvInt("CONTAINER_LIMIT_MB", 512)

	// Create allocator configuration
	cfg := allocator.Config{
		Mode:          allocator.Mode(mode),
		TargetMB:      targetMB,
		LeakRateMBMin: leakRateMBMin,
		SpikeSizeMB:   spikeSizeMB,
		SpikeInterval: spikeInterval,
		SpikeDuration: spikeDuration,
	}

	// Create safety configuration
	safetyCfg := allocator.DefaultSafetyConfig()
	safetyCfg.ContainerLimitMB = containerLimitMB

	// Create allocator
	alloc := allocator.New(cfg, safetyCfg)

	// Create HTTP server
	server := common.NewServer("memory-hog", port)

	// Set up metrics callback
	alloc.SetOnUsageChange(func(bytes int64) {
		server.Metrics().MemoryUsage.Set(float64(bytes))
	})

	// Register API handlers
	handler := api.NewHandler(alloc)
	handler.RegisterRoutes(server.Router())

	// Start allocator
	ctx := context.Background()
	if err := alloc.Start(ctx); err != nil {
		log.Fatalf("Failed to start allocator: %v", err)
	}

	log.Printf("Memory-hog configured: mode=%s, targetMB=%d, containerLimitMB=%d",
		mode, targetMB, containerLimitMB)

	// Run server with graceful shutdown
	server.RunWithGracefulShutdown()

	// Stop allocator on shutdown
	if err := alloc.Stop(); err != nil {
		log.Printf("Error stopping allocator: %v", err)
	}
}
