//! Local metric buffer for offline operation
//!
//! This module provides a ring buffer for storing metrics during API disconnection:
//! - Memory-mapped ring buffer for persistence
//! - 24-hour retention with FIFO eviction
//! - Sync buffered data on reconnection

use crate::models::ContainerMetrics;
use anyhow::{Context, Result};
use std::collections::VecDeque;
use std::fs::{File, OpenOptions};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tracing::{debug, info, warn};

/// Default retention period (24 hours)
const DEFAULT_RETENTION: Duration = Duration::from_secs(24 * 60 * 60);

/// Default maximum buffer size (100,000 entries)
const DEFAULT_MAX_SIZE: usize = 100_000;

/// Configuration for the metrics buffer
#[derive(Debug, Clone)]
pub struct BufferConfig {
    /// Maximum retention period for metrics
    pub max_retention: Duration,
    /// Maximum number of entries in the buffer
    pub max_size: usize,
    /// Path for persistent storage (optional)
    pub persistence_path: Option<PathBuf>,
    /// Flush interval for persistence
    pub flush_interval: Duration,
}

impl Default for BufferConfig {
    fn default() -> Self {
        Self {
            max_retention: DEFAULT_RETENTION,
            max_size: DEFAULT_MAX_SIZE,
            persistence_path: None,
            flush_interval: Duration::from_secs(60),
        }
    }
}

/// Ring buffer for storing metrics during API disconnection
pub struct MetricsBuffer {
    /// In-memory buffer
    buffer: VecDeque<TimestampedMetrics>,
    /// Configuration
    config: BufferConfig,
    /// Last flush time
    last_flush: SystemTime,
    /// Dirty flag for persistence
    dirty: bool,
}

/// Metrics with timestamp for retention management
#[derive(Debug, Clone)]
struct TimestampedMetrics {
    metrics: ContainerMetrics,
    buffered_at: SystemTime,
}

impl MetricsBuffer {
    /// Create a new metrics buffer with default configuration
    pub fn new(max_retention: Duration, max_size: usize) -> Self {
        Self {
            buffer: VecDeque::with_capacity(max_size.min(10_000)),
            config: BufferConfig {
                max_retention,
                max_size,
                ..Default::default()
            },
            last_flush: SystemTime::now(),
            dirty: false,
        }
    }

    /// Create a new metrics buffer with full configuration
    pub fn with_config(config: BufferConfig) -> Self {
        Self {
            buffer: VecDeque::with_capacity(config.max_size.min(10_000)),
            config,
            last_flush: SystemTime::now(),
            dirty: false,
        }
    }

    /// Create a buffer with persistence
    pub fn with_persistence(persistence_path: PathBuf) -> Result<Self> {
        let config = BufferConfig {
            persistence_path: Some(persistence_path.clone()),
            ..Default::default()
        };

        let mut buffer = Self::with_config(config);

        // Try to load existing data
        if persistence_path.exists() {
            if let Err(e) = buffer.load_from_disk() {
                warn!(error = %e, "Failed to load persisted buffer, starting fresh");
            }
        }

        Ok(buffer)
    }

    /// Add metrics to buffer
    pub fn push(&mut self, metrics: ContainerMetrics) {
        // Evict old entries if at capacity
        while self.buffer.len() >= self.config.max_size {
            self.buffer.pop_front();
        }

        // Evict expired entries
        self.evict_expired();

        self.buffer.push_back(TimestampedMetrics {
            metrics,
            buffered_at: SystemTime::now(),
        });
        self.dirty = true;
    }

    /// Add multiple metrics to buffer
    pub fn push_batch(&mut self, metrics: Vec<ContainerMetrics>) {
        for m in metrics {
            self.push(m);
        }
    }

    /// Drain all buffered metrics
    pub fn drain(&mut self) -> Vec<ContainerMetrics> {
        self.dirty = true;
        self.buffer.drain(..).map(|tm| tm.metrics).collect()
    }

    /// Drain metrics up to a limit
    pub fn drain_batch(&mut self, limit: usize) -> Vec<ContainerMetrics> {
        let count = limit.min(self.buffer.len());
        self.dirty = true;
        self.buffer.drain(..count).map(|tm| tm.metrics).collect()
    }

    /// Peek at buffered metrics without removing them
    pub fn peek(&self, limit: usize) -> Vec<&ContainerMetrics> {
        self.buffer
            .iter()
            .take(limit)
            .map(|tm| &tm.metrics)
            .collect()
    }

    /// Get buffer size
    pub fn len(&self) -> usize {
        self.buffer.len()
    }

    /// Check if buffer is empty
    pub fn is_empty(&self) -> bool {
        self.buffer.is_empty()
    }

    /// Get buffer capacity
    pub fn capacity(&self) -> usize {
        self.config.max_size
    }

    /// Get approximate memory usage in bytes
    pub fn memory_usage(&self) -> usize {
        // Rough estimate: each ContainerMetrics is about 200 bytes
        self.buffer.len() * 200
    }

    /// Evict expired entries based on retention period
    fn evict_expired(&mut self) {
        let now = SystemTime::now();
        let cutoff = now - self.config.max_retention;

        while let Some(front) = self.buffer.front() {
            if front.buffered_at < cutoff {
                self.buffer.pop_front();
                self.dirty = true;
            } else {
                break;
            }
        }
    }

    /// Flush buffer to disk if persistence is enabled
    pub fn flush(&mut self) -> Result<()> {
        if !self.dirty {
            return Ok(());
        }

        if let Some(ref path) = self.config.persistence_path {
            self.save_to_disk(path)?;
            self.dirty = false;
            self.last_flush = SystemTime::now();
            debug!(path = %path.display(), entries = self.buffer.len(), "Buffer flushed to disk");
        }

        Ok(())
    }

    /// Check if flush is needed based on interval
    pub fn should_flush(&self) -> bool {
        self.dirty
            && self.config.persistence_path.is_some()
            && self.last_flush.elapsed().unwrap_or_default() >= self.config.flush_interval
    }

    /// Save buffer to disk
    fn save_to_disk(&self, path: &Path) -> Result<()> {
        // Create parent directories if needed
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create directory {:?}", parent))?;
        }

        // Serialize metrics to JSON
        let metrics: Vec<&ContainerMetrics> = self.buffer.iter().map(|tm| &tm.metrics).collect();
        let json = serde_json::to_vec(&metrics).context("Failed to serialize metrics")?;

        // Write atomically using temp file
        let temp_path = path.with_extension("tmp");
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .open(&temp_path)
            .with_context(|| format!("Failed to create temp file {:?}", temp_path))?;

        file.write_all(&json)
            .context("Failed to write buffer data")?;
        file.sync_all().context("Failed to sync buffer file")?;

        // Rename temp file to final path
        std::fs::rename(&temp_path, path)
            .with_context(|| format!("Failed to rename {:?} to {:?}", temp_path, path))?;

        Ok(())
    }

    /// Load buffer from disk
    fn load_from_disk(&mut self) -> Result<()> {
        let path = self
            .config
            .persistence_path
            .as_ref()
            .ok_or_else(|| anyhow::anyhow!("No persistence path configured"))?;

        let mut file =
            File::open(path).with_context(|| format!("Failed to open buffer file {:?}", path))?;

        let mut data = Vec::new();
        file.read_to_end(&mut data)
            .context("Failed to read buffer file")?;

        let metrics: Vec<ContainerMetrics> =
            serde_json::from_slice(&data).context("Failed to deserialize buffer data")?;

        let now = SystemTime::now();
        for m in metrics {
            self.buffer.push_back(TimestampedMetrics {
                metrics: m,
                buffered_at: now,
            });
        }

        info!(path = %path.display(), entries = self.buffer.len(), "Loaded buffer from disk");
        Ok(())
    }

    /// Get statistics about the buffer
    pub fn stats(&self) -> BufferStats {
        let oldest = self.buffer.front().map(|tm| {
            tm.buffered_at
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs()
        });

        let newest = self.buffer.back().map(|tm| {
            tm.buffered_at
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs()
        });

        BufferStats {
            entries: self.buffer.len(),
            capacity: self.config.max_size,
            memory_bytes: self.memory_usage(),
            oldest_timestamp: oldest,
            newest_timestamp: newest,
            retention_seconds: self.config.max_retention.as_secs(),
        }
    }
}

/// Buffer statistics
#[derive(Debug, Clone)]
pub struct BufferStats {
    /// Number of entries in buffer
    pub entries: usize,
    /// Maximum capacity
    pub capacity: usize,
    /// Approximate memory usage in bytes
    pub memory_bytes: usize,
    /// Oldest entry timestamp (Unix seconds)
    pub oldest_timestamp: Option<u64>,
    /// Newest entry timestamp (Unix seconds)
    pub newest_timestamp: Option<u64>,
    /// Retention period in seconds
    pub retention_seconds: u64,
}

/// Offline buffer manager that handles sync on reconnection
pub struct OfflineBufferManager {
    buffer: MetricsBuffer,
    /// Flag indicating if we're currently offline
    offline: bool,
    /// Number of entries buffered while offline
    offline_entries: usize,
}

impl OfflineBufferManager {
    /// Create a new offline buffer manager
    pub fn new(config: BufferConfig) -> Self {
        Self {
            buffer: MetricsBuffer::with_config(config),
            offline: false,
            offline_entries: 0,
        }
    }

    /// Create with persistence
    pub fn with_persistence(path: PathBuf) -> Result<Self> {
        Ok(Self {
            buffer: MetricsBuffer::with_persistence(path)?,
            offline: false,
            offline_entries: 0,
        })
    }

    /// Mark as offline and start buffering
    pub fn go_offline(&mut self) {
        if !self.offline {
            info!("Going offline, starting to buffer metrics");
            self.offline = true;
            self.offline_entries = 0;
        }
    }

    /// Mark as online
    pub fn go_online(&mut self) {
        if self.offline {
            info!(
                buffered_entries = self.offline_entries,
                "Going online, ready to sync buffered data"
            );
            self.offline = false;
        }
    }

    /// Check if currently offline
    pub fn is_offline(&self) -> bool {
        self.offline
    }

    /// Buffer metrics (only when offline)
    pub fn buffer_if_offline(&mut self, metrics: ContainerMetrics) -> bool {
        if self.offline {
            self.buffer.push(metrics);
            self.offline_entries += 1;
            true
        } else {
            false
        }
    }

    /// Buffer metrics unconditionally
    pub fn buffer(&mut self, metrics: ContainerMetrics) {
        self.buffer.push(metrics);
        if self.offline {
            self.offline_entries += 1;
        }
    }

    /// Get buffered metrics for sync (drains the buffer)
    pub fn drain_for_sync(&mut self) -> Vec<ContainerMetrics> {
        self.buffer.drain()
    }

    /// Get a batch of buffered metrics for sync
    pub fn drain_batch_for_sync(&mut self, limit: usize) -> Vec<ContainerMetrics> {
        self.buffer.drain_batch(limit)
    }

    /// Check if there's data to sync
    pub fn has_data_to_sync(&self) -> bool {
        !self.buffer.is_empty()
    }

    /// Get number of entries waiting to sync
    pub fn pending_sync_count(&self) -> usize {
        self.buffer.len()
    }

    /// Flush to disk if needed
    pub fn flush(&mut self) -> Result<()> {
        if self.buffer.should_flush() {
            self.buffer.flush()?;
        }
        Ok(())
    }

    /// Get buffer statistics
    pub fn stats(&self) -> BufferStats {
        self.buffer.stats()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_metrics(id: &str) -> ContainerMetrics {
        ContainerMetrics {
            container_id: id.to_string(),
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
        }
    }

    #[test]
    fn test_buffer_push_and_drain() {
        let mut buffer = MetricsBuffer::new(Duration::from_secs(3600), 100);

        buffer.push(create_test_metrics("container-1"));
        buffer.push(create_test_metrics("container-2"));

        assert_eq!(buffer.len(), 2);

        let drained = buffer.drain();
        assert_eq!(drained.len(), 2);
        assert!(buffer.is_empty());
    }

    #[test]
    fn test_buffer_capacity_limit() {
        let mut buffer = MetricsBuffer::new(Duration::from_secs(3600), 5);

        for i in 0..10 {
            buffer.push(create_test_metrics(&format!("container-{}", i)));
        }

        // Should only have 5 entries (max_size)
        assert_eq!(buffer.len(), 5);

        // Should have the last 5 entries
        let drained = buffer.drain();
        assert_eq!(drained[0].container_id, "container-5");
        assert_eq!(drained[4].container_id, "container-9");
    }

    #[test]
    fn test_buffer_drain_batch() {
        let mut buffer = MetricsBuffer::new(Duration::from_secs(3600), 100);

        for i in 0..10 {
            buffer.push(create_test_metrics(&format!("container-{}", i)));
        }

        let batch = buffer.drain_batch(3);
        assert_eq!(batch.len(), 3);
        assert_eq!(buffer.len(), 7);
    }

    #[test]
    fn test_buffer_peek() {
        let mut buffer = MetricsBuffer::new(Duration::from_secs(3600), 100);

        for i in 0..5 {
            buffer.push(create_test_metrics(&format!("container-{}", i)));
        }

        let peeked = buffer.peek(3);
        assert_eq!(peeked.len(), 3);
        assert_eq!(buffer.len(), 5); // Buffer unchanged
    }

    #[test]
    fn test_buffer_stats() {
        let mut buffer = MetricsBuffer::new(Duration::from_secs(3600), 100);

        for i in 0..5 {
            buffer.push(create_test_metrics(&format!("container-{}", i)));
        }

        let stats = buffer.stats();
        assert_eq!(stats.entries, 5);
        assert_eq!(stats.capacity, 100);
        assert!(stats.oldest_timestamp.is_some());
        assert!(stats.newest_timestamp.is_some());
    }

    #[test]
    fn test_offline_buffer_manager() {
        let config = BufferConfig::default();
        let mut manager = OfflineBufferManager::new(config);

        assert!(!manager.is_offline());

        // Should not buffer when online
        assert!(!manager.buffer_if_offline(create_test_metrics("container-1")));
        assert_eq!(manager.pending_sync_count(), 0);

        // Go offline
        manager.go_offline();
        assert!(manager.is_offline());

        // Should buffer when offline
        assert!(manager.buffer_if_offline(create_test_metrics("container-2")));
        assert_eq!(manager.pending_sync_count(), 1);

        // Go online and sync
        manager.go_online();
        assert!(!manager.is_offline());
        assert!(manager.has_data_to_sync());

        let synced = manager.drain_for_sync();
        assert_eq!(synced.len(), 1);
        assert!(!manager.has_data_to_sync());
    }

    #[test]
    fn test_buffer_config_default() {
        let config = BufferConfig::default();
        assert_eq!(config.max_retention, Duration::from_secs(24 * 60 * 60));
        assert_eq!(config.max_size, 100_000);
        assert!(config.persistence_path.is_none());
    }
}
