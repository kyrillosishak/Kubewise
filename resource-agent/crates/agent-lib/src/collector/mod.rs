//! Metrics collection from container runtimes
//!
//! This module provides collectors for reading container resource metrics
//! from cgroup filesystems. It supports both cgroup v2 (unified hierarchy)
//! and cgroup v1 (legacy hierarchy) with automatic detection.

mod cgroup_v1;
mod cgroup_v2;
mod discovery;
mod r#loop;

#[cfg(test)]
mod tests;

pub use cgroup_v1::{CgroupV1Collector, CgroupVersion, detect_cgroup_version};
pub use cgroup_v2::CgroupV2Collector;
pub use discovery::{
    discover_existing_containers, ContainerEvent, ContainerRegistry, ContainerWatcher,
    K8sMetadataFetcher, WatcherHandle,
};
pub use r#loop::{CollectionConfig, CollectionLoop, CollectionLoopBuilder};

use crate::models::{ContainerInfo, ContainerMetrics};
use anyhow::Result;
use std::path::Path;
use std::sync::Arc;

pub use async_trait::async_trait;

/// Trait for metrics collection implementations
#[async_trait]
pub trait MetricsCollector: Send + Sync {
    /// Collect metrics for a specific container
    async fn collect(&self, container_id: &str) -> Result<ContainerMetrics>;

    /// List all active containers on the node
    async fn list_containers(&self) -> Result<Vec<ContainerInfo>>;
}

/// Create the appropriate collector based on detected cgroup version
pub async fn create_collector(cgroup_root: &Path) -> Result<Arc<dyn MetricsCollector>> {
    let version = detect_cgroup_version(cgroup_root).await;

    match version {
        CgroupVersion::V2 => {
            tracing::info!("Detected cgroup v2, using unified hierarchy collector");
            Ok(Arc::new(CgroupV2Collector::new(cgroup_root)))
        }
        CgroupVersion::V1 => {
            tracing::info!("Detected cgroup v1, using legacy hierarchy collector");
            Ok(Arc::new(CgroupV1Collector::new(cgroup_root)))
        }
        CgroupVersion::Unknown => {
            tracing::warn!("Could not detect cgroup version, defaulting to v2");
            Ok(Arc::new(CgroupV2Collector::new(cgroup_root)))
        }
    }
}
