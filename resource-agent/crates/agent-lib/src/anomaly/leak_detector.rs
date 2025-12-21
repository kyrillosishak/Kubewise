//! Memory leak detection
//!
//! Detects memory leaks by calculating linear regression slope on memory samples
//! and identifying monotonically increasing patterns over a configurable window.

use std::time::Duration;

/// Minimum samples required for leak detection
const MIN_SAMPLES_FOR_DETECTION: usize = 10;

/// Monotonicity threshold - percentage of samples that must be increasing
const MONOTONICITY_THRESHOLD: f64 = 0.95;

/// Detects memory leaks via linear regression on memory samples
pub struct LeakDetector {
    /// Time window for analysis (default: 1 hour)
    pub window_size: Duration,
    /// Minimum slope (bytes/sec) to consider a leak
    pub slope_threshold: f64,
    /// Memory limit for OOM projection (optional)
    pub memory_limit: Option<u64>,
}

impl LeakDetector {
    /// Create a new leak detector with default 1-hour window
    pub fn new(window_size: Duration, slope_threshold: f64) -> Self {
        Self {
            window_size,
            slope_threshold,
            memory_limit: None,
        }
    }

    /// Set memory limit for OOM time projection
    pub fn with_memory_limit(mut self, limit: u64) -> Self {
        self.memory_limit = Some(limit);
        self
    }

    /// Detect memory leak from samples
    ///
    /// # Arguments
    /// * `samples` - Slice of (timestamp_secs, memory_bytes) tuples, sorted by timestamp
    ///
    /// # Returns
    /// * `Some(LeakAnomaly)` if a leak is detected
    /// * `None` if no leak pattern found
    pub fn detect(&self, samples: &[(i64, u64)]) -> Option<LeakAnomaly> {
        if samples.len() < MIN_SAMPLES_FOR_DETECTION {
            return None;
        }

        // Filter samples within window
        let window_samples = self.filter_window(samples);
        if window_samples.len() < MIN_SAMPLES_FOR_DETECTION {
            return None;
        }

        // Calculate linear regression slope
        let slope = self.linear_regression_slope(&window_samples);

        // Check if slope exceeds threshold (positive slope = increasing memory)
        if slope <= self.slope_threshold {
            return None;
        }

        // Check monotonicity - memory should be consistently increasing
        let monotonicity = self.calculate_monotonicity(&window_samples);
        if monotonicity < MONOTONICITY_THRESHOLD {
            return None;
        }

        // Calculate confidence based on R² and monotonicity
        let r_squared = self.calculate_r_squared(&window_samples, slope);
        let confidence = (r_squared * monotonicity) as f32;

        // Project OOM time if memory limit is known
        let projected_oom_time = self.project_oom_time(&window_samples, slope);

        Some(LeakAnomaly {
            slope_bytes_per_sec: slope,
            projected_oom_time,
            confidence,
            current_memory_bytes: window_samples.last().map(|(_, m)| *m).unwrap_or(0),
            samples_analyzed: window_samples.len(),
        })
    }

    /// Filter samples to those within the detection window
    fn filter_window<'a>(&self, samples: &'a [(i64, u64)]) -> Vec<&'a (i64, u64)> {
        if samples.is_empty() {
            return Vec::new();
        }

        let latest_ts = samples.last().map(|(ts, _)| *ts).unwrap_or(0);
        let window_start = latest_ts - self.window_size.as_secs() as i64;

        samples
            .iter()
            .filter(|(ts, _)| *ts >= window_start)
            .collect()
    }

    /// Calculate linear regression slope (bytes per second)
    fn linear_regression_slope(&self, samples: &[&(i64, u64)]) -> f64 {
        let n = samples.len() as f64;
        if n < 2.0 {
            return 0.0;
        }

        // Normalize timestamps to avoid precision issues
        let t0 = samples.first().map(|(ts, _)| *ts).unwrap_or(0) as f64;

        let mut sum_x = 0.0;
        let mut sum_y = 0.0;
        let mut sum_xy = 0.0;
        let mut sum_xx = 0.0;

        for (ts, mem) in samples.iter() {
            let x = (*ts as f64) - t0;
            let y = *mem as f64;
            sum_x += x;
            sum_y += y;
            sum_xy += x * y;
            sum_xx += x * x;
        }

        let denominator = n * sum_xx - sum_x * sum_x;
        if denominator.abs() < f64::EPSILON {
            return 0.0;
        }

        (n * sum_xy - sum_x * sum_y) / denominator
    }

    /// Calculate R² (coefficient of determination) for the linear fit
    fn calculate_r_squared(&self, samples: &[&(i64, u64)], slope: f64) -> f64 {
        if samples.len() < 2 {
            return 0.0;
        }

        let t0 = samples.first().map(|(ts, _)| *ts).unwrap_or(0) as f64;
        let n = samples.len() as f64;

        // Calculate mean of y values
        let mean_y: f64 = samples.iter().map(|(_, m)| *m as f64).sum::<f64>() / n;

        // Calculate intercept
        let mean_x: f64 = samples.iter().map(|(ts, _)| (*ts as f64) - t0).sum::<f64>() / n;
        let intercept = mean_y - slope * mean_x;

        // Calculate SS_res and SS_tot
        let mut ss_res = 0.0;
        let mut ss_tot = 0.0;

        for (ts, mem) in samples.iter() {
            let x = (*ts as f64) - t0;
            let y = *mem as f64;
            let y_pred = slope * x + intercept;

            ss_res += (y - y_pred).powi(2);
            ss_tot += (y - mean_y).powi(2);
        }

        if ss_tot.abs() < f64::EPSILON {
            return 0.0;
        }

        1.0 - (ss_res / ss_tot)
    }

    /// Calculate monotonicity - fraction of samples where memory increased
    fn calculate_monotonicity(&self, samples: &[&(i64, u64)]) -> f64 {
        if samples.len() < 2 {
            return 0.0;
        }

        let mut increasing_count = 0;
        for window in samples.windows(2) {
            if window[1].1 >= window[0].1 {
                increasing_count += 1;
            }
        }

        increasing_count as f64 / (samples.len() - 1) as f64
    }

    /// Project when OOM will occur based on current trend
    fn project_oom_time(&self, samples: &[&(i64, u64)], slope: f64) -> i64 {
        let Some(limit) = self.memory_limit else {
            return 0; // No limit set, can't project
        };

        let (current_ts, current_mem) = samples.last().map(|(ts, m)| (*ts, *m)).unwrap_or((0, 0));

        if current_mem >= limit {
            return current_ts; // Already at or over limit
        }

        if slope <= 0.0 {
            return 0; // Not increasing
        }

        let bytes_remaining = (limit - current_mem) as f64;
        let seconds_until_oom = bytes_remaining / slope;

        current_ts + seconds_until_oom as i64
    }
}

impl Default for LeakDetector {
    fn default() -> Self {
        Self {
            window_size: Duration::from_secs(3600), // 1 hour
            slope_threshold: 1024.0,                // 1 KB/sec minimum
            memory_limit: None,
        }
    }
}

/// Memory leak anomaly details
#[derive(Debug, Clone)]
pub struct LeakAnomaly {
    /// Rate of memory increase in bytes per second
    pub slope_bytes_per_sec: f64,
    /// Projected Unix timestamp when OOM will occur (0 if unknown)
    pub projected_oom_time: i64,
    /// Confidence score 0.0-1.0 based on R² and monotonicity
    pub confidence: f32,
    /// Current memory usage in bytes
    pub current_memory_bytes: u64,
    /// Number of samples used in analysis
    pub samples_analyzed: usize,
}

impl LeakAnomaly {
    /// Get human-readable leak rate
    pub fn leak_rate_per_hour(&self) -> f64 {
        self.slope_bytes_per_sec * 3600.0
    }

    /// Get human-readable leak rate in MB/hour
    pub fn leak_rate_mb_per_hour(&self) -> f64 {
        self.leak_rate_per_hour() / (1024.0 * 1024.0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_leak_flat_memory() {
        let detector = LeakDetector::default();
        let samples: Vec<(i64, u64)> = (0..60)
            .map(|i| (i * 60, 100_000_000)) // Flat 100MB
            .collect();

        assert!(detector.detect(&samples).is_none());
    }

    #[test]
    fn test_detect_clear_leak() {
        let detector = LeakDetector::new(Duration::from_secs(3600), 1000.0);
        // Memory increasing by ~10KB per sample (every 60 seconds)
        let samples: Vec<(i64, u64)> = (0..60)
            .map(|i| (i * 60, 100_000_000 + (i as u64 * 600_000)))
            .collect();

        let result = detector.detect(&samples);
        assert!(result.is_some());
        let anomaly = result.unwrap();
        assert!(anomaly.slope_bytes_per_sec > 1000.0);
        assert!(anomaly.confidence > 0.8);
    }

    #[test]
    fn test_insufficient_samples() {
        let detector = LeakDetector::default();
        let samples: Vec<(i64, u64)> = (0..5)
            .map(|i| (i * 60, 100_000_000 + (i as u64 * 1_000_000)))
            .collect();

        assert!(detector.detect(&samples).is_none());
    }

    #[test]
    fn test_oom_projection() {
        let detector =
            LeakDetector::new(Duration::from_secs(3600), 1000.0).with_memory_limit(200_000_000); // 200MB limit

        // Starting at 100MB, increasing 1MB per minute
        let samples: Vec<(i64, u64)> = (0..60)
            .map(|i| (i * 60, 100_000_000 + (i as u64 * 1_000_000)))
            .collect();

        let result = detector.detect(&samples);
        assert!(result.is_some());
        let anomaly = result.unwrap();
        assert!(anomaly.projected_oom_time > 0);
    }

    #[test]
    fn test_non_monotonic_rejected() {
        let detector = LeakDetector::default();
        // Oscillating memory - not a leak
        let samples: Vec<(i64, u64)> = (0..60)
            .map(|i| {
                let base = 100_000_000u64;
                let variation = if i % 2 == 0 { 10_000_000 } else { 0 };
                (i * 60, base + variation)
            })
            .collect();

        assert!(detector.detect(&samples).is_none());
    }
}
