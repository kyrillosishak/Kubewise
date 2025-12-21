// Package worker provides CPU work generation engines for the cpu-burster component.
package worker

import (
	"context"
	"math"
	"sync"
	"time"
)

var (
	waveStartTime time.Time
	waveMu        sync.Mutex
)

// runWave generates sinusoidal CPU usage patterns over configurable periods.
func (w *Worker) runWave(ctx context.Context, workerID int) {
	waveMu.Lock()
	defer waveMu.Unlock()

	w.mu.RLock()
	waveMin := w.config.WaveMin
	waveMax := w.config.WaveMax
	wavePeriod := w.config.WavePeriod
	workers := w.config.Workers
	w.mu.RUnlock()

	now := time.Now()

	// Initialize wave start time
	if waveStartTime.IsZero() {
		waveStartTime = now
	}

	// Calculate position in wave cycle (0 to 2*PI)
	elapsed := now.Sub(waveStartTime)
	cyclePosition := float64(elapsed.Nanoseconds()) / float64(wavePeriod.Nanoseconds())
	angle := cyclePosition * 2 * math.Pi

	// Calculate current target using sine wave
	// sin ranges from -1 to 1, so we map it to waveMin to waveMax
	amplitude := float64(waveMax-waveMin) / 2
	midpoint := float64(waveMin+waveMax) / 2
	currentTarget := int(midpoint + amplitude*math.Sin(angle))

	// Clamp to valid range
	if currentTarget < waveMin {
		currentTarget = waveMin
	}
	if currentTarget > waveMax {
		currentTarget = waveMax
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

// ResetWaveTimer resets the wave timer (useful when switching modes).
func (w *Worker) ResetWaveTimer() {
	waveMu.Lock()
	defer waveMu.Unlock()
	waveStartTime = time.Time{}
}
