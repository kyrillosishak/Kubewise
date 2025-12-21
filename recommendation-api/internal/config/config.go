// Package config handles application configuration
package config

import (
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	// Environment (development, production)
	Environment string

	// HTTP server address
	HTTPAddr string

	// gRPC server address
	GRPCAddr string

	// Database connection string
	DatabaseURL string

	// Model storage configuration
	ModelStorage ModelStorageConfig

	// TLS configuration
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string

	// Rate limiting
	RateLimitPerAgent int
}

// ModelStorageConfig holds model storage configuration
type ModelStorageConfig struct {
	// Backend type: local, s3, minio
	Backend string
	// Local storage path
	LocalPath string
	// S3/MinIO endpoint (for MinIO or custom S3-compatible storage)
	Endpoint string
	// S3 region
	Region string
	// S3 bucket name
	Bucket string
	// Access key ID for S3/MinIO
	AccessKeyID string
	// Secret access key for S3/MinIO
	SecretAccessKey string
	// Use SSL for S3/MinIO connection
	UseSSL bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		GRPCAddr:    getEnv("GRPC_ADDR", ":9090"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://localhost:5432/predictor?sslmode=disable"),
		ModelStorage: ModelStorageConfig{
			Backend:         getEnv("MODEL_STORAGE_BACKEND", "local"),
			LocalPath:       getEnv("MODEL_STORAGE_LOCAL_PATH", "/var/lib/predictor/models"),
			Endpoint:        getEnv("MODEL_STORAGE_ENDPOINT", ""),
			Region:          getEnv("MODEL_STORAGE_REGION", "us-east-1"),
			Bucket:          getEnv("MODEL_STORAGE_BUCKET", "predictor-models"),
			AccessKeyID:     getEnv("MODEL_STORAGE_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("MODEL_STORAGE_SECRET_ACCESS_KEY", ""),
			UseSSL:          getEnvBool("MODEL_STORAGE_USE_SSL", true),
		},
		TLSEnabled:        getEnvBool("TLS_ENABLED", false),
		TLSCertFile:       getEnv("TLS_CERT_FILE", "/etc/predictor/certs/server.crt"),
		TLSKeyFile:        getEnv("TLS_KEY_FILE", "/etc/predictor/certs/server.key"),
		TLSCAFile:         getEnv("TLS_CA_FILE", "/etc/predictor/certs/ca.crt"),
		RateLimitPerAgent: getEnvInt("RATE_LIMIT_PER_AGENT", 60),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
