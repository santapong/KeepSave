-- Phase 7-12: Observability, Enterprise, Security, Agent, Platform features

-- Phase 9: SSO provider configurations
CREATE TABLE IF NOT EXISTS sso_configs (
    id CHAR(36) PRIMARY KEY,
    organization_id CHAR(36) NOT NULL,
    provider VARCHAR(50) NOT NULL, -- oidc, saml
    issuer_url VARCHAR(500) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    client_secret_encrypted BLOB NOT NULL,
    client_secret_nonce BLOB NOT NULL,
    metadata JSON DEFAULT ('{}'),
    enabled TINYINT(1) DEFAULT 1,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    UNIQUE (organization_id, provider),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

-- Phase 9: IP allowlists
CREATE TABLE IF NOT EXISTS ip_allowlists (
    id CHAR(36) PRIMARY KEY,
    organization_id CHAR(36),
    project_id CHAR(36),
    cidr VARCHAR(50) NOT NULL,
    description VARCHAR(255) DEFAULT '',
    created_by CHAR(36) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE INDEX idx_ip_allowlist_org ON ip_allowlists(organization_id);
CREATE INDEX idx_ip_allowlist_project ON ip_allowlists(project_id);

-- Phase 9: Secret expiration policies
CREATE TABLE IF NOT EXISTS secret_policies (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    max_age_days INTEGER DEFAULT 90,
    rotation_reminder_days INTEGER DEFAULT 7,
    require_rotation TINYINT(1) DEFAULT 0,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    UNIQUE (project_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Phase 9: Compliance reports
CREATE TABLE IF NOT EXISTS compliance_reports (
    id CHAR(36) PRIMARY KEY,
    organization_id CHAR(36) NOT NULL,
    report_type VARCHAR(50) NOT NULL, -- soc2, gdpr, pci
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, generating, completed, failed
    data JSON DEFAULT ('{}'),
    generated_by CHAR(36) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    completed_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (generated_by) REFERENCES users(id)
);

-- Phase 9: Backup snapshots
CREATE TABLE IF NOT EXISTS backup_snapshots (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36),
    snapshot_type VARCHAR(50) NOT NULL, -- full, incremental
    encrypted_data BLOB NOT NULL,
    data_nonce BLOB NOT NULL,
    size_bytes BIGINT DEFAULT 0,
    created_by CHAR(36) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Phase 10: Session tokens for revocation
CREATE TABLE IF NOT EXISTS session_tokens (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    revoked TINYINT(1) DEFAULT 0,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    created_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_session_tokens_user ON session_tokens(user_id);
CREATE INDEX idx_session_tokens_hash ON session_tokens(token_hash);

-- Phase 10: Security events log
CREATE TABLE IF NOT EXISTS security_events (
    id CHAR(36) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL, -- auth_failure, rate_limit, suspicious_access
    user_id CHAR(36),
    ip_address VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    details JSON DEFAULT ('{}'),
    severity VARCHAR(20) NOT NULL DEFAULT 'info', -- info, warning, critical
    created_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_security_events_type ON security_events(event_type);
CREATE INDEX idx_security_events_time ON security_events(created_at);

-- Phase 11: Secret leases (JIT access)
CREATE TABLE IF NOT EXISTS secret_leases (
    id CHAR(36) PRIMARY KEY,
    api_key_id CHAR(36) NOT NULL,
    project_id CHAR(36) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    secret_keys JSON DEFAULT ('[]'),
    granted_at DATETIME DEFAULT NOW(),
    expires_at DATETIME NOT NULL,
    revoked TINYINT(1) DEFAULT 0,
    revoked_at DATETIME,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_leases_apikey ON secret_leases(api_key_id);
CREATE INDEX idx_leases_project ON secret_leases(project_id);
CREATE INDEX idx_leases_expires ON secret_leases(expires_at);

-- Phase 11: Agent activity tracking
CREATE TABLE IF NOT EXISTS agent_activities (
    id CHAR(36) PRIMARY KEY,
    api_key_id CHAR(36) NOT NULL,
    project_id CHAR(36) NOT NULL,
    action VARCHAR(100) NOT NULL,
    environment VARCHAR(50),
    secret_key VARCHAR(255),
    ip_address VARCHAR(45),
    created_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_activity_key ON agent_activities(api_key_id);
CREATE INDEX idx_agent_activity_project ON agent_activities(project_id);
CREATE INDEX idx_agent_activity_time ON agent_activities(created_at);

-- Phase 12: Events for event bus
CREATE TABLE IF NOT EXISTS event_log (
    id CHAR(36) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    aggregate_id CHAR(36),
    payload JSON NOT NULL DEFAULT ('{}'),
    published TINYINT(1) DEFAULT 0,
    created_at DATETIME DEFAULT NOW()
);

CREATE INDEX idx_event_log_type ON event_log(event_type);
CREATE INDEX idx_event_log_unpublished ON event_log(published);

-- Phase 12: Secret access policies
CREATE TABLE IF NOT EXISTS access_policies (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    policy_type VARCHAR(50) NOT NULL, -- time_window, ip_restriction, geo_restriction
    config JSON NOT NULL DEFAULT ('{}'),
    enabled TINYINT(1) DEFAULT 1,
    created_by CHAR(36) NOT NULL,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW(),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE INDEX idx_access_policies_project ON access_policies(project_id);

-- Phase 12: Plugin registry
CREATE TABLE IF NOT EXISTS plugins (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    plugin_type VARCHAR(50) NOT NULL, -- secret_provider, notification, validation
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    config JSON DEFAULT ('{}'),
    enabled TINYINT(1) DEFAULT 1,
    created_at DATETIME DEFAULT NOW(),
    updated_at DATETIME DEFAULT NOW()
);

-- Add last_used_at to api_keys for stale key detection
ALTER TABLE api_keys ADD COLUMN last_used_at DATETIME;

-- Add password complexity tracking
ALTER TABLE users ADD COLUMN password_changed_at DATETIME DEFAULT NOW();
ALTER TABLE users ADD COLUMN failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until DATETIME;
