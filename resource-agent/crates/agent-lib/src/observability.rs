//! Observability infrastructure for the resource agent
//!
//! Provides:
//! - Prometheus metrics (collection latency, prediction latency, buffer size, model version)
//! - Structured JSON logging with tracing

use prometheus::{
    register_gauge_vec, register_histogram, register_int_gauge, GaugeVec, Histogram, IntGauge,
};
use std::sync::OnceLock;
use tracing::{info, warn};

/// Default histogram buckets for latency measurements (in seconds)
const LATENCY_BUCKETS: &[f64] = &[
    0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0,
];

/// Global metrics instance (registered once)
static GLOBAL_METRICS: OnceLock<AgentMetricsInner> = OnceLock::new();

/// Inner metrics structure that holds the actual Prometheus metrics
struct AgentMetricsInner {
    collection_latency_seconds: Histogram,
    prediction_latency_seconds: Histogram,
    buffer_size_bytes: IntGauge,
    buffer_items: IntGauge,
    model_version_info: GaugeVec,
    containers_monitored: IntGauge,
    predictions_generated: IntGauge,
    anomalies_detected: IntGauge,
    collection_errors: IntGauge,
    prediction_errors: IntGauge,
}

impl AgentMetricsInner {
    fn new() -> Self {
        Self {
            collection_latency_seconds: register_histogram!(
                "resource_agent_collection_latency_seconds",
                "Time spent collecting metrics from cgroups",
                LATENCY_BUCKETS.to_vec()
            )
            .expect("Failed to register collection_latency_seconds"),

            prediction_latency_seconds: register_histogram!(
                "resource_agent_prediction_latency_seconds",
                "Time spent running ML inference for predictions",
                LATENCY_BUCKETS.to_vec()
            )
            .expect("Failed to register prediction_latency_seconds"),

            buffer_size_bytes: register_int_gauge!(
                "resource_agent_buffer_size_bytes",
                "Current size of the local metrics buffer in bytes"
            )
            .expect("Failed to register buffer_size_bytes"),

            buffer_items: register_int_gauge!(
                "resource_agent_buffer_items",
                "Number of metric samples in the local buffer"
            )
            .expect("Failed to register buffer_items"),

            model_version_info: register_gauge_vec!(
                "resource_agent_model_version_info",
                "Information about the currently loaded ML model",
                &["version", "quantization"]
            )
            .expect("Failed to register model_version_info"),

            containers_monitored: register_int_gauge!(
                "resource_agent_containers_monitored",
                "Number of containers currently being monitored"
            )
            .expect("Failed to register containers_monitored"),

            predictions_generated: register_int_gauge!(
                "resource_agent_predictions_generated_total",
                "Total number of predictions generated"
            )
            .expect("Failed to register predictions_generated"),

            anomalies_detected: register_int_gauge!(
                "resource_agent_anomalies_detected_total",
                "Total number of anomalies detected"
            )
            .expect("Failed to register anomalies_detected"),

            collection_errors: register_int_gauge!(
                "resource_agent_collection_errors_total",
                "Total number of metric collection errors"
            )
            .expect("Failed to register collection_errors"),

            prediction_errors: register_int_gauge!(
                "resource_agent_prediction_errors_total",
                "Total number of prediction errors"
            )
            .expect("Failed to register prediction_errors"),
        }
    }
}

/// Agent metrics for Prometheus exposition
///
/// This is a lightweight handle to the global metrics instance.
/// Multiple clones share the same underlying metrics.
#[derive(Clone)]
pub struct AgentMetrics {
    // This is just a marker - we use the global instance
    _private: (),
}

impl Default for AgentMetrics {
    fn default() -> Self {
        Self::new()
    }
}

impl AgentMetrics {
    /// Create a new metrics handle (initializes global metrics if needed)
    pub fn new() -> Self {
        // Initialize global metrics on first call
        GLOBAL_METRICS.get_or_init(AgentMetricsInner::new);
        Self { _private: () }
    }

    fn inner(&self) -> &AgentMetricsInner {
        GLOBAL_METRICS.get().expect("Metrics not initialized")
    }

    /// Record a collection latency observation
    pub fn observe_collection_latency(&self, duration_secs: f64) {
        self.inner().collection_latency_seconds.observe(duration_secs);
    }

    /// Record a prediction latency observation
    pub fn observe_prediction_latency(&self, duration_secs: f64) {
        self.inner().prediction_latency_seconds.observe(duration_secs);
    }

    /// Update buffer size metrics
    pub fn set_buffer_size(&self, bytes: i64, items: i64) {
        self.inner().buffer_size_bytes.set(bytes);
        self.inner().buffer_items.set(items);
    }

    /// Update model version info
    pub fn set_model_version(&self, version: &str, quantization: &str) {
        // Reset previous version
        self.inner().model_version_info.reset();
        // Set new version with value 1
        self.inner()
            .model_version_info
            .with_label_values(&[version, quantization])
            .set(1.0);
    }

    /// Update containers monitored count
    pub fn set_containers_monitored(&self, count: i64) {
        self.inner().containers_monitored.set(count);
    }

    /// Increment predictions generated counter
    pub fn inc_predictions_generated(&self) {
        self.inner().predictions_generated.inc();
    }

    /// Increment anomalies detected counter
    pub fn inc_anomalies_detected(&self) {
        self.inner().anomalies_detected.inc();
    }

    /// Increment collection errors counter
    pub fn inc_collection_errors(&self) {
        self.inner().collection_errors.inc();
    }

    /// Increment prediction errors counter
    pub fn inc_prediction_errors(&self) {
        self.inner().prediction_errors.inc();
    }
}

/// Structured logger for agent events
///
/// Provides consistent JSON-formatted logging for predictions,
/// anomalies, and other significant events.
#[derive(Clone)]
pub struct StructuredLogger {
    node_name: String,
}

impl StructuredLogger {
    pub fn new(node_name: impl Into<String>) -> Self {
        Self {
            node_name: node_name.into(),
        }
    }

    /// Log a prediction generation event
    pub fn log_prediction(
        &self,
        container_id: &str,
        pod_name: &str,
        namespace: &str,
        cpu_request_millicores: u32,
        cpu_limit_millicores: u32,
        memory_request_bytes: u64,
        memory_limit_bytes: u64,
        confidence: f32,
        model_version: &str,
    ) {
        info!(
            event = "prediction_generated",
            node = %self.node_name,
            container_id = %container_id,
            pod_name = %pod_name,
            namespace = %namespace,
            cpu_request_millicores = cpu_request_millicores,
            cpu_limit_millicores = cpu_limit_millicores,
            memory_request_bytes = memory_request_bytes,
            memory_limit_bytes = memory_limit_bytes,
            confidence = confidence,
            model_version = %model_version,
            "Generated resource prediction"
        );
    }

    /// Log an anomaly detection event
    pub fn log_anomaly(
        &self,
        container_id: &str,
        pod_name: &str,
        namespace: &str,
        anomaly_type: &str,
        severity: &str,
        details: &str,
    ) {
        match severity {
            "critical" => {
                warn!(
                    event = "anomaly_detected",
                    node = %self.node_name,
                    container_id = %container_id,
                    pod_name = %pod_name,
                    namespace = %namespace,
                    anomaly_type = %anomaly_type,
                    severity = %severity,
                    details = %details,
                    "Critical anomaly detected"
                );
            }
            _ => {
                info!(
                    event = "anomaly_detected",
                    node = %self.node_name,
                    container_id = %container_id,
                    pod_name = %pod_name,
                    namespace = %namespace,
                    anomaly_type = %anomaly_type,
                    severity = %severity,
                    details = %details,
                    "Anomaly detected"
                );
            }
        }
    }

    /// Log a memory leak detection
    pub fn log_memory_leak(
        &self,
        container_id: &str,
        pod_name: &str,
        namespace: &str,
        slope_bytes_per_sec: f64,
        projected_oom_secs: Option<i64>,
        confidence: f64,
    ) {
        let severity = if projected_oom_secs.map(|s| s < 3600).unwrap_or(false) {
            "critical"
        } else {
            "warning"
        };

        warn!(
            event = "memory_leak_detected",
            node = %self.node_name,
            container_id = %container_id,
            pod_name = %pod_name,
            namespace = %namespace,
            anomaly_type = "memory_leak",
            severity = %severity,
            slope_bytes_per_sec = slope_bytes_per_sec,
            projected_oom_secs = ?projected_oom_secs,
            confidence = confidence,
            "Memory leak detected"
        );
    }

    /// Log a CPU spike detection
    pub fn log_cpu_spike(
        &self,
        container_id: &str,
        pod_name: &str,
        namespace: &str,
        current_usage: f64,
        expected_usage: f64,
        z_score: f64,
        severity: &str,
    ) {
        info!(
            event = "cpu_spike_detected",
            node = %self.node_name,
            container_id = %container_id,
            pod_name = %pod_name,
            namespace = %namespace,
            anomaly_type = "cpu_spike",
            severity = %severity,
            current_usage = current_usage,
            expected_usage = expected_usage,
            z_score = z_score,
            "CPU spike detected"
        );
    }

    /// Log prediction deviation from actual usage
    pub fn log_prediction_deviation(
        &self,
        container_id: &str,
        pod_name: &str,
        namespace: &str,
        predicted_cpu: u32,
        actual_cpu: f32,
        predicted_memory: u64,
        actual_memory: u64,
        model_version: &str,
    ) {
        let cpu_deviation = ((actual_cpu * 1000.0) as i64 - predicted_cpu as i64).abs() as f32
            / predicted_cpu as f32
            * 100.0;
        let memory_deviation = (actual_memory as i64 - predicted_memory as i64).unsigned_abs()
            as f64
            / predicted_memory as f64
            * 100.0;

        info!(
            event = "prediction_deviation",
            node = %self.node_name,
            container_id = %container_id,
            pod_name = %pod_name,
            namespace = %namespace,
            predicted_cpu_millicores = predicted_cpu,
            actual_cpu_cores = actual_cpu,
            cpu_deviation_percent = cpu_deviation,
            predicted_memory_bytes = predicted_memory,
            actual_memory_bytes = actual_memory,
            memory_deviation_percent = memory_deviation,
            model_version = %model_version,
            "Prediction deviation recorded for model improvement"
        );
    }

    /// Log agent startup
    pub fn log_startup(&self, version: &str, model_version: &str) {
        info!(
            event = "agent_started",
            node = %self.node_name,
            agent_version = %version,
            model_version = %model_version,
            "Resource agent started"
        );
    }

    /// Log agent shutdown
    pub fn log_shutdown(&self, reason: &str) {
        info!(
            event = "agent_shutdown",
            node = %self.node_name,
            reason = %reason,
            "Resource agent shutting down"
        );
    }

    /// Log model update
    pub fn log_model_update(&self, old_version: &str, new_version: &str, success: bool) {
        if success {
            info!(
                event = "model_updated",
                node = %self.node_name,
                old_version = %old_version,
                new_version = %new_version,
                "ML model updated successfully"
            );
        } else {
            warn!(
                event = "model_update_failed",
                node = %self.node_name,
                old_version = %old_version,
                new_version = %new_version,
                "ML model update failed, keeping previous version"
            );
        }
    }

    /// Log sync status with API
    pub fn log_sync_status(&self, connected: bool, buffered_items: usize) {
        if connected {
            info!(
                event = "api_sync",
                node = %self.node_name,
                connected = true,
                buffered_items = buffered_items,
                "Synced with recommendation API"
            );
        } else {
            warn!(
                event = "api_sync",
                node = %self.node_name,
                connected = false,
                buffered_items = buffered_items,
                "Disconnected from recommendation API, buffering locally"
            );
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_agent_metrics_creation() {
        // Note: This test may fail if run multiple times in the same process
        // due to Prometheus global registry. In practice, metrics are created once.
        // We test the structure here.
        let metrics = AgentMetrics::new();

        // Verify metrics can be observed
        metrics.observe_collection_latency(0.001);
        metrics.observe_prediction_latency(0.002);
        metrics.set_buffer_size(1024, 10);
        metrics.set_model_version("v1.0.0", "int8");
        metrics.set_containers_monitored(5);
        metrics.inc_predictions_generated();
        metrics.inc_anomalies_detected();
    }

    #[test]
    fn test_structured_logger_creation() {
        let logger = StructuredLogger::new("test-node");
        assert_eq!(logger.node_name, "test-node");
    }
}
