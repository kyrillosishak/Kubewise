// Package allocator provides memory allocation engines for the memory-hog component.
package allocator

import (
	"context"
	"sync"
	"time"
)

// Mode represents memory allocation modes.
type Mode string

const (
	ModeSteady Mode = "steady"
	ModeLeak   Mode = "leak"
	ModeSpike  Mode = "spike"
)

// Config holds the configuration for memory allocation.
type Config struct {
	Mode          Mode          `json:"mode"`
	TargetMB      int           `json:"targetMB"`      // Target memory for steady mode
	LeakRateMBMin int           `json:"leakRateMBMin"` // MB per minute for leak mode
	SpikeSizeMB   int           `json:"spikeSizeMB"`   // Size of memory spikes
	SpikeInterval time.Duration `json:"spikeInterval"` // Time between spikes
	SpikeDuration time.Duration `json:"spikeDuration"` // How long to hold spike
}

// DefaultConfig returns the default allocator configuration.
func DefaultConfig() Config {
	return Config{
		Mode:          ModeSteady,
		TargetMB:      256,
		LeakRateMBMin: 10,
		SpikeSizeMB:   128,
		SpikeInterval: 5 * time.Minute,
		SpikeDuration: 30 * time.Second,
	}
}

// SafetyConfig holds safety limits for memory allocation.
type SafetyConfig struct {
	MaxMemoryPercent int           // Max % of container limit
	PauseThreshold   int           // Pause allocation at this %
	ResumeThreshold  int           // Resume at this %
	CheckInterval    time.Duration // How often to check
	ContainerLimitMB int           // Container memory limit in MB
}

// DefaultSafetyConfig returns the default safety configuration.
func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		MaxMemoryPercent: 90,
		PauseThreshold:   85,
		ResumeThreshold:  70,
		CheckInterval:    time.Second,
		ContainerLimitMB: 512, // Default 512MB limit
	}
}

// Allocator manages memory allocation with different modes.
type Allocator struct {
	mu            sync.RWMutex
	config        Config
	safety        SafetyConfig
	allocations   [][]byte
	currentUsage  int64 // bytes
	running       bool
	paused        bool
	cancel        context.CancelFunc
	onUsageChange func(bytes int64)
}

// New creates a new Allocator with the given configuration.
func New(cfg Config, safety SafetyConfig) *Allocator {
	return &Allocator{
		config:      cfg,
		safety:      safety,
		allocations: make([][]byte, 0),
	}
}

// SetOnUsageChange sets a callback for when memory usage changes.
func (a *Allocator) SetOnUsageChange(fn func(bytes int64)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onUsageChange = fn
}

// Start begins memory allocation based on the current mode.
func (a *Allocator) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	ctx, a.cancel = context.WithCancel(ctx)
	a.mu.Unlock()

	go a.run(ctx)
	go a.safetyMonitor(ctx)
	return nil
}

// Stop halts memory allocation and releases all memory.
func (a *Allocator) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cancel != nil {
		a.cancel()
	}
	a.running = false
	a.releaseAll()
	return nil
}

// SetConfig updates the allocator configuration.
func (a *Allocator) SetConfig(cfg Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
	return nil
}

// GetConfig returns the current configuration.
func (a *Allocator) GetConfig() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}


// CurrentUsageMB returns the current memory usage in megabytes.
func (a *Allocator) CurrentUsageMB() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return int(a.currentUsage / (1024 * 1024))
}

// CurrentUsageBytes returns the current memory usage in bytes.
func (a *Allocator) CurrentUsageBytes() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentUsage
}

// IsPaused returns whether allocation is paused due to safety limits.
func (a *Allocator) IsPaused() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.paused
}

// IsRunning returns whether the allocator is running.
func (a *Allocator) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// TriggerSpike triggers an immediate memory spike regardless of mode.
func (a *Allocator) TriggerSpike() {
	a.mu.Lock()
	spikeSizeMB := a.config.SpikeSizeMB
	spikeDuration := a.config.SpikeDuration
	a.mu.Unlock()

	go func() {
		a.allocate(spikeSizeMB)
		time.Sleep(spikeDuration)
		a.release(spikeSizeMB)
	}()
}

// run is the main allocation loop.
func (a *Allocator) run(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			mode := a.config.Mode
			paused := a.paused
			a.mu.RUnlock()

			if paused {
				continue
			}

			switch mode {
			case ModeSteady:
				a.runSteady()
			case ModeLeak:
				a.runLeak()
			case ModeSpike:
				a.runSpike()
			}
		}
	}
}

// safetyMonitor checks memory usage against safety limits.
func (a *Allocator) safetyMonitor(ctx context.Context) {
	ticker := time.NewTicker(a.safety.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.checkSafety()
		}
	}
}

// checkSafety pauses or resumes allocation based on memory usage.
func (a *Allocator) checkSafety() {
	a.mu.Lock()
	defer a.mu.Unlock()

	currentMB := int(a.currentUsage / (1024 * 1024))
	limitMB := a.safety.ContainerLimitMB
	if limitMB == 0 {
		return
	}

	percent := (currentMB * 100) / limitMB

	if percent >= a.safety.PauseThreshold && !a.paused {
		a.paused = true
	} else if a.paused && percent <= a.safety.ResumeThreshold {
		a.paused = false
	}
}

// allocate adds memory allocation.
func (a *Allocator) allocate(sizeMB int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.paused {
		return
	}

	bytes := sizeMB * 1024 * 1024
	data := make([]byte, bytes)
	// Touch the memory to ensure it's actually allocated
	for i := 0; i < len(data); i += 4096 {
		data[i] = byte(i % 256)
	}
	a.allocations = append(a.allocations, data)
	a.currentUsage += int64(bytes)

	if a.onUsageChange != nil {
		a.onUsageChange(a.currentUsage)
	}
}

// release removes memory allocation.
func (a *Allocator) release(sizeMB int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	bytesToRelease := int64(sizeMB * 1024 * 1024)
	released := int64(0)

	for len(a.allocations) > 0 && released < bytesToRelease {
		last := len(a.allocations) - 1
		released += int64(len(a.allocations[last]))
		a.allocations = a.allocations[:last]
	}

	a.currentUsage -= released
	if a.currentUsage < 0 {
		a.currentUsage = 0
	}

	if a.onUsageChange != nil {
		a.onUsageChange(a.currentUsage)
	}
}

// releaseAll releases all allocated memory.
func (a *Allocator) releaseAll() {
	a.allocations = make([][]byte, 0)
	a.currentUsage = 0
	if a.onUsageChange != nil {
		a.onUsageChange(0)
	}
}
