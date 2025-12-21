-- Migration: 003_savings_tracking
-- Description: Savings tracking and pricing configuration tables

-- Savings records table for tracking actual savings after recommendation application
CREATE TABLE IF NOT EXISTS savings_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id UUID NOT NULL REFERENCES recommendations(id),
    namespace TEXT NOT NULL,
    deployment TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL,
    cost_before_monthly DECIMAL(10,2) NOT NULL,
    cost_after_monthly DECIMAL(10,2) NOT NULL,
    actual_savings DECIMAL(10,2) NOT NULL,
    measured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    measurement_period_days INT NOT NULL DEFAULT 0,
    UNIQUE(recommendation_id)
);

CREATE INDEX idx_savings_records_namespace ON savings_records(namespace, measured_at DESC);
CREATE INDEX idx_savings_records_deployment ON savings_records(deployment);

-- Pricing configurations table
CREATE TABLE IF NOT EXISTS pricing_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL,
    region TEXT,
    currency TEXT NOT NULL DEFAULT 'USD',
    cpu_price_per_core_hour DECIMAL(10,6) NOT NULL,
    memory_price_per_gb_hour DECIMAL(10,6) NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pricing_configs_default ON pricing_configs(is_default) WHERE is_default = TRUE;

-- Custom namespace rates table
CREATE TABLE IF NOT EXISTS namespace_pricing (
    namespace TEXT PRIMARY KEY,
    pricing_config_id UUID REFERENCES pricing_configs(id),
    cpu_price_override DECIMAL(10,6),
    memory_price_override DECIMAL(10,6),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default pricing configurations
INSERT INTO pricing_configs (name, provider, region, currency, cpu_price_per_core_hour, memory_price_per_gb_hour, is_default)
VALUES 
    ('aws-us-east-1', 'aws', 'us-east-1', 'USD', 0.0425, 0.00533, TRUE),
    ('gcp-us-central1', 'gcp', 'us-central1', 'USD', 0.0335, 0.00449, FALSE),
    ('azure-eastus', 'azure', 'eastus', 'USD', 0.0420, 0.00525, FALSE),
    ('on-premise-default', 'on_premise', 'default', 'USD', 0.030, 0.004, FALSE)
ON CONFLICT (name) DO NOTHING;

-- Add unique constraint to cost_snapshots for upsert support
ALTER TABLE cost_snapshots DROP CONSTRAINT IF EXISTS cost_snapshots_time_namespace_key;
ALTER TABLE cost_snapshots ADD CONSTRAINT cost_snapshots_time_namespace_key UNIQUE (time, namespace);
