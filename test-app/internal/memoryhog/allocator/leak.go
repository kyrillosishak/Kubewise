// Package allocator provides memory allocation engines for the memory-hog component.
package allocator

import (
	"sync"
	"time"
)

var (
	leakLastAlloc time.Time
	leakMu        sync.Mutex
)

// runLeak increases memory consumption by configurable rate (default 10MB/min).
func (a *Allocator) runLeak() {
	leakMu.Lock()
	defer leakMu.Unlock()

	a.mu.RLock()
	leakRateMBMin := a.config.LeakRateMBMin
	a.mu.RUnlock()

	if leakRateMBMin <= 0 {
		leakRateMBMin = 10 // Default 10MB/min
	}

	// Calculate how often to allocate 1MB based on leak rate
	// If rate is 10MB/min, we allocate 1MB every 6 seconds
	allocInterval := time.Minute / time.Duration(leakRateMBMin)

	now := time.Now()
	if leakLastAlloc.IsZero() {
		leakLastAlloc = now
	}

	if now.Sub(leakLastAlloc) >= allocInterval {
		a.allocate(1) // Allocate 1MB at a time for smooth leak
		leakLastAlloc = now
	}
}

// ResetLeakTimer resets the leak timer (useful when switching modes).
func (a *Allocator) ResetLeakTimer() {
	leakMu.Lock()
	defer leakMu.Unlock()
	leakLastAlloc = time.Time{}
}
