-- Migration: 005_safety_features
-- Description: Safety and rollout features - dry-run mode, approval workflow, outcome tracking

-- Add dry-run configuration per namespace
CREATE TABLE IF NOT EXISTS namespace_config (
    namespace TEXT PRIMARY KEY,
    dry_run_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    auto_approve_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    high_risk_threshold_memory_reduction REAL NOT NULL DEFAULT 0.30, -- 30% reduction is high risk
    high_risk_threshold_cpu_reduction REAL NOT NULL DEFAULT 0.50,    -- 50% reduction is high risk
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add approval tracking columns to recommendations
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS requires_approval BOOLEAN DEFAULT FALSE;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS is_high_risk BOOLEAN DEFAULT FALSE;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS risk_reason TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS dry_run_result JSONB;

-- Approval history table
CREATE TABLE IF NOT EXISTS approval_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id UUID NOT NULL REFERENCES recommendations(id),
    action TEXT NOT NULL, -- 'approved', 'rejected', 'auto_approved'
    approver TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approval_history_recommendation ON approval_history(recommendation_id, created_at DESC);
CREATE INDEX idx_approval_history_approver ON approval_history(approver, created_at DESC);

-- Outcome tracking table
CREATE TABLE IF NOT EXISTS recommendation_outcomes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recommendation_id UUID NOT NULL REFERENCES recommendations(id),
    namespace TEXT NOT NULL,
    deployment TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL,
    check_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    oom_kills_before INT NOT NULL DEFAULT 0,
    oom_kills_after INT NOT NULL DEFAULT 0,
    cpu_throttle_before REAL NOT NULL DEFAULT 0,
    cpu_throttle_after REAL NOT NULL DEFAULT 0,
    memory_usage_p95_before BIGINT,
    memory_usage_p95_after BIGINT,
    cpu_usage_p95_before REAL,
    cpu_usage_p95_after REAL,
    outcome_status TEXT NOT NULL DEFAULT 'monitoring', -- 'monitoring', 'success', 'degraded', 'rolled_back'
    rollback_triggered BOOLEAN NOT NULL DEFAULT FALSE,
    rollback_recommendation_id UUID REFERENCES recommendations(id)
);

CREATE INDEX idx_outcomes_recommendation ON recommendation_outcomes(recommendation_id);
CREATE INDEX idx_outcomes_namespace ON recommendation_outcomes(namespace, check_time DESC);
CREATE INDEX idx_outcomes_status ON recommendation_outcomes(outcome_status) WHERE outcome_status != 'success';

-- Rollback events table
CREATE TABLE IF NOT EXISTS rollback_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_recommendation_id UUID NOT NULL REFERENCES recommendations(id),
    rollback_recommendation_id UUID REFERENCES recommendations(id),
    trigger_reason TEXT NOT NULL, -- 'oom_increase', 'throttle_increase', 'manual'
    oom_kills_detected INT,
    throttle_increase_percent REAL,
    auto_triggered BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alert_sent BOOLEAN NOT NULL DEFAULT FALSE,
    alert_sent_at TIMESTAMPTZ
);

CREATE INDEX idx_rollback_events_original ON rollback_events(original_recommendation_id);
CREATE INDEX idx_rollback_events_created ON rollback_events(created_at DESC);

-- Insert default global config
INSERT INTO namespace_config (namespace, dry_run_enabled, auto_approve_enabled)
VALUES ('_global', FALSE, FALSE)
ON CONFLICT (namespace) DO NOTHING;
