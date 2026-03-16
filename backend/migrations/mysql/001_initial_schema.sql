CREATE TABLE users (
    id CHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    updated_at DATETIME NOT NULL DEFAULT NOW()
);

CREATE TABLE projects (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    owner_id CHAR(36) NOT NULL,
    encrypted_dek BLOB NOT NULL,
    dek_nonce BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    updated_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_projects_owner ON projects(owner_id);

CREATE TABLE environments (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    name VARCHAR(50) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE TABLE secrets (
    id CHAR(36) PRIMARY KEY,
    project_id CHAR(36) NOT NULL,
    environment_id CHAR(36) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    encrypted_value BLOB NOT NULL,
    value_nonce BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    updated_at DATETIME NOT NULL DEFAULT NOW(),
    UNIQUE(environment_id, `key`),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX idx_secrets_project_env ON secrets(project_id, environment_id);

CREATE TABLE api_keys (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    hashed_key VARCHAR(64) NOT NULL UNIQUE,
    user_id CHAR(36) NOT NULL,
    project_id CHAR(36) NOT NULL,
    scopes JSON NOT NULL DEFAULT ('["read"]'),
    environment VARCHAR(50),
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_hashed ON api_keys(hashed_key);

CREATE TABLE audit_log (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    project_id CHAR(36),
    action VARCHAR(100) NOT NULL,
    environment VARCHAR(50),
    details JSON DEFAULT ('{}'),
    ip_address VARCHAR(45),
    created_at DATETIME NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_log_project ON audit_log(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT NOW()
);
