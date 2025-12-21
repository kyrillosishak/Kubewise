// Package main implements the cpu-burster component.
// CPU-burster creates predictable CPU usage patterns for testing.
package main

import (
	"context"
	"log"
	"time"

	"github.com/container-resource-predictor/test-app/internal/cpuburster/api"
	"github.com/container-resource-predictor/test-app/internal/cpuburster/worker"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("Starting cpu-burster...")

	// Load configuration from environment
	port := common.GetEnvInt("PORT", 8083)
	mode := common.GetEnv("MODE", "steady")
	targetPercent := common.GetEnvInt("TARGET_PERCENT", 30)
	spikePercent := common.GetEnvInt("SPIKE_PERCENT", 90)
	spikeInterval := common.GetEnvDuration("SPIKE_INTERVAL", 5*time.Minute)
	spikeDuration := common.GetEnvDuration("SPIKE_DURATION", 30*time.Second)
	wavePeriod := common.GetEnvDuration("WAVE_PERIOD", 10*time.Minute)
	waveMin := common.GetEnvInt("WAVE_MIN", 10)
	waveMax := common.GetEnvInt("WAVE_MAX", 80)
	workers := common.GetEnvInt("WORKERS", 0) // 0 means use NumCPU

	// Create worker configuration
	cfg := worker.Config{
		Mode:          worker.Mode(mode),
		TargetPercent: targetPercent,
		SpikePercent:  spikePercent,
		SpikeInterval: spikeInterval,
		SpikeDuration: spikeDuration,
		WavePeriod:    wavePeriod,
		WaveMin:       waveMin,
		WaveMax:       waveMax,
		Workers:       workers,
	}

	// Create worker
	w := worker.New(cfg)

	// Create HTTP server
	server := common.NewServer("cpu-burster", port)

	// Set up metrics callback
	w.SetOnUsageChange(func(percent float64) {
		server.Metrics().CPUUsage.Set(percent)
	})

	// Register API handlers
	handler := api.NewHandler(w)
	handler.RegisterRoutes(server.Router())

	// Start worker
	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	log.Printf("CPU-burster configured: mode=%s, targetPercent=%d, workers=%d",
		mode, targetPercent, cfg.Workers)

	// Run server with graceful shutdown
	server.RunWithGracefulShutdown()

	// Stop worker on shutdown
	if err := w.Stop(); err != nil {
		log.Printf("Error stopping worker: %v", err)
	}
}
