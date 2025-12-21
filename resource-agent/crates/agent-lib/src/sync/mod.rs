//! Synchronization with Recommendation API
//!
//! This module provides:
//! - gRPC client with mTLS support for secure API communication
//! - Local metrics buffer for offline operation
//! - Metrics streaming with backpressure handling
//! - Model update client with validation

mod buffer;
mod client;
mod model_update;
mod streaming;

#[cfg(test)]
mod tests;

pub use buffer::{BufferConfig, BufferStats, MetricsBuffer, OfflineBufferManager};
pub use client::{ClientConfig, SyncClient, SyncClientBuilder};
pub use model_update::{
    ModelUpdateClient, ModelUpdateConfig, ModelUpdateStats, ModelUpdateWorker, ModelVersion,
    ValidationResult,
};
pub use streaming::{
    AnomalyData, MetricsStreamer, PendingData, StreamingConfig, StreamingStats, StreamingWorker,
};
