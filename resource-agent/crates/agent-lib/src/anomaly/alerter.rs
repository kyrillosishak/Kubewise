//! Alert emission for anomaly detection
//!
//! Handles:
//! - Creating Kubernetes events on affected pods
//! - Formatting alerts for Alertmanager webhook
//! - Deduplication of alerts within a configurable window

use std::collections::HashMap;
use std::sync::RwLock;
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};

use super::{LeakAnomaly, SpikeAnomaly, SpikeSeverity};

/// Default deduplication window (15 minutes)
const DEFAULT_DEDUP_WINDOW_SECS: u64 = 15 * 60;

/// Alert severity levels
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AlertSeverity {
    Warning,
    Critical,
}

impl std::fmt::Display for AlertSeverity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            AlertSeverity::Warning => write!(f, "warning"),
            AlertSeverity::Critical => write!(f, "critical"),
        }
    }
}

/// Alert type classification
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AlertType {
    MemoryLeak,
    CpuSpike,
    OomRisk,
}

impl std::fmt::Display for AlertType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            AlertType::MemoryLeak => write!(f, "MemoryLeak"),
            AlertType::CpuSpike => write!(f, "CpuSpike"),
            AlertType::OomRisk => write!(f, "OOMRisk"),
        }
    }
}

/// Kubernetes event for anomaly notification
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KubernetesEvent {
    /// API version for the event
    pub api_version: String,
    /// Kind is always "Event"
    pub kind: String,
    /// Event metadata
    pub metadata: EventMetadata,
    /// Reference to the affected object
    pub involved_object: ObjectReference,
    /// Reason for the event
    pub reason: String,
    /// Human-readable message
    pub message: String,
    /// Event type (Normal or Warning)
    #[serde(rename = "type")]
    pub event_type: String,
    /// First timestamp
    pub first_timestamp: String,
    /// Last timestamp
    pub last_timestamp: String,
    /// Event count
    pub count: u32,
    /// Source of the event
    pub source: EventSource,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventMetadata {
    pub name: String,
    pub namespace: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ObjectReference {
    pub api_version: String,
    pub kind: String,
    pub name: String,
    pub namespace: String,
    pub uid: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventSource {
    pub component: String,
    pub host: Option<String>,
}

/// Alertmanager webhook alert format
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AlertmanagerAlert {
    /// Alert status (firing or resolved)
    pub status: String,
    /// Alert labels for routing and grouping
    pub labels: HashMap<String, String>,
    /// Alert annotations with details
    pub annotations: HashMap<String, String>,
    /// Start time in RFC3339 format
    pub starts_at: String,
    /// End time (empty for firing alerts)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ends_at: Option<String>,
    /// Generator URL for linking back
    #[serde(skip_serializing_if = "Option::is_none")]
    pub generator_url: Option<String>,
}

/// Alertmanager webhook payload (array of alerts)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlertmanagerPayload {
    pub alerts: Vec<AlertmanagerAlert>,
}

/// Alert context with pod/container information
#[derive(Debug, Clone)]
pub struct AlertContext {
    pub container_id: String,
    pub pod_name: String,
    pub pod_uid: Option<String>,
    pub namespace: String,
    pub node_name: String,
    pub deployment: Option<String>,
}

/// Key for deduplication
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
struct DedupKey {
    alert_type: AlertType,
    namespace: String,
    pod_name: String,
}

/// Alert emitter with deduplication
pub struct Alerter {
    /// Deduplication window
    dedup_window: Duration,
    /// Recent alerts for deduplication (key -> last emission time)
    recent_alerts: RwLock<HashMap<DedupKey, Instant>>,
    /// Node name for event source
    node_name: String,
    /// Component name for event source
    component_name: String,
}

impl Alerter {
    /// Create a new alerter with default 15-minute deduplication window
    pub fn new(node_name: String) -> Self {
        Self {
            dedup_window: Duration::from_secs(DEFAULT_DEDUP_WINDOW_SECS),
            recent_alerts: RwLock::new(HashMap::new()),
            node_name,
            component_name: "resource-agent".to_string(),
        }
    }

    /// Set custom deduplication window
    pub fn with_dedup_window(mut self, window: Duration) -> Self {
        self.dedup_window = window;
        self
    }

    /// Check if an alert should be suppressed due to deduplication
    pub fn should_suppress(&self, alert_type: &AlertType, ctx: &AlertContext) -> bool {
        let key = DedupKey {
            alert_type: alert_type.clone(),
            namespace: ctx.namespace.clone(),
            pod_name: ctx.pod_name.clone(),
        };

        let alerts = self.recent_alerts.read().unwrap();
        if let Some(last_time) = alerts.get(&key) {
            last_time.elapsed() < self.dedup_window
        } else {
            false
        }
    }

    /// Record that an alert was emitted
    pub fn record_alert(&self, alert_type: &AlertType, ctx: &AlertContext) {
        let key = DedupKey {
            alert_type: alert_type.clone(),
            namespace: ctx.namespace.clone(),
            pod_name: ctx.pod_name.clone(),
        };

        let mut alerts = self.recent_alerts.write().unwrap();
        alerts.insert(key, Instant::now());

        // Clean up old entries
        alerts.retain(|_, time| time.elapsed() < self.dedup_window);
    }

    /// Create a Kubernetes event for a memory leak anomaly
    pub fn create_leak_event(
        &self,
        anomaly: &LeakAnomaly,
        ctx: &AlertContext,
        timestamp: &str,
    ) -> Option<KubernetesEvent> {
        if self.should_suppress(&AlertType::MemoryLeak, ctx) {
            return None;
        }

        let severity = if anomaly.projected_oom_time > 0 {
            "Warning"
        } else {
            "Warning"
        };

        let message = format!(
            "Memory leak detected: {:.2} MB/hour increase. Current: {} MB. Confidence: {:.0}%{}",
            anomaly.leak_rate_mb_per_hour(),
            anomaly.current_memory_bytes / (1024 * 1024),
            anomaly.confidence * 100.0,
            if anomaly.projected_oom_time > 0 {
                format!(". Projected OOM at timestamp {}", anomaly.projected_oom_time)
            } else {
                String::new()
            }
        );

        let event = KubernetesEvent {
            api_version: "v1".to_string(),
            kind: "Event".to_string(),
            metadata: EventMetadata {
                name: format!("{}.{}", ctx.pod_name, uuid_v4_simple()),
                namespace: ctx.namespace.clone(),
            },
            involved_object: ObjectReference {
                api_version: "v1".to_string(),
                kind: "Pod".to_string(),
                name: ctx.pod_name.clone(),
                namespace: ctx.namespace.clone(),
                uid: ctx.pod_uid.clone(),
            },
            reason: "MemoryLeak".to_string(),
            message,
            event_type: severity.to_string(),
            first_timestamp: timestamp.to_string(),
            last_timestamp: timestamp.to_string(),
            count: 1,
            source: EventSource {
                component: self.component_name.clone(),
                host: Some(self.node_name.clone()),
            },
        };

        self.record_alert(&AlertType::MemoryLeak, ctx);
        Some(event)
    }

    /// Create a Kubernetes event for a CPU spike anomaly
    pub fn create_spike_event(
        &self,
        anomaly: &SpikeAnomaly,
        ctx: &AlertContext,
        timestamp: &str,
    ) -> Option<KubernetesEvent> {
        if self.should_suppress(&AlertType::CpuSpike, ctx) {
            return None;
        }

        let severity = match anomaly.severity() {
            SpikeSeverity::Critical => "Warning",
            SpikeSeverity::High => "Warning",
            SpikeSeverity::Warning => "Warning",
        };

        let message = format!(
            "CPU spike detected: {:.2} cores (expected {:.2}, z-score: {:.1}). {:.0}% above normal.",
            anomaly.current_usage,
            anomaly.expected_usage,
            anomaly.z_score,
            anomaly.percentage_above_expected()
        );

        let event = KubernetesEvent {
            api_version: "v1".to_string(),
            kind: "Event".to_string(),
            metadata: EventMetadata {
                name: format!("{}.{}", ctx.pod_name, uuid_v4_simple()),
                namespace: ctx.namespace.clone(),
            },
            involved_object: ObjectReference {
                api_version: "v1".to_string(),
                kind: "Pod".to_string(),
                name: ctx.pod_name.clone(),
                namespace: ctx.namespace.clone(),
                uid: ctx.pod_uid.clone(),
            },
            reason: "CPUSpike".to_string(),
            message,
            event_type: severity.to_string(),
            first_timestamp: timestamp.to_string(),
            last_timestamp: timestamp.to_string(),
            count: 1,
            source: EventSource {
                component: self.component_name.clone(),
                host: Some(self.node_name.clone()),
            },
        };

        self.record_alert(&AlertType::CpuSpike, ctx);
        Some(event)
    }

    /// Create an Alertmanager alert for a memory leak
    pub fn create_leak_alertmanager_alert(
        &self,
        anomaly: &LeakAnomaly,
        ctx: &AlertContext,
        timestamp: &str,
    ) -> AlertmanagerAlert {
        let mut labels = HashMap::new();
        labels.insert("alertname".to_string(), "ContainerMemoryLeak".to_string());
        labels.insert("severity".to_string(), AlertSeverity::Warning.to_string());
        labels.insert("namespace".to_string(), ctx.namespace.clone());
        labels.insert("pod".to_string(), ctx.pod_name.clone());
        labels.insert("container_id".to_string(), ctx.container_id.clone());
        labels.insert("node".to_string(), ctx.node_name.clone());
        if let Some(ref deployment) = ctx.deployment {
            labels.insert("deployment".to_string(), deployment.clone());
        }

        let mut annotations = HashMap::new();
        annotations.insert(
            "summary".to_string(),
            format!(
                "Memory leak detected in pod {}/{}",
                ctx.namespace, ctx.pod_name
            ),
        );
        annotations.insert(
            "description".to_string(),
            format!(
                "Container is leaking memory at {:.2} MB/hour. Current usage: {} MB. Confidence: {:.0}%.",
                anomaly.leak_rate_mb_per_hour(),
                anomaly.current_memory_bytes / (1024 * 1024),
                anomaly.confidence * 100.0
            ),
        );
        annotations.insert(
            "leak_rate_bytes_per_sec".to_string(),
            format!("{:.2}", anomaly.slope_bytes_per_sec),
        );
        if anomaly.projected_oom_time > 0 {
            annotations.insert(
                "projected_oom_timestamp".to_string(),
                anomaly.projected_oom_time.to_string(),
            );
        }

        AlertmanagerAlert {
            status: "firing".to_string(),
            labels,
            annotations,
            starts_at: timestamp.to_string(),
            ends_at: None,
            generator_url: None,
        }
    }

    /// Create an Alertmanager alert for a CPU spike
    pub fn create_spike_alertmanager_alert(
        &self,
        anomaly: &SpikeAnomaly,
        ctx: &AlertContext,
        timestamp: &str,
    ) -> AlertmanagerAlert {
        let severity = match anomaly.severity() {
            SpikeSeverity::Critical => AlertSeverity::Critical,
            SpikeSeverity::High | SpikeSeverity::Warning => AlertSeverity::Warning,
        };

        let mut labels = HashMap::new();
        labels.insert("alertname".to_string(), "ContainerCPUSpike".to_string());
        labels.insert("severity".to_string(), severity.to_string());
        labels.insert("namespace".to_string(), ctx.namespace.clone());
        labels.insert("pod".to_string(), ctx.pod_name.clone());
        labels.insert("container_id".to_string(), ctx.container_id.clone());
        labels.insert("node".to_string(), ctx.node_name.clone());
        if let Some(ref deployment) = ctx.deployment {
            labels.insert("deployment".to_string(), deployment.clone());
        }

        let mut annotations = HashMap::new();
        annotations.insert(
            "summary".to_string(),
            format!(
                "CPU spike detected in pod {}/{}",
                ctx.namespace, ctx.pod_name
            ),
        );
        annotations.insert(
            "description".to_string(),
            format!(
                "CPU usage spiked to {:.2} cores (expected {:.2}). Z-score: {:.1} ({:.0}% above normal).",
                anomaly.current_usage,
                anomaly.expected_usage,
                anomaly.z_score,
                anomaly.percentage_above_expected()
            ),
        );
        annotations.insert("z_score".to_string(), format!("{:.2}", anomaly.z_score));
        annotations.insert(
            "current_cpu".to_string(),
            format!("{:.4}", anomaly.current_usage),
        );
        annotations.insert(
            "expected_cpu".to_string(),
            format!("{:.4}", anomaly.expected_usage),
        );

        AlertmanagerAlert {
            status: "firing".to_string(),
            labels,
            annotations,
            starts_at: timestamp.to_string(),
            ends_at: None,
            generator_url: None,
        }
    }

    /// Create an Alertmanager payload from multiple alerts
    pub fn create_alertmanager_payload(alerts: Vec<AlertmanagerAlert>) -> AlertmanagerPayload {
        AlertmanagerPayload { alerts }
    }

    /// Clear expired deduplication entries
    pub fn cleanup_dedup_cache(&self) {
        let mut alerts = self.recent_alerts.write().unwrap();
        alerts.retain(|_, time| time.elapsed() < self.dedup_window);
    }
}

/// Generate a simple UUID-like string for event naming
fn uuid_v4_simple() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    format!("{:x}{:x}", now.as_secs(), now.subsec_nanos())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread::sleep;

    fn test_context() -> AlertContext {
        AlertContext {
            container_id: "abc123".to_string(),
            pod_name: "test-pod".to_string(),
            pod_uid: Some("uid-123".to_string()),
            namespace: "default".to_string(),
            node_name: "node-1".to_string(),
            deployment: Some("test-deployment".to_string()),
        }
    }

    #[test]
    fn test_deduplication() {
        let alerter = Alerter::new("node-1".to_string())
            .with_dedup_window(Duration::from_millis(100));

        let ctx = test_context();
        let anomaly = LeakAnomaly {
            slope_bytes_per_sec: 10000.0,
            projected_oom_time: 0,
            confidence: 0.9,
            current_memory_bytes: 100_000_000,
            samples_analyzed: 60,
        };

        // First alert should succeed
        let event1 = alerter.create_leak_event(&anomaly, &ctx, "2024-01-01T00:00:00Z");
        assert!(event1.is_some());

        // Second alert should be suppressed
        let event2 = alerter.create_leak_event(&anomaly, &ctx, "2024-01-01T00:00:01Z");
        assert!(event2.is_none());

        // Wait for dedup window to expire
        sleep(Duration::from_millis(150));

        // Third alert should succeed
        let event3 = alerter.create_leak_event(&anomaly, &ctx, "2024-01-01T00:00:02Z");
        assert!(event3.is_some());
    }

    #[test]
    fn test_leak_event_creation() {
        let alerter = Alerter::new("node-1".to_string());
        let ctx = test_context();
        let anomaly = LeakAnomaly {
            slope_bytes_per_sec: 10000.0,
            projected_oom_time: 1704067200,
            confidence: 0.85,
            current_memory_bytes: 500_000_000,
            samples_analyzed: 60,
        };

        let event = alerter
            .create_leak_event(&anomaly, &ctx, "2024-01-01T00:00:00Z")
            .unwrap();

        assert_eq!(event.reason, "MemoryLeak");
        assert_eq!(event.involved_object.name, "test-pod");
        assert_eq!(event.involved_object.namespace, "default");
        assert!(event.message.contains("Memory leak detected"));
    }

    #[test]
    fn test_spike_alertmanager_alert() {
        let alerter = Alerter::new("node-1".to_string());
        let ctx = test_context();
        let anomaly = SpikeAnomaly {
            current_usage: 2.5,
            expected_usage: 0.5,
            z_score: 4.5,
            std_dev: 0.1,
            threshold: 3.0,
        };

        let alert = alerter.create_spike_alertmanager_alert(&anomaly, &ctx, "2024-01-01T00:00:00Z");

        assert_eq!(alert.status, "firing");
        assert_eq!(
            alert.labels.get("alertname").unwrap(),
            "ContainerCPUSpike"
        );
        assert_eq!(alert.labels.get("namespace").unwrap(), "default");
        assert_eq!(alert.labels.get("pod").unwrap(), "test-pod");
        assert!(alert.annotations.get("description").unwrap().contains("2.5"));
    }

    #[test]
    fn test_different_alert_types_not_deduplicated() {
        let alerter = Alerter::new("node-1".to_string());
        let ctx = test_context();

        let leak = LeakAnomaly {
            slope_bytes_per_sec: 10000.0,
            projected_oom_time: 0,
            confidence: 0.9,
            current_memory_bytes: 100_000_000,
            samples_analyzed: 60,
        };

        let spike = SpikeAnomaly {
            current_usage: 2.0,
            expected_usage: 0.5,
            z_score: 4.0,
            std_dev: 0.1,
            threshold: 3.0,
        };

        // Both should succeed since they're different alert types
        let event1 = alerter.create_leak_event(&leak, &ctx, "2024-01-01T00:00:00Z");
        let event2 = alerter.create_spike_event(&spike, &ctx, "2024-01-01T00:00:00Z");

        assert!(event1.is_some());
        assert!(event2.is_some());
    }
}
