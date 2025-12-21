//! ML prediction engine

mod features;
mod inference;
mod output;
mod scheduler;

pub use features::{linear_regression_slope, FeatureExtractor, MIN_SAMPLES};
pub use inference::{FallbackPredictor, InferenceStats, OnnxPredictor};
pub use output::{OutputConfig, OutputFormatter, MEMORY_BUFFER_PERCENT};
pub use scheduler::{
    PredictionConfig, PredictionResult, PredictionScheduler, SchedulerStats,
    DEFAULT_PREDICTION_INTERVAL, INFERENCE_TIMEOUT,
};

use crate::models::{FeatureVector, ResourceProfile};
use anyhow::Result;

/// Trait for prediction implementations
pub trait Predictor: Send + Sync {
    /// Generate resource profile prediction from features
    fn predict(&self, features: &FeatureVector) -> Result<ResourceProfile>;

    /// Update model weights
    fn update_model(&mut self, weights: &[u8]) -> Result<()>;

    /// Get current model version
    fn model_version(&self) -> &str;
}
