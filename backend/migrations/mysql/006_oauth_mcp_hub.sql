-- Phase 13: OAuth 2.0 Provider & MCP Server Hub (MySQL)

CREATE TABLE oauth_clients (
    id CHAR(36) PRIMARY KEY,
    client_id VARCHAR(64) UNIQUE NOT NULL,
    client_secret_hash VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id CHAR(36) NOT NULL,
    redirect_uris JSON NOT NULL,
    scopes JSON NOT NULL,
    grant_types JSON NOT NULL,
    logo_url TEXT,
    homepage_url TEXT,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_clients_client_id ON oauth_clients(client_id);
CREATE INDEX idx_oauth_clients_owner ON oauth_clients(owner_id);

CREATE TABLE oauth_authorization_codes (
    id CHAR(36) PRIMARY KEY,
    code VARCHAR(128) UNIQUE NOT NULL,
    client_id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    redirect_uri TEXT NOT NULL,
    scopes JSON NOT NULL,
    code_challenge VARCHAR(128) DEFAULT '',
    code_challenge_method VARCHAR(10) DEFAULT '',
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (client_id) REFERENCES oauth_clients(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE oauth_tokens (
    id CHAR(36) PRIMARY KEY,
    access_token_hash VARCHAR(128) UNIQUE NOT NULL,
    refresh_token_hash VARCHAR(128) UNIQUE,
    client_id CHAR(36) NOT NULL,
    user_id CHAR(36),
    scopes JSON NOT NULL,
    token_type VARCHAR(20) NOT NULL DEFAULT 'Bearer',
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (client_id) REFERENCES oauth_clients(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_tokens_access ON oauth_tokens(access_token_hash);
CREATE INDEX idx_oauth_tokens_refresh ON oauth_tokens(refresh_token_hash);

CREATE TABLE mcp_servers (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id CHAR(36) NOT NULL,
    github_url TEXT NOT NULL,
    github_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    entry_command TEXT,
    transport VARCHAR(20) NOT NULL DEFAULT 'stdio',
    icon_url TEXT,
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    build_log LONGTEXT,
    env_mappings JSON,
    tool_definitions JSON,
    last_synced_at TIMESTAMP NULL,
    install_count INTEGER NOT NULL DEFAULT 0,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_mcp_servers_owner ON mcp_servers(owner_id);
CREATE INDEX idx_mcp_servers_status ON mcp_servers(status);

CREATE TABLE mcp_installations (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    mcp_server_id CHAR(36) NOT NULL,
    project_id CHAR(36),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSON,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE(user_id, mcp_server_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
);

CREATE TABLE mcp_gateway_log (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    mcp_server_id CHAR(36),
    tool_name VARCHAR(255) NOT NULL,
    request_payload JSON,
    response_status VARCHAR(20) NOT NULL DEFAULT 'success',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE SET NULL
);
