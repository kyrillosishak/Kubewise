// Package storage provides database access for the recommendation API
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

// Config holds database configuration
type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns default database configuration
func DefaultConfig() *Config {
	return &Config{
		URL:             "postgres://localhost:5432/predictor?sslmode=disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// DB wraps the database connection pool
type DB struct {
	*sql.DB
	config *Config
}

// New creates a new database connection
func New(config *Config) (*DB, error) {
	if config == nil {
		config = DefaultConfig()
	}

	db, err := sql.Open("postgres", config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("Connected to database", "url", maskConnectionString(config.URL))

	return &DB{DB: db, config: config}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	slog.Info("Closing database connection")
	return db.DB.Close()
}

// Health checks database health
func (db *DB) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// maskConnectionString masks sensitive parts of connection string for logging
func maskConnectionString(url string) string {
	// Simple masking - in production use proper URL parsing
	return "postgres://***:***@***/predictor"
}
