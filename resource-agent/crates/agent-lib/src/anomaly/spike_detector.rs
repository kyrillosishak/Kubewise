//! CPU spike detection
//!
//! Detects CPU spikes by maintaining rolling 24-hour statistics and
//! identifying values exceeding a configurable standard deviation threshold.

use std::collections::VecDeque;
use std::time::Duration;

/// Default rolling window size (24 hours)
const DEFAULT_WINDOW_SECS: u64 = 24 * 60 * 60;

/// Minimum samples required for spike detection
const MIN_SAMPLES_FOR_DETECTION: usize = 10;

/// Detects CPU spikes exceeding standard deviation threshold
pub struct SpikeDetector {
    /// Number of standard deviations to consider a spike
    pub std_dev_threshold: f64,
    /// Rolling window duration for statistics
    pub window_size: Duration,
}

impl SpikeDetector {
    /// Create a new spike detector with given threshold
    pub fn new(std_dev_threshold: f64) -> Self {
        Self {
            std_dev_threshold,
            window_size: Duration::from_secs(DEFAULT_WINDOW_SECS),
        }
    }

    /// Set custom window size
    pub fn with_window_size(mut self, window_size: Duration) -> Self {
        self.window_size = window_size;
        self
    }

    /// Detect CPU spike from current value and rolling stats
    ///
    /// # Arguments
    /// * `current` - Current CPU usage value
    /// * `history` - Rolling statistics from historical data
    ///
    /// # Returns
    /// * `Some(SpikeAnomaly)` if current value exceeds threshold
    /// * `None` if no spike detected
    pub fn detect(&self, current: f64, history: &RollingStats) -> Option<SpikeAnomaly> {
        // Need sufficient history for meaningful detection
        if history.count < MIN_SAMPLES_FOR_DETECTION as u64 {
            return None;
        }

        // Avoid division by zero
        if history.std_dev < f64::EPSILON {
            return None;
        }

        let z_score = (current - history.mean) / history.std_dev;

        if z_score > self.std_dev_threshold {
            Some(SpikeAnomaly {
                current_usage: current,
                expected_usage: history.mean,
                z_score,
                std_dev: history.std_dev,
                threshold: self.std_dev_threshold,
            })
        } else {
            None
        }
    }
}

impl Default for SpikeDetector {
    fn default() -> Self {
        Self {
            std_dev_threshold: 3.0, // 3 sigma
            window_size: Duration::from_secs(DEFAULT_WINDOW_SECS),
        }
    }
}

/// Rolling statistics for spike detection
///
/// Maintains mean and standard deviation using Welford's online algorithm
/// for numerical stability.
#[derive(Debug, Clone)]
pub struct RollingStats {
    /// Current mean value
    pub mean: f64,
    /// Current standard deviation
    pub std_dev: f64,
    /// Number of samples in the window
    pub count: u64,
    /// Internal: sum of squared differences (for Welford's algorithm)
    m2: f64,
    /// Samples with timestamps for windowing
    samples: VecDeque<(i64, f64)>,
    /// Window duration in seconds
    window_secs: i64,
}

impl RollingStats {
    /// Create new rolling stats with specified window
    pub fn new(window: Duration) -> Self {
        Self {
            mean: 0.0,
            std_dev: 0.0,
            count: 0,
            m2: 0.0,
            samples: VecDeque::new(),
            window_secs: window.as_secs() as i64,
        }
    }

    /// Add a new sample with timestamp
    pub fn add_sample(&mut self, timestamp: i64, value: f64) {
        // Remove old samples outside the window
        self.expire_old_samples(timestamp);

        // Add new sample
        self.samples.push_back((timestamp, value));

        // Recalculate statistics (using Welford's algorithm for stability)
        self.recalculate_stats();
    }

    /// Remove samples outside the rolling window
    fn expire_old_samples(&mut self, current_time: i64) {
        let cutoff = current_time - self.window_secs;
        while let Some((ts, _)) = self.samples.front() {
            if *ts < cutoff {
                self.samples.pop_front();
            } else {
                break;
            }
        }
    }

    /// Recalculate mean and std_dev from current samples
    fn recalculate_stats(&mut self) {
        self.count = self.samples.len() as u64;

        if self.count == 0 {
            self.mean = 0.0;
            self.std_dev = 0.0;
            self.m2 = 0.0;
            return;
        }

        // Calculate mean
        let sum: f64 = self.samples.iter().map(|(_, v)| v).sum();
        self.mean = sum / self.count as f64;

        // Calculate variance using two-pass algorithm for stability
        if self.count > 1 {
            let variance: f64 = self
                .samples
                .iter()
                .map(|(_, v)| (v - self.mean).powi(2))
                .sum::<f64>()
                / (self.count - 1) as f64; // Sample variance (Bessel's correction)

            self.std_dev = variance.sqrt();
            self.m2 = variance * (self.count - 1) as f64;
        } else {
            self.std_dev = 0.0;
            self.m2 = 0.0;
        }
    }

    /// Get the minimum value in the window
    pub fn min(&self) -> Option<f64> {
        self.samples
            .iter()
            .map(|(_, v)| *v)
            .min_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal))
    }

    /// Get the maximum value in the window
    pub fn max(&self) -> Option<f64> {
        self.samples
            .iter()
            .map(|(_, v)| *v)
            .max_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal))
    }

    /// Check if we have enough samples for detection
    pub fn has_sufficient_data(&self) -> bool {
        self.count >= MIN_SAMPLES_FOR_DETECTION as u64
    }
}

impl Default for RollingStats {
    fn default() -> Self {
        Self::new(Duration::from_secs(DEFAULT_WINDOW_SECS))
    }
}

/// CPU spike anomaly details
#[derive(Debug, Clone)]
pub struct SpikeAnomaly {
    /// Current CPU usage that triggered the spike
    pub current_usage: f64,
    /// Expected (mean) CPU usage
    pub expected_usage: f64,
    /// Z-score (number of standard deviations from mean)
    pub z_score: f64,
    /// Standard deviation of the rolling window
    pub std_dev: f64,
    /// Threshold that was exceeded
    pub threshold: f64,
}

impl SpikeAnomaly {
    /// Get the percentage above expected usage
    pub fn percentage_above_expected(&self) -> f64 {
        if self.expected_usage < f64::EPSILON {
            return 0.0;
        }
        ((self.current_usage - self.expected_usage) / self.expected_usage) * 100.0
    }

    /// Get severity level based on z-score
    pub fn severity(&self) -> SpikeSeverity {
        if self.z_score >= 5.0 {
            SpikeSeverity::Critical
        } else if self.z_score >= 4.0 {
            SpikeSeverity::High
        } else {
            SpikeSeverity::Warning
        }
    }
}

/// Severity levels for CPU spikes
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SpikeSeverity {
    Warning,
    High,
    Critical,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_spike_normal_usage() {
        let detector = SpikeDetector::new(3.0);
        let mut stats = RollingStats::new(Duration::from_secs(3600));

        // Add normal samples
        for i in 0..100 {
            stats.add_sample(i * 60, 0.5 + (i as f64 % 10.0) * 0.01);
        }

        // Current value within normal range
        let result = detector.detect(0.55, &stats);
        assert!(result.is_none());
    }

    #[test]
    fn test_detect_spike() {
        let detector = SpikeDetector::new(3.0);
        let mut stats = RollingStats::new(Duration::from_secs(3600));

        // Add stable samples around 0.5
        for i in 0..100 {
            stats.add_sample(i * 60, 0.5);
        }

        // Spike to 2.0 (way above 3 std devs since std_dev is ~0)
        // Need some variance first
        let mut stats2 = RollingStats::new(Duration::from_secs(3600));
        for i in 0..100 {
            stats2.add_sample(i * 60, 0.5 + (i as f64 % 5.0) * 0.02);
        }

        let result = detector.detect(2.0, &stats2);
        assert!(result.is_some());
        let anomaly = result.unwrap();
        assert!(anomaly.z_score > 3.0);
    }

    #[test]
    fn test_insufficient_samples() {
        let detector = SpikeDetector::new(3.0);
        let mut stats = RollingStats::new(Duration::from_secs(3600));

        // Only 5 samples
        for i in 0..5 {
            stats.add_sample(i * 60, 0.5);
        }

        let result = detector.detect(2.0, &stats);
        assert!(result.is_none());
    }

    #[test]
    fn test_rolling_window_expiry() {
        let mut stats = RollingStats::new(Duration::from_secs(3600)); // 1 hour

        // Add samples over 2 hours
        for i in 0..120 {
            stats.add_sample(i * 60, 0.5);
        }

        // Should only have ~60 samples (last hour)
        assert!(stats.count <= 61);
        assert!(stats.count >= 59);
    }

    #[test]
    fn test_spike_severity() {
        let anomaly = SpikeAnomaly {
            current_usage: 2.0,
            expected_usage: 0.5,
            z_score: 5.5,
            std_dev: 0.1,
            threshold: 3.0,
        };

        assert_eq!(anomaly.severity(), SpikeSeverity::Critical);

        let warning = SpikeAnomaly {
            z_score: 3.5,
            ..anomaly.clone()
        };
        assert_eq!(warning.severity(), SpikeSeverity::Warning);
    }

    #[test]
    fn test_rolling_stats_calculation() {
        let mut stats = RollingStats::new(Duration::from_secs(3600));

        // Add known values: 1, 2, 3, 4, 5
        for i in 1..=20 {
            stats.add_sample(i * 60, i as f64);
        }

        // Mean should be 10.5
        assert!((stats.mean - 10.5).abs() < 0.01);
        assert!(stats.std_dev > 0.0);
        assert_eq!(stats.count, 20);
    }
}
