// Package common provides shared HTTP utilities for all test-app components.
package common

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server wraps an HTTP server with common functionality.
type Server struct {
	router  *gin.Engine
	server  *http.Server
	metrics *Metrics
	name    string
}

// NewServer creates a new HTTP server with standard endpoints.
func NewServer(name string, port int) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		router:  router,
		name:    name,
		metrics: NewMetrics(name),
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}

	// Register standard endpoints
	s.registerStandardEndpoints()

	return s
}

// registerStandardEndpoints adds health, ready, and metrics endpoints.
func (s *Server) registerStandardEndpoints() {
	s.router.GET("/health", s.healthHandler)
	s.router.GET("/ready", s.readyHandler)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// healthHandler returns basic health status.
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"component": s.name,
		"timestamp": time.Now().UTC(),
	})
}

// readyHandler returns readiness status.
func (s *Server) readyHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ready":     true,
		"component": s.name,
		"timestamp": time.Now().UTC(),
	})
}

// Router returns the underlying gin router for adding custom routes.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Metrics returns the server's metrics instance.
func (s *Server) Metrics() *Metrics {
	return s.metrics
}

// Start begins serving HTTP requests.
func (s *Server) Start() error {
	s.metrics.SetReady()
	log.Printf("[%s] Starting server on %s", s.name, s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.metrics.SetNotReady()
	log.Printf("[%s] Shutting down server", s.name)
	return s.server.Shutdown(ctx)
}

// RunWithGracefulShutdown starts the server and handles graceful shutdown.
func (s *Server) RunWithGracefulShutdown() {
	// Start server in goroutine
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[%s] Server error: %v", s.name, err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("[%s] Received shutdown signal", s.name)

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("[%s] Forced shutdown: %v", s.name, err)
	}

	log.Printf("[%s] Server stopped", s.name)
}
