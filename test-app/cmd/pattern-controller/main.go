// Package main implements the pattern-controller component.
// Pattern-controller orchestrates test scenarios and coordinates all test components.
package main

import (
	"log"

	"github.com/container-resource-predictor/test-app/internal/patterncontroller/api"
	"github.com/container-resource-predictor/test-app/internal/patterncontroller/client"
	"github.com/container-resource-predictor/test-app/internal/patterncontroller/scheduler"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("Starting pattern-controller...")

	// Load configuration from environment
	port := common.GetEnvInt("PORT", 8080)
	timeAcceleration := common.GetEnvFloat("TIME_ACCELERATION", 1.0)

	// Component URLs from environment (with defaults for K8s service discovery)
	memoryHogURL := common.GetEnv("MEMORY_HOG_URL", "http://memory-hog-svc:8082")
	cpuBursterURL := common.GetEnv("CPU_BURSTER_URL", "http://cpu-burster-svc:8083")
	steadyWorkerURL := common.GetEnv("STEADY_WORKER_URL", "http://steady-worker-svc:8084")
	loadGeneratorURL := common.GetEnv("LOAD_GENERATOR_URL", "http://load-generator-svc:8081")

	// Create component client
	clientCfg := client.Config{
		Timeout: common.GetEnvDuration("CLIENT_TIMEOUT", client.DefaultConfig().Timeout),
		Components: map[string]string{
			"memory-hog":     memoryHogURL,
			"cpu-burster":    cpuBursterURL,
			"steady-worker":  steadyWorkerURL,
			"load-generator": loadGeneratorURL,
		},
	}
	componentClient := client.New(clientCfg)

	// Create scheduler
	sched := scheduler.New(componentClient)

	// Set initial time acceleration if configured
	if timeAcceleration != 1.0 {
		if err := sched.SetTimeAcceleration(timeAcceleration); err != nil {
			log.Printf("Warning: failed to set time acceleration: %v", err)
		}
	}

	// Set up event logging
	sched.SetOnEvent(func(event scheduler.ScenarioEvent) {
		if event.Error != "" {
			log.Printf("[Event] %s: %s - %s (error: %s)",
				event.Type, event.Component, event.Message, event.Error)
		} else {
			log.Printf("[Event] %s: %s - %s", event.Type, event.Component, event.Message)
		}
	})

	// Create HTTP server
	server := common.NewServer("pattern-controller", port)

	// Register API handlers
	handler := api.NewHandler(sched, componentClient)
	handler.RegisterRoutes(server.Router())

	log.Printf("Pattern-controller configured:")
	log.Printf("  - Memory Hog URL: %s", memoryHogURL)
	log.Printf("  - CPU Burster URL: %s", cpuBursterURL)
	log.Printf("  - Steady Worker URL: %s", steadyWorkerURL)
	log.Printf("  - Load Generator URL: %s", loadGeneratorURL)
	log.Printf("  - Time Acceleration: %.1fx", timeAcceleration)

	// Run server with graceful shutdown
	server.RunWithGracefulShutdown()
}
