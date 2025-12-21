//! gRPC client for Recommendation API communication with mTLS support
//!
//! This module provides a secure gRPC client that:
//! - Uses mTLS for authentication
//! - Supports certificate rotation
//! - Implements connection pooling and keepalive
//! - Handles reconnection with exponential backoff

use crate::proto::{
    predictor_sync_client::PredictorSyncClient, ModelRequest, ModelResponse, RegisterRequest,
    RegisterResponse,
};
use anyhow::{Context, Result};
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;
use tonic::transport::{Certificate, Channel, ClientTlsConfig, Identity};
use tracing::{debug, info, warn};

/// Configuration for the gRPC client
#[derive(Debug, Clone)]
pub struct ClientConfig {
    /// API endpoint URL (e.g., "https://recommendation-api:8443")
    pub endpoint: String,
    /// Path to CA certificate for server verification
    pub ca_cert_path: PathBuf,
    /// Path to client certificate for mTLS
    pub client_cert_path: PathBuf,
    /// Path to client private key
    pub client_key_path: PathBuf,
    /// Connection timeout
    pub connect_timeout: Duration,
    /// Request timeout
    pub request_timeout: Duration,
    /// Keepalive interval
    pub keepalive_interval: Duration,
    /// Keepalive timeout
    pub keepalive_timeout: Duration,
    /// Initial backoff for reconnection
    pub initial_backoff: Duration,
    /// Maximum backoff for reconnection
    pub max_backoff: Duration,
}

impl Default for ClientConfig {
    fn default() -> Self {
        Self {
            endpoint: "https://recommendation-api:8443".to_string(),
            ca_cert_path: PathBuf::from("/etc/predictor/certs/ca.crt"),
            client_cert_path: PathBuf::from("/etc/predictor/certs/client.crt"),
            client_key_path: PathBuf::from("/etc/predictor/certs/client.key"),
            connect_timeout: Duration::from_secs(10),
            request_timeout: Duration::from_secs(30),
            keepalive_interval: Duration::from_secs(30),
            keepalive_timeout: Duration::from_secs(10),
            initial_backoff: Duration::from_secs(1),
            max_backoff: Duration::from_secs(300), // 5 minutes
        }
    }
}

/// Connection state for tracking reconnection attempts
#[derive(Debug, Clone)]
struct ConnectionState {
    connected: bool,
    last_error: Option<String>,
    reconnect_attempts: u32,
    current_backoff: Duration,
}

impl Default for ConnectionState {
    fn default() -> Self {
        Self {
            connected: false,
            last_error: None,
            reconnect_attempts: 0,
            current_backoff: Duration::from_secs(1),
        }
    }
}

/// TLS configuration holder that can be refreshed
struct TlsState {
    config: ClientTlsConfig,
    cert_modified_time: std::time::SystemTime,
}

/// gRPC client for syncing with Recommendation API
pub struct SyncClient {
    config: ClientConfig,
    agent_id: String,
    node_name: String,
    channel: Arc<RwLock<Option<Channel>>>,
    connection_state: Arc<RwLock<ConnectionState>>,
    tls_state: Arc<RwLock<Option<TlsState>>>,
}

impl SyncClient {
    /// Create a new SyncClient with the given configuration
    pub fn new(config: ClientConfig, agent_id: String, node_name: String) -> Self {
        Self {
            config,
            agent_id,
            node_name,
            channel: Arc::new(RwLock::new(None)),
            connection_state: Arc::new(RwLock::new(ConnectionState::default())),
            tls_state: Arc::new(RwLock::new(None)),
        }
    }

    /// Create a new SyncClient with default configuration
    pub fn with_defaults(endpoint: String, agent_id: String, node_name: String) -> Self {
        let mut config = ClientConfig::default();
        config.endpoint = endpoint;
        Self::new(config, agent_id, node_name)
    }

    /// Get the endpoint URL
    pub fn endpoint(&self) -> &str {
        &self.config.endpoint
    }

    /// Get the agent ID
    pub fn agent_id(&self) -> &str {
        &self.agent_id
    }

    /// Get the node name
    pub fn node_name(&self) -> &str {
        &self.node_name
    }

    /// Get the connect timeout
    pub fn connect_timeout(&self) -> Duration {
        self.config.connect_timeout
    }

    /// Load TLS configuration from certificate files
    async fn load_tls_config(&self) -> Result<ClientTlsConfig> {
        // Read CA certificate
        let ca_cert = tokio::fs::read(&self.config.ca_cert_path)
            .await
            .with_context(|| {
                format!(
                    "Failed to read CA certificate from {:?}",
                    self.config.ca_cert_path
                )
            })?;
        let ca = Certificate::from_pem(ca_cert);

        // Read client certificate and key
        let client_cert = tokio::fs::read(&self.config.client_cert_path)
            .await
            .with_context(|| {
                format!(
                    "Failed to read client certificate from {:?}",
                    self.config.client_cert_path
                )
            })?;
        let client_key = tokio::fs::read(&self.config.client_key_path)
            .await
            .with_context(|| {
                format!(
                    "Failed to read client key from {:?}",
                    self.config.client_key_path
                )
            })?;
        let identity = Identity::from_pem(client_cert, client_key);

        // Build TLS config
        let tls_config = ClientTlsConfig::new()
            .ca_certificate(ca)
            .identity(identity)
            .domain_name(self.extract_domain()?);

        Ok(tls_config)
    }

    /// Extract domain name from endpoint URL
    fn extract_domain(&self) -> Result<String> {
        let url = url::Url::parse(&self.config.endpoint)
            .with_context(|| format!("Invalid endpoint URL: {}", self.config.endpoint))?;
        url.host_str()
            .map(|s| s.to_string())
            .ok_or_else(|| anyhow::anyhow!("No host in endpoint URL"))
    }

    /// Check if certificates have been rotated
    async fn check_cert_rotation(&self) -> Result<bool> {
        let metadata = tokio::fs::metadata(&self.config.client_cert_path).await?;
        let modified = metadata.modified()?;

        let tls_state = self.tls_state.read().await;
        if let Some(state) = tls_state.as_ref() {
            Ok(modified > state.cert_modified_time)
        } else {
            Ok(true) // No previous state, need to load
        }
    }

    /// Refresh TLS configuration if certificates have changed
    async fn refresh_tls_if_needed(&self) -> Result<bool> {
        if !self.check_cert_rotation().await? {
            return Ok(false);
        }

        info!("Certificate rotation detected, refreshing TLS configuration");

        let new_config = self.load_tls_config().await?;
        let modified_time = tokio::fs::metadata(&self.config.client_cert_path)
            .await?
            .modified()?;

        let mut tls_state = self.tls_state.write().await;
        *tls_state = Some(TlsState {
            config: new_config,
            cert_modified_time: modified_time,
        });

        // Force reconnection with new certificates
        let mut channel = self.channel.write().await;
        *channel = None;

        Ok(true)
    }

    /// Create a new gRPC channel with mTLS
    async fn create_channel(&self) -> Result<Channel> {
        // Ensure TLS config is loaded
        self.refresh_tls_if_needed().await?;

        let tls_state = self.tls_state.read().await;
        let tls_config = tls_state
            .as_ref()
            .map(|s| s.config.clone())
            .ok_or_else(|| anyhow::anyhow!("TLS configuration not loaded"))?;

        let channel = Channel::from_shared(self.config.endpoint.clone())?
            .tls_config(tls_config)?
            .connect_timeout(self.config.connect_timeout)
            .timeout(self.config.request_timeout)
            .http2_keep_alive_interval(self.config.keepalive_interval)
            .keep_alive_timeout(self.config.keepalive_timeout)
            .keep_alive_while_idle(true)
            .connect()
            .await
            .with_context(|| format!("Failed to connect to {}", self.config.endpoint))?;

        Ok(channel)
    }

    /// Get or create a connected channel
    async fn get_channel(&self) -> Result<Channel> {
        // Check for certificate rotation
        if self.check_cert_rotation().await.unwrap_or(false) {
            self.refresh_tls_if_needed().await?;
        }

        // Try to use existing channel
        {
            let channel = self.channel.read().await;
            if let Some(ch) = channel.as_ref() {
                return Ok(ch.clone());
            }
        }

        // Create new channel
        let new_channel = self.create_channel().await?;

        // Store and return
        let mut channel = self.channel.write().await;
        *channel = Some(new_channel.clone());

        // Update connection state
        let mut state = self.connection_state.write().await;
        state.connected = true;
        state.reconnect_attempts = 0;
        state.current_backoff = self.config.initial_backoff;
        state.last_error = None;

        info!(
            endpoint = %self.config.endpoint,
            "Connected to Recommendation API"
        );

        Ok(new_channel)
    }

    /// Handle connection failure with exponential backoff
    async fn handle_connection_failure(&self, error: &str) {
        let mut state = self.connection_state.write().await;
        state.connected = false;
        state.last_error = Some(error.to_string());
        state.reconnect_attempts += 1;

        // Calculate next backoff with exponential increase
        let next_backoff = std::cmp::min(
            state.current_backoff * 2,
            self.config.max_backoff,
        );
        state.current_backoff = next_backoff;

        // Clear the channel
        let mut channel = self.channel.write().await;
        *channel = None;

        warn!(
            error = %error,
            attempts = state.reconnect_attempts,
            next_backoff_secs = next_backoff.as_secs(),
            "Connection to Recommendation API failed"
        );
    }

    /// Get current backoff duration for reconnection
    pub async fn get_reconnect_backoff(&self) -> Duration {
        let state = self.connection_state.read().await;
        state.current_backoff
    }

    /// Check if client is currently connected
    pub async fn is_connected(&self) -> bool {
        let state = self.connection_state.read().await;
        state.connected
    }

    /// Get connection statistics
    pub async fn connection_stats(&self) -> (bool, u32, Option<String>) {
        let state = self.connection_state.read().await;
        (
            state.connected,
            state.reconnect_attempts,
            state.last_error.clone(),
        )
    }

    /// Register agent with the API
    pub async fn register(
        &self,
        kubernetes_version: &str,
        agent_version: &str,
        model_version: &str,
    ) -> Result<RegisterResponse> {
        let channel = match self.get_channel().await {
            Ok(ch) => ch,
            Err(e) => {
                self.handle_connection_failure(&e.to_string()).await;
                return Err(e);
            }
        };

        let mut client = PredictorSyncClient::new(channel);

        let request = tonic::Request::new(RegisterRequest {
            agent_id: self.agent_id.clone(),
            node_name: self.node_name.clone(),
            kubernetes_version: kubernetes_version.to_string(),
            agent_version: agent_version.to_string(),
            model_version: model_version.to_string(),
        });

        match client.register(request).await {
            Ok(response) => {
                debug!(
                    agent_id = %self.agent_id,
                    "Successfully registered with API"
                );
                Ok(response.into_inner())
            }
            Err(e) => {
                self.handle_connection_failure(&e.to_string()).await;
                Err(anyhow::anyhow!("Registration failed: {}", e))
            }
        }
    }

    /// Check for model updates
    pub async fn get_model_update(&self, current_version: &str) -> Result<Option<ModelResponse>> {
        let channel = match self.get_channel().await {
            Ok(ch) => ch,
            Err(e) => {
                self.handle_connection_failure(&e.to_string()).await;
                return Err(e);
            }
        };

        let mut client = PredictorSyncClient::new(channel);

        let request = tonic::Request::new(ModelRequest {
            agent_id: self.agent_id.clone(),
            current_model_version: current_version.to_string(),
        });

        match client.get_model_update(request).await {
            Ok(response) => {
                let model_response = response.into_inner();
                if model_response.update_available {
                    info!(
                        current_version = %current_version,
                        new_version = %model_response.new_version,
                        "Model update available"
                    );
                    Ok(Some(model_response))
                } else {
                    debug!("No model update available");
                    Ok(None)
                }
            }
            Err(e) => {
                self.handle_connection_failure(&e.to_string()).await;
                Err(anyhow::anyhow!("Model update check failed: {}", e))
            }
        }
    }

    /// Get a client for streaming operations
    pub async fn get_streaming_client(&self) -> Result<PredictorSyncClient<Channel>> {
        let channel = self.get_channel().await?;
        Ok(PredictorSyncClient::new(channel))
    }

    /// Force reconnection (useful after certificate rotation)
    pub async fn force_reconnect(&self) -> Result<()> {
        info!("Forcing reconnection to Recommendation API");

        // Clear existing channel
        {
            let mut channel = self.channel.write().await;
            *channel = None;
        }

        // Reset connection state
        {
            let mut state = self.connection_state.write().await;
            state.connected = false;
            state.current_backoff = self.config.initial_backoff;
        }

        // Attempt to reconnect
        self.get_channel().await?;
        Ok(())
    }

    /// Disconnect from the API
    pub async fn disconnect(&self) {
        let mut channel = self.channel.write().await;
        *channel = None;

        let mut state = self.connection_state.write().await;
        state.connected = false;

        info!("Disconnected from Recommendation API");
    }
}

/// Builder for SyncClient configuration
pub struct SyncClientBuilder {
    config: ClientConfig,
    agent_id: Option<String>,
    node_name: Option<String>,
}

impl SyncClientBuilder {
    pub fn new() -> Self {
        Self {
            config: ClientConfig::default(),
            agent_id: None,
            node_name: None,
        }
    }

    pub fn endpoint(mut self, endpoint: impl Into<String>) -> Self {
        self.config.endpoint = endpoint.into();
        self
    }

    pub fn ca_cert_path(mut self, path: impl Into<PathBuf>) -> Self {
        self.config.ca_cert_path = path.into();
        self
    }

    pub fn client_cert_path(mut self, path: impl Into<PathBuf>) -> Self {
        self.config.client_cert_path = path.into();
        self
    }

    pub fn client_key_path(mut self, path: impl Into<PathBuf>) -> Self {
        self.config.client_key_path = path.into();
        self
    }

    pub fn connect_timeout(mut self, timeout: Duration) -> Self {
        self.config.connect_timeout = timeout;
        self
    }

    pub fn request_timeout(mut self, timeout: Duration) -> Self {
        self.config.request_timeout = timeout;
        self
    }

    pub fn keepalive_interval(mut self, interval: Duration) -> Self {
        self.config.keepalive_interval = interval;
        self
    }

    pub fn keepalive_timeout(mut self, timeout: Duration) -> Self {
        self.config.keepalive_timeout = timeout;
        self
    }

    pub fn initial_backoff(mut self, backoff: Duration) -> Self {
        self.config.initial_backoff = backoff;
        self
    }

    pub fn max_backoff(mut self, backoff: Duration) -> Self {
        self.config.max_backoff = backoff;
        self
    }

    pub fn agent_id(mut self, id: impl Into<String>) -> Self {
        self.agent_id = Some(id.into());
        self
    }

    pub fn node_name(mut self, name: impl Into<String>) -> Self {
        self.node_name = Some(name.into());
        self
    }

    pub fn build(self) -> Result<SyncClient> {
        let agent_id = self
            .agent_id
            .ok_or_else(|| anyhow::anyhow!("agent_id is required"))?;
        let node_name = self
            .node_name
            .ok_or_else(|| anyhow::anyhow!("node_name is required"))?;

        Ok(SyncClient::new(self.config, agent_id, node_name))
    }
}

impl Default for SyncClientBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_client_config_default() {
        let config = ClientConfig::default();
        assert_eq!(config.connect_timeout, Duration::from_secs(10));
        assert_eq!(config.max_backoff, Duration::from_secs(300));
    }

    #[test]
    fn test_builder_pattern() {
        let client = SyncClientBuilder::new()
            .endpoint("https://test:8443")
            .agent_id("test-agent")
            .node_name("test-node")
            .connect_timeout(Duration::from_secs(5))
            .build()
            .unwrap();

        assert_eq!(client.config.endpoint, "https://test:8443");
        assert_eq!(client.agent_id, "test-agent");
        assert_eq!(client.node_name, "test-node");
        assert_eq!(client.config.connect_timeout, Duration::from_secs(5));
    }

    #[test]
    fn test_builder_missing_agent_id() {
        let result = SyncClientBuilder::new()
            .endpoint("https://test:8443")
            .node_name("test-node")
            .build();

        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_connection_state_default() {
        let client = SyncClientBuilder::new()
            .endpoint("https://test:8443")
            .agent_id("test-agent")
            .node_name("test-node")
            .build()
            .unwrap();

        assert!(!client.is_connected().await);
        let (connected, attempts, error) = client.connection_stats().await;
        assert!(!connected);
        assert_eq!(attempts, 0);
        assert!(error.is_none());
    }
}
