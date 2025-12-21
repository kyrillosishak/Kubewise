//! API client for communicating with the Recommendation API

use anyhow::{Context, Result};
use reqwest::Client;
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use url::Url;

/// API client for the Recommendation API
pub struct ApiClient {
    client: Client,
    base_url: Url,
}

impl ApiClient {
    /// Create a new API client
    pub fn new(base_url: &str) -> Result<Self> {
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .context("Failed to create HTTP client")?;

        let base_url = Url::parse(base_url).context("Invalid API URL")?;

        Ok(Self { client, base_url })
    }

    /// Make a GET request
    pub async fn get<T: DeserializeOwned>(&self, path: &str) -> Result<T> {
        let url = self.base_url.join(path).context("Invalid path")?;

        let response = self
            .client
            .get(url)
            .send()
            .await
            .context("Failed to send request")?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            anyhow::bail!("API error ({}): {}", status, body);
        }

        response.json().await.context("Failed to parse response")
    }

    /// Make a POST request with JSON body
    pub async fn post<T: DeserializeOwned, B: Serialize>(&self, path: &str, body: &B) -> Result<T> {
        let url = self.base_url.join(path).context("Invalid path")?;

        let response = self
            .client
            .post(url)
            .json(body)
            .send()
            .await
            .context("Failed to send request")?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            anyhow::bail!("API error ({}): {}", status, body);
        }

        response.json().await.context("Failed to parse response")
    }
}

// API response types

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Recommendation {
    pub id: String,
    pub namespace: String,
    pub deployment: String,
    pub cpu_request_millicores: u32,
    pub cpu_limit_millicores: u32,
    pub memory_request_bytes: u64,
    pub memory_limit_bytes: u64,
    pub confidence: f32,
    pub model_version: String,
    pub status: String,
    pub created_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applied_at: Option<String>,
    pub time_window: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub current_resources: Option<ResourceSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub recommended_resources: Option<ResourceSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceSpec {
    pub cpu_request: String,
    pub cpu_limit: String,
    pub memory_request: String,
    pub memory_limit: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RecommendationList {
    pub recommendations: Vec<Recommendation>,
    pub total: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApplyRequest {
    pub dry_run: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApplyResponse {
    pub id: String,
    pub status: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub yaml_patch: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApproveRequest {
    pub approver: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApproveResponse {
    pub id: String,
    pub status: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub approver: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CostAnalysis {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub namespace: Option<String>,
    pub current_monthly_cost: f64,
    pub recommended_monthly_cost: f64,
    pub potential_savings: f64,
    pub currency: String,
    pub deployment_count: i32,
    pub last_updated: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SavingsReport {
    pub total_savings: f64,
    pub currency: String,
    pub period: String,
    pub savings_by_month: Vec<MonthlySaving>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub savings_by_team: Option<Vec<TeamSaving>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MonthlySaving {
    pub month: String,
    pub savings: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TeamSaving {
    pub team: String,
    pub savings: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelVersion {
    pub version: String,
    pub created_at: String,
    pub validation_accuracy: f32,
    pub size_bytes: i64,
    pub is_active: bool,
    pub rollback_count: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelList {
    pub models: Vec<ModelVersion>,
    pub total: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PredictionHistory {
    pub deployment: String,
    pub namespace: String,
    pub predictions: Vec<Prediction>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Prediction {
    pub timestamp: String,
    pub cpu_request_millicores: u32,
    pub cpu_limit_millicores: u32,
    pub memory_request_bytes: u64,
    pub memory_limit_bytes: u64,
    pub confidence: f32,
    pub model_version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentStatus {
    pub node: String,
    pub status: String,
    pub last_seen: String,
    pub containers_monitored: i32,
    pub model_version: String,
    pub buffer_size_bytes: u64,
    pub collection_latency_ms: f64,
    pub prediction_latency_ms: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricsExport {
    pub namespace: Option<String>,
    pub since: String,
    pub metrics: Vec<MetricEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricEntry {
    pub timestamp: String,
    pub container_id: String,
    pub pod_name: String,
    pub namespace: String,
    pub cpu_usage_cores: f32,
    pub memory_usage_bytes: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorResponse {
    pub error: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
}
