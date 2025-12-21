//! Model update client for downloading and validating ML models
//!
//! This module provides:
//! - Polling for model updates during low-activity periods
//! - Checksum validation before applying updates
//! - Rollback support on validation failure

use crate::proto::{ModelResponse, PredictorSyncClient};
use anyhow::{Context, Result};
use chrono::Timelike;
use sha2::{Digest, Sha256};
use std::fs::{self, File};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::Duration;
use tokio::sync::RwLock;
use tonic::transport::Channel;
use tracing::{debug, error, info, warn};

/// Configuration for model updates
#[derive(Debug, Clone)]
pub struct ModelUpdateConfig {
    /// Directory to store model files
    pub model_dir: PathBuf,
    /// Low-activity hours for updates (start hour, 0-23)
    pub update_window_start: u8,
    /// Low-activity hours for updates (end hour, 0-23)
    pub update_window_end: u8,
    /// Poll interval for checking updates
    pub poll_interval: Duration,
    /// Maximum model size in bytes
    pub max_model_size: usize,
    /// Number of previous versions to keep for rollback
    pub versions_to_keep: usize,
    /// Maximum deviation threshold for auto-rollback (0.0-1.0)
    pub max_deviation_threshold: f32,
}

impl Default for ModelUpdateConfig {
    fn default() -> Self {
        Self {
            model_dir: PathBuf::from("/var/lib/predictor/models"),
            update_window_start: 2,  // 02:00
            update_window_end: 4,    // 04:00
            poll_interval: Duration::from_secs(3600), // 1 hour
            max_model_size: 100 * 1024, // 100KB
            versions_to_keep: 5,
            max_deviation_threshold: 0.20, // 20%
        }
    }
}

/// Model version information
#[derive(Debug, Clone)]
pub struct ModelVersion {
    pub version: String,
    pub path: PathBuf,
    pub checksum: String,
    pub size_bytes: usize,
    pub validation_accuracy: Option<f32>,
    pub downloaded_at: i64,
}

/// Model update client
pub struct ModelUpdateClient {
    config: ModelUpdateConfig,
    agent_id: String,
    current_version: RwLock<Option<ModelVersion>>,
    previous_versions: RwLock<Vec<ModelVersion>>,
}

impl ModelUpdateClient {
    /// Create a new model update client
    pub fn new(config: ModelUpdateConfig, agent_id: String) -> Result<Self> {
        // Ensure model directory exists
        fs::create_dir_all(&config.model_dir)
            .with_context(|| format!("Failed to create model directory {:?}", config.model_dir))?;

        let client = Self {
            config,
            agent_id,
            current_version: RwLock::new(None),
            previous_versions: RwLock::new(Vec::new()),
        };

        Ok(client)
    }

    /// Check if we're in the update window
    pub fn is_update_window(&self) -> bool {
        let now = chrono::Local::now();
        let hour = now.hour() as u8;

        if self.config.update_window_start <= self.config.update_window_end {
            hour >= self.config.update_window_start && hour < self.config.update_window_end
        } else {
            // Window spans midnight
            hour >= self.config.update_window_start || hour < self.config.update_window_end
        }
    }

    /// Get current model version
    pub async fn current_version(&self) -> Option<String> {
        self.current_version.read().await.as_ref().map(|v| v.version.clone())
    }

    /// Get current model path
    pub async fn current_model_path(&self) -> Option<PathBuf> {
        self.current_version.read().await.as_ref().map(|v| v.path.clone())
    }

    /// Check for and download model updates
    pub async fn check_for_update(
        &self,
        client: &mut PredictorSyncClient<Channel>,
    ) -> Result<Option<ModelVersion>> {
        let current_version = self.current_version().await.unwrap_or_default();

        debug!(
            current_version = %current_version,
            "Checking for model updates"
        );

        // Request model update from API
        let request = tonic::Request::new(crate::proto::ModelRequest {
            agent_id: self.agent_id.clone(),
            current_model_version: current_version.clone(),
        });

        let response = client
            .get_model_update(request)
            .await
            .context("Failed to check for model update")?
            .into_inner();

        if !response.update_available {
            debug!("No model update available");
            return Ok(None);
        }

        info!(
            current = %current_version,
            new = %response.new_version,
            "Model update available"
        );

        // Validate and apply the update
        let new_version = self.apply_update(response).await?;

        Ok(Some(new_version))
    }

    /// Apply a model update
    async fn apply_update(&self, response: ModelResponse) -> Result<ModelVersion> {
        // Validate model size
        if response.model_weights.len() > self.config.max_model_size {
            return Err(anyhow::anyhow!(
                "Model size {} exceeds maximum {}",
                response.model_weights.len(),
                self.config.max_model_size
            ));
        }

        // Validate checksum
        let computed_checksum = compute_checksum(&response.model_weights);
        if computed_checksum != response.checksum {
            return Err(anyhow::anyhow!(
                "Checksum mismatch: expected {}, got {}",
                response.checksum,
                computed_checksum
            ));
        }

        info!(
            version = %response.new_version,
            size = response.model_weights.len(),
            checksum = %computed_checksum,
            "Model checksum validated"
        );

        // Save model to disk
        let model_path = self.config.model_dir.join(format!("model_{}.onnx", response.new_version));
        self.save_model(&model_path, &response.model_weights)?;

        // Create version info
        let validation_accuracy = response.metadata.as_ref().map(|m| m.validation_accuracy);
        let new_version = ModelVersion {
            version: response.new_version.clone(),
            path: model_path,
            checksum: computed_checksum,
            size_bytes: response.model_weights.len(),
            validation_accuracy,
            downloaded_at: chrono::Utc::now().timestamp(),
        };

        // Move current version to previous versions
        {
            let mut current = self.current_version.write().await;
            if let Some(old_version) = current.take() {
                let mut previous = self.previous_versions.write().await;
                previous.insert(0, old_version);

                // Keep only the configured number of versions
                while previous.len() > self.config.versions_to_keep {
                    if let Some(removed) = previous.pop() {
                        // Clean up old model file
                        if let Err(e) = fs::remove_file(&removed.path) {
                            warn!(
                                path = %removed.path.display(),
                                error = %e,
                                "Failed to remove old model file"
                            );
                        }
                    }
                }
            }

            *current = Some(new_version.clone());
        }

        info!(
            version = %new_version.version,
            path = %new_version.path.display(),
            "Model update applied successfully"
        );

        Ok(new_version)
    }

    /// Save model weights to disk
    fn save_model(&self, path: &Path, weights: &[u8]) -> Result<()> {
        // Write to temp file first
        let temp_path = path.with_extension("tmp");
        let mut file = File::create(&temp_path)
            .with_context(|| format!("Failed to create temp model file {:?}", temp_path))?;

        file.write_all(weights)
            .context("Failed to write model weights")?;
        file.sync_all()
            .context("Failed to sync model file")?;

        // Rename to final path
        fs::rename(&temp_path, path)
            .with_context(|| format!("Failed to rename {:?} to {:?}", temp_path, path))?;

        Ok(())
    }

    /// Validate new model against historical data
    pub async fn validate_model(
        &self,
        _model_path: &Path,
        _historical_predictions: &[(f32, f32)], // (predicted, actual)
    ) -> Result<ValidationResult> {
        // TODO: Implement actual model validation
        // For now, return a placeholder result
        Ok(ValidationResult {
            passed: true,
            deviation: 0.0,
            samples_tested: 0,
            message: "Validation not yet implemented".to_string(),
        })
    }

    /// Check if model deviation exceeds threshold
    pub fn exceeds_deviation_threshold(&self, deviation: f32) -> bool {
        deviation > self.config.max_deviation_threshold
    }

    /// Rollback to previous model version
    pub async fn rollback(&self) -> Result<Option<ModelVersion>> {
        let mut previous = self.previous_versions.write().await;

        if previous.is_empty() {
            warn!("No previous model version available for rollback");
            return Ok(None);
        }

        let rollback_version = previous.remove(0);

        // Verify the rollback model file exists
        if !rollback_version.path.exists() {
            return Err(anyhow::anyhow!(
                "Rollback model file not found: {:?}",
                rollback_version.path
            ));
        }

        // Move current to previous (if exists)
        {
            let mut current = self.current_version.write().await;
            if let Some(failed_version) = current.take() {
                // Don't keep the failed version in history
                if let Err(e) = fs::remove_file(&failed_version.path) {
                    warn!(
                        path = %failed_version.path.display(),
                        error = %e,
                        "Failed to remove failed model file"
                    );
                }
            }

            *current = Some(rollback_version.clone());
        }

        info!(
            version = %rollback_version.version,
            "Rolled back to previous model version"
        );

        Ok(Some(rollback_version))
    }

    /// Get list of available versions for rollback
    pub async fn available_rollback_versions(&self) -> Vec<String> {
        self.previous_versions
            .read()
            .await
            .iter()
            .map(|v| v.version.clone())
            .collect()
    }

    /// Load an existing model from disk
    pub async fn load_existing_model(&self, version: &str, path: &Path) -> Result<()> {
        if !path.exists() {
            return Err(anyhow::anyhow!("Model file not found: {:?}", path));
        }

        let weights = fs::read(path)
            .with_context(|| format!("Failed to read model file {:?}", path))?;

        let checksum = compute_checksum(&weights);

        let model_version = ModelVersion {
            version: version.to_string(),
            path: path.to_path_buf(),
            checksum,
            size_bytes: weights.len(),
            validation_accuracy: None,
            downloaded_at: chrono::Utc::now().timestamp(),
        };

        let mut current = self.current_version.write().await;
        *current = Some(model_version);

        info!(
            version = %version,
            path = %path.display(),
            "Loaded existing model"
        );

        Ok(())
    }

    /// Get model update statistics
    pub async fn stats(&self) -> ModelUpdateStats {
        let current = self.current_version.read().await;
        let previous = self.previous_versions.read().await;

        ModelUpdateStats {
            current_version: current.as_ref().map(|v| v.version.clone()),
            current_size_bytes: current.as_ref().map(|v| v.size_bytes),
            available_rollback_versions: previous.len(),
            last_update_time: current.as_ref().map(|v| v.downloaded_at),
        }
    }
}

/// Compute SHA256 checksum of data
fn compute_checksum(data: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(data);
    hex::encode(hasher.finalize())
}

/// Model validation result
#[derive(Debug, Clone)]
pub struct ValidationResult {
    pub passed: bool,
    pub deviation: f32,
    pub samples_tested: usize,
    pub message: String,
}

/// Model update statistics
#[derive(Debug, Clone)]
pub struct ModelUpdateStats {
    pub current_version: Option<String>,
    pub current_size_bytes: Option<usize>,
    pub available_rollback_versions: usize,
    pub last_update_time: Option<i64>,
}

/// Background model update worker
pub struct ModelUpdateWorker {
    client: ModelUpdateClient,
    grpc_client: Option<PredictorSyncClient<Channel>>,
}

impl ModelUpdateWorker {
    /// Create a new model update worker
    pub fn new(config: ModelUpdateConfig, agent_id: String) -> Result<Self> {
        Ok(Self {
            client: ModelUpdateClient::new(config, agent_id)?,
            grpc_client: None,
        })
    }

    /// Set the gRPC client
    pub fn set_grpc_client(&mut self, client: PredictorSyncClient<Channel>) {
        self.grpc_client = Some(client);
    }

    /// Run the update check loop
    pub async fn run(&mut self) {
        let poll_interval = self.client.config.poll_interval;

        loop {
            // Wait for poll interval
            tokio::time::sleep(poll_interval).await;

            // Check if we're in the update window
            if !self.client.is_update_window() {
                debug!("Not in update window, skipping model check");
                continue;
            }

            // Check for updates
            if let Some(ref mut grpc_client) = self.grpc_client {
                match self.client.check_for_update(grpc_client).await {
                    Ok(Some(version)) => {
                        info!(version = %version.version, "Model updated successfully");
                    }
                    Ok(None) => {
                        debug!("No model update available");
                    }
                    Err(e) => {
                        error!(error = %e, "Failed to check for model update");
                    }
                }
            } else {
                warn!("No gRPC client configured for model updates");
            }
        }
    }

    /// Get the underlying client for direct access
    pub fn client(&self) -> &ModelUpdateClient {
        &self.client
    }

    /// Get mutable access to the underlying client
    pub fn client_mut(&mut self) -> &mut ModelUpdateClient {
        &mut self.client
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_compute_checksum() {
        let data = b"test model weights";
        let checksum = compute_checksum(data);
        assert!(!checksum.is_empty());
        assert_eq!(checksum.len(), 64); // SHA256 hex is 64 chars
    }

    #[test]
    fn test_checksum_consistency() {
        let data = b"test model weights";
        let checksum1 = compute_checksum(data);
        let checksum2 = compute_checksum(data);
        assert_eq!(checksum1, checksum2);
    }

    #[test]
    fn test_model_update_config_default() {
        let config = ModelUpdateConfig::default();
        assert_eq!(config.update_window_start, 2);
        assert_eq!(config.update_window_end, 4);
        assert_eq!(config.versions_to_keep, 5);
        assert_eq!(config.max_model_size, 100 * 1024);
    }

    #[tokio::test]
    async fn test_model_update_client_creation() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string());
        assert!(client.is_ok());

        let client = client.unwrap();
        assert!(client.current_version().await.is_none());
    }

    #[tokio::test]
    async fn test_load_existing_model() {
        let temp_dir = TempDir::new().unwrap();
        let model_path = temp_dir.path().join("test_model.onnx");

        // Create a test model file
        let test_weights = b"test model weights";
        fs::write(&model_path, test_weights).unwrap();

        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Load the model
        let result = client.load_existing_model("v1.0.0", &model_path).await;
        assert!(result.is_ok());

        // Verify it's loaded
        let version = client.current_version().await;
        assert_eq!(version, Some("v1.0.0".to_string()));
    }

    #[tokio::test]
    async fn test_rollback_no_previous() {
        let temp_dir = TempDir::new().unwrap();
        let config = ModelUpdateConfig {
            model_dir: temp_dir.path().to_path_buf(),
            ..Default::default()
        };

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        // Try to rollback with no previous versions
        let result = client.rollback().await;
        assert!(result.is_ok());
        assert!(result.unwrap().is_none());
    }

    #[test]
    fn test_exceeds_deviation_threshold() {
        let config = ModelUpdateConfig {
            max_deviation_threshold: 0.20,
            ..Default::default()
        };

        let temp_dir = TempDir::new().unwrap();
        let mut config = config;
        config.model_dir = temp_dir.path().to_path_buf();

        let client = ModelUpdateClient::new(config, "test-agent".to_string()).unwrap();

        assert!(!client.exceeds_deviation_threshold(0.15));
        assert!(client.exceeds_deviation_threshold(0.25));
    }
}
