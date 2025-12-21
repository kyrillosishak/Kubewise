// Package worker provides CPU work generation engines for the cpu-burster component.
package worker

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Mode represents CPU work modes.
type Mode string

const (
	ModeSteady Mode = "steady"
	ModeSpike  Mode = "spike"
	ModeWave   Mode = "wave"
	ModeRandom Mode = "random"
)

// Config holds the configuration for CPU work generation.
type Config struct {
	Mode          Mode          `json:"mode"`
	TargetPercent int           `json:"targetPercent"` // Target CPU % for steady mode
	SpikePercent  int           `json:"spikePercent"`  // CPU % during spikes
	SpikeInterval time.Duration `json:"spikeInterval"` // Time between spikes
	SpikeDuration time.Duration `json:"spikeDuration"` // Duration of each spike
	WavePeriod    time.Duration `json:"wavePeriod"`    // Period for wave mode
	WaveMin       int           `json:"waveMin"`       // Min CPU % for wave
	WaveMax       int           `json:"waveMax"`       // Max CPU % for wave
	Workers       int           `json:"workers"`       // Number of worker goroutines
}

// DefaultConfig returns the default worker configuration.
func DefaultConfig() Config {
	return Config{
		Mode:          ModeSteady,
		TargetPercent: 30,
		SpikePercent:  90,
		SpikeInterval: 5 * time.Minute,
		SpikeDuration: 30 * time.Second,
		WavePeriod:    10 * time.Minute,
		WaveMin:       10,
		WaveMax:       80,
		Workers:       runtime.NumCPU(),
	}
}

// Worker manages CPU work generation with different modes.
type Worker struct {
	mu              sync.RWMutex
	config          Config
	running         bool
	cancel          context.CancelFunc
	currentUsage    atomic.Int64 // Current CPU usage percentage * 100 for precision
	onUsageChange   func(percent float64)
	spikeTriggered  chan struct{}
}

// New creates a new Worker with the given configuration.
func New(cfg Config) *Worker {
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	return &Worker{
		config:         cfg,
		spikeTriggered: make(chan struct{}, 1),
	}
}

// SetOnUsageChange sets a callback for when CPU usage changes.
func (w *Worker) SetOnUsageChange(fn func(percent float64)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onUsageChange = fn
}

// Start begins CPU work generation based on the current mode.
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	ctx, w.cancel = context.WithCancel(ctx)
	workers := w.config.Workers
	w.mu.Unlock()

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go w.workLoop(ctx, i)
	}

	return nil
}


// Stop halts CPU work generation.
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	if w.cancel != nil {
		w.cancel()
	}
	w.running = false
	w.currentUsage.Store(0)
	return nil
}

// SetConfig updates the worker configuration.
func (w *Worker) SetConfig(cfg Config) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	w.config = cfg
	return nil
}

// GetConfig returns the current configuration.
func (w *Worker) GetConfig() Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// CurrentUsagePercent returns the current CPU usage percentage.
func (w *Worker) CurrentUsagePercent() float64 {
	return float64(w.currentUsage.Load()) / 100.0
}

// IsRunning returns whether the worker is running.
func (w *Worker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// TriggerSpike triggers an immediate CPU spike regardless of mode.
func (w *Worker) TriggerSpike() {
	select {
	case w.spikeTriggered <- struct{}{}:
	default:
		// Channel full, spike already pending
	}
}

// workLoop is the main work loop for each worker goroutine.
func (w *Worker) workLoop(ctx context.Context, workerID int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.spikeTriggered:
			w.doManualSpike(ctx)
		case <-ticker.C:
			w.mu.RLock()
			mode := w.config.Mode
			w.mu.RUnlock()

			switch mode {
			case ModeSteady:
				w.runSteady(ctx, workerID)
			case ModeSpike:
				w.runSpike(ctx, workerID)
			case ModeWave:
				w.runWave(ctx, workerID)
			case ModeRandom:
				w.runRandom(ctx, workerID)
			}
		}
	}
}

// doWork performs CPU-intensive work for the specified duration.
func (w *Worker) doWork(duration time.Duration) {
	start := time.Now()
	// CPU-intensive work: compute hash-like operations
	var result uint64
	for time.Since(start) < duration {
		for i := 0; i < 10000; i++ {
			result = result*31 + uint64(i)
			result ^= result >> 17
			result *= 0xed5ad4bb
		}
	}
	// Prevent compiler optimization
	_ = result
}

// updateUsage updates the current usage and notifies callback.
func (w *Worker) updateUsage(percent float64) {
	w.currentUsage.Store(int64(percent * 100))
	w.mu.RLock()
	callback := w.onUsageChange
	w.mu.RUnlock()
	if callback != nil {
		callback(percent)
	}
}

// doManualSpike performs a manually triggered spike.
func (w *Worker) doManualSpike(ctx context.Context) {
	w.mu.RLock()
	spikePercent := w.config.SpikePercent
	spikeDuration := w.config.SpikeDuration
	w.mu.RUnlock()

	w.updateUsage(float64(spikePercent))
	
	deadline := time.Now().Add(spikeDuration)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
			workDuration := time.Duration(spikePercent) * time.Millisecond
			w.doWork(workDuration)
			sleepDuration := time.Duration(100-spikePercent) * time.Millisecond
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
		}
	}
}
