// Package main is the entry point for the recommendation-api server
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcapi "github.com/container-resource-predictor/recommendation-api/internal/api/grpc"
	"github.com/container-resource-predictor/recommendation-api/internal/api/rest"
	"github.com/container-resource-predictor/recommendation-api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting recommendation-api")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Register routes
	rest.RegisterRoutes(router)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server
	go func() {
		slog.Info("Starting HTTP server", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("Failed to listen for gRPC", "error", err)
		os.Exit(1)
	}

	grpcConfig := &grpcapi.ServerConfig{
		Address:           cfg.GRPCAddr,
		EnableTLS:         cfg.TLSEnabled,
		CertFile:          cfg.TLSCertFile,
		KeyFile:           cfg.TLSKeyFile,
		CAFile:            cfg.TLSCAFile,
		RateLimitPerAgent: cfg.RateLimitPerAgent,
	}

	grpcServer, err := grpcapi.NewServer(grpcConfig)
	if err != nil {
		slog.Error("Failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("Starting gRPC server", "addr", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("gRPC server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	grpcServer.GracefulStop()

	slog.Info("Server shutdown complete")
}
