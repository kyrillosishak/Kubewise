// Package grpc provides gRPC server implementation for agent communication
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	predictorv1 "github.com/container-resource-predictor/recommendation-api/api/proto/predictor/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ServerConfig holds gRPC server configuration
type ServerConfig struct {
	// Address to listen on
	Address string
	// TLS configuration
	CertFile   string
	KeyFile    string
	CAFile     string
	EnableTLS  bool
	// Rate limiting
	RateLimitPerAgent int // requests per minute per agent
	// Handlers
	AgentStore AgentStore
	ModelStore ModelStore
}

// AgentStore interface for agent data persistence
type AgentStore interface {
	RegisterAgent(ctx context.Context, agent *AgentInfo) error
	GetAgent(ctx context.Context, agentID string) (*AgentInfo, error)
	UpdateAgentLastSeen(ctx context.Context, agentID string) error
	StoreMetrics(ctx context.Context, batch *predictorv1.MetricsBatch) error
	StorePredictions(ctx context.Context, predictions []*predictorv1.ResourceProfile) error
	StoreAnomalies(ctx context.Context, anomalies []*predictorv1.Anomaly) error
}

// ModelStore interface for model management
type ModelStore interface {
	GetLatestModel(ctx context.Context) (*ModelInfo, error)
	GetModelWeights(ctx context.Context, version string) ([]byte, string, error)
	StoreGradients(ctx context.Context, agentID, modelVersion string, gradients []byte, sampleCount int64) error
}

// AgentInfo holds registered agent information
type AgentInfo struct {
	AgentID           string
	NodeName          string
	KubernetesVersion string
	AgentVersion      string
	ModelVersion      string
	RegisteredAt      time.Time
	LastSeenAt        time.Time
}

// ModelInfo holds model metadata
type ModelInfo struct {
	Version            string
	CreatedAt          time.Time
	ValidationAccuracy float32
	SizeBytes          int64
}


// Server implements the gRPC PredictorSync service
type Server struct {
	predictorv1.UnimplementedPredictorSyncServer
	config      *ServerConfig
	grpcServer  *grpc.Server
	rateLimiter *RateLimiter
	agentStore  AgentStore
	modelStore  ModelStore
}

// NewServer creates a new gRPC server
func NewServer(config *ServerConfig) (*Server, error) {
	s := &Server{
		config:      config,
		rateLimiter: NewRateLimiter(config.RateLimitPerAgent),
		agentStore:  config.AgentStore,
		modelStore:  config.ModelStore,
	}

	var opts []grpc.ServerOption

	// Configure mTLS if enabled
	if config.EnableTLS {
		tlsConfig, err := s.loadTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	// Add interceptors for rate limiting and logging
	opts = append(opts,
		grpc.ChainUnaryInterceptor(
			s.loggingUnaryInterceptor,
			s.rateLimitUnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			s.loggingStreamInterceptor,
			s.rateLimitStreamInterceptor,
		),
	)

	s.grpcServer = grpc.NewServer(opts...)
	predictorv1.RegisterPredictorSyncServer(s.grpcServer, s)

	return s, nil
}

// loadTLSConfig loads mTLS configuration
func (s *Server) loadTLSConfig() (*tls.Config, error) {
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCert, err := os.ReadFile(s.config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Serve starts the gRPC server
func (s *Server) Serve(listener net.Listener) error {
	slog.Info("Starting gRPC server", "address", listener.Addr().String())
	return s.grpcServer.Serve(listener)
}

// GracefulStop gracefully stops the server
func (s *Server) GracefulStop() {
	slog.Info("Gracefully stopping gRPC server")
	s.grpcServer.GracefulStop()
}

// Stop immediately stops the server
func (s *Server) Stop() {
	slog.Info("Stopping gRPC server")
	s.grpcServer.Stop()
}


// Register handles agent registration
func (s *Server) Register(ctx context.Context, req *predictorv1.RegisterRequest) (*predictorv1.RegisterResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.NodeName == "" {
		return nil, status.Error(codes.InvalidArgument, "node_name is required")
	}

	slog.Info("Agent registration request",
		"agent_id", req.AgentId,
		"node_name", req.NodeName,
		"agent_version", req.AgentVersion,
		"model_version", req.ModelVersion,
	)

	// Store agent info
	agent := &AgentInfo{
		AgentID:           req.AgentId,
		NodeName:          req.NodeName,
		KubernetesVersion: req.KubernetesVersion,
		AgentVersion:      req.AgentVersion,
		ModelVersion:      req.ModelVersion,
		RegisteredAt:      time.Now(),
		LastSeenAt:        time.Now(),
	}

	if s.agentStore != nil {
		if err := s.agentStore.RegisterAgent(ctx, agent); err != nil {
			slog.Error("Failed to register agent", "error", err, "agent_id", req.AgentId)
			return nil, status.Error(codes.Internal, "failed to register agent")
		}
	}

	// Return default configuration
	return &predictorv1.RegisterResponse{
		Success: true,
		Message: "Agent registered successfully",
		Config: &predictorv1.AgentConfig{
			CollectionIntervalSeconds: 10,
			PredictionIntervalSeconds: 300, // 5 minutes
			SyncIntervalSeconds:       60,
			AnomalyDetectionEnabled:   true,
		},
	}, nil
}

// SyncMetrics handles streaming metrics from agents
func (s *Server) SyncMetrics(stream predictorv1.PredictorSync_SyncMetricsServer) error {
	var totalMetrics, totalPredictions int64
	var agentID string

	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			slog.Info("Metrics sync completed",
				"agent_id", agentID,
				"total_metrics", totalMetrics,
				"total_predictions", totalPredictions,
			)
			return stream.SendAndClose(&predictorv1.SyncResponse{
				Success:             true,
				Message:             "Metrics synced successfully",
				MetricsReceived:     totalMetrics,
				PredictionsReceived: totalPredictions,
			})
		}
		if err != nil {
			slog.Error("Error receiving metrics batch", "error", err)
			return status.Error(codes.Internal, "failed to receive metrics")
		}

		agentID = batch.AgentId
		metricsCount := int64(len(batch.Metrics))
		predictionsCount := int64(len(batch.Predictions))

		slog.Debug("Received metrics batch",
			"agent_id", batch.AgentId,
			"node_name", batch.NodeName,
			"metrics_count", metricsCount,
			"predictions_count", predictionsCount,
			"anomalies_count", len(batch.Anomalies),
		)

		// Store metrics
		if s.agentStore != nil {
			if err := s.agentStore.StoreMetrics(stream.Context(), batch); err != nil {
				slog.Error("Failed to store metrics", "error", err)
				// Continue processing, don't fail the stream
			}

			if len(batch.Predictions) > 0 {
				if err := s.agentStore.StorePredictions(stream.Context(), batch.Predictions); err != nil {
					slog.Error("Failed to store predictions", "error", err)
				}
			}

			if len(batch.Anomalies) > 0 {
				if err := s.agentStore.StoreAnomalies(stream.Context(), batch.Anomalies); err != nil {
					slog.Error("Failed to store anomalies", "error", err)
				}
			}

			// Update agent last seen
			if err := s.agentStore.UpdateAgentLastSeen(stream.Context(), batch.AgentId); err != nil {
				slog.Error("Failed to update agent last seen", "error", err)
			}
		}

		totalMetrics += metricsCount
		totalPredictions += predictionsCount
	}
}


// GetModelUpdate handles model update requests
func (s *Server) GetModelUpdate(ctx context.Context, req *predictorv1.ModelRequest) (*predictorv1.ModelResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	slog.Info("Model update request",
		"agent_id", req.AgentId,
		"current_version", req.CurrentModelVersion,
	)

	// Check if model store is available
	if s.modelStore == nil {
		return &predictorv1.ModelResponse{
			UpdateAvailable: false,
		}, nil
	}

	// Get latest model info
	latestModel, err := s.modelStore.GetLatestModel(ctx)
	if err != nil {
		slog.Error("Failed to get latest model", "error", err)
		return nil, status.Error(codes.Internal, "failed to get model info")
	}

	if latestModel == nil {
		return &predictorv1.ModelResponse{
			UpdateAvailable: false,
		}, nil
	}

	// Check if update is needed
	if req.CurrentModelVersion == latestModel.Version {
		return &predictorv1.ModelResponse{
			UpdateAvailable: false,
		}, nil
	}

	// Get model weights
	weights, checksum, err := s.modelStore.GetModelWeights(ctx, latestModel.Version)
	if err != nil {
		slog.Error("Failed to get model weights", "error", err, "version", latestModel.Version)
		return nil, status.Error(codes.Internal, "failed to get model weights")
	}

	slog.Info("Sending model update",
		"agent_id", req.AgentId,
		"new_version", latestModel.Version,
		"size_bytes", len(weights),
	)

	return &predictorv1.ModelResponse{
		UpdateAvailable: true,
		NewVersion:      latestModel.Version,
		ModelWeights:    weights,
		Checksum:        checksum,
		Metadata: &predictorv1.ModelMetadata{
			Version:            latestModel.Version,
			CreatedAt:          timestamppb.New(latestModel.CreatedAt),
			ValidationAccuracy: latestModel.ValidationAccuracy,
			SizeBytes:          latestModel.SizeBytes,
		},
	}, nil
}

// UploadGradients handles federated learning gradient uploads
func (s *Server) UploadGradients(ctx context.Context, req *predictorv1.GradientsRequest) (*predictorv1.GradientsResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.ModelVersion == "" {
		return nil, status.Error(codes.InvalidArgument, "model_version is required")
	}
	if len(req.Gradients) == 0 {
		return nil, status.Error(codes.InvalidArgument, "gradients are required")
	}

	slog.Info("Gradients upload request",
		"agent_id", req.AgentId,
		"model_version", req.ModelVersion,
		"sample_count", req.SampleCount,
		"gradients_size", len(req.Gradients),
	)

	if s.modelStore != nil {
		if err := s.modelStore.StoreGradients(ctx, req.AgentId, req.ModelVersion, req.Gradients, req.SampleCount); err != nil {
			slog.Error("Failed to store gradients", "error", err)
			return nil, status.Error(codes.Internal, "failed to store gradients")
		}
	}

	return &predictorv1.GradientsResponse{
		Success: true,
		Message: "Gradients uploaded successfully",
	}, nil
}
