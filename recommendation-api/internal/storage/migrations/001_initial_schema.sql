-- Migration: 001_initial_schema
-- Description: Initial database schema for Container Resource Predictor

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Agents table
CREATE TABLE IF NOT EXISTS agents (
    agent_id TEXT PRIMARY KEY,
    node_name TEXT NOT NULL,
    kubernetes_version TEXT,
    agent_version TEXT,
    model_version TEXT,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_node_name ON agents(node_name);
CREATE INDEX idx_agents_last_seen ON agents(last_seen_at);

-- Container metrics hypertable (time-series optimized)
CREATE TABLE IF NOT EXISTS container_metrics (
    time TIMESTAMPTZ NOT NULL,
    container_id TEXT NOT NULL,
    pod_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    deployment TEXT,
    node_name TEXT NOT NULL,
    cpu_usage_cores REAL,
    cpu_throttled_periods BIGINT,
    cpu_throttled_time_ns BIGINT,
    memory_usage_bytes BIGINT,
    memory_working_set_bytes BIGINT,
    memory_cache_bytes BIGINT,
    memory_rss_bytes BIGINT,
    network_rx_bytes BIGINT,
    network_tx_bytes BIGINT
);

-- Convert to hypertable
SELECT create_hypertable('container_metrics', 'time', if_not_exists => TRUE);

-- Create indexes for common queries
CREATE INDEX idx_metrics_container ON container_metrics(container_id, time DESC);
CREATE INDEX idx_metrics_namespace ON container_metrics(namespace, time DESC);
CREATE INDEX idx_metrics_deployment ON container_metrics(deployment, time DESC) WHERE deployment IS NOT NULL;

-- Set retention policy (30 days by default)
SELECT add_retention_policy('container_metrics', INTERVAL '30 days', if_not_exists => TRUE);

-- Predictions table
CREATE TABLE IF NOT EXISTS predictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    container_id TEXT NOT NULL,
    pod_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    deployment TEXT,
    cpu_request_millicores INT NOT NULL,
    cpu_limit_millicores INT NOT NULL,
    memory_request_bytes BIGINT NOT NULL,
    memory_limit_bytes BIGINT NOT NULL,
    confidence REAL NOT NULL,
    model_version TEXT NOT NULL,
    time_window TEXT NOT NULL DEFAULT 'peak'
);

CREATE INDEX idx_predictions_namespace ON predictions(namespace, created_at DESC);
CREATE INDEX idx_predictions_deployment ON predictions(deployment, created_at DESC) WHERE deployment IS NOT NULL;
