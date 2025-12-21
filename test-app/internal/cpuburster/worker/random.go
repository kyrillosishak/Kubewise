// Package worker provides CPU work generation engines for the cpu-burster component.
package worker

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

var (
	randomLastChange  time.Time
	randomTarget      int
	randomMu          sync.Mutex
	randomChangeEvery = 5 * time.Second // Change target every 5 seconds
)

// runRandom generates unpredictable CPU usage for testing prediction robustness.
func (w *Worker) runRandom(ctx context.Context, workerID int) {
	randomMu.Lock()
	defer randomMu.Unlock()

	w.mu.RLock()
	waveMin := w.config.WaveMin // Use wave min/max as bounds for random
	waveMax := w.config.WaveMax
	workers := w.config.Workers
	w.mu.RUnlock()

	now := time.Now()

	// Initialize or update random target
	if randomLastChange.IsZero() || now.Sub(randomLastChange) >= randomChangeEvery {
		// Generate new random target between waveMin and waveMax
		if waveMax > waveMin {
			randomTarget = waveMin + rand.Intn(waveMax-waveMin+1)
		} else {
			randomTarget = waveMin
		}
		randomLastChange = now
	}

	currentTarget := randomTarget

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

// ResetRandomTimer resets the random timer (useful when switching modes).
func (w *Worker) ResetRandomTimer() {
	randomMu.Lock()
	defer randomMu.Unlock()
	randomLastChange = time.Time{}
	randomTarget = 0
}
