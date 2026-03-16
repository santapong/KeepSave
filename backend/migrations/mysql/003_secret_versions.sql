CREATE TABLE secret_versions (
    id CHAR(36) PRIMARY KEY,
    secret_id CHAR(36) NOT NULL,
    project_id CHAR(36) NOT NULL,
    environment_id CHAR(36) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    encrypted_value BLOB NOT NULL,
    value_nonce BLOB NOT NULL,
    created_by CHAR(36),
    created_at DATETIME NOT NULL DEFAULT NOW(),
    UNIQUE(secret_id, version),
    FOREIGN KEY (secret_id) REFERENCES secrets(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_secret_versions_secret ON secret_versions(secret_id, version DESC);
CREATE INDEX idx_secret_versions_project_env ON secret_versions(project_id, environment_id);

CREATE TABLE webhook_configs (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) DEFAULT '',
    events JSON NOT NULL DEFAULT ('["*"]'),
    active TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    updated_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_configs_project ON webhook_configs(project_id);

CREATE TABLE webhook_deliveries (
    id CHAR(36) PRIMARY KEY,
    webhook_id CHAR(36) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSON NOT NULL DEFAULT ('{}'),
    status_code INTEGER,
    success TINYINT(1) NOT NULL DEFAULT 0,
    error_message TEXT DEFAULT '',
    delivered_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (webhook_id) REFERENCES webhook_configs(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, delivered_at DESC);
