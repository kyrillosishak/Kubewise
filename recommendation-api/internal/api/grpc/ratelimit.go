// Package grpc provides gRPC server implementation
package grpc

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
)

// RateLimiter implements per-agent rate limiting
type RateLimiter struct {
	mu              sync.RWMutex
	limitPerMinute  int
	agentCounters   map[string]*agentCounter
	cleanupInterval time.Duration
}

type agentCounter struct {
	count     int
	windowEnd time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limitPerMinute int) *RateLimiter {
	if limitPerMinute <= 0 {
		limitPerMinute = 60 // default: 60 requests per minute
	}

	rl := &RateLimiter{
		limitPerMinute:  limitPerMinute,
		agentCounters:   make(map[string]*agentCounter),
		cleanupInterval: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given agent is allowed
func (rl *RateLimiter) Allow(agentID string) bool {
	if agentID == "" {
		return true // Allow requests without agent ID (will be rejected by handler)
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	counter, exists := rl.agentCounters[agentID]

	if !exists || now.After(counter.windowEnd) {
		// New window
		rl.agentCounters[agentID] = &agentCounter{
			count:     1,
			windowEnd: now.Add(time.Minute),
		}
		return true
	}

	if counter.count >= rl.limitPerMinute {
		return false
	}

	counter.count++
	return true
}

// cleanup removes stale entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for agentID, counter := range rl.agentCounters {
			if now.After(counter.windowEnd.Add(time.Minute)) {
				delete(rl.agentCounters, agentID)
			}
		}
		rl.mu.Unlock()
	}
}

// extractAgentID extracts agent ID from gRPC metadata
func extractAgentID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("x-agent-id")
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
