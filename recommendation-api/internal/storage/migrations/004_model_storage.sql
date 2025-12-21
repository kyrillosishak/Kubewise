-- Migration: 004_model_storage
-- Description: Enhanced model storage with S3/MinIO support and metadata

-- Add additional columns to model_versions for enhanced metadata
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS training_samples BIGINT DEFAULT 0;
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS training_duration_seconds INT DEFAULT 0;
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS checksum TEXT;
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS storage_backend TEXT DEFAULT 'local';
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS storage_path TEXT;
ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- Model deployment tracking table
CREATE TABLE IF NOT EXISTS model_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_version TEXT NOT NULL REFERENCES model_versions(version),
    agent_id TEXT NOT NULL REFERENCES agents(agent_id),
    deployed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'pending', -- pending, deployed, failed, rolled_back
    validation_passed BOOLEAN,
    validation_error TEXT,
    previous_version TEXT,
    UNIQUE(model_version, agent_id)
);

CREATE INDEX idx_model_deployments_agent ON model_deployments(agent_id, deployed_at DESC);
CREATE INDEX idx_model_deployments_version ON model_deployments(model_version);
CREATE INDEX idx_model_deployments_status ON model_deployments(status);

-- Model validation results table
CREATE TABLE IF NOT EXISTS model_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_version TEXT NOT NULL REFERENCES model_versions(version),
    agent_id TEXT REFERENCES agents(agent_id),
    validated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accuracy REAL NOT NULL,
    precision_score REAL,
    recall_score REAL,
    f1_score REAL,
    sample_count BIGINT NOT NULL,
    validation_type TEXT NOT NULL DEFAULT 'holdout', -- holdout, cross_validation, live
    passed BOOLEAN NOT NULL,
    details JSONB DEFAULT '{}'
);

CREATE INDEX idx_model_validations_version ON model_validations(model_version, validated_at DESC);
CREATE INDEX idx_model_validations_agent ON model_validations(agent_id) WHERE agent_id IS NOT NULL;

-- Model rollback history table
CREATE TABLE IF NOT EXISTS model_rollbacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_version TEXT,
    to_version TEXT NOT NULL,
    reason TEXT,
    rolled_back_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_model_rollbacks_time ON model_rollbacks(rolled_back_at DESC);

-- Update model_versions to track storage location
UPDATE model_versions 
SET storage_path = weights_path, storage_backend = 'local' 
WHERE storage_path IS NULL;
