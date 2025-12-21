//! Prediction output formatting and post-processing
//!
//! Handles conversion of raw model outputs to ResourceProfile with
//! safety margins and confidence scoring.

use crate::models::ResourceProfile;

/// Memory safety buffer percentage (20% as per requirement 3.7)
pub const MEMORY_BUFFER_PERCENT: f64 = 0.20;

/// Minimum memory limit in bytes (64MB)
pub const MIN_MEMORY_BYTES: u64 = 64 * 1024 * 1024;

/// Minimum CPU limit in millicores (10m)
pub const MIN_CPU_MILLICORES: u32 = 10;

/// Maximum CPU for normalization (16 cores)
pub const MAX_CPU_CORES: f32 = 16.0;

/// Maximum memory for normalization (64GB)
pub const MAX_MEMORY_GB: f64 = 64.0;

/// Configuration for output formatting
#[derive(Debug, Clone)]
pub struct OutputConfig {
    /// Memory buffer percentage to add to limits (default: 20%)
    pub memory_buffer_percent: f64,
    /// Minimum memory limit in bytes
    pub min_memory_bytes: u64,
    /// Minimum CPU limit in millicores
    pub min_cpu_millicores: u32,
    /// Low confidence threshold
    pub low_confidence_threshold: f32,
}

impl Default for OutputConfig {
    fn default() -> Self {
        Self {
            memory_buffer_percent: MEMORY_BUFFER_PERCENT,
            min_memory_bytes: MIN_MEMORY_BYTES,
            min_cpu_millicores: MIN_CPU_MILLICORES,
            low_confidence_threshold: 0.7,
        }
    }
}

/// Formats raw model outputs into a ResourceProfile with safety margins
pub struct OutputFormatter {
    config: OutputConfig,
}

impl OutputFormatter {
    pub fn new() -> Self {
        Self { config: OutputConfig::default() }
    }

    pub fn with_config(config: OutputConfig) -> Self {
        Self { config }
    }

    /// Format raw model outputs into a ResourceProfile
    /// 
    /// # Arguments
    /// * `raw_outputs` - Raw model outputs [cpu_req, cpu_lim, mem_req, mem_lim, confidence]
    /// * `model_version` - Version string of the model
    pub fn format(&self, raw_outputs: &[f32; 5], model_version: &str) -> ResourceProfile {
        let cpu_request = self.denormalize_cpu(raw_outputs[0]);
        let cpu_limit = self.denormalize_cpu(raw_outputs[1]);
        let mem_request = self.denormalize_memory(raw_outputs[2]);
        let mem_limit = self.denormalize_memory(raw_outputs[3]);
        let raw_confidence = raw_outputs[4];

        // Apply 20% memory buffer to limit (requirement 3.7)
        let mem_limit_with_buffer = self.apply_memory_buffer(mem_limit);

        // Ensure limits are at least as large as requests
        let final_cpu_limit = cpu_limit.max(cpu_request).max(self.config.min_cpu_millicores);
        let final_mem_limit = mem_limit_with_buffer.max(mem_request).max(self.config.min_memory_bytes);

        // Calculate final confidence score
        let confidence = self.calculate_confidence(raw_confidence);

        ResourceProfile {
            cpu_request_millicores: cpu_request.max(self.config.min_cpu_millicores),
            cpu_limit_millicores: final_cpu_limit,
            memory_request_bytes: mem_request.max(self.config.min_memory_bytes),
            memory_limit_bytes: final_mem_limit,
            confidence,
            model_version: model_version.to_string(),
            generated_at: chrono::Utc::now().timestamp(),
        }
    }

    /// Denormalize CPU value from 0-1 to millicores
    fn denormalize_cpu(&self, normalized: f32) -> u32 {
        let clamped = normalized.clamp(0.0, 1.0);
        (clamped * MAX_CPU_CORES * 1000.0) as u32
    }

    /// Denormalize memory value from 0-1 to bytes
    fn denormalize_memory(&self, normalized: f32) -> u64 {
        let clamped = normalized.clamp(0.0, 1.0);
        (clamped as f64 * MAX_MEMORY_GB * 1024.0 * 1024.0 * 1024.0) as u64
    }

    /// Apply memory buffer to prevent OOM kills (requirement 3.7)
    fn apply_memory_buffer(&self, memory_bytes: u64) -> u64 {
        let buffer = (memory_bytes as f64 * self.config.memory_buffer_percent) as u64;
        memory_bytes.saturating_add(buffer)
    }

    /// Calculate confidence score with adjustments
    fn calculate_confidence(&self, raw_confidence: f32) -> f32 {
        raw_confidence.clamp(0.0, 1.0)
    }

    /// Check if a profile has low confidence
    pub fn is_low_confidence(&self, profile: &ResourceProfile) -> bool {
        profile.confidence < self.config.low_confidence_threshold
    }

    /// Get the reason for low confidence (if applicable)
    pub fn low_confidence_reason(&self, profile: &ResourceProfile) -> Option<String> {
        if profile.confidence < 0.5 {
            Some("Insufficient historical data for reliable prediction".to_string())
        } else if profile.confidence < self.config.low_confidence_threshold {
            Some("High variance in resource usage patterns".to_string())
        } else {
            None
        }
    }
}

impl Default for OutputFormatter {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_buffer_applied() {
        let formatter = OutputFormatter::new();
        let raw = [0.1, 0.2, 0.1, 0.2, 0.9]; // mem_limit = 0.2 normalized
        let profile = formatter.format(&raw, "v1.0.0");

        // Memory limit should have 20% buffer
        let expected_base = (0.2 * MAX_MEMORY_GB * 1024.0 * 1024.0 * 1024.0) as u64;
        let expected_with_buffer = expected_base + (expected_base as f64 * 0.20) as u64;
        
        // Allow small floating point difference
        let diff = (profile.memory_limit_bytes as i64 - expected_with_buffer as i64).abs();
        assert!(diff < 1000, "Memory limit {} differs from expected {} by {}", 
            profile.memory_limit_bytes, expected_with_buffer, diff);
    }

    #[test]
    fn test_limits_at_least_requests() {
        let formatter = OutputFormatter::new();
        // Request > Limit scenario
        let raw = [0.5, 0.3, 0.5, 0.3, 0.8];
        let profile = formatter.format(&raw, "v1.0.0");

        assert!(profile.cpu_limit_millicores >= profile.cpu_request_millicores);
        assert!(profile.memory_limit_bytes >= profile.memory_request_bytes);
    }

    #[test]
    fn test_minimum_values_enforced() {
        let formatter = OutputFormatter::new();
        let raw = [0.0, 0.0, 0.0, 0.0, 0.9];
        let profile = formatter.format(&raw, "v1.0.0");

        assert!(profile.cpu_request_millicores >= MIN_CPU_MILLICORES);
        assert!(profile.cpu_limit_millicores >= MIN_CPU_MILLICORES);
        assert!(profile.memory_request_bytes >= MIN_MEMORY_BYTES);
        assert!(profile.memory_limit_bytes >= MIN_MEMORY_BYTES);
    }

    #[test]
    fn test_confidence_clamped() {
        let formatter = OutputFormatter::new();
        
        let raw_high = [0.1, 0.2, 0.1, 0.2, 1.5];
        let profile_high = formatter.format(&raw_high, "v1.0.0");
        assert!(profile_high.confidence <= 1.0);

        let raw_low = [0.1, 0.2, 0.1, 0.2, -0.5];
        let profile_low = formatter.format(&raw_low, "v1.0.0");
        assert!(profile_low.confidence >= 0.0);
    }

    #[test]
    fn test_low_confidence_detection() {
        let formatter = OutputFormatter::new();
        
        let raw_low = [0.1, 0.2, 0.1, 0.2, 0.5];
        let profile = formatter.format(&raw_low, "v1.0.0");
        
        assert!(formatter.is_low_confidence(&profile));
        assert!(formatter.low_confidence_reason(&profile).is_some());
    }

    #[test]
    fn test_high_confidence_no_reason() {
        let formatter = OutputFormatter::new();
        
        let raw = [0.1, 0.2, 0.1, 0.2, 0.9];
        let profile = formatter.format(&raw, "v1.0.0");
        
        assert!(!formatter.is_low_confidence(&profile));
        assert!(formatter.low_confidence_reason(&profile).is_none());
    }
}
