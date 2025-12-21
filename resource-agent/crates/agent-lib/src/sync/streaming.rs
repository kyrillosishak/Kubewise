//! Metrics streaming with backpressure handling
//!
//! This module provides streaming functionality for syncing metrics to the API:
//! - Batches metrics into MetricsBatch messages
//! - Streams to API with backpressure handling
//! - Handles connection failures gracefully

use crate::models::{ContainerMetrics as LocalMetrics, ResourceProfile as LocalProfile};
use crate::proto::{
    Anomaly as ProtoAnomaly, ContainerMetrics as ProtoMetrics, MetricsBatch,
    PredictorSyncClient, ResourceProfile as ProtoProfile, SyncResponse,
};
use anyhow::{Context, Result};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::mpsc;
use tokio::time::Instant;
use tonic::transport::Channel;
use tracing::{debug, error, info, warn};

/// Configuration for metrics streaming
#[derive(Debug, Clone)]
pub struct StreamingConfig {
    /// Maximum batch size before sending
    pub max_batch_size: usize,
    /// Maximum time to wait before sending a partial batch
    pub max_batch_delay: Duration,
    /// Channel buffer size for backpressure
    pub channel_buffer_size: usize,
    /// Retry delay on failure
    pub retry_delay: Duration,
    /// Maximum retries before giving up
    pub max_retries: u32,
}

impl Default for StreamingConfig {
    fn default() -> Self {
        Self {
            max_batch_size: 100,
            max_batch_delay: Duration::from_secs(10),
            channel_buffer_size: 1000,
            retry_delay: Duration::from_secs(5),
            max_retries: 3,
        }
    }
}

/// Pending data to be synced
#[derive(Debug, Clone)]
pub struct PendingData {
    pub metrics: Vec<LocalMetrics>,
    pub predictions: Vec<LocalProfile>,
    pub anomalies: Vec<AnomalyData>,
}

/// Anomaly data for streaming
#[derive(Debug, Clone)]
pub struct AnomalyData {
    pub container_id: String,
    pub pod_name: String,
    pub namespace: String,
    pub anomaly_type: i32,
    pub severity: i32,
    pub message: String,
    pub detected_at: i64,
}

impl Default for PendingData {
    fn default() -> Self {
        Self {
            metrics: Vec::new(),
            predictions: Vec::new(),
            anomalies: Vec::new(),
        }
    }
}

/// Metrics streamer for sending data to the API
pub struct MetricsStreamer {
    config: StreamingConfig,
    agent_id: String,
    node_name: String,
    sender: mpsc::Sender<PendingData>,
    stats: Arc<tokio::sync::RwLock<StreamingStats>>,
}

/// Statistics for streaming operations
#[derive(Debug, Default, Clone)]
pub struct StreamingStats {
    pub batches_sent: u64,
    pub metrics_sent: u64,
    pub predictions_sent: u64,
    pub anomalies_sent: u64,
    pub failures: u64,
    pub last_sync_time: Option<Instant>,
    pub last_error: Option<String>,
}

impl MetricsStreamer {
    /// Create a new metrics streamer
    pub fn new(
        config: StreamingConfig,
        agent_id: String,
        node_name: String,
    ) -> (Self, mpsc::Receiver<PendingData>) {
        let (sender, receiver) = mpsc::channel(config.channel_buffer_size);
        let streamer = Self {
            config,
            agent_id,
            node_name,
            sender,
            stats: Arc::new(tokio::sync::RwLock::new(StreamingStats::default())),
        };
        (streamer, receiver)
    }

    /// Queue metrics for streaming (non-blocking with backpressure)
    pub async fn queue_metrics(&self, metrics: Vec<LocalMetrics>) -> Result<()> {
        if metrics.is_empty() {
            return Ok(());
        }

        let data = PendingData {
            metrics,
            ..Default::default()
        };

        self.sender
            .send(data)
            .await
            .map_err(|_| anyhow::anyhow!("Streaming channel closed"))?;

        Ok(())
    }

    /// Queue predictions for streaming
    pub async fn queue_predictions(&self, predictions: Vec<LocalProfile>) -> Result<()> {
        if predictions.is_empty() {
            return Ok(());
        }

        let data = PendingData {
            predictions,
            ..Default::default()
        };

        self.sender
            .send(data)
            .await
            .map_err(|_| anyhow::anyhow!("Streaming channel closed"))?;

        Ok(())
    }

    /// Queue anomalies for streaming
    pub async fn queue_anomalies(&self, anomalies: Vec<AnomalyData>) -> Result<()> {
        if anomalies.is_empty() {
            return Ok(());
        }

        let data = PendingData {
            anomalies,
            ..Default::default()
        };

        self.sender
            .send(data)
            .await
            .map_err(|_| anyhow::anyhow!("Streaming channel closed"))?;

        Ok(())
    }

    /// Try to queue data without blocking (returns false if channel is full)
    pub fn try_queue(&self, data: PendingData) -> bool {
        self.sender.try_send(data).is_ok()
    }

    /// Get current streaming statistics
    pub async fn stats(&self) -> StreamingStats {
        self.stats.read().await.clone()
    }

    /// Get the agent ID
    pub fn agent_id(&self) -> &str {
        &self.agent_id
    }

    /// Get the node name
    pub fn node_name(&self) -> &str {
        &self.node_name
    }

    /// Get the config
    pub fn config(&self) -> &StreamingConfig {
        &self.config
    }

    /// Get a clone of the stats handle
    pub fn stats_handle(&self) -> Arc<tokio::sync::RwLock<StreamingStats>> {
        Arc::clone(&self.stats)
    }
}

/// Background streaming worker
pub struct StreamingWorker {
    config: StreamingConfig,
    agent_id: String,
    node_name: String,
    receiver: mpsc::Receiver<PendingData>,
    stats: Arc<tokio::sync::RwLock<StreamingStats>>,
    pending_batch: PendingData,
    last_batch_time: Instant,
}

impl StreamingWorker {
    /// Create a new streaming worker
    pub fn new(
        config: StreamingConfig,
        agent_id: String,
        node_name: String,
        receiver: mpsc::Receiver<PendingData>,
        stats: Arc<tokio::sync::RwLock<StreamingStats>>,
    ) -> Self {
        Self {
            config,
            agent_id,
            node_name,
            receiver,
            stats,
            pending_batch: PendingData::default(),
            last_batch_time: Instant::now(),
        }
    }

    /// Run the streaming worker
    pub async fn run(&mut self, mut client: PredictorSyncClient<Channel>) {
        info!(
            agent_id = %self.agent_id,
            "Starting metrics streaming worker"
        );

        loop {
            tokio::select! {
                // Receive new data
                Some(data) = self.receiver.recv() => {
                    self.add_to_batch(data);

                    // Check if batch is ready to send
                    if self.should_send_batch() {
                        self.send_batch(&mut client).await;
                    }
                }

                // Timeout - send partial batch
                _ = tokio::time::sleep(self.config.max_batch_delay) => {
                    if !self.is_batch_empty() {
                        debug!("Sending partial batch due to timeout");
                        self.send_batch(&mut client).await;
                    }
                }
            }
        }
    }

    /// Add data to the pending batch
    fn add_to_batch(&mut self, data: PendingData) {
        self.pending_batch.metrics.extend(data.metrics);
        self.pending_batch.predictions.extend(data.predictions);
        self.pending_batch.anomalies.extend(data.anomalies);
    }

    /// Check if batch should be sent
    fn should_send_batch(&self) -> bool {
        let total_items = self.pending_batch.metrics.len()
            + self.pending_batch.predictions.len()
            + self.pending_batch.anomalies.len();

        total_items >= self.config.max_batch_size
            || self.last_batch_time.elapsed() >= self.config.max_batch_delay
    }

    /// Check if batch is empty
    fn is_batch_empty(&self) -> bool {
        self.pending_batch.metrics.is_empty()
            && self.pending_batch.predictions.is_empty()
            && self.pending_batch.anomalies.is_empty()
    }

    /// Send the current batch
    async fn send_batch(&mut self, client: &mut PredictorSyncClient<Channel>) {
        let batch = std::mem::take(&mut self.pending_batch);
        self.last_batch_time = Instant::now();

        let metrics_count = batch.metrics.len();
        let predictions_count = batch.predictions.len();
        let anomalies_count = batch.anomalies.len();

        // Convert to proto batch
        let proto_batch = self.create_proto_batch(batch);

        // Try to send with retries
        let mut retries = 0;
        loop {
            match self.send_single_batch(client, proto_batch.clone()).await {
                Ok(response) => {
                    debug!(
                        metrics = metrics_count,
                        predictions = predictions_count,
                        anomalies = anomalies_count,
                        "Batch sent successfully"
                    );

                    // Update stats
                    let mut stats = self.stats.write().await;
                    stats.batches_sent += 1;
                    stats.metrics_sent += metrics_count as u64;
                    stats.predictions_sent += predictions_count as u64;
                    stats.anomalies_sent += anomalies_count as u64;
                    stats.last_sync_time = Some(Instant::now());
                    stats.last_error = None;

                    if !response.success {
                        warn!(message = %response.message, "API reported sync issue");
                    }
                    break;
                }
                Err(e) => {
                    retries += 1;
                    if retries >= self.config.max_retries {
                        error!(
                            error = %e,
                            retries = retries,
                            "Failed to send batch after max retries"
                        );

                        // Update failure stats
                        let mut stats = self.stats.write().await;
                        stats.failures += 1;
                        stats.last_error = Some(e.to_string());
                        break;
                    }

                    warn!(
                        error = %e,
                        retry = retries,
                        "Failed to send batch, retrying"
                    );
                    tokio::time::sleep(self.config.retry_delay).await;
                }
            }
        }
    }

    /// Send a single batch to the API
    async fn send_single_batch(
        &self,
        client: &mut PredictorSyncClient<Channel>,
        batch: MetricsBatch,
    ) -> Result<SyncResponse> {
        // Create a stream with a single batch
        let stream = tokio_stream::once(batch);

        let response = client
            .sync_metrics(stream)
            .await
            .context("Failed to sync metrics")?;

        Ok(response.into_inner())
    }

    /// Create a proto batch from local data
    fn create_proto_batch(&self, data: PendingData) -> MetricsBatch {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default();

        MetricsBatch {
            agent_id: self.agent_id.clone(),
            node_name: self.node_name.clone(),
            timestamp: Some(prost_types::Timestamp {
                seconds: now.as_secs() as i64,
                nanos: now.subsec_nanos() as i32,
            }),
            metrics: data.metrics.into_iter().map(convert_metrics).collect(),
            predictions: data.predictions.into_iter().map(convert_profile).collect(),
            anomalies: data.anomalies.into_iter().map(convert_anomaly).collect(),
        }
    }
}

/// Convert local metrics to proto format
fn convert_metrics(m: LocalMetrics) -> ProtoMetrics {
    let timestamp = prost_types::Timestamp {
        seconds: m.timestamp,
        nanos: 0,
    };

    ProtoMetrics {
        container_id: m.container_id,
        pod_name: m.pod_name,
        namespace: m.namespace,
        deployment: m.deployment.unwrap_or_default(),
        timestamp: Some(timestamp),
        cpu_usage_cores: m.cpu_usage_cores,
        cpu_throttled_periods: m.cpu_throttled_periods,
        cpu_throttled_time_ns: 0,
        memory_usage_bytes: m.memory_usage_bytes,
        memory_working_set_bytes: m.memory_working_set_bytes,
        memory_cache_bytes: m.memory_cache_bytes,
        memory_rss_bytes: 0,
        network_rx_bytes: m.network_rx_bytes,
        network_tx_bytes: m.network_tx_bytes,
    }
}

/// Convert local profile to proto format
fn convert_profile(p: LocalProfile) -> ProtoProfile {
    let timestamp = prost_types::Timestamp {
        seconds: p.generated_at,
        nanos: 0,
    };

    ProtoProfile {
        container_id: String::new(),
        pod_name: String::new(),
        namespace: String::new(),
        deployment: String::new(),
        cpu_request_millicores: p.cpu_request_millicores,
        cpu_limit_millicores: p.cpu_limit_millicores,
        memory_request_bytes: p.memory_request_bytes,
        memory_limit_bytes: p.memory_limit_bytes,
        confidence: p.confidence,
        model_version: p.model_version,
        generated_at: Some(timestamp),
        time_window: 0,
    }
}

/// Convert anomaly data to proto format
fn convert_anomaly(a: AnomalyData) -> ProtoAnomaly {
    let timestamp = prost_types::Timestamp {
        seconds: a.detected_at,
        nanos: 0,
    };

    ProtoAnomaly {
        container_id: a.container_id,
        pod_name: a.pod_name,
        namespace: a.namespace,
        r#type: a.anomaly_type,
        severity: a.severity,
        message: a.message,
        detected_at: Some(timestamp),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_streaming_config_default() {
        let config = StreamingConfig::default();
        assert_eq!(config.max_batch_size, 100);
        assert_eq!(config.max_batch_delay, Duration::from_secs(10));
    }

    #[test]
    fn test_pending_data_default() {
        let data = PendingData::default();
        assert!(data.metrics.is_empty());
        assert!(data.predictions.is_empty());
        assert!(data.anomalies.is_empty());
    }

    #[tokio::test]
    async fn test_streamer_creation() {
        let config = StreamingConfig::default();
        let (streamer, _receiver) =
            MetricsStreamer::new(config, "test-agent".to_string(), "test-node".to_string());

        assert_eq!(streamer.agent_id(), "test-agent");
        assert_eq!(streamer.node_name(), "test-node");
    }

    #[tokio::test]
    async fn test_queue_empty_metrics() {
        let config = StreamingConfig::default();
        let (streamer, _receiver) =
            MetricsStreamer::new(config, "test-agent".to_string(), "test-node".to_string());

        // Should succeed with empty metrics
        let result = streamer.queue_metrics(vec![]).await;
        assert!(result.is_ok());
    }

    #[test]
    fn test_convert_metrics() {
        let local = LocalMetrics {
            container_id: "test-container".to_string(),
            pod_name: "test-pod".to_string(),
            namespace: "default".to_string(),
            deployment: Some("test-deployment".to_string()),
            timestamp: 1234567890,
            cpu_usage_cores: 0.5,
            cpu_throttled_periods: 10,
            memory_usage_bytes: 1024 * 1024,
            memory_working_set_bytes: 512 * 1024,
            memory_cache_bytes: 256 * 1024,
            network_rx_bytes: 1000,
            network_tx_bytes: 2000,
        };

        let proto = convert_metrics(local);
        assert_eq!(proto.container_id, "test-container");
        assert_eq!(proto.pod_name, "test-pod");
        assert_eq!(proto.namespace, "default");
        assert_eq!(proto.deployment, "test-deployment");
        assert_eq!(proto.cpu_usage_cores, 0.5);
    }
}
