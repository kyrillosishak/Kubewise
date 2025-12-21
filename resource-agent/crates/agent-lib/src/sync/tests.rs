//! Integration tests for sync module
//!
//! These tests verify:
//! - Reconnection with buffered data
//! - Model update flow

use super::*;
use crate::models::ContainerMetrics;
use std::time::Duration;
use tempfile::TempDir;

/// Helper to create test metrics
fn create_test_metrics(id: &str, timestamp: i64) -> ContainerMetrics {
    ContainerMetrics {
        container_id: id.to_string(),
        pod_name: format!("pod-{}", id),
        namespace: "default".to_string(),
        deployment: Some("test-deployment".to_string()),
        timestamp,
        cpu_usage_cores: 0.5,
        cpu_throttled_periods: 10,
        memory_usage_bytes: 1024 * 1024,
        memory_working_set_bytes: 512 * 1024,
        memory_cache_bytes: 256 * 1024,
        network_rx_bytes: 1000,
        network_tx_bytes: 2000,
    }
}

mod buffer_reconnection_tests {
    use super::*;

    #[tokio::test]
    async fn test_offline_buffering_and_sync() {
        let config = BufferConfig {
            max_retention: Duration::from_secs(3600),
            max_size: 1000,
            persistence_path: None,
            flush_interval: Duration::from_secs(60),
        };

        let mut manager = OfflineBufferManager::new(config);

        // Initially online - should not buffer
        assert!(!manager.is_offline());
        assert!(!manager.buffer_if_offline(create_test_metrics("c1", 1000)));
        assert_eq!(manager.pending_sync_count(), 0);

        // Go offline
        manager.go_offline();
        assert!(manager.is_offline());

        // Buffer metrics while offline
        for i in 0..10 {
            assert!(
                manager.buffer_if_offline(create_test_metrics(&format!("c{}", i), 1000 + i as i64))
            );
        }
        assert_eq!(manager.pending_sync_count(), 10);

        // Go online
        manager.go_online();
        assert!(!manager.is_offline());
        assert!(manager.has_data_to_sync());

        // Drain for sync
        let synced = manager.drain_for_sync();
        assert_eq!(synced.len(), 10);
        assert!(!manager.has_data_to_sync());
    }

    #[tokio::test]
    async fn test_batch_sync_on_reconnection() {
        let config = BufferConfig {
            max_retention: Duration::from_secs(3600),
            max_size: 1000,
            persistence_path: None,
            flush_interval: Duration::from_secs(60),
        };

        let mut manager = OfflineBufferManager::new(config);

        // Go offline and buffer many metrics
        manager.go_offline();
        for i in 0..100 {
            manager.buffer(create_test_metrics(&format!("c{}", i), 1000 + i as i64));
        }

        // Go online and sync in batches
        manager.go_online();

        let batch1 = manager.drain_batch_for_sync(30);
        assert_eq!(batch1.len(), 30);
        assert_eq!(manager.pending_sync_count(), 70);

        let batch2 = manager.drain_batch_for_sync(30);
        assert_eq!(batch2.len(), 30);
        assert_eq!(manager.pending_sync_count(), 40);

        let batch3 = manager.drain_batch_for_sync(50);
        assert_eq!(batch3.len(), 40); // Only 40 remaining
        assert_eq!(manager.pending_sync_count(), 0);
    }

    #[tokio::test]
    async fn test_buffer_persistence() {
        let temp_dir = TempDir::new().unwrap();
        let persistence_path = temp_dir.path().join("buffer.json");

        // Create buffer with persistence and add data
        {
            let config = BufferConfig {
                max_retention: Duration::from_secs(3600),
                max_size: 1000,
                persistence_path: Some(persistence_path.clone()),
                flush_interval: Duration::from_secs(1),
            };

            let mut buffer = MetricsBuffer::with_config(config);

            for i in 0..5 {
                buffer.push(create_test_metrics(&format!("c{}", i), 1000 + i as i64));
            }

            // Flush to disk
            buffer.flush().unwrap();
        }

        // Verify file exists
        assert!(persistence_path.exists());

        // Create new buffer and load from disk
        {
            let buffer = MetricsBuffer::with_persistence(persistence_path.clone()).unwrap();
            assert_eq!(buffer.len(), 5);
        }
    }

    #[tokio::test]
    async fn test_buffer_retention_eviction() {
        let config = BufferConfig {
            max_retention: Duration::from_millis(100), // Very short retention
            max_size: 1000,
            persistence_path: None,
            flush_interval: Duration::from_secs(60),
        };

        let mut buffer = MetricsBuffer::with_config(config);

        // Add some metrics
        for i in 0..5 {
            buffer.push(create_test_metrics(&format!("c{}", i), 1000 + i as i64));
        }
        assert_eq!(buffer.len(), 5);

        // Wait for retention to expire
        tokio::time::sleep(Duration::from_millis(150)).await;

        // Add new metric - should trigger eviction of old ones
        buffer.push(create_test_metrics("new", 2000));

        // Old metrics should be evicted
        assert_eq!(buffer.len(), 1);
    }

    #[tokio::test]
    async fn test_buffer_capacity_eviction() {
        let config = BufferConfig {
            max_retention: Duration::from_secs(3600),
            max_size: 10, // Small capacity
            persistence_path: None,
            flush_interval: Duration::from_secs(60),
        };

        let mut buffer = MetricsBuffer::with_config(config);

        // Add more than capacity
        for i in 0..20 {
            buffer.push(create_test_metrics(&format!("c{}", i), 1000 + i as i64));
        }

        // Should only have max_size entries
        assert_eq!(buffer.len(), 10);

        // Should have the newest entries
        let drained = buffer.drain();
        assert_eq!(drained[0].container_id, "c10");
        assert_eq!(drained[9].container_id, "c19");
    }
}

mod model_update_tests {
    use super::*;
    use std::fs;

    #[tokio::test]
    async fn test_model_update_flow() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            versions_to_keep: 3,
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Initially no model
        assert!(client.current_version().await.is_none());

        // Create and load a model
        let model_path = temp_dir.path().join("model_v1.onnx");
        fs::write(&model_path, b"model v1 weights").unwrap();

        client
            .load_existing_model("v1.0.0", &model_path)
            .await
            .unwrap();

        // Verify model is loaded
        assert_eq!(client.current_version().await, Some("v1.0.0".to_string()));
        assert!(client.current_model_path().await.is_some());
    }

    #[tokio::test]
    async fn test_model_rollback_flow() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            versions_to_keep: 3,
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Load initial model
        let model_v1_path = temp_dir.path().join("model_v1.onnx");
        fs::write(&model_v1_path, b"model v1 weights").unwrap();
        client
            .load_existing_model("v1.0.0", &model_v1_path)
            .await
            .unwrap();

        // No rollback available yet
        let rollback_versions = client.available_rollback_versions().await;
        assert!(rollback_versions.is_empty());

        // Try rollback with no previous versions
        let result = client.rollback().await.unwrap();
        assert!(result.is_none());
    }

    #[tokio::test]
    async fn test_model_checksum_validation() {
        let data1 = b"model weights version 1";
        let data2 = b"model weights version 2";

        let checksum1 = sha2_checksum(data1);
        let checksum2 = sha2_checksum(data2);

        // Different data should have different checksums
        assert_ne!(checksum1, checksum2);

        // Same data should have same checksum
        let checksum1_again = sha2_checksum(data1);
        assert_eq!(checksum1, checksum1_again);
    }

    #[tokio::test]
    async fn test_model_update_stats() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Initial stats
        let stats = client.stats().await;
        assert!(stats.current_version.is_none());
        assert_eq!(stats.available_rollback_versions, 0);

        // Load a model
        let model_path = temp_dir.path().join("model_v1.onnx");
        fs::write(&model_path, b"model v1 weights").unwrap();
        client
            .load_existing_model("v1.0.0", &model_path)
            .await
            .unwrap();

        // Updated stats
        let stats = client.stats().await;
        assert_eq!(stats.current_version, Some("v1.0.0".to_string()));
        assert!(stats.current_size_bytes.is_some());
        assert!(stats.last_update_time.is_some());
    }

    #[test]
    fn test_update_window_check() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            update_window_start: 2,
            update_window_end: 4,
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // The is_update_window check depends on current time
        // Just verify it doesn't panic
        let _ = client.is_update_window();
    }

    #[test]
    fn test_deviation_threshold() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            max_deviation_threshold: 0.20,
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Below threshold
        assert!(!client.exceeds_deviation_threshold(0.10));
        assert!(!client.exceeds_deviation_threshold(0.19));

        // At threshold
        assert!(!client.exceeds_deviation_threshold(0.20));

        // Above threshold
        assert!(client.exceeds_deviation_threshold(0.21));
        assert!(client.exceeds_deviation_threshold(0.50));
    }

    /// Helper to compute SHA256 checksum
    fn sha2_checksum(data: &[u8]) -> String {
        use sha2::{Digest, Sha256};
        let mut hasher = Sha256::new();
        hasher.update(data);
        hex::encode(hasher.finalize())
    }
}

mod streaming_tests {
    use super::*;

    #[tokio::test]
    async fn test_streamer_queue_and_receive() {
        let config = StreamingConfig {
            max_batch_size: 10,
            channel_buffer_size: 100,
            ..Default::default()
        };

        let (streamer, mut receiver) =
            MetricsStreamer::new(config, "test-agent".to_string(), "test-node".to_string());

        // Queue some metrics
        let metrics = vec![
            create_test_metrics("c1", 1000),
            create_test_metrics("c2", 1001),
        ];

        streamer.queue_metrics(metrics).await.unwrap();

        // Receive the data
        let received = receiver.recv().await.unwrap();
        assert_eq!(received.metrics.len(), 2);
    }

    #[tokio::test]
    async fn test_streamer_backpressure() {
        let config = StreamingConfig {
            max_batch_size: 10,
            channel_buffer_size: 2, // Very small buffer
            ..Default::default()
        };

        let (streamer, _receiver) =
            MetricsStreamer::new(config, "test-agent".to_string(), "test-node".to_string());

        // Fill the buffer
        for i in 0..2 {
            let metrics = vec![create_test_metrics(&format!("c{}", i), 1000 + i as i64)];
            streamer.queue_metrics(metrics).await.unwrap();
        }

        // Try to queue more - should apply backpressure
        let data = PendingData {
            metrics: vec![create_test_metrics("overflow", 2000)],
            ..Default::default()
        };

        // try_queue should return false when buffer is full
        let result = streamer.try_queue(data);
        assert!(!result);
    }

    #[tokio::test]
    async fn test_streaming_stats() {
        let config = StreamingConfig::default();
        let (streamer, _receiver) =
            MetricsStreamer::new(config, "test-agent".to_string(), "test-node".to_string());

        let stats = streamer.stats().await;
        assert_eq!(stats.batches_sent, 0);
        assert_eq!(stats.metrics_sent, 0);
        assert!(stats.last_error.is_none());
    }
}

mod client_tests {
    use super::*;

    #[tokio::test]
    async fn test_client_builder() {
        let client = SyncClientBuilder::new()
            .endpoint("https://test-api:8443")
            .agent_id("test-agent")
            .node_name("test-node")
            .connect_timeout(Duration::from_secs(5))
            .request_timeout(Duration::from_secs(30))
            .initial_backoff(Duration::from_secs(1))
            .max_backoff(Duration::from_secs(300))
            .build()
            .unwrap();

        assert_eq!(client.endpoint(), "https://test-api:8443");
        assert_eq!(client.agent_id(), "test-agent");
        assert_eq!(client.node_name(), "test-node");
        assert_eq!(client.connect_timeout(), Duration::from_secs(5));
    }

    #[tokio::test]
    async fn test_client_connection_state() {
        let client = SyncClientBuilder::new()
            .endpoint("https://test-api:8443")
            .agent_id("test-agent")
            .node_name("test-node")
            .build()
            .unwrap();

        // Initially not connected
        assert!(!client.is_connected().await);

        let (connected, attempts, error) = client.connection_stats().await;
        assert!(!connected);
        assert_eq!(attempts, 0);
        assert!(error.is_none());
    }

    #[tokio::test]
    async fn test_client_backoff() {
        let client = SyncClientBuilder::new()
            .endpoint("https://test-api:8443")
            .agent_id("test-agent")
            .node_name("test-node")
            .initial_backoff(Duration::from_secs(1))
            .build()
            .unwrap();

        let backoff = client.get_reconnect_backoff().await;
        assert_eq!(backoff, Duration::from_secs(1));
    }
}
