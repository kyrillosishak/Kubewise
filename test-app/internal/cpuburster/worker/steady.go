// Package worker provides CPU work generation engines for the cpu-burster component.
package worker

import (
	"context"
	"time"
)

// runSteady maintains constant CPU utilization at a configurable percentage.
func (w *Worker) runSteady(ctx context.Context, workerID int) {
	w.mu.RLock()
	targetPercent := w.config.TargetPercent
	workers := w.config.Workers
	w.mu.RUnlock()

	// Distribute work across workers
	perWorkerPercent := targetPercent / workers
	if workerID == 0 {
		// First worker takes any remainder
		perWorkerPercent += targetPercent % workers
	}

	if perWorkerPercent <= 0 {
		time.Sleep(100 * time.Millisecond)
		return
	}

	// Calculate work/sleep ratio to achieve target CPU
	// In a 100ms window, work for perWorkerPercent ms
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

	// Update usage metric (only from worker 0 to avoid contention)
	if workerID == 0 {
		w.updateUsage(float64(targetPercent))
	}
}
