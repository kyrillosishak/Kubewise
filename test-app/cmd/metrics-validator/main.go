// Package main provides the entry point for the metrics-validator component.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/api"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/reporter"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/storage"
	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/validator"
	"github.com/container-resource-predictor/test-app/pkg/common"
)

func main() {
	log.Println("[metrics-validator] Starting...")

	// Load configuration from environment
	config := loadConfig()

	// Create validator
	v := validator.New(config)

	// Create reporter
	r := reporter.New(v)

	// Create history storage
	h := storage.NewValidationHistory(getEnv("DATA_DIR", "/data"))
	if err := h.Load(); err != nil {
		log.Printf("[metrics-validator] Warning: could not load history: %v", err)
	}

	// Create webhook notifier
	var webhook *reporter.WebhookNotifier
	if webhookURL := os.Getenv("WEBHOOK_URL"); webhookURL != "" {
		webhook = reporter.NewWebhookNotifier(webhookURL)
		log.Printf("[metrics-validator] Webhook notifications enabled: %s", webhookURL)
	}

	// Create HTTP server
	port := getEnvInt("PORT", 8080)
	server := common.NewServer("metrics-validator", port)

	// Register API handlers
	handlers := api.NewHandlers(v, r, h, webhook)
	handlers.RegisterRoutes(server.Router())

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start validator in background
	go func() {
		if err := v.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("[metrics-validator] Validator error: %v", err)
		}
	}()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in background
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("[metrics-validator] Server error: %v", err)
		}
	}()

	log.Printf("[metrics-validator] Server listening on port %d", port)

	// Wait for shutdown signal
	<-sigCh
	log.Println("[metrics-validator] Shutting down...")

	// Cancel context to stop validator
	cancel()

	// Save history
	if err := h.Save(); err != nil {
		log.Printf("[metrics-validator] Warning: could not save history: %v", err)
	}

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[metrics-validator] Shutdown error: %v", err)
	}

	log.Println("[metrics-validator] Stopped")
}

func loadConfig() validator.ValidationConfig {
	config := validator.DefaultConfig()

	if url := os.Getenv("KUBEWISE_URL"); url != "" {
		config.KubewiseURL = url
	}
	if url := os.Getenv("PROMETHEUS_URL"); url != "" {
		config.PrometheusURL = url
	}
	if ns := os.Getenv("TARGET_NAMESPACE"); ns != "" {
		config.TargetNamespace = ns
	}
	if interval := os.Getenv("VALIDATION_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			config.ValidationInterval = d
		}
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
