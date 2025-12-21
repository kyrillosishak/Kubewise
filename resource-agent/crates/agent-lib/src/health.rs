//! Health check infrastructure for the resource agent
//!
//! Provides component health tracking and status reporting for
//! Kubernetes liveness and readiness probes.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Health status of a component
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ComponentStatus {
    /// Component is functioning normally
    Healthy,
    /// Component is experiencing issues but still operational
    Degraded,
    /// Component has failed
    Unhealthy,
}

impl ComponentStatus {
    /// Returns true if the component is at least partially operational
    pub fn is_operational(&self) -> bool {
        matches!(self, ComponentStatus::Healthy | ComponentStatus::Degraded)
    }
}

/// Information about a component's health
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComponentHealth {
    pub status: ComponentStatus,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    pub last_check_timestamp: i64,
}

impl ComponentHealth {
    pub fn healthy() -> Self {
        Self {
            status: ComponentStatus::Healthy,
            message: None,
            last_check_timestamp: chrono::Utc::now().timestamp(),
        }
    }

    pub fn degraded(message: impl Into<String>) -> Self {
        Self {
            status: ComponentStatus::Degraded,
            message: Some(message.into()),
            last_check_timestamp: chrono::Utc::now().timestamp(),
        }
    }

    pub fn unhealthy(message: impl Into<String>) -> Self {
        Self {
            status: ComponentStatus::Unhealthy,
            message: Some(message.into()),
            last_check_timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

/// Overall health response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthResponse {
    pub status: ComponentStatus,
    pub components: HashMap<String, ComponentHealth>,
}

impl HealthResponse {
    /// Compute overall status from component statuses
    pub fn compute_status(components: &HashMap<String, ComponentHealth>) -> ComponentStatus {
        let mut has_degraded = false;
        
        for health in components.values() {
            match health.status {
                ComponentStatus::Unhealthy => return ComponentStatus::Unhealthy,
                ComponentStatus::Degraded => has_degraded = true,
                ComponentStatus::Healthy => {}
            }
        }
        
        if has_degraded {
            ComponentStatus::Degraded
        } else {
            ComponentStatus::Healthy
        }
    }
}

/// Readiness response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReadinessResponse {
    pub ready: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Component names for health tracking
pub mod components {
    pub const COLLECTOR: &str = "collector";
    pub const PREDICTOR: &str = "predictor";
    pub const SYNC_CLIENT: &str = "sync_client";
    pub const BUFFER: &str = "buffer";
}

/// Health registry for tracking component health
#[derive(Debug, Clone)]
pub struct HealthRegistry {
    components: Arc<RwLock<HashMap<String, ComponentHealth>>>,
    ready: Arc<RwLock<bool>>,
}

impl Default for HealthRegistry {
    fn default() -> Self {
        Self::new()
    }
}

impl HealthRegistry {
    pub fn new() -> Self {
        Self {
            components: Arc::new(RwLock::new(HashMap::new())),
            ready: Arc::new(RwLock::new(false)),
        }
    }

    /// Register a component with initial healthy status
    pub async fn register(&self, name: &str) {
        let mut components = self.components.write().await;
        components.insert(name.to_string(), ComponentHealth::healthy());
    }

    /// Update component health status
    pub async fn update(&self, name: &str, health: ComponentHealth) {
        let mut components = self.components.write().await;
        components.insert(name.to_string(), health);
    }

    /// Mark component as healthy
    pub async fn set_healthy(&self, name: &str) {
        self.update(name, ComponentHealth::healthy()).await;
    }

    /// Mark component as degraded
    pub async fn set_degraded(&self, name: &str, message: impl Into<String>) {
        self.update(name, ComponentHealth::degraded(message)).await;
    }

    /// Mark component as unhealthy
    pub async fn set_unhealthy(&self, name: &str, message: impl Into<String>) {
        self.update(name, ComponentHealth::unhealthy(message)).await;
    }

    /// Set readiness status
    pub async fn set_ready(&self, ready: bool) {
        let mut r = self.ready.write().await;
        *r = ready;
    }

    /// Get health response
    pub async fn health(&self) -> HealthResponse {
        let components = self.components.read().await.clone();
        let status = HealthResponse::compute_status(&components);
        HealthResponse { status, components }
    }

    /// Get readiness response
    pub async fn readiness(&self) -> ReadinessResponse {
        let ready = *self.ready.read().await;
        let health = self.health().await;
        
        // Not ready if any critical component is unhealthy
        let critical_healthy = health.status != ComponentStatus::Unhealthy;
        
        if !ready {
            ReadinessResponse {
                ready: false,
                reason: Some("Agent not yet initialized".to_string()),
            }
        } else if !critical_healthy {
            ReadinessResponse {
                ready: false,
                reason: Some("Critical component unhealthy".to_string()),
            }
        } else {
            ReadinessResponse {
                ready: true,
                reason: None,
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_health_registry_initial_state() {
        let registry = HealthRegistry::new();
        let health = registry.health().await;
        
        assert_eq!(health.status, ComponentStatus::Healthy);
        assert!(health.components.is_empty());
    }

    #[tokio::test]
    async fn test_health_registry_component_registration() {
        let registry = HealthRegistry::new();
        registry.register(components::COLLECTOR).await;
        
        let health = registry.health().await;
        assert!(health.components.contains_key(components::COLLECTOR));
        assert_eq!(
            health.components[components::COLLECTOR].status,
            ComponentStatus::Healthy
        );
    }

    #[tokio::test]
    async fn test_health_registry_degraded_status() {
        let registry = HealthRegistry::new();
        registry.register(components::COLLECTOR).await;
        registry.register(components::PREDICTOR).await;
        
        registry.set_degraded(components::COLLECTOR, "High latency").await;
        
        let health = registry.health().await;
        assert_eq!(health.status, ComponentStatus::Degraded);
    }

    #[tokio::test]
    async fn test_health_registry_unhealthy_status() {
        let registry = HealthRegistry::new();
        registry.register(components::COLLECTOR).await;
        registry.register(components::PREDICTOR).await;
        
        registry.set_unhealthy(components::COLLECTOR, "Failed to read cgroups").await;
        
        let health = registry.health().await;
        assert_eq!(health.status, ComponentStatus::Unhealthy);
    }

    #[tokio::test]
    async fn test_readiness_not_ready_initially() {
        let registry = HealthRegistry::new();
        let readiness = registry.readiness().await;
        
        assert!(!readiness.ready);
        assert!(readiness.reason.is_some());
    }

    #[tokio::test]
    async fn test_readiness_ready_when_set() {
        let registry = HealthRegistry::new();
        registry.set_ready(true).await;
        
        let readiness = registry.readiness().await;
        assert!(readiness.ready);
    }

    #[tokio::test]
    async fn test_readiness_not_ready_when_unhealthy() {
        let registry = HealthRegistry::new();
        registry.register(components::COLLECTOR).await;
        registry.set_ready(true).await;
        registry.set_unhealthy(components::COLLECTOR, "Failed").await;
        
        let readiness = registry.readiness().await;
        assert!(!readiness.ready);
    }
}
