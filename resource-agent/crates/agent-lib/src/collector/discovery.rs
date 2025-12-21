//! Container discovery and lifecycle tracking
//!
//! Watches for container start/stop events via filesystem notifications
//! on cgroup directories and maintains an active container registry.

use super::MetricsCollector;
use crate::models::ContainerInfo;
use anyhow::{Context, Result};
use dashmap::DashMap;
use notify::{Event, EventKind, RecommendedWatcher, RecursiveMode, Watcher};
use std::path::{Path, PathBuf};
use tokio::sync::mpsc;
use tracing::{debug, info, warn};

/// Container lifecycle events
#[derive(Debug, Clone)]
pub enum ContainerEvent {
    /// A new container was discovered
    Started(ContainerInfo),
    /// A container was removed
    Stopped(String), // container_id
}

/// Registry of active containers on the node
pub struct ContainerRegistry {
    /// Map of container_id -> ContainerInfo
    containers: DashMap<String, ContainerInfo>,
    /// Node name for this agent
    node_name: String,
}

impl ContainerRegistry {
    /// Create a new container registry
    pub fn new(node_name: impl Into<String>) -> Self {
        Self {
            containers: DashMap::new(),
            node_name: node_name.into(),
        }
    }

    /// Register a new container
    pub fn register(&self, mut info: ContainerInfo) {
        info.node_name = self.node_name.clone();
        debug!(container_id = %info.container_id, "Registering container");
        self.containers.insert(info.container_id.clone(), info);
    }

    /// Unregister a container
    pub fn unregister(&self, container_id: &str) -> Option<ContainerInfo> {
        debug!(container_id = %container_id, "Unregistering container");
        self.containers.remove(container_id).map(|(_, v)| v)
    }

    /// Get container info by ID
    pub fn get(&self, container_id: &str) -> Option<ContainerInfo> {
        self.containers.get(container_id).map(|r| r.clone())
    }

    /// List all registered containers
    pub fn list(&self) -> Vec<ContainerInfo> {
        self.containers.iter().map(|r| r.value().clone()).collect()
    }

    /// Get the number of registered containers
    pub fn len(&self) -> usize {
        self.containers.len()
    }

    /// Check if registry is empty
    pub fn is_empty(&self) -> bool {
        self.containers.is_empty()
    }

    /// Update container metadata (e.g., from Kubernetes API)
    pub fn update_metadata(
        &self,
        container_id: &str,
        pod_name: Option<String>,
        namespace: Option<String>,
        deployment: Option<String>,
    ) {
        if let Some(mut entry) = self.containers.get_mut(container_id) {
            if let Some(name) = pod_name {
                entry.pod_name = name;
            }
            if let Some(ns) = namespace {
                entry.namespace = ns;
            }
            if deployment.is_some() {
                entry.deployment = deployment;
            }
        }
    }
}

/// Watches cgroup directories for container lifecycle events
pub struct ContainerWatcher {
    /// Root path for cgroup filesystem
    cgroup_root: PathBuf,
    /// Whether using cgroup v2
    is_v2: bool,
    /// Event sender
    event_tx: mpsc::Sender<ContainerEvent>,
}

impl ContainerWatcher {
    /// Create a new container watcher
    pub fn new(
        cgroup_root: impl Into<PathBuf>,
        is_v2: bool,
        event_tx: mpsc::Sender<ContainerEvent>,
    ) -> Self {
        Self {
            cgroup_root: cgroup_root.into(),
            is_v2,
            event_tx,
        }
    }

    /// Start watching for container events
    /// Returns a handle that stops watching when dropped
    pub async fn start(self) -> Result<WatcherHandle> {
        let (tx, rx) = std::sync::mpsc::channel();

        let mut watcher = RecommendedWatcher::new(
            move |res: Result<Event, notify::Error>| {
                if let Ok(event) = res {
                    let _ = tx.send(event);
                }
            },
            notify::Config::default(),
        )
        .context("Failed to create filesystem watcher")?;

        // Determine which paths to watch based on cgroup version
        let watch_paths = self.get_watch_paths();

        for path in &watch_paths {
            if path.exists() {
                watcher
                    .watch(path, RecursiveMode::Recursive)
                    .with_context(|| format!("Failed to watch {}", path.display()))?;
                info!(path = %path.display(), "Watching cgroup directory");
            }
        }

        let event_tx = self.event_tx.clone();
        let cgroup_root = self.cgroup_root.clone();
        let is_v2 = self.is_v2;

        // Spawn task to process filesystem events
        let handle = tokio::spawn(async move {
            loop {
                match rx.recv() {
                    Ok(event) => {
                        if let Err(e) =
                            Self::process_event(&event, &cgroup_root, is_v2, &event_tx).await
                        {
                            warn!(error = %e, "Error processing filesystem event");
                        }
                    }
                    Err(_) => {
                        debug!("Watcher channel closed");
                        break;
                    }
                }
            }
        });

        Ok(WatcherHandle {
            _watcher: watcher,
            _task: handle,
        })
    }

    /// Get paths to watch based on cgroup version
    fn get_watch_paths(&self) -> Vec<PathBuf> {
        let mut paths = Vec::new();

        if self.is_v2 {
            // cgroup v2: watch kubepods.slice and system.slice
            paths.push(self.cgroup_root.join("kubepods.slice"));
            paths.push(self.cgroup_root.join("system.slice"));
        } else {
            // cgroup v1: watch memory controller hierarchy
            let memory_root = self.cgroup_root.join("memory");
            paths.push(memory_root.join("kubepods"));
            paths.push(memory_root.join("docker"));
            paths.push(memory_root.join("system.slice"));
        }

        paths
    }

    /// Process a filesystem event
    async fn process_event(
        event: &Event,
        cgroup_root: &Path,
        is_v2: bool,
        event_tx: &mpsc::Sender<ContainerEvent>,
    ) -> Result<()> {
        for path in &event.paths {
            match event.kind {
                EventKind::Create(_) => {
                    // Check if this is a container cgroup directory
                    if let Some(info) = Self::try_parse_container_path(path, cgroup_root, is_v2) {
                        debug!(container_id = %info.container_id, path = %path.display(), "Container started");
                        let _ = event_tx.send(ContainerEvent::Started(info)).await;
                    }
                }
                EventKind::Remove(_) => {
                    // Check if this was a container cgroup directory
                    if let Some(container_id) = Self::try_extract_container_id(path, is_v2) {
                        debug!(container_id = %container_id, path = %path.display(), "Container stopped");
                        let _ = event_tx.send(ContainerEvent::Stopped(container_id)).await;
                    }
                }
                _ => {}
            }
        }

        Ok(())
    }

    /// Try to parse a path as a container cgroup directory
    fn try_parse_container_path(
        path: &Path,
        _cgroup_root: &Path,
        is_v2: bool,
    ) -> Option<ContainerInfo> {
        let path_str = path.to_string_lossy();

        // Extract container ID
        let container_id = if is_v2 {
            super::cgroup_v2::CgroupV2Collector::extract_container_id(&path_str)?
        } else {
            super::cgroup_v1::CgroupV1Collector::extract_container_id(&path_str)?
        };

        // Only consider 64-char hex IDs as valid container IDs
        if container_id.len() != 64 || !container_id.chars().all(|c| c.is_ascii_hexdigit()) {
            return None;
        }

        // Verify this looks like a container cgroup (has expected files)
        let has_cgroup_files = if is_v2 {
            path.join("cpu.stat").exists() || path.join("memory.current").exists()
        } else {
            path.join("memory.usage_in_bytes").exists() || path.join("cpuacct.usage").exists()
        };

        if !has_cgroup_files {
            return None;
        }

        Some(ContainerInfo {
            container_id,
            pod_name: String::new(),
            namespace: String::new(),
            deployment: None,
            node_name: String::new(),
            cgroup_path: path_str.to_string(),
        })
    }

    /// Try to extract container ID from a path
    fn try_extract_container_id(path: &Path, is_v2: bool) -> Option<String> {
        let path_str = path.to_string_lossy();

        let container_id = if is_v2 {
            super::cgroup_v2::CgroupV2Collector::extract_container_id(&path_str)?
        } else {
            super::cgroup_v1::CgroupV1Collector::extract_container_id(&path_str)?
        };

        // Only consider 64-char hex IDs as valid container IDs
        if container_id.len() == 64 && container_id.chars().all(|c| c.is_ascii_hexdigit()) {
            Some(container_id)
        } else {
            None
        }
    }
}

/// Handle to a running watcher
/// Stops watching when dropped
pub struct WatcherHandle {
    _watcher: RecommendedWatcher,
    _task: tokio::task::JoinHandle<()>,
}

/// Kubernetes metadata fetcher
/// Queries the Kubernetes API for pod/deployment labels
pub struct K8sMetadataFetcher {
    /// Kubernetes API endpoint (typically from in-cluster config)
    #[allow(dead_code)]
    api_endpoint: String,
    /// Service account token path
    token_path: PathBuf,
}

impl K8sMetadataFetcher {
    /// Create a new metadata fetcher with in-cluster configuration
    pub fn in_cluster() -> Self {
        Self {
            api_endpoint: std::env::var("KUBERNETES_SERVICE_HOST")
                .map(|host| {
                    let port =
                        std::env::var("KUBERNETES_SERVICE_PORT").unwrap_or_else(|_| "443".into());
                    format!("https://{}:{}", host, port)
                })
                .unwrap_or_else(|_| "https://kubernetes.default.svc".into()),
            token_path: PathBuf::from("/var/run/secrets/kubernetes.io/serviceaccount/token"),
        }
    }

    /// Create with custom endpoint (for testing)
    pub fn with_endpoint(api_endpoint: impl Into<String>, token_path: impl Into<PathBuf>) -> Self {
        Self {
            api_endpoint: api_endpoint.into(),
            token_path: token_path.into(),
        }
    }

    /// Fetch metadata for a container
    /// Returns (pod_name, namespace, deployment)
    pub async fn fetch_metadata(
        &self,
        container_id: &str,
    ) -> Result<(String, String, Option<String>)> {
        // In a full implementation, this would:
        // 1. Read the service account token
        // 2. Query the Kubernetes API for pods on this node
        // 3. Match container ID to pod
        // 4. Extract deployment from owner references

        // For now, return placeholder - full implementation requires HTTP client
        warn!(
            container_id = %container_id,
            "K8s metadata fetch not fully implemented"
        );

        Ok((String::new(), String::new(), None))
    }

    /// Check if running in a Kubernetes cluster
    pub fn is_in_cluster(&self) -> bool {
        self.token_path.exists()
    }
}

/// Perform initial container discovery by scanning cgroup filesystem
pub async fn discover_existing_containers(
    cgroup_root: &Path,
    is_v2: bool,
) -> Result<Vec<ContainerInfo>> {
    let containers = if is_v2 {
        // Scan cgroup v2 hierarchy
        let collector = super::cgroup_v2::CgroupV2Collector::new(cgroup_root);
        collector.list_containers().await?
    } else {
        // Scan cgroup v1 hierarchy
        let collector = super::cgroup_v1::CgroupV1Collector::new(cgroup_root);
        collector.list_containers().await?
    };

    info!(count = containers.len(), "Discovered existing containers");
    Ok(containers)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_container_registry() {
        let registry = ContainerRegistry::new("test-node");

        let info = ContainerInfo {
            container_id: "abc123".to_string(),
            pod_name: "test-pod".to_string(),
            namespace: "default".to_string(),
            deployment: Some("test-deploy".to_string()),
            node_name: String::new(),
            cgroup_path: "/test/path".to_string(),
        };

        registry.register(info.clone());
        assert_eq!(registry.len(), 1);

        let retrieved = registry.get("abc123").unwrap();
        assert_eq!(retrieved.pod_name, "test-pod");
        assert_eq!(retrieved.node_name, "test-node");

        registry.unregister("abc123");
        assert!(registry.is_empty());
    }

    #[test]
    fn test_container_registry_update_metadata() {
        let registry = ContainerRegistry::new("test-node");

        let info = ContainerInfo {
            container_id: "abc123".to_string(),
            pod_name: String::new(),
            namespace: String::new(),
            deployment: None,
            node_name: String::new(),
            cgroup_path: "/test/path".to_string(),
        };

        registry.register(info);

        registry.update_metadata(
            "abc123",
            Some("updated-pod".to_string()),
            Some("kube-system".to_string()),
            Some("my-deployment".to_string()),
        );

        let retrieved = registry.get("abc123").unwrap();
        assert_eq!(retrieved.pod_name, "updated-pod");
        assert_eq!(retrieved.namespace, "kube-system");
        assert_eq!(retrieved.deployment, Some("my-deployment".to_string()));
    }

    #[test]
    fn test_k8s_metadata_fetcher_in_cluster_detection() {
        let fetcher = K8sMetadataFetcher::in_cluster();
        // In test environment, we're not in a cluster
        assert!(!fetcher.is_in_cluster());
    }
}
