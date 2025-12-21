// Package workload provides CPU and memory workload generation for steady-worker.
package workload

import (
	"crypto/sha256"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Config defines the workload configuration.
type Config struct {
	CPUWorkMs       int `json:"cpuWorkMs"`       // CPU work per request in milliseconds
	MemoryAllocKB   int `json:"memoryAllocKB"`   // Memory to allocate per request
	MemoryHoldMs    int `json:"memoryHoldMs"`    // How long to hold memory
	ResponseDelayMs int `json:"responseDelayMs"` // Additional response delay
	BaseMemoryMB    int `json:"baseMemoryMB"`    // Baseline memory allocation
}

// DefaultConfig returns the default workload configuration.
func DefaultConfig() Config {
	return Config{
		CPUWorkMs:       10,
		MemoryAllocKB:   64,
		MemoryHoldMs:    50,
		ResponseDelayMs: 0,
		BaseMemoryMB:    64,
	}
}

// Workload manages CPU and memory work for requests.
type Workload struct {
	config       Config
	configMu     sync.RWMutex
	baseMemory   []byte
	baseMemoryMu sync.RWMutex

	// Metrics
	totalRequests   atomic.Int64
	totalCPUTimeNs  atomic.Int64
	totalMemoryKB   atomic.Int64
	activeRequests  atomic.Int32
	currentMemoryKB atomic.Int64

	// Callback for metrics updates
	onMetricsChange func(cpuPercent float64, memoryBytes int64)
}

// New creates a new Workload instance.
func New(cfg Config) *Workload {
	w := &Workload{
		config: cfg,
	}
	return w
}

// Start initializes the workload with baseline memory.
func (w *Workload) Start() error {
	w.configMu.RLock()
	baseMemMB := w.config.BaseMemoryMB
	w.configMu.RUnlock()

	if baseMemMB > 0 {
		w.allocateBaseMemory(baseMemMB)
	}
	return nil
}

// Stop releases baseline memory.
func (w *Workload) Stop() error {
	w.baseMemoryMu.Lock()
	w.baseMemory = nil
	w.baseMemoryMu.Unlock()
	runtime.GC()
	return nil
}


// allocateBaseMemory allocates and holds baseline memory.
func (w *Workload) allocateBaseMemory(mb int) {
	w.baseMemoryMu.Lock()
	defer w.baseMemoryMu.Unlock()

	size := mb * 1024 * 1024
	w.baseMemory = make([]byte, size)
	// Touch memory to ensure it's actually allocated
	for i := 0; i < len(w.baseMemory); i += 4096 {
		w.baseMemory[i] = byte(i % 256)
	}
	w.currentMemoryKB.Store(int64(mb * 1024))
}

// GetConfig returns the current configuration.
func (w *Workload) GetConfig() Config {
	w.configMu.RLock()
	defer w.configMu.RUnlock()
	return w.config
}

// SetConfig updates the workload configuration.
func (w *Workload) SetConfig(cfg Config) error {
	w.configMu.Lock()
	oldBaseMemMB := w.config.BaseMemoryMB
	w.config = cfg
	w.configMu.Unlock()

	// Update base memory if changed
	if cfg.BaseMemoryMB != oldBaseMemMB {
		w.allocateBaseMemory(cfg.BaseMemoryMB)
	}
	return nil
}

// SetOnMetricsChange sets a callback for metrics updates.
func (w *Workload) SetOnMetricsChange(fn func(cpuPercent float64, memoryBytes int64)) {
	w.onMetricsChange = fn
}

// DoWork performs CPU and memory work for a single request.
// Returns the actual CPU time spent and memory allocated.
func (w *Workload) DoWork() (cpuTimeMs int64, memoryKB int64) {
	w.activeRequests.Add(1)
	defer w.activeRequests.Add(-1)

	w.configMu.RLock()
	cfg := w.config
	w.configMu.RUnlock()

	start := time.Now()

	// Allocate memory for this request
	var requestMemory []byte
	if cfg.MemoryAllocKB > 0 {
		requestMemory = make([]byte, cfg.MemoryAllocKB*1024)
		rand.Read(requestMemory)
		w.currentMemoryKB.Add(int64(cfg.MemoryAllocKB))
	}

	// Do CPU work
	if cfg.CPUWorkMs > 0 {
		w.doCPUWork(time.Duration(cfg.CPUWorkMs) * time.Millisecond)
	}

	// Hold memory for specified duration
	if cfg.MemoryHoldMs > 0 && len(requestMemory) > 0 {
		time.Sleep(time.Duration(cfg.MemoryHoldMs) * time.Millisecond)
	}

	// Additional response delay
	if cfg.ResponseDelayMs > 0 {
		time.Sleep(time.Duration(cfg.ResponseDelayMs) * time.Millisecond)
	}

	// Release request memory
	if cfg.MemoryAllocKB > 0 {
		w.currentMemoryKB.Add(-int64(cfg.MemoryAllocKB))
	}

	elapsed := time.Since(start)
	cpuTimeMs = elapsed.Milliseconds()
	memoryKB = int64(cfg.MemoryAllocKB)

	w.totalRequests.Add(1)
	w.totalCPUTimeNs.Add(elapsed.Nanoseconds())
	w.totalMemoryKB.Add(memoryKB)

	return cpuTimeMs, memoryKB
}

// doCPUWork performs CPU-intensive work for the specified duration.
func (w *Workload) doCPUWork(duration time.Duration) {
	deadline := time.Now().Add(duration)
	data := make([]byte, 1024)
	rand.Read(data)

	for time.Now().Before(deadline) {
		// CPU-intensive work: repeated hashing
		for i := 0; i < 100; i++ {
			hash := sha256.Sum256(data)
			data = hash[:]
			data = append(data, make([]byte, 1024-len(data))...)
		}
	}
}

// Stats returns current workload statistics.
type Stats struct {
	TotalRequests   int64   `json:"totalRequests"`
	ActiveRequests  int32   `json:"activeRequests"`
	CurrentMemoryKB int64   `json:"currentMemoryKB"`
	AvgCPUTimeMs    float64 `json:"avgCpuTimeMs"`
	AvgMemoryKB     float64 `json:"avgMemoryKB"`
}

// GetStats returns current workload statistics.
func (w *Workload) GetStats() Stats {
	total := w.totalRequests.Load()
	var avgCPU, avgMem float64
	if total > 0 {
		avgCPU = float64(w.totalCPUTimeNs.Load()) / float64(total) / 1e6
		avgMem = float64(w.totalMemoryKB.Load()) / float64(total)
	}

	return Stats{
		TotalRequests:   total,
		ActiveRequests:  w.activeRequests.Load(),
		CurrentMemoryKB: w.currentMemoryKB.Load(),
		AvgCPUTimeMs:    avgCPU,
		AvgMemoryKB:     avgMem,
	}
}

// CurrentMemoryBytes returns the current memory usage in bytes.
func (w *Workload) CurrentMemoryBytes() int64 {
	return w.currentMemoryKB.Load() * 1024
}
