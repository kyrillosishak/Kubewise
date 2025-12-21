// Package main implements the load-generator component.
// Load-generator creates HTTP traffic to simulate real workloads.
package main

import (
	"context"
	"log"
	"time"

	"github.com/container-resource-predictor/test-app/internal/loadgen/api"
	"github.com/container-resource-predictor/test-app/internal/loadgen/generator"
	"github.com/container-resource-predictor/test-app/internal/loadgen/metrics"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("Starting load-generator...")

	// Load configuration from environment
	port := common.GetEnvInt("PORT", 8081)
	mode := common.GetEnv("MODE", "constant")
	targetURL := common.GetEnv("TARGET_URL", "http://localhost:8080/work")
	rps := common.GetEnvInt("RPS", 10)
	rampStartRPS := common.GetEnvInt("RAMP_START_RPS", 10)
	rampEndRPS := common.GetEnvInt("RAMP_END_RPS", 100)
	rampDuration := common.GetEnvDuration("RAMP_DURATION", 10*time.Minute)
	burstRPS := common.GetEnvInt("BURST_RPS", 100)
	burstInterval := common.GetEnvDuration("BURST_INTERVAL", 5*time.Minute)
	burstDuration := common.GetEnvDuration("BURST_DURATION", 30*time.Second)
	payloadSizeKB := common.GetEnvInt("PAYLOAD_SIZE_KB", 1)
	timeout := common.GetEnvDuration("TIMEOUT", 10*time.Second)
	concurrency := common.GetEnvInt("CONCURRENCY", 50)
	autoStart := common.GetEnvBool("AUTO_START", false)

	// Create generator configuration
	cfg := generator.Config{
		Mode:          generator.Mode(mode),
		TargetURL:     targetURL,
		RPS:           rps,
		RampStartRPS:  rampStartRPS,
		RampEndRPS:    rampEndRPS,
		RampDuration:  rampDuration,
		BurstRPS:      burstRPS,
		BurstInterval: burstInterval,
		BurstDuration: burstDuration,
		PayloadSizeKB: payloadSizeKB,
		Timeout:       timeout,
		Concurrency:   concurrency,
	}

	// Create generator
	g := generator.New(cfg)

	// Create metrics
	m := metrics.New()
	m.SetMode(mode)
	m.TargetRPS.Set(float64(rps))

	// Create HTTP server
	server := common.NewServer("load-generator", port)

	// Set up metrics callback
	g.SetOnStatsChange(func(stats generator.Stats) {
		successRate := float64(0)
		if stats.TotalRequests > 0 {
			successRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests)
		}
		m.UpdateStats(
			stats.CurrentRPS,
			float64(cfg.RPS),
			successRate,
			stats.AvgLatencyMs,
			stats.P50LatencyMs,
			stats.P95LatencyMs,
			stats.P99LatencyMs,
			g.IsRunning(),
		)
	})

	// Register API handlers
	handler := api.NewHandler(g)
	handler.RegisterRoutes(server.Router())

	// Auto-start generator if configured
	if autoStart {
		ctx := context.Background()
		if err := g.Start(ctx); err != nil {
			log.Fatalf("Failed to start generator: %v", err)
		}
		log.Printf("Load-generator auto-started: mode=%s, targetURL=%s, rps=%d",
			mode, targetURL, rps)
	} else {
		log.Printf("Load-generator configured (not started): mode=%s, targetURL=%s, rps=%d",
			mode, targetURL, rps)
		log.Println("Use POST /api/v1/start to begin load generation")
	}

	// Run server with graceful shutdown
	server.RunWithGracefulShutdown()

	// Stop generator on shutdown
	if err := g.Stop(); err != nil {
		log.Printf("Error stopping generator: %v", err)
	}
}
