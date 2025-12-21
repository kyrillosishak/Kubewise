//! cgroup v2 metrics collection
//!
//! Reads metrics from the unified cgroup v2 hierarchy:
//! - cpu.stat for CPU usage and throttling
//! - memory.current for current memory usage
//! - memory.stat for detailed memory statistics

use super::MetricsCollector;
use crate::models::{ContainerInfo, ContainerMetrics};
use anyhow::{Context, Result};
use async_trait::async_trait;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use tokio::fs;

/// Collector for cgroup v2 unified hierarchy
pub struct CgroupV2Collector {
    cgroup_root: PathBuf,
    proc_path: PathBuf,
}

impl CgroupV2Collector {
    /// Create a new cgroup v2 collector
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

    /// Check if cgroup v2 is available on this system
    pub async fn is_available(&self) -> bool {
        let cgroup_type_file = self.cgroup_root.join("cgroup.controllers");
        fs::metadata(&cgroup_type_file).await.is_ok()
    }

    /// Parse cpu.stat file contents
    /// Returns (usage_usec, throttled_periods)
    pub fn parse_cpu_stat(content: &str) -> Result<(u64, u64)> {
        let mut usage_usec = 0u64;
        let mut throttled_periods = 0u64;

        for line in content.lines() {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 2 {
                match parts[0] {
                    "usage_usec" => {
                        usage_usec = parts[1].parse().unwrap_or(0);
                    }
                    "nr_throttled" => {
                        throttled_periods = parts[1].parse().unwrap_or(0);
                    }
                    _ => {}
                }
            }
        }

        Ok((usage_usec, throttled_periods))
    }

    /// Parse memory.stat file contents
    /// Returns HashMap of stat name to value
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

    /// Read a single value from a cgroup file
    async fn read_cgroup_value(&self, cgroup_path: &Path, filename: &str) -> Result<u64> {
        let file_path = cgroup_path.join(filename);
        let content = fs::read_to_string(&file_path)
            .await
            .with_context(|| format!("Failed to read {}", file_path.display()))?;
        
        content
            .trim()
            .parse()
            .with_context(|| format!("Failed to parse {} value", filename))
    }

    /// Extract container ID from cgroup path
    /// Handles various container runtime formats:
    /// - Docker: /docker/<container_id>
    /// - containerd: /system.slice/containerd.service/kubepods-.../<container_id>
    /// - CRI-O: /kubepods.slice/kubepods-...-pod<pod_id>.slice/crio-<container_id>.scope
    pub fn extract_container_id(cgroup_path: &str) -> Option<String> {
        // Try to find container ID patterns
        let path_parts: Vec<&str> = cgroup_path.split('/').collect();
        
        for part in path_parts.iter().rev() {
            // CRI-O format: crio-<container_id>.scope or crio-<container_id>
            if let Some(stripped) = part.strip_prefix("crio-") {
                // Handle with or without .scope suffix
                let id = stripped.strip_suffix(".scope").unwrap_or(stripped);
                if id.len() == 64 && id.chars().all(|c| c.is_ascii_hexdigit()) {
                    return Some(id.to_string());
                }
            }
            
            // Docker/containerd format: plain 64-char hex ID
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

    /// Parse /proc/{pid}/cgroup to get cgroup path for a process
    pub async fn get_cgroup_path_for_pid(&self, pid: u32) -> Result<String> {
        let cgroup_file = self.proc_path.join(format!("{}/cgroup", pid));
        let content = fs::read_to_string(&cgroup_file)
            .await
            .with_context(|| format!("Failed to read cgroup for pid {}", pid))?;

        // cgroup v2 format: "0::/path/to/cgroup"
        for line in content.lines() {
            let parts: Vec<&str> = line.splitn(3, ':').collect();
            if parts.len() == 3 && parts[0] == "0" {
                return Ok(parts[2].to_string());
            }
        }

        anyhow::bail!("No cgroup v2 path found for pid {}", pid)
    }

    /// Build full cgroup filesystem path from relative cgroup path
    pub fn build_cgroup_fs_path(&self, cgroup_path: &str) -> PathBuf {
        self.cgroup_root.join(cgroup_path.trim_start_matches('/'))
    }

    /// Collect metrics from a specific cgroup path
    async fn collect_from_path(
        &self,
        cgroup_path: &Path,
        container_id: &str,
        metadata: &ContainerMetadata,
    ) -> Result<ContainerMetrics> {
        let timestamp = chrono::Utc::now().timestamp();

        // Read cpu.stat
        let cpu_stat_content = fs::read_to_string(cgroup_path.join("cpu.stat"))
            .await
            .unwrap_or_default();
        let (cpu_usage_usec, cpu_throttled_periods) = Self::parse_cpu_stat(&cpu_stat_content)?;

        // Convert CPU usage from microseconds to cores (assuming 1 second sample)
        // This is a cumulative value, so actual usage calculation needs delta
        let cpu_usage_cores = cpu_usage_usec as f32 / 1_000_000.0;

        // Read memory.current
        let memory_usage_bytes = self
            .read_cgroup_value(cgroup_path, "memory.current")
            .await
            .unwrap_or(0);

        // Read memory.stat for detailed breakdown
        let memory_stat_content = fs::read_to_string(cgroup_path.join("memory.stat"))
            .await
            .unwrap_or_default();
        let memory_stats = Self::parse_memory_stat(&memory_stat_content);

        // Working set = total - inactive_file (approximation)
        let inactive_file = memory_stats.get("inactive_file").copied().unwrap_or(0);
        let memory_working_set_bytes = memory_usage_bytes.saturating_sub(inactive_file);

        // Cache = file (page cache)
        let memory_cache_bytes = memory_stats.get("file").copied().unwrap_or(0);

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
    pub node_name: String,
}

#[async_trait]
impl MetricsCollector for CgroupV2Collector {
    async fn collect(&self, container_id: &str) -> Result<ContainerMetrics> {
        // Find the cgroup path for this container
        // In production, this would be cached from container discovery
        let cgroup_path = self.cgroup_root.join(container_id);
        
        if !cgroup_path.exists() {
            anyhow::bail!("Cgroup path not found for container {}", container_id);
        }

        let metadata = ContainerMetadata::default();
        self.collect_from_path(&cgroup_path, container_id, &metadata)
            .await
    }

    async fn list_containers(&self) -> Result<Vec<ContainerInfo>> {
        // This will be fully implemented in Task 2.3 (container discovery)
        // For now, provide a basic implementation that scans kubepods
        let mut containers = Vec::new();
        
        // Look for kubepods hierarchy (common in Kubernetes)
        let kubepods_path = self.cgroup_root.join("kubepods.slice");
        if kubepods_path.exists() {
            if let Ok(entries) = Self::scan_cgroup_dir(&kubepods_path).await {
                containers.extend(entries);
            }
        }

        // Also check system.slice for containerd
        let system_slice = self.cgroup_root.join("system.slice");
        if system_slice.exists() {
            if let Ok(entries) = Self::scan_cgroup_dir(&system_slice).await {
                containers.extend(entries);
            }
        }

        Ok(containers)
    }
}

impl CgroupV2Collector {
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
                    if entry_path.join("cpu.stat").exists() {
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_cpu_stat() {
        let content = r#"usage_usec 123456789
user_usec 100000000
system_usec 23456789
nr_periods 1000
nr_throttled 50
throttled_usec 5000000"#;

        let (usage, throttled) = CgroupV2Collector::parse_cpu_stat(content).unwrap();
        assert_eq!(usage, 123456789);
        assert_eq!(throttled, 50);
    }

    #[test]
    fn test_parse_memory_stat() {
        let content = r#"anon 104857600
file 52428800
kernel 10485760
inactive_file 26214400"#;

        let stats = CgroupV2Collector::parse_memory_stat(content);
        assert_eq!(stats.get("anon"), Some(&104857600));
        assert_eq!(stats.get("file"), Some(&52428800));
        assert_eq!(stats.get("inactive_file"), Some(&26214400));
    }

    #[test]
    fn test_extract_container_id_docker() {
        let path = "/docker/abc123def456789012345678901234567890123456789012345678901234abcd";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(
            id,
            Some("abc123def456789012345678901234567890123456789012345678901234abcd".to_string())
        );
    }

    #[test]
    fn test_extract_container_id_crio() {
        let path = "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice/crio-abc123def456789012345678901234567890123456789012345678901234abcd.scope";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(
            id,
            Some("abc123def456789012345678901234567890123456789012345678901234abcd".to_string())
        );
    }

    #[test]
    fn test_extract_container_id_fallback() {
        let path = "/some/unknown/format/container-name";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(id, Some("container-name".to_string()));
    }
}
