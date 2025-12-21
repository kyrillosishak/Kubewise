// Package validator provides validation logic for Kubewise predictions.
package validator

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/collector"
)

// Validator validates Kubewise predictions against actual usage.
type Validator struct {
	kubewise   *collector.KubewiseClient
	prometheus *collector.PrometheusClient
	config     ValidationConfig

	mu                sync.RWMutex
	results           []ValidationResult
	anomalyValidations map[string]*AnomalyValidation
	costValidations   []CostValidation
	running           bool
	stopCh            chan struct{}
}

// New creates a new Validator.
func New(config ValidationConfig) *Validator {
	return &Validator{
		kubewise:           collector.NewKubewiseClient(config.KubewiseURL),
		prometheus:         collector.NewPrometheusClient(config.PrometheusURL),
		config:             config,
		results:            make([]ValidationResult, 0),
		anomalyValidations: make(map[string]*AnomalyValidation),
		costValidations:    make([]CostValidation, 0),
		stopCh:             make(chan struct{}),
	}
}

// Start begins the validation loop.
func (v *Validator) Start(ctx context.Context) error {
	v.mu.Lock()
	if v.running {
		v.mu.Unlock()
		return nil
	}
	v.running = true
	v.mu.Unlock()

	log.Printf("[validator] Starting validation loop with interval %v", v.config.ValidationInterval)

	ticker := time.NewTicker(v.config.ValidationInterval)
	defer ticker.Stop()

	// Run initial validation
	v.runValidation(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-v.stopCh:
			return nil
		case <-ticker.C:
			v.runValidation(ctx)
		}
	}
}

// Stop stops the validation loop.
func (v *Validator) Stop() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.running {
		close(v.stopCh)
		v.running = false
	}
}

func (v *Validator) runValidation(ctx context.Context) {
	log.Printf("[validator] Running validation cycle")

	// Validate predictions
	if err := v.validatePredictions(ctx); err != nil {
		log.Printf("[validator] Error validating predictions: %v", err)
	}

	// Check anomaly detections
	v.checkAnomalyDetections(ctx)

	// Validate costs
	if err := v.validateCosts(ctx); err != nil {
		log.Printf("[validator] Error validating costs: %v", err)
	}
}


func (v *Validator) validatePredictions(ctx context.Context) error {
	recommendations, err := v.kubewise.GetRecommendationsByNamespace(ctx, v.config.TargetNamespace)
	if err != nil {
		return err
	}

	for _, rec := range recommendations {
		usage, err := v.prometheus.GetResourceUsageByDeployment(ctx, rec.Namespace, rec.Deployment)
		if err != nil {
			log.Printf("[validator] Error getting usage for %s/%s: %v", rec.Namespace, rec.Deployment, err)
			continue
		}

		result := v.calculatePredictionAccuracy(rec, usage)
		v.addResult(result)

		log.Printf("[validator] Validated %s: CPU accuracy=%.1f%%, Memory accuracy=%.1f%%, accurate=%v",
			rec.Deployment, result.CPUAccuracy*100, result.MemoryAccuracy*100, result.IsAccurate)
	}

	return nil
}

func (v *Validator) calculatePredictionAccuracy(rec collector.Recommendation, usage *collector.ResourceUsage) ValidationResult {
	// Convert CPU cores to millicores
	actualCPUMillicores := uint32(usage.CPUCores * 1000)

	// Calculate accuracy as 1 - |predicted - actual| / actual
	cpuAccuracy := calculateAccuracy(float64(rec.CPURequestMillicores), float64(actualCPUMillicores))
	memAccuracy := calculateAccuracy(float64(rec.MemoryRequestBytes), float64(usage.MemoryBytes))

	// A prediction is accurate if both CPU and memory are within threshold
	isAccurate := cpuAccuracy >= v.config.AccuracyThreshold && memAccuracy >= v.config.AccuracyThreshold

	return ValidationResult{
		Timestamp:       time.Now(),
		Component:       rec.Deployment,
		Namespace:       rec.Namespace,
		PredictedCPU:    rec.CPURequestMillicores,
		ActualCPU:       actualCPUMillicores,
		CPUAccuracy:     cpuAccuracy,
		PredictedMemory: rec.MemoryRequestBytes,
		ActualMemory:    usage.MemoryBytes,
		MemoryAccuracy:  memAccuracy,
		Confidence:      rec.Confidence,
		IsAccurate:      isAccurate,
	}
}

func calculateAccuracy(predicted, actual float64) float64 {
	if actual == 0 {
		if predicted == 0 {
			return 1.0
		}
		return 0.0
	}
	deviation := math.Abs(predicted-actual) / actual
	accuracy := 1.0 - deviation
	if accuracy < 0 {
		accuracy = 0
	}
	return accuracy
}

func (v *Validator) addResult(result ValidationResult) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.results = append(v.results, result)
	
	// Keep only last 1000 results
	if len(v.results) > 1000 {
		v.results = v.results[len(v.results)-1000:]
	}
}

// GetResults returns all validation results.
func (v *Validator) GetResults() []ValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()
	results := make([]ValidationResult, len(v.results))
	copy(results, v.results)
	return results
}

// GetResultsByComponent returns validation results for a specific component.
func (v *Validator) GetResultsByComponent(component string) []ValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()
	var results []ValidationResult
	for _, r := range v.results {
		if r.Component == component {
			results = append(results, r)
		}
	}
	return results
}
