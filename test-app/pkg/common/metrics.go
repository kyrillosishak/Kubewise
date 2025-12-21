// Package common provides shared metrics utilities for all test-app components.
package common

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds common Prometheus metrics used across components.
type Metrics struct {
	CPUUsage       prometheus.Gauge
	MemoryUsage    prometheus.Gauge
	RequestsTotal  prometheus.Counter
	RequestLatency prometheus.Histogram
	ErrorsTotal    prometheus.Counter
	ComponentReady prometheus.Gauge
}

// NewMetrics creates a new Metrics instance with the given component name.
func NewMetrics(component string) *Metrics {
	labels := prometheus.Labels{"component": component}

	return &Metrics{
		CPUUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "testapp_cpu_usage_percent",
			Help:        "Current CPU usage percentage",
			ConstLabels: labels,
		}),
		MemoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "testapp_memory_usage_bytes",
			Help:        "Current memory usage in bytes",
			ConstLabels: labels,
		}),
		RequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "testapp_requests_total",
			Help:        "Total number of requests processed",
			ConstLabels: labels,
		}),
		RequestLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "testapp_request_latency_seconds",
			Help:        "Request latency in seconds",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}),
		ErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "testapp_errors_total",
			Help:        "Total number of errors",
			ConstLabels: labels,
		}),
		ComponentReady: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "testapp_component_ready",
			Help:        "Whether the component is ready (1) or not (0)",
			ConstLabels: labels,
		}),
	}
}

// SetReady marks the component as ready.
func (m *Metrics) SetReady() {
	m.ComponentReady.Set(1)
}

// SetNotReady marks the component as not ready.
func (m *Metrics) SetNotReady() {
	m.ComponentReady.Set(0)
}
