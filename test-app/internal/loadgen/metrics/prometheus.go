// Package metrics provides Prometheus metrics for the load-generator component.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds load-generator specific Prometheus metrics.
type Metrics struct {
	// Request metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Throughput metrics
	CurrentRPS prometheus.Gauge
	TargetRPS  prometheus.Gauge

	// Success/failure metrics
	SuccessTotal prometheus.Counter
	FailureTotal prometheus.Counter
	SuccessRate  prometheus.Gauge

	// Latency percentiles (gauges for current values)
	LatencyP50 prometheus.Gauge
	LatencyP95 prometheus.Gauge
	LatencyP99 prometheus.Gauge
	LatencyAvg prometheus.Gauge

	// Mode and status
	GeneratorRunning prometheus.Gauge
	CurrentMode      *prometheus.GaugeVec
}

// New creates a new Metrics instance with all load-generator metrics.
func New() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "loadgen_requests_total",
				Help: "Total number of HTTP requests sent",
			},
			[]string{"status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "loadgen_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"status"},
		),
		RequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_requests_in_flight",
				Help: "Number of HTTP requests currently in flight",
			},
		),
		CurrentRPS: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_current_rps",
				Help: "Current requests per second being sent",
			},
		),
		TargetRPS: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_target_rps",
				Help: "Target requests per second",
			},
		),
		SuccessTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "loadgen_success_total",
				Help: "Total number of successful requests",
			},
		),
		FailureTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "loadgen_failure_total",
				Help: "Total number of failed requests",
			},
		),
		SuccessRate: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_success_rate",
				Help: "Current success rate (0-1)",
			},
		),
		LatencyP50: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_latency_p50_ms",
				Help: "50th percentile latency in milliseconds",
			},
		),
		LatencyP95: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_latency_p95_ms",
				Help: "95th percentile latency in milliseconds",
			},
		),
		LatencyP99: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_latency_p99_ms",
				Help: "99th percentile latency in milliseconds",
			},
		),
		LatencyAvg: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_latency_avg_ms",
				Help: "Average latency in milliseconds",
			},
		),
		GeneratorRunning: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "loadgen_running",
				Help: "Whether the load generator is running (1) or stopped (0)",
			},
		),
		CurrentMode: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "loadgen_mode",
				Help: "Current load generation mode (1 for active mode)",
			},
			[]string{"mode"},
		),
	}
}

// SetMode sets the current mode metric.
func (m *Metrics) SetMode(mode string) {
	// Reset all modes
	m.CurrentMode.WithLabelValues("constant").Set(0)
	m.CurrentMode.WithLabelValues("ramp-up").Set(0)
	m.CurrentMode.WithLabelValues("ramp-down").Set(0)
	m.CurrentMode.WithLabelValues("burst").Set(0)
	// Set active mode
	m.CurrentMode.WithLabelValues(mode).Set(1)
}

// RecordRequest records a request with its status and duration.
func (m *Metrics) RecordRequest(success bool, durationSeconds float64) {
	status := "success"
	if !success {
		status = "failure"
		m.FailureTotal.Inc()
	} else {
		m.SuccessTotal.Inc()
	}
	m.RequestsTotal.WithLabelValues(status).Inc()
	m.RequestDuration.WithLabelValues(status).Observe(durationSeconds)
}

// UpdateStats updates all gauge metrics from stats.
func (m *Metrics) UpdateStats(currentRPS, targetRPS float64, successRate float64,
	latencyAvg, latencyP50, latencyP95, latencyP99 float64, running bool) {
	m.CurrentRPS.Set(currentRPS)
	m.TargetRPS.Set(targetRPS)
	m.SuccessRate.Set(successRate)
	m.LatencyAvg.Set(latencyAvg)
	m.LatencyP50.Set(latencyP50)
	m.LatencyP95.Set(latencyP95)
	m.LatencyP99.Set(latencyP99)
	if running {
		m.GeneratorRunning.Set(1)
	} else {
		m.GeneratorRunning.Set(0)
	}
}
