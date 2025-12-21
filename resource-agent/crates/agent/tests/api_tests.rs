//! Integration tests for the agent API endpoints

use agent_lib::{
    health::{components, ComponentStatus, HealthRegistry},
    observability::AgentMetrics,
};
use axum::{
    body::Body,
    extract::State,
    http::{Request, StatusCode},
    response::IntoResponse,
    routing::get,
    Json, Router,
};
use prometheus::{Encoder, TextEncoder};
use std::sync::Arc;
use tower::ServiceExt;

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

async fn healthz(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let health = state.health_registry.health().await;
    let status_code = match health.status {
        ComponentStatus::Healthy => StatusCode::OK,
        ComponentStatus::Degraded => StatusCode::OK,
        ComponentStatus::Unhealthy => StatusCode::SERVICE_UNAVAILABLE,
    };
    (status_code, Json(health))
}

async fn readyz(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let readiness = state.health_registry.readiness().await;
    let status_code = if readiness.ready {
        StatusCode::OK
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    };
    (status_code, Json(readiness))
}

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

fn create_test_router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/readyz", get(readyz))
        .route("/metrics", get(metrics))
        .with_state(state)
}

async fn setup_test_app() -> (Router, Arc<AppState>) {
    let health_registry = HealthRegistry::new();
    health_registry.register(components::COLLECTOR).await;
    health_registry.register(components::PREDICTOR).await;

    let metrics = AgentMetrics::new();
    let state = Arc::new(AppState::new(health_registry, metrics));
    let router = create_test_router(state.clone());

    (router, state)
}

#[tokio::test]
async fn test_healthz_returns_ok_when_healthy() {
    let (app, _state) = setup_test_app().await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::OK);

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let health: serde_json::Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(health["status"], "healthy");
}

#[tokio::test]
async fn test_healthz_returns_ok_when_degraded() {
    let (app, state) = setup_test_app().await;

    // Set a component to degraded
    state
        .health_registry
        .set_degraded(components::COLLECTOR, "High latency")
        .await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    // Degraded still returns 200 (operational)
    assert_eq!(response.status(), StatusCode::OK);

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let health: serde_json::Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(health["status"], "degraded");
}

#[tokio::test]
async fn test_healthz_returns_503_when_unhealthy() {
    let (app, state) = setup_test_app().await;

    // Set a component to unhealthy
    state
        .health_registry
        .set_unhealthy(components::COLLECTOR, "Failed to read cgroups")
        .await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let health: serde_json::Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(health["status"], "unhealthy");
}

#[tokio::test]
async fn test_readyz_returns_503_when_not_ready() {
    let (app, _state) = setup_test_app().await;

    // By default, agent is not ready
    let response = app
        .oneshot(
            Request::builder()
                .uri("/readyz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let readiness: serde_json::Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(readiness["ready"], false);
}

#[tokio::test]
async fn test_readyz_returns_ok_when_ready() {
    let (app, state) = setup_test_app().await;

    // Mark as ready
    state.health_registry.set_ready(true).await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/readyz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::OK);

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let readiness: serde_json::Value = serde_json::from_slice(&body).unwrap();

    assert_eq!(readiness["ready"], true);
}

#[tokio::test]
async fn test_readyz_returns_503_when_ready_but_unhealthy() {
    let (app, state) = setup_test_app().await;

    // Mark as ready but set component unhealthy
    state.health_registry.set_ready(true).await;
    state
        .health_registry
        .set_unhealthy(components::COLLECTOR, "Failed")
        .await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/readyz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);
}

#[tokio::test]
async fn test_metrics_endpoint_returns_prometheus_format() {
    let (app, state) = setup_test_app().await;

    // Record some metrics
    state.metrics.observe_collection_latency(0.001);
    state.metrics.observe_prediction_latency(0.002);
    state.metrics.set_buffer_size(1024, 10);
    state.metrics.set_model_version("v1.0.0", "int8");

    let response = app
        .oneshot(
            Request::builder()
                .uri("/metrics")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), StatusCode::OK);

    let content_type = response.headers().get("content-type").unwrap();
    assert!(content_type.to_str().unwrap().contains("text/plain"));

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let metrics_text = String::from_utf8(body.to_vec()).unwrap();

    // Verify expected metrics are present
    assert!(metrics_text.contains("resource_agent_collection_latency_seconds"));
    assert!(metrics_text.contains("resource_agent_prediction_latency_seconds"));
    assert!(metrics_text.contains("resource_agent_buffer_size_bytes"));
    assert!(metrics_text.contains("resource_agent_model_version_info"));
}

#[tokio::test]
async fn test_metrics_contains_histogram_buckets() {
    let (app, state) = setup_test_app().await;

    // Record some latency observations
    state.metrics.observe_collection_latency(0.001);
    state.metrics.observe_collection_latency(0.005);
    state.metrics.observe_collection_latency(0.01);

    let response = app
        .oneshot(
            Request::builder()
                .uri("/metrics")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let metrics_text = String::from_utf8(body.to_vec()).unwrap();

    // Verify histogram has bucket labels
    assert!(metrics_text.contains("resource_agent_collection_latency_seconds_bucket"));
    assert!(metrics_text.contains("resource_agent_collection_latency_seconds_count"));
    assert!(metrics_text.contains("resource_agent_collection_latency_seconds_sum"));
}

#[tokio::test]
async fn test_healthz_includes_component_details() {
    let (app, _state) = setup_test_app().await;

    let response = app
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    let body = axum::body::to_bytes(response.into_body(), usize::MAX)
        .await
        .unwrap();
    let health: serde_json::Value = serde_json::from_slice(&body).unwrap();

    // Verify components are included
    assert!(health["components"].is_object());
    assert!(health["components"]["collector"].is_object());
    assert!(health["components"]["predictor"].is_object());
}
