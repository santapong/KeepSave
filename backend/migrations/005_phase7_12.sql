-- Phase 7-12: Observability, Enterprise, Security, Agent, Platform features

-- Phase 9: SSO provider configurations
CREATE TABLE IF NOT EXISTS sso_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- oidc, saml
    issuer_url VARCHAR(500) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    client_secret_encrypted BYTEA NOT NULL,
    client_secret_nonce BYTEA NOT NULL,
    metadata JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (organization_id, provider)
);

-- Phase 9: IP allowlists
CREATE TABLE IF NOT EXISTS ip_allowlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    cidr VARCHAR(50) NOT NULL,
    description VARCHAR(255) DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ip_allowlist_org ON ip_allowlists(organization_id);
CREATE INDEX IF NOT EXISTS idx_ip_allowlist_project ON ip_allowlists(project_id);

-- Phase 9: Secret expiration policies
CREATE TABLE IF NOT EXISTS secret_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    max_age_days INTEGER DEFAULT 90,
    rotation_reminder_days INTEGER DEFAULT 7,
    require_rotation BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (project_id)
);

-- Phase 9: Compliance reports
CREATE TABLE IF NOT EXISTS compliance_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    report_type VARCHAR(50) NOT NULL, -- soc2, gdpr, pci
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, generating, completed, failed
    data JSONB DEFAULT '{}',
    generated_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Phase 9: Backup snapshots
CREATE TABLE IF NOT EXISTS backup_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    snapshot_type VARCHAR(50) NOT NULL, -- full, incremental
    encrypted_data BYTEA NOT NULL,
    data_nonce BYTEA NOT NULL,
    size_bytes BIGINT DEFAULT 0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Phase 10: Session tokens for revocation
CREATE TABLE IF NOT EXISTS session_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_session_tokens_user ON session_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_session_tokens_hash ON session_tokens(token_hash);

-- Phase 10: Security events log
CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL, -- auth_failure, rate_limit, suspicious_access
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ip_address VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    details JSONB DEFAULT '{}',
    severity VARCHAR(20) NOT NULL DEFAULT 'info', -- info, warning, critical
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_security_events_type ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_time ON security_events(created_at);

-- Phase 11: Secret leases (JIT access)
CREATE TABLE IF NOT EXISTS secret_leases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment VARCHAR(50) NOT NULL,
    secret_keys TEXT[] DEFAULT '{}',
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_leases_apikey ON secret_leases(api_key_id);
CREATE INDEX IF NOT EXISTS idx_leases_project ON secret_leases(project_id);
CREATE INDEX IF NOT EXISTS idx_leases_expires ON secret_leases(expires_at);

-- Phase 11: Agent activity tracking
CREATE TABLE IF NOT EXISTS agent_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    environment VARCHAR(50),
    secret_key VARCHAR(255),
    ip_address VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_activity_key ON agent_activities(api_key_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_project ON agent_activities(project_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_time ON agent_activities(created_at);

-- Phase 12: Events for event bus
CREATE TABLE IF NOT EXISTS event_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    aggregate_id UUID,
    payload JSONB NOT NULL DEFAULT '{}',
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_log_type ON event_log(event_type);
CREATE INDEX IF NOT EXISTS idx_event_log_unpublished ON event_log(published) WHERE published = FALSE;

-- Phase 12: Secret access policies
CREATE TABLE IF NOT EXISTS access_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    policy_type VARCHAR(50) NOT NULL, -- time_window, ip_restriction, geo_restriction
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_access_policies_project ON access_policies(project_id);

-- Phase 12: Plugin registry
CREATE TABLE IF NOT EXISTS plugins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    plugin_type VARCHAR(50) NOT NULL, -- secret_provider, notification, validation
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add last_used_at to api_keys for stale key detection
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMP WITH TIME ZONE;

-- Add password complexity tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;
