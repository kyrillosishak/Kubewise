// Package allocator provides memory allocation engines for the memory-hog component.
package allocator

import (
	"sync"
	"time"
)

var (
	spikeLastTrigger time.Time
	spikeActive      bool
	spikeStartTime   time.Time
	spikeMu          sync.Mutex
)

// runSpike periodically allocates large memory blocks then releases them.
func (a *Allocator) runSpike() {
	spikeMu.Lock()
	defer spikeMu.Unlock()

	a.mu.RLock()
	spikeInterval := a.config.SpikeInterval
	spikeDuration := a.config.SpikeDuration
	spikeSizeMB := a.config.SpikeSizeMB
	targetMB := a.config.TargetMB
	currentMB := int(a.currentUsage / (1024 * 1024))
	a.mu.RUnlock()

	now := time.Now()

	// Initialize last trigger time
	if spikeLastTrigger.IsZero() {
		spikeLastTrigger = now
	}

	// Check if we're in an active spike
	if spikeActive {
		// Check if spike duration has elapsed
		if now.Sub(spikeStartTime) >= spikeDuration {
			// End spike - release the spike memory
			a.release(spikeSizeMB)
			spikeActive = false
			spikeLastTrigger = now
		}
		return
	}

	// Check if it's time for a new spike
	if now.Sub(spikeLastTrigger) >= spikeInterval {
		// Start spike - allocate spike memory
		a.allocate(spikeSizeMB)
		spikeActive = true
		spikeStartTime = now
		return
	}

	// Between spikes, maintain baseline memory (similar to steady mode)
	varianceThreshold := targetMB * 5 / 100
	if varianceThreshold < 1 {
		varianceThreshold = 1
	}

	diff := currentMB - targetMB

	if diff < -varianceThreshold {
		toAllocate := -diff
		if toAllocate > 10 {
			toAllocate = 10
		}
		a.allocate(toAllocate)
	}

	if diff > varianceThreshold {
		toRelease := diff
		if toRelease > 10 {
			toRelease = 10
		}
		a.release(toRelease)
	}
}

// ResetSpikeTimer resets the spike timer (useful when switching modes).
func (a *Allocator) ResetSpikeTimer() {
	spikeMu.Lock()
	defer spikeMu.Unlock()
	spikeLastTrigger = time.Time{}
	spikeActive = false
	spikeStartTime = time.Time{}
}
