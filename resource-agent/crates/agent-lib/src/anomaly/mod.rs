//! Anomaly detection for resource usage patterns
//!
//! This module provides detection for:
//! - Memory leaks (monotonically increasing memory over time)
//! - CPU spikes (values exceeding standard deviation thresholds)
//! - Alert emission to Kubernetes and Alertmanager

mod alerter;
mod leak_detector;
mod spike_detector;

pub use alerter::{
    AlertContext, AlertSeverity, AlertType, Alerter, AlertmanagerAlert, AlertmanagerPayload,
    EventMetadata, EventSource, KubernetesEvent, ObjectReference,
};
pub use leak_detector::{LeakAnomaly, LeakDetector};
pub use spike_detector::{RollingStats, SpikeAnomaly, SpikeDetector, SpikeSeverity};
