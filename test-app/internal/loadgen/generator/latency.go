// Package generator provides HTTP load generation engines for the load-generator component.
package generator

import (
	"sort"
	"sync"
)

// latencyTracker tracks latency values for percentile calculations.
type latencyTracker struct {
	mu       sync.RWMutex
	values   []float64
	maxSize  int
	total    float64
	count    int64
}

// newLatencyTracker creates a new latency tracker with the given max size.
func newLatencyTracker(maxSize int) *latencyTracker {
	return &latencyTracker{
		values:  make([]float64, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a latency value to the tracker.
func (t *latencyTracker) Add(latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.total += latencyMs
	t.count++

	// Keep a sliding window of recent values for percentile calculation
	if len(t.values) >= t.maxSize {
		// Remove oldest value
		t.values = t.values[1:]
	}
	t.values = append(t.values, latencyMs)
}

// Average returns the average latency.
func (t *latencyTracker) Average() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.count == 0 {
		return 0
	}
	return t.total / float64(t.count)
}

// Percentile returns the given percentile of latencies.
func (t *latencyTracker) Percentile(p int) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.values) == 0 {
		return 0
	}

	// Make a copy and sort
	sorted := make([]float64, len(t.values))
	copy(sorted, t.values)
	sort.Float64s(sorted)

	// Calculate index
	idx := int(float64(len(sorted)-1) * float64(p) / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Reset clears all tracked values.
func (t *latencyTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.values = t.values[:0]
	t.total = 0
	t.count = 0
}

// Count returns the total number of latencies recorded.
func (t *latencyTracker) Count() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}
