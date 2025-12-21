//! Resource Agent - Container resource prediction agent
//!
//! This binary runs as a DaemonSet on each Kubernetes node,
//! collecting metrics and running local ML inference.

use agent_lib::{
    health::{components, HealthRegistry},
    observability::{AgentMetrics, StructuredLogger},
};
use anyhow::Result;
use std::sync::Arc;
use tracing::info;
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

mod api;
mod config;

const AGENT_VERSION: &str = env!("CARGO_PKG_VERSION");

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing with JSON output and env filter
    tracing_subscriber::registry()
        .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        .with(fmt::layer().json())
        .init();

    info!("Starting resource-agent");

    // Load configuration
    let config = config::AgentConfig::load()?;
    info!(node_name = %config.node_name, "Agent configured");

    // Initialize health registry
    let health_registry = HealthRegistry::new();
    health_registry.register(components::COLLECTOR).await;
    health_registry.register(components::PREDICTOR).await;
    health_registry.register(components::SYNC_CLIENT).await;
    health_registry.register(components::BUFFER).await;

    // Initialize metrics
    let metrics = AgentMetrics::new();
    metrics.set_model_version("v0.1.0", "int8");

    // Initialize structured logger
    let logger = StructuredLogger::new(&config.node_name);
    logger.log_startup(AGENT_VERSION, "v0.1.0");

    // Create shared application state
    let app_state = Arc::new(api::AppState::new(health_registry.clone(), metrics.clone()));

    // Mark agent as ready after initialization
    health_registry.set_ready(true).await;

    // Start health and metrics server
    let api_handle = tokio::spawn(api::serve(config.api_port, app_state));

    // Wait for shutdown signal
    tokio::signal::ctrl_c().await?;
    logger.log_shutdown("SIGINT received");
    info!("Shutting down");

    Ok(())
}
