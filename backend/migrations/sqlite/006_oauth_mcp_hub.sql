-- Phase 13: OAuth 2.0 Provider & MCP Server Hub (SQLite)

CREATE TABLE IF NOT EXISTS oauth_clients (
    id TEXT PRIMARY KEY,
    client_id TEXT UNIQUE NOT NULL,
    client_secret_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uris TEXT NOT NULL DEFAULT '[]',
    scopes TEXT NOT NULL DEFAULT '["read"]',
    grant_types TEXT NOT NULL DEFAULT '["authorization_code"]',
    logo_url TEXT DEFAULT '',
    homepage_url TEXT DEFAULT '',
    is_public INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients(client_id);
CREATE INDEX IF NOT EXISTS idx_oauth_clients_owner ON oauth_clients(owner_id);

CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    client_id TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT '[]',
    code_challenge TEXT DEFAULT '',
    code_challenge_method TEXT DEFAULT '',
    expires_at TEXT NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id TEXT PRIMARY KEY,
    access_token_hash TEXT UNIQUE NOT NULL,
    refresh_token_hash TEXT UNIQUE,
    client_id TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    scopes TEXT NOT NULL DEFAULT '[]',
    token_type TEXT NOT NULL DEFAULT 'Bearer',
    expires_at TEXT NOT NULL,
    refresh_expires_at TEXT,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_access ON oauth_tokens(access_token_hash);
CREATE INDEX IF NOT EXISTS idx_oauth_tokens_refresh ON oauth_tokens(refresh_token_hash);

CREATE TABLE IF NOT EXISTS mcp_servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_url TEXT NOT NULL,
    github_branch TEXT NOT NULL DEFAULT 'main',
    entry_command TEXT DEFAULT '',
    transport TEXT NOT NULL DEFAULT 'stdio',
    icon_url TEXT DEFAULT '',
    version TEXT NOT NULL DEFAULT '1.0.0',
    status TEXT NOT NULL DEFAULT 'pending',
    build_log TEXT DEFAULT '',
    env_mappings TEXT DEFAULT '{}',
    tool_definitions TEXT DEFAULT '[]',
    last_synced_at TEXT,
    install_count INTEGER NOT NULL DEFAULT 0,
    is_public INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_owner ON mcp_servers(owner_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);

CREATE TABLE IF NOT EXISTS mcp_installations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mcp_server_id TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    config TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, mcp_server_id)
);

CREATE TABLE IF NOT EXISTS mcp_gateway_log (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    mcp_server_id TEXT REFERENCES mcp_servers(id) ON DELETE SET NULL,
    tool_name TEXT NOT NULL,
    request_payload TEXT DEFAULT '{}',
    response_status TEXT NOT NULL DEFAULT 'success',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
