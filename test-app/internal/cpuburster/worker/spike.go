// Package worker provides CPU work generation engines for the cpu-burster component.
package worker

import (
	"context"
	"sync"
	"time"
)

var (
	spikeLastTrigger time.Time
	spikeActive      bool
	spikeStartTime   time.Time
	spikeMu          sync.Mutex
)

// runSpike generates CPU bursts exceeding 3 standard deviations from baseline.
// Between spikes, maintains baseline CPU usage at targetPercent.
func (w *Worker) runSpike(ctx context.Context, workerID int) {
	spikeMu.Lock()
	defer spikeMu.Unlock()

	w.mu.RLock()
	targetPercent := w.config.TargetPercent
	spikePercent := w.config.SpikePercent
	spikeInterval := w.config.SpikeInterval
	spikeDuration := w.config.SpikeDuration
	workers := w.config.Workers
	w.mu.RUnlock()

	now := time.Now()

	// Initialize last trigger time
	if spikeLastTrigger.IsZero() {
		spikeLastTrigger = now
	}

	// Determine current CPU target based on spike state
	var currentTarget int

	// Check if we're in an active spike
	if spikeActive {
		// Check if spike duration has elapsed
		if now.Sub(spikeStartTime) >= spikeDuration {
			// End spike
			spikeActive = false
			spikeLastTrigger = now
			currentTarget = targetPercent
		} else {
			// Still in spike
			currentTarget = spikePercent
		}
	} else {
		// Check if it's time for a new spike
		if now.Sub(spikeLastTrigger) >= spikeInterval {
			// Start spike
			spikeActive = true
			spikeStartTime = now
			currentTarget = spikePercent
		} else {
			// Between spikes - maintain baseline
			currentTarget = targetPercent
		}
	}

	// Distribute work across workers
	perWorkerPercent := currentTarget / workers
	if workerID == 0 {
		perWorkerPercent += currentTarget % workers
	}

	if perWorkerPercent <= 0 {
		time.Sleep(100 * time.Millisecond)
		return
	}

	// Execute work
	workDuration := time.Duration(perWorkerPercent) * time.Millisecond
	sleepDuration := time.Duration(100-perWorkerPercent) * time.Millisecond

	select {
	case <-ctx.Done():
		return
	default:
		w.doWork(workDuration)
		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
	}

	// Update usage metric
	if workerID == 0 {
		w.updateUsage(float64(currentTarget))
	}
}

// ResetSpikeTimer resets the spike timer (useful when switching modes).
func (w *Worker) ResetSpikeTimer() {
	spikeMu.Lock()
	defer spikeMu.Unlock()
	spikeLastTrigger = time.Time{}
	spikeActive = false
	spikeStartTime = time.Time{}
}
