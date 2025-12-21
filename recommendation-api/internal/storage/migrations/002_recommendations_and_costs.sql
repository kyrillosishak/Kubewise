-- Migration: 002_recommendations_and_costs
-- Description: Recommendations, costs, and model management tables

-- Recommendations table (aggregated from predictions)
CREATE TABLE IF NOT EXISTS recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    namespace TEXT NOT NULL,
    deployment TEXT NOT NULL,
    cpu_request_millicores INT NOT NULL,
    cpu_limit_millicores INT NOT NULL,
    memory_request_bytes BIGINT NOT NULL,
    memory_limit_bytes BIGINT NOT NULL,
    confidence REAL NOT NULL,
    model_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, applied, rolled_back
    time_window TEXT NOT NULL DEFAULT 'peak',
    applied_at TIMESTAMPTZ,
    approved_at TIMESTAMPTZ,
    approved_by TEXT,
    outcome_oom_kills INT DEFAULT 0,
    outcome_throttle_increase REAL,
    current_cpu_request_millicores INT,
    current_cpu_limit_millicores INT,
    current_memory_request_bytes BIGINT,
    current_memory_limit_bytes BIGINT,
    UNIQUE(namespace, deployment, time_window)
);

CREATE INDEX idx_recommendations_namespace ON recommendations(namespace, created_at DESC);
CREATE INDEX idx_recommendations_status ON recommendations(status);
CREATE INDEX idx_recommendations_deployment ON recommendations(deployment);

-- Cost snapshots hypertable
CREATE TABLE IF NOT EXISTS cost_snapshots (
    time TIMESTAMPTZ NOT NULL,
    namespace TEXT NOT NULL,
    current_cost_monthly DECIMAL(10,2) NOT NULL,
    recommended_cost_monthly DECIMAL(10,2) NOT NULL,
    savings_monthly DECIMAL(10,2) NOT NULL,
    deployment_count INT NOT NULL DEFAULT 0
);

SELECT create_hypertable('cost_snapshots', 'time', if_not_exists => TRUE);

CREATE INDEX idx_cost_snapshots_namespace ON cost_snapshots(namespace, time DESC);

-- Set retention policy for cost snapshots (1 year)
SELECT add_retention_policy('cost_snapshots', INTERVAL '365 days', if_not_exists => TRUE);

-- Model versions table
CREATE TABLE IF NOT EXISTS model_versions (
    version TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    weights_path TEXT NOT NULL,
    validation_accuracy REAL NOT NULL,
    size_bytes BIGINT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    rollback_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_model_versions_active ON model_versions(is_active) WHERE is_active = TRUE;

-- Model gradients table (for federated learning)
CREATE TABLE IF NOT EXISTS model_gradients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    agent_id TEXT NOT NULL REFERENCES agents(agent_id),
    model_version TEXT NOT NULL REFERENCES model_versions(version),
    gradients BYTEA NOT NULL,
    sample_count BIGINT NOT NULL,
    aggregated BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_gradients_model ON model_gradients(model_version, aggregated);
CREATE INDEX idx_gradients_agent ON model_gradients(agent_id, created_at DESC);

-- Anomalies table
CREATE TABLE IF NOT EXISTS anomalies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    container_id TEXT NOT NULL,
    pod_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    anomaly_type TEXT NOT NULL, -- memory_leak, cpu_spike, oom_risk
    severity TEXT NOT NULL,     -- warning, critical
    message TEXT,
    details JSONB,
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by TEXT
);

CREATE INDEX idx_anomalies_namespace ON anomalies(namespace, detected_at DESC);
CREATE INDEX idx_anomalies_type ON anomalies(anomaly_type, detected_at DESC);
CREATE INDEX idx_anomalies_severity ON anomalies(severity) WHERE acknowledged = FALSE;
