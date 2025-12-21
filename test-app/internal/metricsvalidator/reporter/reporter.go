// Package reporter provides report generation for validation results.
package reporter

import (
	"fmt"
	"time"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/validator"
)

// ValidationReport represents a complete validation report.
type ValidationReport struct {
	GeneratedAt   time.Time `json:"generated_at"`
	TestDuration  string    `json:"test_duration"`
	ScenarioName  string    `json:"scenario_name"`

	// Prediction Accuracy
	OverallCPUAccuracy    float64 `json:"overall_cpu_accuracy"`
	OverallMemAccuracy    float64 `json:"overall_mem_accuracy"`
	ConfidenceCorrelation float64 `json:"confidence_correlation"`
	TotalPredictions      int     `json:"total_predictions"`
	AccuratePredictions   int     `json:"accurate_predictions"`

	// Anomaly Detection
	AnomalyStats validator.AnomalyStats `json:"anomaly_stats"`

	// Cost Estimation
	CostStats validator.CostStats `json:"cost_stats"`

	// Overall
	PassCriteria []PassCriterion `json:"pass_criteria"`
	OverallPass  bool            `json:"overall_pass"`
}

// PassCriterion represents a single pass/fail criterion.
type PassCriterion struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Pass     bool   `json:"pass"`
}

// Reporter generates validation reports.
type Reporter struct {
	validator    *validator.Validator
	scenarioName string
	startTime    time.Time
	history      []ValidationReport
}

// New creates a new Reporter.
func New(v *validator.Validator) *Reporter {
	return &Reporter{
		validator: v,
		startTime: time.Now(),
		history:   make([]ValidationReport, 0),
	}
}

// SetScenario sets the current scenario name.
func (r *Reporter) SetScenario(name string) {
	r.scenarioName = name
	r.startTime = time.Now()
}

// GenerateReport generates a validation report.
func (r *Reporter) GenerateReport() ValidationReport {
	results := r.validator.GetResults()
	anomalyStats := r.validator.CalculateAnomalyStats()
	costStats := r.validator.CalculateCostStats()

	// Calculate prediction accuracy
	var totalCPUAccuracy, totalMemAccuracy float64
	var accurateCount int
	for _, result := range results {
		totalCPUAccuracy += result.CPUAccuracy
		totalMemAccuracy += result.MemoryAccuracy
		if result.IsAccurate {
			accurateCount++
		}
	}

	var avgCPUAccuracy, avgMemAccuracy float64
	if len(results) > 0 {
		avgCPUAccuracy = totalCPUAccuracy / float64(len(results))
		avgMemAccuracy = totalMemAccuracy / float64(len(results))
	}

	// Build pass criteria
	passCriteria := r.buildPassCriteria(avgCPUAccuracy, avgMemAccuracy, anomalyStats, costStats)

	// Determine overall pass
	overallPass := true
	for _, criterion := range passCriteria {
		if !criterion.Pass {
			overallPass = false
			break
		}
	}

	report := ValidationReport{
		GeneratedAt:           time.Now(),
		TestDuration:          time.Since(r.startTime).String(),
		ScenarioName:          r.scenarioName,
		OverallCPUAccuracy:    avgCPUAccuracy * 100,
		OverallMemAccuracy:    avgMemAccuracy * 100,
		ConfidenceCorrelation: r.calculateConfidenceCorrelation(results),
		TotalPredictions:      len(results),
		AccuratePredictions:   accurateCount,
		AnomalyStats:          anomalyStats,
		CostStats:             costStats,
		PassCriteria:          passCriteria,
		OverallPass:           overallPass,
	}

	// Store in history
	r.history = append(r.history, report)
	if len(r.history) > 100 {
		r.history = r.history[len(r.history)-100:]
	}

	return report
}


func (r *Reporter) buildPassCriteria(cpuAccuracy, memAccuracy float64, anomalyStats validator.AnomalyStats, costStats validator.CostStats) []PassCriterion {
	criteria := []PassCriterion{
		{
			Name:     "CPU Prediction Accuracy",
			Expected: ">= 70%",
			Actual:   fmt.Sprintf("%.1f%%", cpuAccuracy*100),
			Pass:     cpuAccuracy >= 0.70,
		},
		{
			Name:     "Memory Prediction Accuracy",
			Expected: ">= 70%",
			Actual:   fmt.Sprintf("%.1f%%", memAccuracy*100),
			Pass:     memAccuracy >= 0.70,
		},
		{
			Name:     "Anomaly Detection Rate",
			Expected: ">= 90%",
			Actual:   fmt.Sprintf("%.1f%%", anomalyStats.DetectionRate*100),
			Pass:     anomalyStats.DetectionRate >= 0.90 || anomalyStats.Total == 0,
		},
		{
			Name:     "False Positive Rate",
			Expected: "<= 10%",
			Actual:   fmt.Sprintf("%.1f%%", anomalyStats.FalsePositiveRate*100),
			Pass:     anomalyStats.FalsePositiveRate <= 0.10,
		},
		{
			Name:     "Cost Estimation Accuracy",
			Expected: ">= 80%",
			Actual:   fmt.Sprintf("%.1f%%", costStats.AverageAccuracy*100),
			Pass:     costStats.AverageAccuracy >= 0.80 || costStats.TotalValidations == 0,
		},
	}

	// Add memory leak detection time if applicable
	if leakStats, ok := anomalyStats.ByType[validator.AnomalyTypeMemoryLeak]; ok && leakStats.Total > 0 {
		criteria = append(criteria, PassCriterion{
			Name:     "Memory Leak Detection Time",
			Expected: "<= 30 minutes",
			Actual:   leakStats.AvgDetectionTime.String(),
			Pass:     leakStats.AvgDetectionTime <= 30*time.Minute || leakStats.Detected == 0,
		})
	}

	// Add CPU spike detection time if applicable
	if spikeStats, ok := anomalyStats.ByType[validator.AnomalyTypeCPUSpike]; ok && spikeStats.Total > 0 {
		criteria = append(criteria, PassCriterion{
			Name:     "CPU Spike Detection Time",
			Expected: "<= 5 minutes",
			Actual:   spikeStats.AvgDetectionTime.String(),
			Pass:     spikeStats.AvgDetectionTime <= 5*time.Minute || spikeStats.Detected == 0,
		})
	}

	return criteria
}

func (r *Reporter) calculateConfidenceCorrelation(results []validator.ValidationResult) float64 {
	if len(results) < 2 {
		return 0
	}

	// Simple correlation: check if higher confidence correlates with higher accuracy
	var highConfAccurate, highConfTotal int
	var lowConfAccurate, lowConfTotal int

	for _, result := range results {
		if result.Confidence >= 0.7 {
			highConfTotal++
			if result.IsAccurate {
				highConfAccurate++
			}
		} else {
			lowConfTotal++
			if result.IsAccurate {
				lowConfAccurate++
			}
		}
	}

	var highConfRate, lowConfRate float64
	if highConfTotal > 0 {
		highConfRate = float64(highConfAccurate) / float64(highConfTotal)
	}
	if lowConfTotal > 0 {
		lowConfRate = float64(lowConfAccurate) / float64(lowConfTotal)
	}

	// Correlation is positive if high confidence has higher accuracy rate
	if highConfTotal == 0 || lowConfTotal == 0 {
		return 0
	}
	return highConfRate - lowConfRate
}

// GetHistory returns the report history.
func (r *Reporter) GetHistory() []ValidationReport {
	history := make([]ValidationReport, len(r.history))
	copy(history, r.history)
	return history
}

// GetLatestReport returns the most recent report.
func (r *Reporter) GetLatestReport() *ValidationReport {
	if len(r.history) == 0 {
		return nil
	}
	return &r.history[len(r.history)-1]
}
