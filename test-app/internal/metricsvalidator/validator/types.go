// Package validator provides validation logic for Kubewise predictions.
package validator

import "time"

// ValidationResult represents the result of validating a prediction.
type ValidationResult struct {
	Timestamp       time.Time `json:"timestamp"`
	Component       string    `json:"component"`
	Namespace       string    `json:"namespace"`
	PredictedCPU    uint32    `json:"predicted_cpu_millicores"`
	ActualCPU       uint32    `json:"actual_cpu_millicores"`
	CPUAccuracy     float64   `json:"cpu_accuracy"`
	PredictedMemory uint64    `json:"predicted_memory_bytes"`
	ActualMemory    uint64    `json:"actual_memory_bytes"`
	MemoryAccuracy  float64   `json:"memory_accuracy"`
	Confidence      float32   `json:"confidence"`
	IsAccurate      bool      `json:"is_accurate"`
}

// AnomalyValidation represents the validation of anomaly detection.
type AnomalyValidation struct {
	ID            string        `json:"id"`
	AnomalyType   string        `json:"anomaly_type"` // memory_leak, cpu_spike
	Component     string        `json:"component"`
	TriggeredAt   time.Time     `json:"triggered_at"`
	DetectedAt    *time.Time    `json:"detected_at,omitempty"`
	TimeToDetect  time.Duration `json:"time_to_detect,omitempty"`
	WasDetected   bool          `json:"was_detected"`
	FalsePositive bool          `json:"false_positive"`
	ExpectedBy    time.Time     `json:"expected_by"`
}

// CostValidation represents the validation of cost estimation.
type CostValidation struct {
	Timestamp          time.Time `json:"timestamp"`
	Namespace          string    `json:"namespace"`
	EstimatedCost      float64   `json:"estimated_cost"`
	CalculatedCost     float64   `json:"calculated_cost"`
	CostAccuracy       float64   `json:"cost_accuracy"`
	EstimatedSavings   float64   `json:"estimated_savings"`
	CalculatedSavings  float64   `json:"calculated_savings"`
	SavingsAccuracy    float64   `json:"savings_accuracy"`
	IsAccurate         bool      `json:"is_accurate"`
}

// ValidationConfig holds configuration for the validator.
type ValidationConfig struct {
	KubewiseURL           string        `json:"kubewise_url"`
	PrometheusURL         string        `json:"prometheus_url"`
	ValidationInterval    time.Duration `json:"validation_interval"`
	AccuracyThreshold     float64       `json:"accuracy_threshold"`      // Default 0.7 (70%)
	CostAccuracyThreshold float64       `json:"cost_accuracy_threshold"` // Default 0.8 (80%)
	LeakDetectionTimeout  time.Duration `json:"leak_detection_timeout"`  // Default 30m
	SpikeDetectionTimeout time.Duration `json:"spike_detection_timeout"` // Default 5m
	TargetNamespace       string        `json:"target_namespace"`
}

// DefaultConfig returns the default validation configuration.
func DefaultConfig() ValidationConfig {
	return ValidationConfig{
		KubewiseURL:           "http://kubewise-api.kubewise-system:8080",
		PrometheusURL:         "http://prometheus.monitoring:9090",
		ValidationInterval:    5 * time.Minute,
		AccuracyThreshold:     0.7,
		CostAccuracyThreshold: 0.8,
		LeakDetectionTimeout:  30 * time.Minute,
		SpikeDetectionTimeout: 5 * time.Minute,
		TargetNamespace:       "kubewise-test",
	}
}

// AnomalyType constants.
const (
	AnomalyTypeMemoryLeak = "memory_leak"
	AnomalyTypeCPUSpike   = "cpu_spike"
)
