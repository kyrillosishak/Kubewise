// Package model provides model storage and management functionality
package model

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StorageBackend defines the type of storage backend
type StorageBackend string

const (
	StorageBackendLocal StorageBackend = "local"
	StorageBackendS3    StorageBackend = "s3"
	StorageBackendMinIO StorageBackend = "minio"
)

// StorageConfig holds configuration for model storage
type StorageConfig struct {
	Backend StorageBackend

	// Local storage config
	LocalPath string

	// S3/MinIO config
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		Backend:   StorageBackendLocal,
		LocalPath: "/var/lib/predictor/models",
		Region:    "us-east-1",
		Bucket:    "predictor-models",
		UseSSL:    true,
	}
}

// Storage provides model weight storage operations
type Storage struct {
	config   *StorageConfig
	s3Client *s3.Client
}

// NewStorage creates a new model storage instance
func NewStorage(cfg *StorageConfig) (*Storage, error) {
	if cfg == nil {
		cfg = DefaultStorageConfig()
	}

	s := &Storage{config: cfg}

	switch cfg.Backend {
	case StorageBackendLocal:
		// Ensure local directory exists
		if err := os.MkdirAll(cfg.LocalPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create local storage directory: %w", err)
		}
		slog.Info("Initialized local model storage", "path", cfg.LocalPath)

	case StorageBackendS3, StorageBackendMinIO:
		client, err := s.createS3Client()
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 client: %w", err)
		}
		s.s3Client = client
		slog.Info("Initialized S3/MinIO model storage",
			"endpoint", cfg.Endpoint,
			"bucket", cfg.Bucket,
		)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", cfg.Backend)
	}

	return s, nil
}

// createS3Client creates an S3 client for S3 or MinIO
func (s *Storage) createS3Client() (*s3.Client, error) {
	var opts []func(*config.LoadOptions) error

	opts = append(opts, config.WithRegion(s.config.Region))

	// Use static credentials if provided
	if s.config.AccessKeyID != "" && s.config.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				s.config.AccessKeyID,
				s.config.SecretAccessKey,
				"",
			),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint for MinIO
	clientOpts := []func(*s3.Options){}
	if s.config.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(s.config.Endpoint)
			o.UsePathStyle = true // Required for MinIO
		})
	}

	return s3.NewFromConfig(cfg, clientOpts...), nil
}

// StoreModel stores model weights and returns the storage path and checksum
func (s *Storage) StoreModel(ctx context.Context, version string, weights []byte) (string, string, error) {
	checksum := calculateChecksum(weights)

	switch s.config.Backend {
	case StorageBackendLocal:
		return s.storeLocal(version, weights, checksum)
	case StorageBackendS3, StorageBackendMinIO:
		return s.storeS3(ctx, version, weights, checksum)
	default:
		return "", "", fmt.Errorf("unsupported storage backend: %s", s.config.Backend)
	}
}

// storeLocal stores model weights to local filesystem
func (s *Storage) storeLocal(version string, weights []byte, checksum string) (string, string, error) {
	filename := fmt.Sprintf("model_%s.onnx", version)
	path := filepath.Join(s.config.LocalPath, filename)

	if err := os.WriteFile(path, weights, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write model file: %w", err)
	}

	slog.Info("Stored model locally",
		"version", version,
		"path", path,
		"size_bytes", len(weights),
		"checksum", checksum,
	)

	return path, checksum, nil
}

// storeS3 stores model weights to S3/MinIO
func (s *Storage) storeS3(ctx context.Context, version string, weights []byte, checksum string) (string, string, error) {
	key := fmt.Sprintf("models/model_%s.onnx", version)

	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:         aws.String(s.config.Bucket),
		Key:            aws.String(key),
		Body:           bytes.NewReader(weights),
		ContentType:    aws.String("application/octet-stream"),
		ChecksumSHA256: aws.String(checksum),
		ContentLength:  aws.Int64(int64(len(weights))),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to upload model to S3: %w", err)
	}

	storagePath := fmt.Sprintf("s3://%s/%s", s.config.Bucket, key)

	slog.Info("Stored model in S3",
		"version", version,
		"path", storagePath,
		"size_bytes", len(weights),
		"checksum", checksum,
	)

	return storagePath, checksum, nil
}

// GetModel retrieves model weights from storage
func (s *Storage) GetModel(ctx context.Context, storagePath string) ([]byte, error) {
	switch s.config.Backend {
	case StorageBackendLocal:
		return s.getLocal(storagePath)
	case StorageBackendS3, StorageBackendMinIO:
		return s.getS3(ctx, storagePath)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", s.config.Backend)
	}
}

// getLocal retrieves model weights from local filesystem
func (s *Storage) getLocal(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}
	return data, nil
}

// getS3 retrieves model weights from S3/MinIO
func (s *Storage) getS3(ctx context.Context, storagePath string) ([]byte, error) {
	// Parse s3://bucket/key format
	key := extractS3Key(storagePath)

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get model from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read model data: %w", err)
	}

	return data, nil
}

// DeleteModel deletes model weights from storage
func (s *Storage) DeleteModel(ctx context.Context, storagePath string) error {
	switch s.config.Backend {
	case StorageBackendLocal:
		return s.deleteLocal(storagePath)
	case StorageBackendS3, StorageBackendMinIO:
		return s.deleteS3(ctx, storagePath)
	default:
		return fmt.Errorf("unsupported storage backend: %s", s.config.Backend)
	}
}

// deleteLocal deletes model from local filesystem
func (s *Storage) deleteLocal(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete model file: %w", err)
	}
	return nil
}

// deleteS3 deletes model from S3/MinIO
func (s *Storage) deleteS3(ctx context.Context, storagePath string) error {
	key := extractS3Key(storagePath)

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete model from S3: %w", err)
	}

	return nil
}

// ListModels lists all models in storage
func (s *Storage) ListModels(ctx context.Context) ([]string, error) {
	switch s.config.Backend {
	case StorageBackendLocal:
		return s.listLocal()
	case StorageBackendS3, StorageBackendMinIO:
		return s.listS3(ctx)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", s.config.Backend)
	}
}

// listLocal lists models from local filesystem
func (s *Storage) listLocal() ([]string, error) {
	entries, err := os.ReadDir(s.config.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list local models: %w", err)
	}

	var models []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".onnx" {
			models = append(models, filepath.Join(s.config.LocalPath, entry.Name()))
		}
	}

	return models, nil
}

// listS3 lists models from S3/MinIO
func (s *Storage) listS3(ctx context.Context) ([]string, error) {
	result, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String("models/"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 models: %w", err)
	}

	var models []string
	for _, obj := range result.Contents {
		if obj.Key != nil {
			models = append(models, fmt.Sprintf("s3://%s/%s", s.config.Bucket, *obj.Key))
		}
	}

	return models, nil
}

// VerifyChecksum verifies the checksum of model weights
func (s *Storage) VerifyChecksum(weights []byte, expectedChecksum string) bool {
	actualChecksum := calculateChecksum(weights)
	return actualChecksum == expectedChecksum
}

// calculateChecksum calculates SHA256 checksum of data
func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// extractS3Key extracts the key from an s3://bucket/key path
func extractS3Key(storagePath string) string {
	// Remove s3://bucket/ prefix
	if len(storagePath) > 5 && storagePath[:5] == "s3://" {
		// Find the first / after s3://
		rest := storagePath[5:]
		for i, c := range rest {
			if c == '/' {
				return rest[i+1:]
			}
		}
	}
	return storagePath
}

// ModelInfo contains model metadata
type ModelInfo struct {
	Version            string
	StoragePath        string
	Checksum           string
	SizeBytes          int64
	CreatedAt          time.Time
	ValidationAccuracy float32
	IsActive           bool
	Metadata           map[string]interface{}
}
