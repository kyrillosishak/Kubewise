// Package metrics provides Prometheus metrics for the metrics validator.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/container-resource-predictor/test-app/internal/metricsvalidator/validator"
)

// Metrics holds Prometheus metrics for the validator.
type Metrics struct {
	// Prediction accuracy
	PredictionAccuracy *prometheus.GaugeVec
	PredictionTotal    *prometheus.CounterVec
	AccuratePredictions *prometheus.CounterVec

	// Anomaly detection
	AnomalyTotal          *prometheus.CounterVec
	AnomalyDetected       *prometheus.CounterVec
	AnomalyDetectionTime  *prometheus.HistogramVec
	AnomalyFalsePositives *prometheus.CounterVec
	AnomalyFalseNegatives *prometheus.CounterVec

	// Cost validation
	CostAccuracy     prometheus.Gauge
	CostDeviation    prometheus.Gauge
	CostValidations  prometheus.Counter

	// Overall
	ValidationCycles prometheus.Counter
	LastValidation   prometheus.Gauge
}

// NewMetrics creates new Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		PredictionAccuracy: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_prediction_accuracy",
				Help: "Prediction accuracy percentage",
			},
			[]string{"component", "resource_type"},
		),
		PredictionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_predictions_total",
				Help: "Total number of predictions validated",
			},
			[]string{"component"},
		),
		AccuratePredictions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_accurate_predictions_total",
				Help: "Total number of accurate predictions",
			},
			[]string{"component"},
		),
		AnomalyTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_anomalies_total",
				Help: "Total number of anomalies registered",
			},
			[]string{"anomaly_type"},
		),
		AnomalyDetected: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_anomalies_detected_total",
				Help: "Total number of anomalies detected",
			},
			[]string{"anomaly_type"},
		),
		AnomalyDetectionTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "validator_anomaly_detection_seconds",
				Help:    "Time to detect anomalies in seconds",
				Buckets: []float64{60, 120, 300, 600, 900, 1200, 1800},
			},
			[]string{"anomaly_type"},
		),
		AnomalyFalsePositives: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_anomaly_false_positives_total",
				Help: "Total number of false positive anomaly detections",
			},
			[]string{"anomaly_type"},
		),
		AnomalyFalseNegatives: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_anomaly_false_negatives_total",
				Help: "Total number of false negative anomaly detections",
			},
			[]string{"anomaly_type"},
		),
		CostAccuracy: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "validator_cost_accuracy",
				Help: "Cost estimation accuracy percentage",
			},
		),
		CostDeviation: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "validator_cost_deviation_usd",
				Help: "Average cost deviation in USD",
			},
		),
		CostValidations: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "validator_cost_validations_total",
				Help: "Total number of cost validations",
			},
		),
		ValidationCycles: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "validator_cycles_total",
				Help: "Total number of validation cycles",
			},
		),
		LastValidation: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "validator_last_validation_timestamp",
				Help: "Timestamp of last validation cycle",
			},
		),
	}
}

// RecordPredictionResult records a prediction validation result.
func (m *Metrics) RecordPredictionResult(result validator.ValidationResult) {
	m.PredictionAccuracy.WithLabelValues(result.Component, "cpu").Set(result.CPUAccuracy * 100)
	m.PredictionAccuracy.WithLabelValues(result.Component, "memory").Set(result.MemoryAccuracy * 100)
	m.PredictionTotal.WithLabelValues(result.Component).Inc()
	if result.IsAccurate {
		m.AccuratePredictions.WithLabelValues(result.Component).Inc()
	}
}

// RecordAnomalyRegistered records a registered anomaly.
func (m *Metrics) RecordAnomalyRegistered(anomalyType string) {
	m.AnomalyTotal.WithLabelValues(anomalyType).Inc()
}

// RecordAnomalyDetected records a detected anomaly.
func (m *Metrics) RecordAnomalyDetected(validation *validator.AnomalyValidation) {
	m.AnomalyDetected.WithLabelValues(validation.AnomalyType).Inc()
	m.AnomalyDetectionTime.WithLabelValues(validation.AnomalyType).Observe(validation.TimeToDetect.Seconds())
}

// RecordFalsePositive records a false positive.
func (m *Metrics) RecordFalsePositive(anomalyType string) {
	m.AnomalyFalsePositives.WithLabelValues(anomalyType).Inc()
}

// RecordFalseNegative records a false negative.
func (m *Metrics) RecordFalseNegative(anomalyType string) {
	m.AnomalyFalseNegatives.WithLabelValues(anomalyType).Inc()
}

// RecordCostValidation records a cost validation result.
func (m *Metrics) RecordCostValidation(validation validator.CostValidation) {
	m.CostAccuracy.Set(validation.CostAccuracy * 100)
	m.CostDeviation.Set(validation.EstimatedCost - validation.CalculatedCost)
	m.CostValidations.Inc()
}

// RecordValidationCycle records a validation cycle.
func (m *Metrics) RecordValidationCycle() {
	m.ValidationCycles.Inc()
	m.LastValidation.SetToCurrentTime()
}
