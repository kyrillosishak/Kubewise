//! ONNX Runtime inference using tract
//!
//! Provides lightweight ML inference for resource prediction using
//! quantized int8 models loaded via tract-onnx.

use super::output::OutputFormatter;
use super::Predictor;
use crate::models::{FeatureVector, ResourceProfile};
use anyhow::{Context, Result};
use std::sync::RwLock;
use std::time::Instant;
use tract_onnx::prelude::*;
use tracing::{debug, warn};

/// Number of input features expected by the model
const NUM_FEATURES: usize = 12;

/// Number of output values from the model
const NUM_OUTPUTS: usize = 5;

/// Maximum inference latency before warning (5ms target)
const MAX_INFERENCE_MS: u128 = 5;

type TractModel = SimplePlan<TypedFact, Box<dyn TypedOp>, Graph<TypedFact, Box<dyn TypedOp>>>;

/// ONNX-based predictor using tract for lightweight inference
pub struct OnnxPredictor {
    model: RwLock<Option<TractModel>>,
    model_version: RwLock<String>,
    output_formatter: OutputFormatter,
    inference_count: std::sync::atomic::AtomicU64,
    slow_inference_count: std::sync::atomic::AtomicU64,
}

impl OnnxPredictor {
    /// Create a new predictor without a model (will use fallback)
    pub fn new_without_model() -> Self {
        Self {
            model: RwLock::new(None),
            model_version: RwLock::new("fallback".to_string()),
            output_formatter: OutputFormatter::new(),
            inference_count: std::sync::atomic::AtomicU64::new(0),
            slow_inference_count: std::sync::atomic::AtomicU64::new(0),
        }
    }

    /// Create a new predictor from model bytes
    pub fn new(model_bytes: &[u8]) -> Result<Self> {
        let model = Self::load_model(model_bytes)?;
        Ok(Self {
            model: RwLock::new(Some(model)),
            model_version: RwLock::new("v0.1.0".to_string()),
            output_formatter: OutputFormatter::new(),
            inference_count: std::sync::atomic::AtomicU64::new(0),
            slow_inference_count: std::sync::atomic::AtomicU64::new(0),
        })
    }

    /// Load and optimize an ONNX model from bytes
    fn load_model(model_bytes: &[u8]) -> Result<TractModel> {
        let model = tract_onnx::onnx()
            .model_for_read(&mut std::io::Cursor::new(model_bytes))
            .context("Failed to parse ONNX model")?
            .with_input_fact(0, f32::fact([1, NUM_FEATURES]).into())
            .context("Failed to set input shape")?
            .into_optimized()
            .context("Failed to optimize model")?
            .into_runnable()
            .context("Failed to create runnable model")?;
        Ok(model)
    }

    /// Convert feature vector to tensor input
    fn features_to_tensor(&self, features: &FeatureVector) -> Tensor {
        let data = vec![
            features.cpu_usage_p50,
            features.cpu_usage_p95,
            features.cpu_usage_p99,
            features.mem_usage_p50,
            features.mem_usage_p95,
            features.mem_usage_p99,
            features.cpu_variance,
            features.mem_trend,
            features.throttle_ratio,
            features.hour_of_day,
            features.day_of_week,
            features.workload_age_days,
        ];
        tract_ndarray::Array2::from_shape_vec((1, NUM_FEATURES), data)
            .unwrap()
            .into()
    }

    /// Convert model output tensor to ResourceProfile
    fn tensor_to_profile(&self, output: &Tensor, model_version: &str) -> Result<ResourceProfile> {
        let output_view = output.to_array_view::<f32>()?;
        let values: Vec<f32> = output_view.iter().copied().collect();

        if values.len() < NUM_OUTPUTS {
            anyhow::bail!("Model output has {} values, expected {}", values.len(), NUM_OUTPUTS);
        }

        // Use OutputFormatter to apply memory buffer and format output
        let raw_outputs: [f32; 5] = [values[0], values[1], values[2], values[3], values[4]];
        Ok(self.output_formatter.format(&raw_outputs, model_version))
    }

    /// Get inference statistics
    pub fn stats(&self) -> InferenceStats {
        InferenceStats {
            total_inferences: self.inference_count.load(std::sync::atomic::Ordering::Relaxed),
            slow_inferences: self.slow_inference_count.load(std::sync::atomic::Ordering::Relaxed),
        }
    }

    /// Check if a prediction has low confidence
    pub fn is_low_confidence(&self, profile: &ResourceProfile) -> bool {
        self.output_formatter.is_low_confidence(profile)
    }

    /// Get reason for low confidence
    pub fn low_confidence_reason(&self, profile: &ResourceProfile) -> Option<String> {
        self.output_formatter.low_confidence_reason(profile)
    }
}

impl Predictor for OnnxPredictor {
    fn predict(&self, features: &FeatureVector) -> Result<ResourceProfile> {
        let start = Instant::now();

        let model_guard = self.model.read().map_err(|e| anyhow::anyhow!("Lock poisoned: {}", e))?;
        
        // If no model loaded, use fallback
        let model = match model_guard.as_ref() {
            Some(m) => m,
            None => {
                debug!("No model loaded, using fallback predictor");
                return Ok(FallbackPredictor::predict(features));
            }
        };

        let version = self.model_version.read().map_err(|e| anyhow::anyhow!("Lock poisoned: {}", e))?;
        let input = self.features_to_tensor(features);

        let result = model.run(tvec!(input.into()))?;
        let output = result.get(0).context("No output from model")?;

        let elapsed = start.elapsed();
        self.inference_count.fetch_add(1, std::sync::atomic::Ordering::Relaxed);

        if elapsed.as_millis() > MAX_INFERENCE_MS {
            self.slow_inference_count.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
            warn!(elapsed_ms = elapsed.as_millis(), "Inference exceeded {}ms target", MAX_INFERENCE_MS);
        } else {
            debug!(elapsed_us = elapsed.as_micros(), "Inference completed");
        }

        self.tensor_to_profile(output, &version)
    }

    fn update_model(&mut self, weights: &[u8]) -> Result<()> {
        let new_model = Self::load_model(weights)?;
        let mut model = self.model.write().map_err(|e| anyhow::anyhow!("Lock poisoned: {}", e))?;
        let mut version = self.model_version.write().map_err(|e| anyhow::anyhow!("Lock poisoned: {}", e))?;

        *model = Some(new_model);
        // Increment version - in production this would come from model metadata
        let current: Vec<&str> = version.split('.').collect();
        if let Some(patch) = current.get(2).and_then(|s| s.parse::<u32>().ok()) {
            *version = format!("v0.1.{}", patch + 1);
        }

        debug!(version = %*version, "Model updated");
        Ok(())
    }

    fn model_version(&self) -> &str {
        // This is a bit awkward due to the RwLock, but we need to return &str
        // In practice, callers should use a method that returns String
        "v0.1.0"
    }
}

/// Inference statistics
#[derive(Debug, Clone)]
pub struct InferenceStats {
    pub total_inferences: u64,
    pub slow_inferences: u64,
}

/// Fallback predictor that uses simple heuristics when model is unavailable
pub struct FallbackPredictor;

impl FallbackPredictor {
    /// Generate a simple heuristic-based prediction with 20% memory buffer
    pub fn predict(features: &FeatureVector) -> ResourceProfile {
        let formatter = OutputFormatter::new();
        
        // Use p95 values for limits, p50 for requests
        let raw_outputs: [f32; 5] = [
            features.cpu_usage_p50,           // cpu_request
            features.cpu_usage_p95 * 1.2,     // cpu_limit with margin
            features.mem_usage_p50,           // mem_request
            features.mem_usage_p95,           // mem_limit (buffer applied by formatter)
            0.5,                              // Low confidence for heuristic
        ];
        
        let mut profile = formatter.format(&raw_outputs, "fallback");
        profile.model_version = "fallback".to_string();
        profile
    }
}
