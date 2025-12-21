//! Metrics collection loop
//!
//! Implements the main collection loop that periodically gathers metrics
//! from all active containers with configurable intervals and jitter.

use super::{ContainerRegistry, MetricsCollector};
use crate::models::ContainerMetrics;
use anyhow::Result;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::mpsc;
use tokio::time::{interval, Instant};
use tracing::{debug, info, warn};

/// Configuration for the metrics collection loop
#[derive(Debug, Clone)]
pub struct CollectionConfig {
    /// Base collection interval (default: 10 seconds)
    pub interval: Duration,
    /// Maximum jitter to add to interval (default: 1 second)
    pub jitter: Duration,
    /// Degraded mode interval when under resource pressure (default: 60 seconds)
    pub degraded_interval: Duration,
    /// Maximum CPU usage percentage before entering degraded mode
    pub cpu_threshold_percent: f32,
    /// Channel buffer size for collected metrics
    pub buffer_size: usize,
}

impl Default for CollectionConfig {
    fn default() -> Self {
        Self {
            interval: Duration::from_secs(10),
            jitter: Duration::from_secs(1),
            degraded_interval: Duration::from_secs(60),
            cpu_threshold_percent: 2.0,
            buffer_size: 1000,
        }
    }
}

/// Metrics collection loop that periodically collects from all containers
pub struct CollectionLoop {
    /// Metrics collector implementation
    collector: Arc<dyn MetricsCollector>,
    /// Container registry
    registry: Arc<ContainerRegistry>,
    /// Configuration
    config: CollectionConfig,
    /// Channel to send collected metrics
    metrics_tx: mpsc::Sender<ContainerMetrics>,
    /// Whether running in degraded mode
    degraded_mode: bool,
}

impl CollectionLoop {
    /// Create a new collection loop
    pub fn new(
        collector: Arc<dyn MetricsCollector>,
        registry: Arc<ContainerRegistry>,
        config: CollectionConfig,
    ) -> (Self, mpsc::Receiver<ContainerMetrics>) {
        let (metrics_tx, metrics_rx) = mpsc::channel(config.buffer_size);

        let loop_instance = Self {
            collector,
            registry,
            config,
            metrics_tx,
            degraded_mode: false,
        };

        (loop_instance, metrics_rx)
    }

    /// Start the collection loop
    /// Returns a handle that can be used to stop the loop
    pub async fn run(mut self, mut shutdown: tokio::sync::broadcast::Receiver<()>) {
        info!(
            interval_secs = self.config.interval.as_secs(),
            "Starting metrics collection loop"
        );

        let mut ticker = interval(self.current_interval());
        let mut collection_count = 0u64;

        loop {
            tokio::select! {
                _ = ticker.tick() => {
                    let start = Instant::now();

                    // Collect metrics from all containers
                    let results = self.collect_all().await;

                    let elapsed = start.elapsed();
                    collection_count += 1;

                    // Log collection stats periodically
                    if collection_count % 6 == 0 {
                        // Every minute at 10s interval
                        debug!(
                            containers = results.success_count,
                            errors = results.error_count,
                            elapsed_ms = elapsed.as_millis(),
                            degraded = self.degraded_mode,
                            "Collection cycle complete"
                        );
                    }

                    // Check if we need to adjust collection interval
                    self.check_resource_pressure(elapsed);

                    // Update ticker if interval changed
                    ticker = interval(self.current_interval());
                }
                _ = shutdown.recv() => {
                    info!("Shutting down metrics collection loop");
                    break;
                }
            }
        }
    }

    /// Get the current collection interval (accounting for degraded mode)
    fn current_interval(&self) -> Duration {
        let base = if self.degraded_mode {
            self.config.degraded_interval
        } else {
            self.config.interval
        };

        // Add jitter to prevent thundering herd
        let jitter_ms = rand_jitter(self.config.jitter.as_millis() as u64);
        base + Duration::from_millis(jitter_ms)
    }

    /// Collect metrics from all registered containers
    async fn collect_all(&self) -> CollectionResults {
        let containers = self.registry.list();
        let mut results = CollectionResults::default();

        for container in containers {
            match self.collect_container(&container.container_id).await {
                Ok(metrics) => {
                    results.success_count += 1;

                    // Send metrics to channel
                    if let Err(e) = self.metrics_tx.send(metrics).await {
                        warn!(error = %e, "Failed to send metrics to channel");
                    }
                }
                Err(e) => {
                    results.error_count += 1;
                    debug!(
                        container_id = %container.container_id,
                        error = %e,
                        "Failed to collect metrics"
                    );
                }
            }
        }

        results
    }

    /// Collect metrics for a single container
    async fn collect_container(&self, container_id: &str) -> Result<ContainerMetrics> {
        self.collector.collect(container_id).await
    }

    /// Check resource pressure and adjust collection mode
    fn check_resource_pressure(&mut self, collection_duration: Duration) {
        // Simple heuristic: if collection takes too long, we might be under pressure
        // In production, this would check actual CPU usage
        let threshold = Duration::from_millis(500);

        if collection_duration > threshold && !self.degraded_mode {
            warn!(
                elapsed_ms = collection_duration.as_millis(),
                "Entering degraded mode due to slow collection"
            );
            self.degraded_mode = true;
        } else if collection_duration < threshold / 2 && self.degraded_mode {
            info!("Exiting degraded mode, collection performance improved");
            self.degraded_mode = false;
        }
    }
}

/// Results from a collection cycle
#[derive(Debug, Default)]
struct CollectionResults {
    success_count: usize,
    error_count: usize,
}

/// Generate a random jitter value between 0 and max_ms
fn rand_jitter(max_ms: u64) -> u64 {
    if max_ms == 0 {
        return 0;
    }

    // Simple pseudo-random based on current time
    // In production, use a proper RNG
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos() as u64;

    now % max_ms
}

/// Builder for creating and starting the collection loop
pub struct CollectionLoopBuilder {
    collector: Option<Arc<dyn MetricsCollector>>,
    registry: Option<Arc<ContainerRegistry>>,
    config: CollectionConfig,
}

impl CollectionLoopBuilder {
    /// Create a new builder with default configuration
    pub fn new() -> Self {
        Self {
            collector: None,
            registry: None,
            config: CollectionConfig::default(),
        }
    }

    /// Set the metrics collector
    pub fn collector(mut self, collector: Arc<dyn MetricsCollector>) -> Self {
        self.collector = Some(collector);
        self
    }

    /// Set the container registry
    pub fn registry(mut self, registry: Arc<ContainerRegistry>) -> Self {
        self.registry = Some(registry);
        self
    }

    /// Set the collection interval
    pub fn interval(mut self, interval: Duration) -> Self {
        self.config.interval = interval;
        self
    }

    /// Set the jitter duration
    pub fn jitter(mut self, jitter: Duration) -> Self {
        self.config.jitter = jitter;
        self
    }

    /// Set the degraded mode interval
    pub fn degraded_interval(mut self, interval: Duration) -> Self {
        self.config.degraded_interval = interval;
        self
    }

    /// Set the CPU threshold for degraded mode
    pub fn cpu_threshold(mut self, percent: f32) -> Self {
        self.config.cpu_threshold_percent = percent;
        self
    }

    /// Set the buffer size
    pub fn buffer_size(mut self, size: usize) -> Self {
        self.config.buffer_size = size;
        self
    }

    /// Build the collection loop
    pub fn build(self) -> Result<(CollectionLoop, mpsc::Receiver<ContainerMetrics>)> {
        let collector = self
            .collector
            .ok_or_else(|| anyhow::anyhow!("Collector is required"))?;
        let registry = self
            .registry
            .ok_or_else(|| anyhow::anyhow!("Registry is required"))?;

        Ok(CollectionLoop::new(collector, registry, self.config))
    }
}

impl Default for CollectionLoopBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::models::ContainerInfo;
    use async_trait::async_trait;
    use std::sync::atomic::{AtomicUsize, Ordering};

    /// Mock collector for testing
    struct MockCollector {
        call_count: AtomicUsize,
    }

    impl MockCollector {
        fn new() -> Self {
            Self {
                call_count: AtomicUsize::new(0),
            }
        }
    }

    #[async_trait]
    impl MetricsCollector for MockCollector {
        async fn collect(&self, container_id: &str) -> Result<ContainerMetrics> {
            self.call_count.fetch_add(1, Ordering::SeqCst);

            Ok(ContainerMetrics {
                container_id: container_id.to_string(),
                pod_name: "test-pod".to_string(),
                namespace: "default".to_string(),
                deployment: None,
                timestamp: chrono::Utc::now().timestamp(),
                cpu_usage_cores: 0.5,
                cpu_throttled_periods: 0,
                memory_usage_bytes: 100_000_000,
                memory_working_set_bytes: 80_000_000,
                memory_cache_bytes: 20_000_000,
                network_rx_bytes: 1000,
                network_tx_bytes: 500,
            })
        }

        async fn list_containers(&self) -> Result<Vec<ContainerInfo>> {
            Ok(vec![])
        }
    }

    #[test]
    fn test_collection_config_default() {
        let config = CollectionConfig::default();
        assert_eq!(config.interval, Duration::from_secs(10));
        assert_eq!(config.jitter, Duration::from_secs(1));
        assert_eq!(config.degraded_interval, Duration::from_secs(60));
    }

    #[test]
    fn test_rand_jitter() {
        let jitter = rand_jitter(1000);
        assert!(jitter < 1000);

        // Zero max should return zero
        assert_eq!(rand_jitter(0), 0);
    }

    #[tokio::test]
    async fn test_collection_loop_builder() {
        let collector = Arc::new(MockCollector::new());
        let registry = Arc::new(ContainerRegistry::new("test-node"));

        let result = CollectionLoopBuilder::new()
            .collector(collector)
            .registry(registry)
            .interval(Duration::from_secs(5))
            .build();

        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_collection_loop_builder_missing_collector() {
        let registry = Arc::new(ContainerRegistry::new("test-node"));

        let result = CollectionLoopBuilder::new().registry(registry).build();

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_collect_all_empty_registry() {
        let collector = Arc::new(MockCollector::new());
        let registry = Arc::new(ContainerRegistry::new("test-node"));

        let (collection_loop, _rx) =
            CollectionLoop::new(collector.clone(), registry, CollectionConfig::default());

        let results = collection_loop.collect_all().await;

        assert_eq!(results.success_count, 0);
        assert_eq!(results.error_count, 0);
    }

    #[tokio::test]
    async fn test_collect_all_with_containers() {
        let collector = Arc::new(MockCollector::new());
        let registry = Arc::new(ContainerRegistry::new("test-node"));

        // Register some containers
        registry.register(ContainerInfo {
            container_id: "container1".to_string(),
            pod_name: "pod1".to_string(),
            namespace: "default".to_string(),
            deployment: None,
            node_name: String::new(),
            cgroup_path: "/test/path1".to_string(),
        });

        registry.register(ContainerInfo {
            container_id: "container2".to_string(),
            pod_name: "pod2".to_string(),
            namespace: "default".to_string(),
            deployment: None,
            node_name: String::new(),
            cgroup_path: "/test/path2".to_string(),
        });

        let (collection_loop, mut rx) =
            CollectionLoop::new(collector.clone(), registry, CollectionConfig::default());

        let results = collection_loop.collect_all().await;

        assert_eq!(results.success_count, 2);
        assert_eq!(results.error_count, 0);

        // Verify metrics were sent
        let metrics1 = rx.try_recv().unwrap();
        let metrics2 = rx.try_recv().unwrap();

        assert!(
            metrics1.container_id == "container1" || metrics1.container_id == "container2"
        );
        assert!(
            metrics2.container_id == "container1" || metrics2.container_id == "container2"
        );
    }
}
