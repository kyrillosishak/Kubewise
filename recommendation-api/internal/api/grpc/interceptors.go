// Package grpc provides gRPC server implementation
package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// loggingUnaryInterceptor logs unary RPC calls
func (s *Server) loggingUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	// Get peer info
	peerAddr := "unknown"
	if p, ok := peer.FromContext(ctx); ok {
		peerAddr = p.Addr.String()
	}

	// Call handler
	resp, err := handler(ctx, req)

	// Log the call
	duration := time.Since(start)
	logLevel := slog.LevelInfo
	if err != nil {
		logLevel = slog.LevelError
	}

	slog.Log(ctx, logLevel, "gRPC unary call",
		"method", info.FullMethod,
		"peer", peerAddr,
		"duration_ms", duration.Milliseconds(),
		"error", err,
	)

	return resp, err
}

// loggingStreamInterceptor logs streaming RPC calls
func (s *Server) loggingStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()

	// Get peer info
	peerAddr := "unknown"
	if p, ok := peer.FromContext(ss.Context()); ok {
		peerAddr = p.Addr.String()
	}

	slog.Info("gRPC stream started",
		"method", info.FullMethod,
		"peer", peerAddr,
	)

	// Call handler
	err := handler(srv, ss)

	// Log completion
	duration := time.Since(start)
	logLevel := slog.LevelInfo
	if err != nil {
		logLevel = slog.LevelError
	}

	slog.Log(ss.Context(), logLevel, "gRPC stream completed",
		"method", info.FullMethod,
		"peer", peerAddr,
		"duration_ms", duration.Milliseconds(),
		"error", err,
	)

	return err
}

// rateLimitUnaryInterceptor applies rate limiting to unary calls
func (s *Server) rateLimitUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	agentID := extractAgentID(ctx)

	if !s.rateLimiter.Allow(agentID) {
		slog.Warn("Rate limit exceeded",
			"agent_id", agentID,
			"method", info.FullMethod,
		)
		return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}

	return handler(ctx, req)
}

// rateLimitStreamInterceptor applies rate limiting to streaming calls
func (s *Server) rateLimitStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	agentID := extractAgentID(ss.Context())

	if !s.rateLimiter.Allow(agentID) {
		slog.Warn("Rate limit exceeded for stream",
			"agent_id", agentID,
			"method", info.FullMethod,
		)
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}

	return handler(srv, ss)
}
