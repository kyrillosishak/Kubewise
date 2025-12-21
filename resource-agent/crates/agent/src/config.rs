//! Agent configuration

use anyhow::Result;
use serde::Deserialize;

/// Agent configuration
#[derive(Debug, Clone, Deserialize)]
pub struct AgentConfig {
    /// Node name from Kubernetes downward API
    #[serde(default = "default_node_name")]
    pub node_name: String,

    /// API server port for health/metrics
    #[serde(default = "default_api_port")]
    pub api_port: u16,

    /// Recommendation API endpoint
    #[serde(default = "default_api_endpoint")]
    pub api_endpoint: String,

    /// Metrics collection interval in seconds
    #[serde(default = "default_collection_interval")]
    pub collection_interval_secs: u64,

    /// Prediction interval in seconds
    #[serde(default = "default_prediction_interval")]
    pub prediction_interval_secs: u64,
}

fn default_node_name() -> String {
    std::env::var("NODE_NAME").unwrap_or_else(|_| "unknown".to_string())
}

fn default_api_port() -> u16 {
    8080
}

fn default_api_endpoint() -> String {
    "http://recommendation-api:9090".to_string()
}

fn default_collection_interval() -> u64 {
    10
}

fn default_prediction_interval() -> u64 {
    300
}

impl AgentConfig {
    /// Load configuration from environment and config file
    pub fn load() -> Result<Self> {
        let config = config::Config::builder()
            .add_source(config::Environment::with_prefix("AGENT"))
            .build()?;

        Ok(config.try_deserialize().unwrap_or_else(|_| AgentConfig {
            node_name: default_node_name(),
            api_port: default_api_port(),
            api_endpoint: default_api_endpoint(),
            collection_interval_secs: default_collection_interval(),
            prediction_interval_secs: default_prediction_interval(),
        }))
    }
}
