//! Agent library for container resource prediction
//!
//! This crate provides the core functionality for:
//! - Metrics collection from cgroups
//! - ML-based resource prediction
//! - Anomaly detection
//! - API synchronization
//! - Health checks and observability

pub mod anomaly;
pub mod collector;
pub mod health;
pub mod models;
pub mod observability;
pub mod predictor;
pub mod proto;
pub mod sync;

pub use health::{
    ComponentHealth, ComponentStatus, HealthRegistry, HealthResponse, ReadinessResponse,
};
pub use models::*;
pub use observability::{AgentMetrics, StructuredLogger};
