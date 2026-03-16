CREATE TABLE IF NOT EXISTS sso_configs (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_encrypted BLOB NOT NULL,
    client_secret_nonce BLOB NOT NULL,
    metadata TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE (organization_id, provider)
);

CREATE TABLE IF NOT EXISTS ip_allowlists (
    id TEXT PRIMARY KEY,
    organization_id TEXT REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    cidr TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ip_allowlist_org ON ip_allowlists(organization_id);
CREATE INDEX IF NOT EXISTS idx_ip_allowlist_project ON ip_allowlists(project_id);

CREATE TABLE IF NOT EXISTS secret_policies (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    max_age_days INTEGER DEFAULT 90,
    rotation_reminder_days INTEGER DEFAULT 7,
    require_rotation INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE (project_id)
);

CREATE TABLE IF NOT EXISTS compliance_reports (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    report_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    data TEXT DEFAULT '{}',
    generated_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS backup_snapshots (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    snapshot_type TEXT NOT NULL,
    encrypted_data BLOB NOT NULL,
    data_nonce BLOB NOT NULL,
    size_bytes INTEGER DEFAULT 0,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS session_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked INTEGER DEFAULT 0,
    ip_address TEXT,
    user_agent TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_session_tokens_user ON session_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_session_tokens_hash ON session_tokens(token_hash);

CREATE TABLE IF NOT EXISTS security_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    details TEXT DEFAULT '{}',
    severity TEXT NOT NULL DEFAULT 'info',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_security_events_type ON security_events(event_type);
CREATE INDEX IF NOT EXISTS idx_security_events_time ON security_events(created_at);

CREATE TABLE IF NOT EXISTS secret_leases (
    id TEXT PRIMARY KEY,
    api_key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment TEXT NOT NULL,
    secret_keys TEXT DEFAULT '[]',
    granted_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    revoked INTEGER DEFAULT 0,
    revoked_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_leases_apikey ON secret_leases(api_key_id);
CREATE INDEX IF NOT EXISTS idx_leases_project ON secret_leases(project_id);
CREATE INDEX IF NOT EXISTS idx_leases_expires ON secret_leases(expires_at);

CREATE TABLE IF NOT EXISTS agent_activities (
    id TEXT PRIMARY KEY,
    api_key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    environment TEXT,
    secret_key TEXT,
    ip_address TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_activity_key ON agent_activities(api_key_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_project ON agent_activities(project_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_time ON agent_activities(created_at);

CREATE TABLE IF NOT EXISTS event_log (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_id TEXT,
    payload TEXT NOT NULL DEFAULT '{}',
    published INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_event_log_type ON event_log(event_type);
CREATE INDEX IF NOT EXISTS idx_event_log_unpublished ON event_log(published) WHERE published = 0;

CREATE TABLE IF NOT EXISTS access_policies (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    policy_type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_access_policies_project ON access_policies(project_id);

CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    plugin_type TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '1.0.0',
    config TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

ALTER TABLE api_keys ADD COLUMN last_used_at TEXT;
ALTER TABLE users ADD COLUMN password_changed_at TEXT DEFAULT (datetime('now'));
ALTER TABLE users ADD COLUMN failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until TEXT;
