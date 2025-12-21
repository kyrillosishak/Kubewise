// Package validator provides validation logic for Kubewise predictions.
package validator

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/collector"
)

// Cost calculation constants (simplified pricing model).
const (
	// Cost per CPU core per hour (USD)
	CPUCostPerCoreHour = 0.05
	// Cost per GB memory per hour (USD)
	MemoryCostPerGBHour = 0.01
)

func (v *Validator) validateCosts(ctx context.Context) error {
	// Get Kubewise cost analysis
	costs, err := v.kubewise.GetCostsByNamespace(ctx, v.config.TargetNamespace)
	if err != nil {
		return err
	}

	// Get actual resource usage from Prometheus
	cpuUsage, err := v.prometheus.GetCPUUsage(ctx, v.config.TargetNamespace)
	if err != nil {
		return err
	}

	memUsage, err := v.prometheus.GetMemoryUsage(ctx, v.config.TargetNamespace)
	if err != nil {
		return err
	}

	// Calculate actual costs based on usage
	calculatedCost := v.calculateActualCost(cpuUsage, memUsage)

	// Calculate accuracy
	costAccuracy := calculateAccuracy(costs.CurrentMonthlyCost, calculatedCost)
	isAccurate := costAccuracy >= v.config.CostAccuracyThreshold

	validation := CostValidation{
		Timestamp:      time.Now(),
		Namespace:      v.config.TargetNamespace,
		EstimatedCost:  costs.CurrentMonthlyCost,
		CalculatedCost: calculatedCost,
		CostAccuracy:   costAccuracy,
		EstimatedSavings:  costs.PotentialSavings,
		CalculatedSavings: costs.CurrentMonthlyCost - costs.RecommendedMonthlyCost,
		SavingsAccuracy:   calculateAccuracy(costs.PotentialSavings, costs.CurrentMonthlyCost-costs.RecommendedMonthlyCost),
		IsAccurate:        isAccurate,
	}

	v.addCostValidation(validation)

	log.Printf("[validator] Cost validation: estimated=$%.2f, calculated=$%.2f, accuracy=%.1f%%, accurate=%v",
		validation.EstimatedCost, validation.CalculatedCost, validation.CostAccuracy*100, validation.IsAccurate)

	return nil
}

func (v *Validator) calculateActualCost(cpuUsage, memUsage []collector.ResourceUsage) float64 {
	var totalCPUCores float64
	var totalMemoryGB float64

	// Sum up CPU usage
	for _, usage := range cpuUsage {
		totalCPUCores += usage.CPUCores
	}

	// Sum up memory usage
	for _, usage := range memUsage {
		totalMemoryGB += float64(usage.MemoryBytes) / (1024 * 1024 * 1024)
	}

	// Calculate monthly cost (assuming 730 hours per month)
	hoursPerMonth := 730.0
	cpuCost := totalCPUCores * CPUCostPerCoreHour * hoursPerMonth
	memoryCost := totalMemoryGB * MemoryCostPerGBHour * hoursPerMonth

	return cpuCost + memoryCost
}

func (v *Validator) addCostValidation(validation CostValidation) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.costValidations = append(v.costValidations, validation)

	// Keep only last 100 cost validations
	if len(v.costValidations) > 100 {
		v.costValidations = v.costValidations[len(v.costValidations)-100:]
	}
}

// GetCostValidations returns all cost validations.
func (v *Validator) GetCostValidations() []CostValidation {
	v.mu.RLock()
	defer v.mu.RUnlock()
	validations := make([]CostValidation, len(v.costValidations))
	copy(validations, v.costValidations)
	return validations
}

// CalculateCostStats calculates cost validation statistics.
func (v *Validator) CalculateCostStats() CostStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.costValidations) == 0 {
		return CostStats{}
	}

	var totalAccuracy float64
	var accurateCount int
	var totalDeviation float64

	for _, val := range v.costValidations {
		totalAccuracy += val.CostAccuracy
		if val.IsAccurate {
			accurateCount++
		}
		totalDeviation += math.Abs(val.EstimatedCost - val.CalculatedCost)
	}

	count := float64(len(v.costValidations))
	return CostStats{
		TotalValidations: len(v.costValidations),
		AccurateCount:    accurateCount,
		AverageAccuracy:  totalAccuracy / count,
		AccuracyRate:     float64(accurateCount) / count,
		AverageDeviation: totalDeviation / count,
	}
}

// CostStats holds statistics about cost validation.
type CostStats struct {
	TotalValidations int     `json:"total_validations"`
	AccurateCount    int     `json:"accurate_count"`
	AverageAccuracy  float64 `json:"average_accuracy"`
	AccuracyRate     float64 `json:"accuracy_rate"`
	AverageDeviation float64 `json:"average_deviation_usd"`
}
