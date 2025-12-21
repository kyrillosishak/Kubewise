//! Prediction scheduling loop
//!
//! Runs predictions periodically for each container, handling timeouts
//! and insufficient data gracefully.

use super::{FeatureExtractor, OnnxPredictor, Predictor, MIN_SAMPLES};
use crate::models::{ContainerMetrics, ResourceProfile};
use anyhow::Result;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{mpsc, RwLock};
use tokio::time::interval;
use tracing::{debug, info, warn};

/// Default prediction interval (5 minutes as per requirement 2.4)
pub const DEFAULT_PREDICTION_INTERVAL: Duration = Duration::from_secs(5 * 60);

/// Maximum inference timeout before using fallback
pub const INFERENCE_TIMEOUT: Duration = Duration::from_millis(100);

/// Configuration for the prediction scheduler
#[derive(Debug, Clone)]
pub struct PredictionConfig {
    /// Interval between predictions per container
    pub prediction_interval: Duration,
    /// Minimum samples required before generating predictions
    pub min_samples: usize,
    /// Feature extraction window size
    pub feature_window_size: usize,
    /// Maximum inference timeout
    pub inference_timeout: Duration,
}

impl Default for PredictionConfig {
    fn default() -> Self {
        Self {
            prediction_interval: DEFAULT_PREDICTION_INTERVAL,
            min_samples: MIN_SAMPLES,
            feature_window_size: 360, // 1 hour at 10s intervals
            inference_timeout: INFERENCE_TIMEOUT,
        }
    }
}

/// Metrics buffer for a single container
#[derive(Debug)]
struct ContainerBuffer {
    metrics: Vec<ContainerMetrics>,
    last_prediction: Option<Instant>,
    last_profile: Option<ResourceProfile>,
}

impl ContainerBuffer {
    fn new() -> Self {
        Self {
            metrics: Vec::new(),
            last_prediction: None,
            last_profile: None,
        }
    }

    fn add_metrics(&mut self, metrics: ContainerMetrics) {
        self.metrics.push(metrics);
        // Keep only the most recent samples (24 hours at 10s = 8640 samples)
        const MAX_SAMPLES: usize = 8640;
        if self.metrics.len() > MAX_SAMPLES {
            self.metrics.drain(0..self.metrics.len() - MAX_SAMPLES);
        }
    }

    fn should_predict(&self, interval: Duration) -> bool {
        match self.last_prediction {
            None => true,
            Some(last) => last.elapsed() >= interval,
        }
    }
}

/// Prediction scheduler that runs predictions for all containers
pub struct PredictionScheduler {
    predictor: Arc<RwLock<OnnxPredictor>>,
    feature_extractor: FeatureExtractor,
    config: PredictionConfig,
    buffers: RwLock<HashMap<String, ContainerBuffer>>,
    prediction_tx: mpsc::Sender<PredictionResult>,
}

/// Result of a prediction attempt
#[derive(Debug, Clone)]
pub struct PredictionResult {
    pub container_id: String,
    pub pod_name: String,
    pub namespace: String,
    pub deployment: Option<String>,
    pub profile: Option<ResourceProfile>,
    pub skipped_reason: Option<String>,
    pub duration_us: u64,
}

impl PredictionScheduler {
    /// Create a new prediction scheduler
    pub fn new(
        predictor: Arc<RwLock<OnnxPredictor>>,
        config: PredictionConfig,
    ) -> (Self, mpsc::Receiver<PredictionResult>) {
        let (tx, rx) = mpsc::channel(100);
        let scheduler = Self {
            predictor,
            feature_extractor: FeatureExtractor::new(config.feature_window_size),
            config,
            buffers: RwLock::new(HashMap::new()),
            prediction_tx: tx,
        };
        (scheduler, rx)
    }

    /// Add metrics to the buffer for a container
    pub async fn add_metrics(&self, metrics: ContainerMetrics) {
        let container_id = metrics.container_id.clone();
        let mut buffers = self.buffers.write().await;
        buffers
            .entry(container_id)
            .or_insert_with(ContainerBuffer::new)
            .add_metrics(metrics);
    }

    /// Run the prediction loop
    pub async fn run(self: Arc<Self>, mut shutdown: tokio::sync::broadcast::Receiver<()>) {
        info!(
            interval_secs = self.config.prediction_interval.as_secs(),
            "Starting prediction scheduler"
        );

        let mut ticker = interval(Duration::from_secs(30)); // Check every 30s

        loop {
            tokio::select! {
                _ = ticker.tick() => {
                    self.run_predictions().await;
                }
                _ = shutdown.recv() => {
                    info!("Shutting down prediction scheduler");
                    break;
                }
            }
        }
    }

    /// Run predictions for all containers that need them
    async fn run_predictions(&self) {
        let container_ids: Vec<String> = {
            let buffers = self.buffers.read().await;
            buffers.keys().cloned().collect()
        };

        for container_id in container_ids {
            if let Err(e) = self.predict_container(&container_id).await {
                warn!(container_id = %container_id, error = %e, "Prediction failed");
            }
        }
    }

    /// Run prediction for a single container
    async fn predict_container(&self, container_id: &str) -> Result<()> {
        let start = Instant::now();

        let (should_predict, metrics_snapshot, metadata) = {
            let buffers = self.buffers.read().await;
            let buffer = match buffers.get(container_id) {
                Some(b) => b,
                None => return Ok(()),
            };

            let should = buffer.should_predict(self.config.prediction_interval);
            let metrics = buffer.metrics.clone();
            let meta = metrics.last().map(|m| {
                (
                    m.pod_name.clone(),
                    m.namespace.clone(),
                    m.deployment.clone(),
                )
            });
            (should, metrics, meta)
        };

        if !should_predict {
            return Ok(());
        }

        let (pod_name, namespace, deployment) = metadata.unwrap_or_default();

        // Check if we have enough samples
        if metrics_snapshot.len() < self.config.min_samples {
            let result = PredictionResult {
                container_id: container_id.to_string(),
                pod_name,
                namespace,
                deployment,
                profile: None,
                skipped_reason: Some(format!(
                    "Insufficient data: {} samples, need {}",
                    metrics_snapshot.len(),
                    self.config.min_samples
                )),
                duration_us: start.elapsed().as_micros() as u64,
            };
            let _ = self.prediction_tx.send(result).await;
            return Ok(());
        }

        // Extract features
        let features = match self.feature_extractor.extract(&metrics_snapshot) {
            Some(f) => f,
            None => {
                let result = PredictionResult {
                    container_id: container_id.to_string(),
                    pod_name,
                    namespace,
                    deployment,
                    profile: None,
                    skipped_reason: Some("Feature extraction failed".to_string()),
                    duration_us: start.elapsed().as_micros() as u64,
                };
                let _ = self.prediction_tx.send(result).await;
                return Ok(());
            }
        };

        // Run prediction with timeout
        let profile = {
            let predictor = self.predictor.read().await;
            tokio::time::timeout(self.config.inference_timeout, async {
                predictor.predict(&features)
            })
            .await
        };

        let (profile, skipped_reason) = match profile {
            Ok(Ok(p)) => (Some(p), None),
            Ok(Err(e)) => {
                warn!(error = %e, "Inference error, using fallback");
                let fallback = super::FallbackPredictor::predict(&features);
                (Some(fallback), Some(format!("Fallback used: {}", e)))
            }
            Err(_) => {
                warn!("Inference timeout, using fallback");
                let fallback = super::FallbackPredictor::predict(&features);
                (Some(fallback), Some("Inference timeout".to_string()))
            }
        };

        // Update last prediction time
        {
            let mut buffers = self.buffers.write().await;
            if let Some(buffer) = buffers.get_mut(container_id) {
                buffer.last_prediction = Some(Instant::now());
                buffer.last_profile = profile.clone();
            }
        }

        let result = PredictionResult {
            container_id: container_id.to_string(),
            pod_name,
            namespace,
            deployment,
            profile,
            skipped_reason,
            duration_us: start.elapsed().as_micros() as u64,
        };

        debug!(
            container_id = %container_id,
            duration_us = result.duration_us,
            has_profile = result.profile.is_some(),
            "Prediction completed"
        );

        let _ = self.prediction_tx.send(result).await;
        Ok(())
    }

    /// Get the last prediction for a container
    pub async fn get_last_prediction(&self, container_id: &str) -> Option<ResourceProfile> {
        let buffers = self.buffers.read().await;
        buffers
            .get(container_id)
            .and_then(|b| b.last_profile.clone())
    }

    /// Get statistics about the scheduler
    pub async fn stats(&self) -> SchedulerStats {
        let buffers = self.buffers.read().await;
        let total_containers = buffers.len();
        let containers_with_predictions = buffers
            .values()
            .filter(|b| b.last_profile.is_some())
            .count();
        let total_samples: usize = buffers.values().map(|b| b.metrics.len()).sum();

        SchedulerStats {
            total_containers,
            containers_with_predictions,
            total_samples,
        }
    }

    /// Remove a container from tracking
    pub async fn remove_container(&self, container_id: &str) {
        let mut buffers = self.buffers.write().await;
        buffers.remove(container_id);
    }
}

/// Statistics about the prediction scheduler
#[derive(Debug, Clone)]
pub struct SchedulerStats {
    pub total_containers: usize,
    pub containers_with_predictions: usize,
    pub total_samples: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_metrics(container_id: &str, count: usize) -> Vec<ContainerMetrics> {
        let now = chrono::Utc::now().timestamp();
        (0..count)
            .map(|i| ContainerMetrics {
                container_id: container_id.to_string(),
                pod_name: "test-pod".to_string(),
                namespace: "default".to_string(),
                deployment: Some("test-deploy".to_string()),
                timestamp: now - (count - i - 1) as i64 * 10,
                cpu_usage_cores: 0.5 + (i as f32 * 0.01),
                cpu_throttled_periods: i as u64 * 10,
                memory_usage_bytes: 100_000_000 + (i as u64 * 1_000_000),
                memory_working_set_bytes: 100_000_000 + (i as u64 * 1_000_000),
                memory_cache_bytes: 10_000_000,
                network_rx_bytes: 1000,
                network_tx_bytes: 500,
            })
            .collect()
    }

    #[tokio::test]
    async fn test_add_metrics() {
        let predictor = Arc::new(RwLock::new(OnnxPredictor::new_without_model()));
        let (scheduler, _rx) = PredictionScheduler::new(predictor, PredictionConfig::default());

        let metrics = create_test_metrics("container1", 1);
        scheduler.add_metrics(metrics[0].clone()).await;

        let stats = scheduler.stats().await;
        assert_eq!(stats.total_containers, 1);
        assert_eq!(stats.total_samples, 1);
    }

    #[tokio::test]
    async fn test_insufficient_samples_skipped() {
        let predictor = Arc::new(RwLock::new(OnnxPredictor::new_without_model()));
        let (scheduler, mut rx) = PredictionScheduler::new(predictor, PredictionConfig::default());

        // Add only 5 samples (less than MIN_SAMPLES)
        for m in create_test_metrics("container1", 5) {
            scheduler.add_metrics(m).await;
        }

        scheduler.predict_container("container1").await.unwrap();

        let result = rx.try_recv().unwrap();
        assert!(result.profile.is_none());
        assert!(result.skipped_reason.is_some());
        assert!(result.skipped_reason.unwrap().contains("Insufficient"));
    }

    #[tokio::test]
    async fn test_prediction_with_sufficient_samples() {
        let predictor = Arc::new(RwLock::new(OnnxPredictor::new_without_model()));
        let (scheduler, mut rx) = PredictionScheduler::new(predictor, PredictionConfig::default());

        // Add enough samples
        for m in create_test_metrics("container1", 15) {
            scheduler.add_metrics(m).await;
        }

        scheduler.predict_container("container1").await.unwrap();

        let result = rx.try_recv().unwrap();
        assert!(result.profile.is_some()); // Should use fallback predictor
    }

    #[tokio::test]
    async fn test_remove_container() {
        let predictor = Arc::new(RwLock::new(OnnxPredictor::new_without_model()));
        let (scheduler, _rx) = PredictionScheduler::new(predictor, PredictionConfig::default());

        for m in create_test_metrics("container1", 5) {
            scheduler.add_metrics(m).await;
        }

        assert_eq!(scheduler.stats().await.total_containers, 1);

        scheduler.remove_container("container1").await;

        assert_eq!(scheduler.stats().await.total_containers, 0);
    }
}
