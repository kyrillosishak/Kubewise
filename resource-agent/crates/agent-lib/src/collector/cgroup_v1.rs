//! cgroup v1 metrics collection (fallback)
//!
//! Reads metrics from the legacy cgroup v1 hierarchy:
//! - cpuacct controller for CPU usage
//! - cpu controller for throttling stats
//! - memory controller for memory usage

use super::MetricsCollector;
use crate::models::{ContainerInfo, ContainerMetrics};
use anyhow::{Context, Result};
use async_trait::async_trait;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use tokio::fs;

/// Collector for legacy cgroup v1 hierarchy
pub struct CgroupV1Collector {
    /// Root path for cgroup v1 controllers (typically /sys/fs/cgroup)
    cgroup_root: PathBuf,
    /// Path to /proc filesystem
    proc_path: PathBuf,
}

impl CgroupV1Collector {
    /// Create a new cgroup v1 collector
    pub fn new(cgroup_root: impl Into<PathBuf>) -> Self {
        Self {
            cgroup_root: cgroup_root.into(),
            proc_path: PathBuf::from("/proc"),
        }
    }

    /// Create collector with custom proc path (for testing)
    pub fn with_proc_path(cgroup_root: impl Into<PathBuf>, proc_path: impl Into<PathBuf>) -> Self {
        Self {
            cgroup_root: cgroup_root.into(),
            proc_path: proc_path.into(),
        }
    }

    /// Check if cgroup v1 is available on this system
    pub async fn is_available(&self) -> bool {
        // cgroup v1 has separate controller directories
        let cpuacct_path = self.cgroup_root.join("cpuacct");
        let memory_path = self.cgroup_root.join("memory");

        fs::metadata(&cpuacct_path).await.is_ok() && fs::metadata(&memory_path).await.is_ok()
    }

    /// Read CPU usage from cpuacct.usage (nanoseconds)
    async fn read_cpu_usage(&self, cgroup_path: &Path) -> Result<u64> {
        let usage_file = cgroup_path.join("cpuacct.usage");
        let content = fs::read_to_string(&usage_file)
            .await
            .with_context(|| format!("Failed to read {}", usage_file.display()))?;

        content
            .trim()
            .parse()
            .with_context(|| "Failed to parse cpuacct.usage")
    }

    /// Read CPU throttling stats from cpu.stat
    pub fn parse_cpu_stat(content: &str) -> (u64, u64) {
        let mut nr_periods = 0u64;
        let mut nr_throttled = 0u64;

        for line in content.lines() {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 2 {
                match parts[0] {
                    "nr_periods" => nr_periods = parts[1].parse().unwrap_or(0),
                    "nr_throttled" => nr_throttled = parts[1].parse().unwrap_or(0),
                    _ => {}
                }
            }
        }

        (nr_periods, nr_throttled)
    }

    /// Read memory usage from memory.usage_in_bytes
    async fn read_memory_usage(&self, cgroup_path: &Path) -> Result<u64> {
        let usage_file = cgroup_path.join("memory.usage_in_bytes");
        let content = fs::read_to_string(&usage_file)
            .await
            .with_context(|| format!("Failed to read {}", usage_file.display()))?;

        content
            .trim()
            .parse()
            .with_context(|| "Failed to parse memory.usage_in_bytes")
    }

    /// Parse memory.stat file contents
    pub fn parse_memory_stat(content: &str) -> HashMap<String, u64> {
        let mut stats = HashMap::new();

        for line in content.lines() {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 2 {
                if let Ok(value) = parts[1].parse::<u64>() {
                    stats.insert(parts[0].to_string(), value);
                }
            }
        }

        stats
    }

    /// Extract container ID from cgroup path
    /// Handles various container runtime formats for cgroup v1
    pub fn extract_container_id(cgroup_path: &str) -> Option<String> {
        let path_parts: Vec<&str> = cgroup_path.split('/').collect();

        for part in path_parts.iter().rev() {
            // containerd format with .scope suffix: cri-containerd-<container_id>.scope
            if let Some(stripped) = part.strip_suffix(".scope") {
                if let Some(id) = stripped.strip_prefix("cri-containerd-") {
                    if id.len() == 64 && id.chars().all(|c| c.is_ascii_hexdigit()) {
                        return Some(id.to_string());
                    }
                }
            }

            // CRI-O format: crio-<container_id>
            if let Some(stripped) = part.strip_prefix("crio-") {
                // Handle with or without .scope suffix
                let id = stripped.strip_suffix(".scope").unwrap_or(stripped);
                if id.len() == 64 && id.chars().all(|c| c.is_ascii_hexdigit()) {
                    return Some(id.to_string());
                }
            }

            // Docker format: docker/<container_id> - plain 64-char hex ID
            if part.len() == 64 && part.chars().all(|c| c.is_ascii_hexdigit()) {
                return Some(part.to_string());
            }
        }

        // Fallback: use the last non-empty path component
        path_parts
            .iter()
            .rev()
            .find(|p| !p.is_empty())
            .map(|s| s.to_string())
    }

    /// Parse /proc/{pid}/cgroup to get cgroup paths for a process (v1 format)
    /// Returns a map of controller -> path
    pub async fn get_cgroup_paths_for_pid(&self, pid: u32) -> Result<HashMap<String, String>> {
        let cgroup_file = self.proc_path.join(format!("{}/cgroup", pid));
        let content = fs::read_to_string(&cgroup_file)
            .await
            .with_context(|| format!("Failed to read cgroup for pid {}", pid))?;

        let mut paths = HashMap::new();

        // cgroup v1 format: "hierarchy-ID:controller-list:cgroup-path"
        // Example: "4:memory:/docker/abc123..."
        for line in content.lines() {
            let parts: Vec<&str> = line.splitn(3, ':').collect();
            if parts.len() == 3 {
                let controllers = parts[1];
                let path = parts[2];

                // Skip cgroup v2 unified hierarchy (empty controller list)
                if controllers.is_empty() {
                    continue;
                }

                // Handle comma-separated controllers (e.g., "cpu,cpuacct")
                for controller in controllers.split(',') {
                    paths.insert(controller.to_string(), path.to_string());
                }
            }
        }

        Ok(paths)
    }

    /// Build full cgroup filesystem path for a specific controller
    pub fn build_controller_path(&self, controller: &str, cgroup_path: &str) -> PathBuf {
        self.cgroup_root
            .join(controller)
            .join(cgroup_path.trim_start_matches('/'))
    }

    /// Collect metrics from cgroup v1 paths
    async fn collect_from_paths(
        &self,
        cpuacct_path: &Path,
        cpu_path: &Path,
        memory_path: &Path,
        container_id: &str,
        metadata: &ContainerMetadata,
    ) -> Result<ContainerMetrics> {
        let timestamp = chrono::Utc::now().timestamp();

        // Read CPU usage (nanoseconds -> cores)
        let cpu_usage_ns = self.read_cpu_usage(cpuacct_path).await.unwrap_or(0);
        let cpu_usage_cores = cpu_usage_ns as f32 / 1_000_000_000.0;

        // Read CPU throttling stats
        let cpu_stat_content = fs::read_to_string(cpu_path.join("cpu.stat"))
            .await
            .unwrap_or_default();
        let (_, cpu_throttled_periods) = Self::parse_cpu_stat(&cpu_stat_content);

        // Read memory usage
        let memory_usage_bytes = self.read_memory_usage(memory_path).await.unwrap_or(0);

        // Read memory.stat for detailed breakdown
        let memory_stat_content = fs::read_to_string(memory_path.join("memory.stat"))
            .await
            .unwrap_or_default();
        let memory_stats = Self::parse_memory_stat(&memory_stat_content);

        // Working set = total - inactive_file (approximation)
        // In cgroup v1, it's "total_inactive_file"
        let inactive_file = memory_stats
            .get("total_inactive_file")
            .or_else(|| memory_stats.get("inactive_file"))
            .copied()
            .unwrap_or(0);
        let memory_working_set_bytes = memory_usage_bytes.saturating_sub(inactive_file);

        // Cache = total_cache or cache
        let memory_cache_bytes = memory_stats
            .get("total_cache")
            .or_else(|| memory_stats.get("cache"))
            .copied()
            .unwrap_or(0);

        Ok(ContainerMetrics {
            container_id: container_id.to_string(),
            pod_name: metadata.pod_name.clone(),
            namespace: metadata.namespace.clone(),
            deployment: metadata.deployment.clone(),
            timestamp,
            cpu_usage_cores,
            cpu_throttled_periods,
            memory_usage_bytes,
            memory_working_set_bytes,
            memory_cache_bytes,
            network_rx_bytes: 0, // Network metrics require different source
            network_tx_bytes: 0,
        })
    }
}

/// Container metadata extracted from Kubernetes
#[derive(Debug, Clone, Default)]
pub struct ContainerMetadata {
    pub pod_name: String,
    pub namespace: String,
    pub deployment: Option<String>,
    #[allow(dead_code)]
    pub node_name: String,
}

#[async_trait]
impl MetricsCollector for CgroupV1Collector {
    async fn collect(&self, container_id: &str) -> Result<ContainerMetrics> {
        // Build paths for each controller
        let cpuacct_path = self.cgroup_root.join("cpuacct").join(container_id);
        let cpu_path = self.cgroup_root.join("cpu").join(container_id);
        let memory_path = self.cgroup_root.join("memory").join(container_id);

        // Verify at least one path exists
        if !cpuacct_path.exists() && !memory_path.exists() {
            anyhow::bail!("Cgroup paths not found for container {}", container_id);
        }

        let metadata = ContainerMetadata::default();
        self.collect_from_paths(
            &cpuacct_path,
            &cpu_path,
            &memory_path,
            container_id,
            &metadata,
        )
        .await
    }

    async fn list_containers(&self) -> Result<Vec<ContainerInfo>> {
        // This will be fully implemented in Task 2.3 (container discovery)
        // For now, provide a basic implementation that scans kubepods
        let mut containers = Vec::new();

        // Scan memory controller hierarchy (most reliable for container detection)
        let memory_root = self.cgroup_root.join("memory");

        // Look for kubepods hierarchy
        let kubepods_path = memory_root.join("kubepods");
        if kubepods_path.exists() {
            if let Ok(entries) = Self::scan_cgroup_dir(&kubepods_path).await {
                containers.extend(entries);
            }
        }

        // Also check docker hierarchy
        let docker_path = memory_root.join("docker");
        if docker_path.exists() {
            if let Ok(entries) = Self::scan_cgroup_dir(&docker_path).await {
                containers.extend(entries);
            }
        }

        Ok(containers)
    }
}

impl CgroupV1Collector {
    /// Recursively scan cgroup directory for containers
    async fn scan_cgroup_dir(path: &Path) -> Result<Vec<ContainerInfo>> {
        let mut containers = Vec::new();
        let mut entries = fs::read_dir(path).await?;

        while let Some(entry) = entries.next_entry().await? {
            let entry_path = entry.path();

            if entry_path.is_dir() {
                let name = entry.file_name().to_string_lossy().to_string();

                // Check if this looks like a container cgroup
                if let Some(container_id) = Self::extract_container_id(&name) {
                    // Verify it has cgroup files
                    if entry_path.join("memory.usage_in_bytes").exists() {
                        containers.push(ContainerInfo {
                            container_id: container_id.clone(),
                            pod_name: String::new(), // Will be populated by discovery
                            namespace: String::new(),
                            deployment: None,
                            node_name: String::new(),
                            cgroup_path: entry_path.to_string_lossy().to_string(),
                        });
                    }
                }

                // Recurse into subdirectories
                if let Ok(sub_containers) = Box::pin(Self::scan_cgroup_dir(&entry_path)).await {
                    containers.extend(sub_containers);
                }
            }
        }

        Ok(containers)
    }
}

/// Detect which cgroup version is available on the system
pub async fn detect_cgroup_version(cgroup_root: &Path) -> CgroupVersion {
    // Check for cgroup v2 unified hierarchy
    let v2_controllers = cgroup_root.join("cgroup.controllers");
    if fs::metadata(&v2_controllers).await.is_ok() {
        return CgroupVersion::V2;
    }

    // Check for cgroup v1 controllers
    let v1_memory = cgroup_root.join("memory");
    let v1_cpuacct = cgroup_root.join("cpuacct");
    if fs::metadata(&v1_memory).await.is_ok() && fs::metadata(&v1_cpuacct).await.is_ok() {
        return CgroupVersion::V1;
    }

    CgroupVersion::Unknown
}

/// Cgroup version detected on the system
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CgroupVersion {
    V1,
    V2,
    Unknown,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_cpu_stat() {
        let content = r#"nr_periods 1000
nr_throttled 50
throttled_time 5000000"#;

        let (periods, throttled) = CgroupV1Collector::parse_cpu_stat(content);
        assert_eq!(periods, 1000);
        assert_eq!(throttled, 50);
    }

    #[test]
    fn test_parse_memory_stat() {
        let content = r#"cache 104857600
rss 52428800
total_cache 104857600
total_rss 52428800
total_inactive_file 26214400"#;

        let stats = CgroupV1Collector::parse_memory_stat(content);
        assert_eq!(stats.get("cache"), Some(&104857600));
        assert_eq!(stats.get("total_cache"), Some(&104857600));
        assert_eq!(stats.get("total_inactive_file"), Some(&26214400));
    }

    #[test]
    fn test_extract_container_id_docker() {
        let path = "/docker/abc123def456789012345678901234567890123456789012345678901234abcd";
        let id = CgroupV1Collector::extract_container_id(path);
        assert_eq!(
            id,
            Some("abc123def456789012345678901234567890123456789012345678901234abcd".to_string())
        );
    }

    #[test]
    fn test_extract_container_id_crio() {
        let path = "/kubepods/besteffort/pod123/crio-abc123def456789012345678901234567890123456789012345678901234abcd";
        let id = CgroupV1Collector::extract_container_id(path);
        assert_eq!(
            id,
            Some("abc123def456789012345678901234567890123456789012345678901234abcd".to_string())
        );
    }

    #[test]
    fn test_extract_container_id_containerd() {
        let path = "/kubepods/pod123/cri-containerd-abc123def456789012345678901234567890123456789012345678901234abcd.scope";
        let id = CgroupV1Collector::extract_container_id(path);
        assert_eq!(
            id,
            Some("abc123def456789012345678901234567890123456789012345678901234abcd".to_string())
        );
    }

    #[tokio::test]
    async fn test_detect_cgroup_version_unknown() {
        // Test with non-existent path
        let version = detect_cgroup_version(Path::new("/nonexistent/path")).await;
        assert_eq!(version, CgroupVersion::Unknown);
    }
}
