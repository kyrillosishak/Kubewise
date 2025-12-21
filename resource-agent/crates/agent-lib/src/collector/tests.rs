//! Integration tests for metrics collection
//!
//! These tests use a mock cgroup filesystem to test metric parsing
//! and collection without requiring actual container runtime.

#[cfg(test)]
mod mock_cgroup_tests {
    use crate::collector::{CgroupV1Collector, CgroupV2Collector, MetricsCollector};
    use std::path::PathBuf;
    use tempfile::TempDir;
    use tokio::fs;

    /// Helper to create a mock cgroup v2 filesystem
    async fn create_mock_cgroup_v2(temp_dir: &TempDir, container_id: &str) -> PathBuf {
        let cgroup_root = temp_dir.path().to_path_buf();

        // Create cgroup.controllers to indicate v2
        fs::write(cgroup_root.join("cgroup.controllers"), "cpu memory io\n")
            .await
            .unwrap();

        // Create container cgroup directory
        let container_path = cgroup_root.join(container_id);
        fs::create_dir_all(&container_path).await.unwrap();

        // Create cpu.stat
        let cpu_stat = r#"usage_usec 5000000
user_usec 3000000
system_usec 2000000
nr_periods 100
nr_throttled 5
throttled_usec 50000
"#;
        fs::write(container_path.join("cpu.stat"), cpu_stat)
            .await
            .unwrap();

        // Create memory.current
        fs::write(container_path.join("memory.current"), "104857600\n")
            .await
            .unwrap();

        // Create memory.stat
        let memory_stat = r#"anon 52428800
file 26214400
kernel 10485760
inactive_file 13107200
active_file 13107200
"#;
        fs::write(container_path.join("memory.stat"), memory_stat)
            .await
            .unwrap();

        cgroup_root
    }

    /// Helper to create a mock cgroup v1 filesystem
    async fn create_mock_cgroup_v1(temp_dir: &TempDir, container_id: &str) -> PathBuf {
        let cgroup_root = temp_dir.path().to_path_buf();

        // Create controller directories
        let cpuacct_path = cgroup_root.join("cpuacct").join(container_id);
        let cpu_path = cgroup_root.join("cpu").join(container_id);
        let memory_path = cgroup_root.join("memory").join(container_id);

        fs::create_dir_all(&cpuacct_path).await.unwrap();
        fs::create_dir_all(&cpu_path).await.unwrap();
        fs::create_dir_all(&memory_path).await.unwrap();

        // Create cpuacct.usage (nanoseconds)
        fs::write(cpuacct_path.join("cpuacct.usage"), "5000000000\n")
            .await
            .unwrap();

        // Create cpu.stat
        let cpu_stat = r#"nr_periods 100
nr_throttled 5
throttled_time 50000000
"#;
        fs::write(cpu_path.join("cpu.stat"), cpu_stat)
            .await
            .unwrap();

        // Create memory.usage_in_bytes
        fs::write(memory_path.join("memory.usage_in_bytes"), "104857600\n")
            .await
            .unwrap();

        // Create memory.stat
        let memory_stat = r#"cache 26214400
rss 52428800
total_cache 26214400
total_rss 52428800
total_inactive_file 13107200
"#;
        fs::write(memory_path.join("memory.stat"), memory_stat)
            .await
            .unwrap();

        cgroup_root
    }

    #[tokio::test]
    async fn test_cgroup_v2_collect_metrics() {
        let temp_dir = TempDir::new().unwrap();
        let container_id = "test_container_abc123";
        let cgroup_root = create_mock_cgroup_v2(&temp_dir, container_id).await;

        let collector = CgroupV2Collector::new(&cgroup_root);
        let metrics = collector.collect(container_id).await.unwrap();

        assert_eq!(metrics.container_id, container_id);
        // CPU usage: 5000000 usec = 5 seconds worth of CPU time
        assert!(metrics.cpu_usage_cores > 0.0);
        assert_eq!(metrics.cpu_throttled_periods, 5);
        assert_eq!(metrics.memory_usage_bytes, 104857600);
        // Working set = total - inactive_file = 104857600 - 13107200
        assert_eq!(metrics.memory_working_set_bytes, 91750400);
        // Cache = file
        assert_eq!(metrics.memory_cache_bytes, 26214400);
    }

    #[tokio::test]
    async fn test_cgroup_v1_collect_metrics() {
        let temp_dir = TempDir::new().unwrap();
        let container_id = "test_container_abc123";
        let cgroup_root = create_mock_cgroup_v1(&temp_dir, container_id).await;

        let collector = CgroupV1Collector::new(&cgroup_root);
        let metrics = collector.collect(container_id).await.unwrap();

        assert_eq!(metrics.container_id, container_id);
        // CPU usage: 5000000000 ns = 5 seconds worth of CPU time
        assert!(metrics.cpu_usage_cores > 0.0);
        assert_eq!(metrics.cpu_throttled_periods, 5);
        assert_eq!(metrics.memory_usage_bytes, 104857600);
        // Working set = total - total_inactive_file = 104857600 - 13107200
        assert_eq!(metrics.memory_working_set_bytes, 91750400);
        // Cache = total_cache
        assert_eq!(metrics.memory_cache_bytes, 26214400);
    }

    #[tokio::test]
    async fn test_cgroup_v2_missing_container() {
        let temp_dir = TempDir::new().unwrap();
        let cgroup_root = temp_dir.path().to_path_buf();

        // Create cgroup.controllers but no container
        fs::write(cgroup_root.join("cgroup.controllers"), "cpu memory\n")
            .await
            .unwrap();

        let collector = CgroupV2Collector::new(&cgroup_root);
        let result = collector.collect("nonexistent").await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_cgroup_v1_missing_container() {
        let temp_dir = TempDir::new().unwrap();
        let cgroup_root = temp_dir.path().to_path_buf();

        // Create controller directories but no container
        fs::create_dir_all(cgroup_root.join("cpuacct"))
            .await
            .unwrap();
        fs::create_dir_all(cgroup_root.join("memory"))
            .await
            .unwrap();

        let collector = CgroupV1Collector::new(&cgroup_root);
        let result = collector.collect("nonexistent").await;

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_cgroup_v2_partial_stats() {
        let temp_dir = TempDir::new().unwrap();
        let container_id = "partial_container";
        let cgroup_root = temp_dir.path().to_path_buf();

        // Create container with only memory.current (no cpu.stat)
        let container_path = cgroup_root.join(container_id);
        fs::create_dir_all(&container_path).await.unwrap();
        fs::write(container_path.join("memory.current"), "50000000\n")
            .await
            .unwrap();

        let collector = CgroupV2Collector::new(&cgroup_root);
        let metrics = collector.collect(container_id).await.unwrap();

        // Should handle missing cpu.stat gracefully
        assert_eq!(metrics.memory_usage_bytes, 50000000);
        assert_eq!(metrics.cpu_usage_cores, 0.0);
        assert_eq!(metrics.cpu_throttled_periods, 0);
    }
}

#[cfg(test)]
mod parsing_tests {
    use crate::collector::{CgroupV1Collector, CgroupV2Collector};

    #[test]
    fn test_parse_cpu_stat_v2_complete() {
        let content = r#"usage_usec 123456789
user_usec 100000000
system_usec 23456789
nr_periods 1000
nr_throttled 50
throttled_usec 5000000
nr_bursts 10
burst_usec 100000
"#;

        let (usage, throttled) = CgroupV2Collector::parse_cpu_stat(content).unwrap();
        assert_eq!(usage, 123456789);
        assert_eq!(throttled, 50);
    }

    #[test]
    fn test_parse_cpu_stat_v2_minimal() {
        let content = "usage_usec 1000\n";

        let (usage, throttled) = CgroupV2Collector::parse_cpu_stat(content).unwrap();
        assert_eq!(usage, 1000);
        assert_eq!(throttled, 0);
    }

    #[test]
    fn test_parse_cpu_stat_v2_empty() {
        let content = "";

        let (usage, throttled) = CgroupV2Collector::parse_cpu_stat(content).unwrap();
        assert_eq!(usage, 0);
        assert_eq!(throttled, 0);
    }

    #[test]
    fn test_parse_memory_stat_v2() {
        let content = r#"anon 104857600
file 52428800
kernel 10485760
kernel_stack 1048576
pagetables 2097152
sec_pagetables 0
percpu 524288
sock 0
vmalloc 0
shmem 0
zswap 0
zswapped 0
file_mapped 26214400
file_dirty 0
file_writeback 0
swapcached 0
anon_thp 0
file_thp 0
shmem_thp 0
inactive_anon 0
active_anon 104857600
inactive_file 26214400
active_file 26214400
unevictable 0
slab_reclaimable 5242880
slab_unreclaimable 2621440
"#;

        let stats = CgroupV2Collector::parse_memory_stat(content);
        assert_eq!(stats.get("anon"), Some(&104857600));
        assert_eq!(stats.get("file"), Some(&52428800));
        assert_eq!(stats.get("inactive_file"), Some(&26214400));
        assert_eq!(stats.get("kernel"), Some(&10485760));
    }

    #[test]
    fn test_parse_cpu_stat_v1() {
        let content = r#"nr_periods 1000
nr_throttled 50
throttled_time 5000000000
"#;

        let (periods, throttled) = CgroupV1Collector::parse_cpu_stat(content);
        assert_eq!(periods, 1000);
        assert_eq!(throttled, 50);
    }

    #[test]
    fn test_parse_memory_stat_v1() {
        let content = r#"cache 104857600
rss 52428800
rss_huge 0
shmem 0
mapped_file 26214400
dirty 0
writeback 0
pgpgin 100000
pgpgout 50000
pgfault 200000
pgmajfault 100
inactive_anon 0
active_anon 52428800
inactive_file 26214400
active_file 78643200
unevictable 0
hierarchical_memory_limit 536870912
total_cache 104857600
total_rss 52428800
total_inactive_file 26214400
"#;

        let stats = CgroupV1Collector::parse_memory_stat(content);
        assert_eq!(stats.get("cache"), Some(&104857600));
        assert_eq!(stats.get("total_cache"), Some(&104857600));
        assert_eq!(stats.get("total_inactive_file"), Some(&26214400));
        assert_eq!(stats.get("rss"), Some(&52428800));
    }
}

#[cfg(test)]
mod container_id_extraction_tests {
    use crate::collector::{CgroupV1Collector, CgroupV2Collector};

    // Valid 64-char hex container ID for testing
    const VALID_CONTAINER_ID: &str =
        "abc123def456789012345678901234567890123456789012345678901234abcd";

    #[test]
    fn test_extract_docker_format_v2() {
        let path = format!("/docker/{}", VALID_CONTAINER_ID);
        let id = CgroupV2Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_crio_format_v2() {
        let path = format!(
            "/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice/crio-{}.scope",
            VALID_CONTAINER_ID
        );
        let id = CgroupV2Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_containerd_format_v2() {
        let path = format!(
            "/system.slice/containerd.service/kubepods-besteffort-pod123.slice/{}",
            VALID_CONTAINER_ID
        );
        let id = CgroupV2Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_docker_format_v1() {
        let path = format!("/docker/{}", VALID_CONTAINER_ID);
        let id = CgroupV1Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_crio_format_v1() {
        let path = format!("/kubepods/besteffort/pod123/crio-{}", VALID_CONTAINER_ID);
        let id = CgroupV1Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_containerd_scope_format_v1() {
        let path = format!(
            "/kubepods/pod123/cri-containerd-{}.scope",
            VALID_CONTAINER_ID
        );
        let id = CgroupV1Collector::extract_container_id(&path);
        assert_eq!(id, Some(VALID_CONTAINER_ID.to_string()));
    }

    #[test]
    fn test_extract_fallback_to_last_component() {
        let path = "/some/unknown/format/my-container-name";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(id, Some("my-container-name".to_string()));
    }

    #[test]
    fn test_extract_empty_path() {
        let path = "";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(id, None);
    }

    #[test]
    fn test_extract_root_path() {
        let path = "/";
        let id = CgroupV2Collector::extract_container_id(path);
        assert_eq!(id, None);
    }
}
