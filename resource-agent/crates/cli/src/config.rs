//! Configuration management for the CLI

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

/// CLI configuration
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Config {
    /// API endpoint URL
    pub api_url: Option<String>,
    /// Default namespace
    pub default_namespace: Option<String>,
    /// Default output format
    pub default_format: Option<String>,
}

impl Config {
    /// Load configuration from file
    #[allow(dead_code)]
    pub fn load() -> Result<Self> {
        let config_path = Self::config_path()?;
        
        if !config_path.exists() {
            return Ok(Self::default());
        }

        let content = std::fs::read_to_string(&config_path)
            .context("Failed to read config file")?;
        
        serde_json::from_str(&content).context("Failed to parse config file")
    }

    /// Save configuration to file
    #[allow(dead_code)]
    pub fn save(&self) -> Result<()> {
        let config_path = Self::config_path()?;
        
        if let Some(parent) = config_path.parent() {
            std::fs::create_dir_all(parent).context("Failed to create config directory")?;
        }

        let content = serde_json::to_string_pretty(self).context("Failed to serialize config")?;
        std::fs::write(&config_path, content).context("Failed to write config file")?;
        
        Ok(())
    }

    /// Get the configuration file path
    fn config_path() -> Result<PathBuf> {
        let home = dirs_next::home_dir().context("Could not determine home directory")?;
        Ok(home.join(".config").join("crp").join("config.json"))
    }
}

/// Get kubeconfig path
#[allow(dead_code)]
pub fn kubeconfig_path(override_path: Option<&str>) -> Result<PathBuf> {
    if let Some(path) = override_path {
        return Ok(PathBuf::from(path));
    }

    if let Ok(path) = std::env::var("KUBECONFIG") {
        return Ok(PathBuf::from(path));
    }

    let home = dirs_next::home_dir().context("Could not determine home directory")?;
    Ok(home.join(".kube").join("config"))
}
