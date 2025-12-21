//! Core data models for the resource agent

use serde::{Deserialize, Serialize};

/// Container metrics collected from cgroups
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerMetrics {
    pub container_id: String,
    pub pod_name: String,
    pub namespace: String,
    pub deployment: Option<String>,
    pub timestamp: i64,
    pub cpu_usage_cores: f32,
    pub cpu_throttled_periods: u64,
    pub memory_usage_bytes: u64,
    pub memory_working_set_bytes: u64,
    pub memory_cache_bytes: u64,
    pub network_rx_bytes: u64,
    pub network_tx_bytes: u64,
}

/// Resource profile recommendation output
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceProfile {
    pub cpu_request_millicores: u32,
    pub cpu_limit_millicores: u32,
    pub memory_request_bytes: u64,
    pub memory_limit_bytes: u64,
    pub confidence: f32,
    pub model_version: String,
    pub generated_at: i64,
}

/// Feature vector for ML inference
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeatureVector {
    pub cpu_usage_p50: f32,
    pub cpu_usage_p95: f32,
    pub cpu_usage_p99: f32,
    pub mem_usage_p50: f32,
    pub mem_usage_p95: f32,
    pub mem_usage_p99: f32,
    pub cpu_variance: f32,
    pub mem_trend: f32,
    pub throttle_ratio: f32,
    pub hour_of_day: f32,
    pub day_of_week: f32,
    pub workload_age_days: f32,
}

/// Container information for discovery
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContainerInfo {
    pub container_id: String,
    pub pod_name: String,
    pub namespace: String,
    pub deployment: Option<String>,
    pub node_name: String,
    pub cgroup_path: String,
}
