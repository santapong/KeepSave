-- Phase 15 gap: Usage quotas table

CREATE TABLE IF NOT EXISTS usage_quotas (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL UNIQUE,
    max_secrets INTEGER NOT NULL DEFAULT 1000,
    max_projects INTEGER NOT NULL DEFAULT 50,
    max_api_keys INTEGER NOT NULL DEFAULT 100,
    max_requests_per_day INTEGER NOT NULL DEFAULT 100000,
    current_secrets INTEGER NOT NULL DEFAULT 0,
    current_projects INTEGER NOT NULL DEFAULT 0,
    current_api_keys INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_quotas_org ON usage_quotas(organization_id);
