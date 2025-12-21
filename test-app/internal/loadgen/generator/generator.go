// Package generator provides HTTP load generation engines for the load-generator component.
package generator

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Mode represents load generation modes.
type Mode string

const (
	ModeConstant Mode = "constant"
	ModeRampUp   Mode = "ramp-up"
	ModeRampDown Mode = "ramp-down"
	ModeBurst    Mode = "burst"
)

// Config holds the configuration for load generation.
type Config struct {
	Mode          Mode          `json:"mode"`
	TargetURL     string        `json:"targetURL"`     // URL to hit
	RPS           int           `json:"rps"`           // Requests per second
	RampStartRPS  int           `json:"rampStartRPS"`  // Starting RPS for ramp
	RampEndRPS    int           `json:"rampEndRPS"`    // Ending RPS for ramp
	RampDuration  time.Duration `json:"rampDuration"`  // Duration of ramp
	BurstRPS      int           `json:"burstRPS"`      // RPS during burst (10x default)
	BurstInterval time.Duration `json:"burstInterval"` // Time between bursts
	BurstDuration time.Duration `json:"burstDuration"` // Duration of each burst
	PayloadSizeKB int           `json:"payloadSizeKB"` // Request payload size
	Timeout       time.Duration `json:"timeout"`       // Request timeout
	Concurrency   int           `json:"concurrency"`   // Max concurrent requests
}

// DefaultConfig returns the default generator configuration.
func DefaultConfig() Config {
	return Config{
		Mode:          ModeConstant,
		TargetURL:     "http://localhost:8080/work",
		RPS:           10,
		RampStartRPS:  10,
		RampEndRPS:    100,
		RampDuration:  10 * time.Minute,
		BurstRPS:      100, // 10x default RPS
		BurstInterval: 5 * time.Minute,
		BurstDuration: 30 * time.Second,
		PayloadSizeKB: 1,
		Timeout:       10 * time.Second,
		Concurrency:   50,
	}
}


// Stats holds load generation statistics.
type Stats struct {
	TotalRequests   int64   `json:"totalRequests"`
	SuccessRequests int64   `json:"successRequests"`
	FailedRequests  int64   `json:"failedRequests"`
	AvgLatencyMs    float64 `json:"avgLatencyMs"`
	P50LatencyMs    float64 `json:"p50LatencyMs"`
	P95LatencyMs    float64 `json:"p95LatencyMs"`
	P99LatencyMs    float64 `json:"p99LatencyMs"`
	CurrentRPS      float64 `json:"currentRPS"`
}

// Generator manages HTTP load generation with different modes.
type Generator struct {
	mu            sync.RWMutex
	config        Config
	running       bool
	cancel        context.CancelFunc
	client        *http.Client
	payload       []byte
	semaphore     chan struct{}
	rampStartTime time.Time

	// Statistics
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	failedRequests  atomic.Int64
	currentRPS      atomic.Int64 // RPS * 100 for precision
	latencies       *latencyTracker

	// Callbacks
	onStatsChange func(stats Stats)

	// Burst control
	burstActive   atomic.Bool
	lastBurstTime time.Time
}

// New creates a new Generator with the given configuration.
func New(cfg Config) *Generator {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	g := &Generator{
		config:    cfg,
		latencies: newLatencyTracker(1000),
		semaphore: make(chan struct{}, cfg.Concurrency),
		client: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Concurrency,
				MaxIdleConnsPerHost: cfg.Concurrency,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	g.generatePayload()
	return g
}

// generatePayload creates the request payload based on config.
func (g *Generator) generatePayload() {
	size := g.config.PayloadSizeKB * 1024
	if size <= 0 {
		size = 1024
	}
	g.payload = make([]byte, size)
	rand.Read(g.payload)
}

// SetOnStatsChange sets a callback for when stats change.
func (g *Generator) SetOnStatsChange(fn func(stats Stats)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onStatsChange = fn
}


// Start begins load generation based on the current mode.
func (g *Generator) Start(ctx context.Context) error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = true
	g.rampStartTime = time.Now()
	ctx, g.cancel = context.WithCancel(ctx)
	g.mu.Unlock()

	// Start the main load generation loop
	go g.runLoop(ctx)

	// Start stats reporter
	go g.statsReporter(ctx)

	return nil
}

// Stop halts load generation.
func (g *Generator) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil
	}

	if g.cancel != nil {
		g.cancel()
	}
	g.running = false
	g.currentRPS.Store(0)
	return nil
}

// SetConfig updates the generator configuration.
func (g *Generator) SetConfig(cfg Config) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 50
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	// Update client if timeout changed
	if cfg.Timeout != g.config.Timeout {
		g.client.Timeout = cfg.Timeout
	}

	// Update semaphore if concurrency changed
	if cfg.Concurrency != g.config.Concurrency {
		g.semaphore = make(chan struct{}, cfg.Concurrency)
	}

	// Reset ramp start time if mode changed to ramp
	if cfg.Mode == ModeRampUp || cfg.Mode == ModeRampDown {
		g.rampStartTime = time.Now()
	}

	g.config = cfg

	// Regenerate payload if size changed
	if cfg.PayloadSizeKB != g.config.PayloadSizeKB {
		g.generatePayload()
	}

	return nil
}

// GetConfig returns the current configuration.
func (g *Generator) GetConfig() Config {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config
}

// GetStats returns the current statistics.
func (g *Generator) GetStats() Stats {
	return Stats{
		TotalRequests:   g.totalRequests.Load(),
		SuccessRequests: g.successRequests.Load(),
		FailedRequests:  g.failedRequests.Load(),
		AvgLatencyMs:    g.latencies.Average(),
		P50LatencyMs:    g.latencies.Percentile(50),
		P95LatencyMs:    g.latencies.Percentile(95),
		P99LatencyMs:    g.latencies.Percentile(99),
		CurrentRPS:      float64(g.currentRPS.Load()) / 100.0,
	}
}

// IsRunning returns whether the generator is running.
func (g *Generator) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

// ResetStats resets all statistics.
func (g *Generator) ResetStats() {
	g.totalRequests.Store(0)
	g.successRequests.Store(0)
	g.failedRequests.Store(0)
	g.latencies.Reset()
}

// ResetRampTimer resets the ramp start time.
func (g *Generator) ResetRampTimer() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rampStartTime = time.Now()
}


// runLoop is the main load generation loop.
func (g *Generator) runLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond) // 100 ticks per second for fine-grained control
	defer ticker.Stop()

	var requestCounter int
	lastSecond := time.Now()
	requestsThisSecond := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.mu.RLock()
			mode := g.config.Mode
			g.mu.RUnlock()

			targetRPS := g.calculateTargetRPS(mode)

			// Calculate how many requests to send this tick
			// 100 ticks per second, so divide RPS by 100
			requestsPerTick := float64(targetRPS) / 100.0
			requestCounter++

			// Use fractional accumulation for sub-1 RPS rates
			requestsToSend := int(float64(requestCounter) * requestsPerTick / float64(requestCounter))
			if requestsPerTick >= 1 {
				requestsToSend = int(requestsPerTick)
			} else if requestCounter%int(1/requestsPerTick) == 0 {
				requestsToSend = 1
			} else {
				requestsToSend = 0
			}

			// Send requests
			for i := 0; i < requestsToSend; i++ {
				select {
				case g.semaphore <- struct{}{}:
					go g.sendRequest(ctx)
					requestsThisSecond++
				default:
					// Concurrency limit reached, skip
				}
			}

			// Update current RPS every second
			if time.Since(lastSecond) >= time.Second {
				g.currentRPS.Store(int64(requestsThisSecond * 100))
				requestsThisSecond = 0
				lastSecond = time.Now()
			}
		}
	}
}

// calculateTargetRPS calculates the target RPS based on mode.
func (g *Generator) calculateTargetRPS(mode Mode) int {
	g.mu.RLock()
	cfg := g.config
	rampStart := g.rampStartTime
	g.mu.RUnlock()

	switch mode {
	case ModeConstant:
		return cfg.RPS

	case ModeRampUp:
		elapsed := time.Since(rampStart)
		if elapsed >= cfg.RampDuration {
			return cfg.RampEndRPS
		}
		progress := float64(elapsed) / float64(cfg.RampDuration)
		return cfg.RampStartRPS + int(float64(cfg.RampEndRPS-cfg.RampStartRPS)*progress)

	case ModeRampDown:
		elapsed := time.Since(rampStart)
		if elapsed >= cfg.RampDuration {
			return cfg.RampEndRPS
		}
		progress := float64(elapsed) / float64(cfg.RampDuration)
		return cfg.RampStartRPS - int(float64(cfg.RampStartRPS-cfg.RampEndRPS)*progress)

	case ModeBurst:
		// Check if we should be in burst mode
		if g.burstActive.Load() {
			return cfg.BurstRPS
		}

		g.mu.Lock()
		timeSinceLastBurst := time.Since(g.lastBurstTime)
		if timeSinceLastBurst >= cfg.BurstInterval {
			g.burstActive.Store(true)
			g.lastBurstTime = time.Now()
			g.mu.Unlock()

			// Schedule burst end
			go func() {
				time.Sleep(cfg.BurstDuration)
				g.burstActive.Store(false)
			}()
			return cfg.BurstRPS
		}
		g.mu.Unlock()
		return cfg.RPS

	default:
		return cfg.RPS
	}
}


// sendRequest sends a single HTTP request to the target.
func (g *Generator) sendRequest(ctx context.Context) {
	defer func() { <-g.semaphore }()

	g.mu.RLock()
	targetURL := g.config.TargetURL
	payload := g.payload
	g.mu.RUnlock()

	start := time.Now()
	g.totalRequests.Add(1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, nil)
	if err != nil {
		g.failedRequests.Add(1)
		return
	}

	// Add payload if configured
	if len(payload) > 0 {
		req.Body = io.NopCloser(&payloadReader{data: payload})
		req.ContentLength = int64(len(payload))
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	resp, err := g.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		g.failedRequests.Add(1)
		return
	}
	defer resp.Body.Close()

	// Drain response body
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		g.successRequests.Add(1)
	} else {
		g.failedRequests.Add(1)
	}

	g.latencies.Add(float64(latency.Milliseconds()))
}

// statsReporter periodically reports stats via callback.
func (g *Generator) statsReporter(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.mu.RLock()
			callback := g.onStatsChange
			g.mu.RUnlock()

			if callback != nil {
				callback(g.GetStats())
			}
		}
	}
}

// payloadReader is a simple reader for the payload.
type payloadReader struct {
	data []byte
	pos  int
}

func (r *payloadReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
