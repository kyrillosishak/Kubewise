//! HTTP API for health checks and Prometheus metrics

use agent_lib::{
    health::{ComponentStatus, HealthRegistry},
    observability::AgentMetrics,
};
use axum::{
    extract::State,
    http::StatusCode,
    response::IntoResponse,
    routing::get,
    Json, Router,
};
use prometheus::{Encoder, TextEncoder};
use std::sync::Arc;
use tracing::info;

/// Shared application state
#[derive(Clone)]
pub struct AppState {
    pub health_registry: HealthRegistry,
    pub metrics: AgentMetrics,
}

impl AppState {
    pub fn new(health_registry: HealthRegistry, metrics: AgentMetrics) -> Self {
        Self {
            health_registry,
            metrics,
        }
    }
}

/// Health check response - returns 200 if healthy, 503 if degraded/unhealthy
async fn healthz(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let health = state.health_registry.health().await;

    let status_code = match health.status {
        ComponentStatus::Healthy => StatusCode::OK,
        ComponentStatus::Degraded => StatusCode::OK, // Still operational
        ComponentStatus::Unhealthy => StatusCode::SERVICE_UNAVAILABLE,
    };

    (status_code, Json(health))
}

/// Readiness check response - returns 200 if ready, 503 if not ready
async fn readyz(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let readiness = state.health_registry.readiness().await;

    let status_code = if readiness.ready {
        StatusCode::OK
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    };

    (status_code, Json(readiness))
}

/// Prometheus metrics endpoint
async fn metrics() -> impl IntoResponse {
    let encoder = TextEncoder::new();
    let metric_families = prometheus::gather();
    let mut buffer = Vec::new();

    encoder.encode(&metric_families, &mut buffer).unwrap();

    (
        StatusCode::OK,
        [("content-type", "text/plain; charset=utf-8")],
        buffer,
    )
}

/// Create the API router
pub fn create_router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/readyz", get(readyz))
        .route("/metrics", get(metrics))
        .with_state(state)
}

/// Start the API server
pub async fn serve(port: u16, state: Arc<AppState>) -> anyhow::Result<()> {
    let app = create_router(state);

    let addr = format!("0.0.0.0:{}", port);
    info!(addr = %addr, "Starting API server");

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
