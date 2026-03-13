-- Secret Versions (keeps N previous values for each secret)
CREATE TABLE secret_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_id UUID NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    encrypted_value BYTEA NOT NULL,
    value_nonce BYTEA NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(secret_id, version)
);

CREATE INDEX idx_secret_versions_secret ON secret_versions(secret_id, version DESC);
CREATE INDEX idx_secret_versions_project_env ON secret_versions(project_id, environment_id);

-- Webhook configurations
CREATE TABLE webhook_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret VARCHAR(255) DEFAULT '',
    events TEXT[] NOT NULL DEFAULT '{"*"}',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_configs_project ON webhook_configs(project_id);

-- Webhook delivery log
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhook_configs(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status_code INTEGER,
    success BOOLEAN NOT NULL DEFAULT false,
    error_message TEXT DEFAULT '',
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, delivered_at DESC);
