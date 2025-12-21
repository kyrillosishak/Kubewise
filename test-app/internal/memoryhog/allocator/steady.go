// Package allocator provides memory allocation engines for the memory-hog component.
package allocator

// runSteady maintains constant memory allocation within 5% variance.
func (a *Allocator) runSteady() {
	a.mu.RLock()
	targetMB := a.config.TargetMB
	currentMB := int(a.currentUsage / (1024 * 1024))
	a.mu.RUnlock()

	// Calculate 5% variance threshold
	varianceThreshold := targetMB * 5 / 100
	if varianceThreshold < 1 {
		varianceThreshold = 1
	}

	diff := currentMB - targetMB

	// If we're below target minus variance, allocate more
	if diff < -varianceThreshold {
		toAllocate := -diff
		if toAllocate > 10 {
			toAllocate = 10 // Allocate in 10MB chunks max
		}
		a.allocate(toAllocate)
	}

	// If we're above target plus variance, release some
	if diff > varianceThreshold {
		toRelease := diff
		if toRelease > 10 {
			toRelease = 10 // Release in 10MB chunks max
		}
		a.release(toRelease)
	}
}
