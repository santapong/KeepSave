-- Phase 13: OAuth 2.0 Provider & MCP Server Hub

-- OAuth 2.0 Clients (applications that can request authorization)
CREATE TABLE oauth_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(64) UNIQUE NOT NULL,
    client_secret_hash VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uris TEXT[] NOT NULL DEFAULT '{}',
    scopes TEXT[] NOT NULL DEFAULT '{"read"}',
    grant_types TEXT[] NOT NULL DEFAULT '{"authorization_code"}',
    logo_url TEXT DEFAULT '',
    homepage_url TEXT DEFAULT '',
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_clients_client_id ON oauth_clients(client_id);
CREATE INDEX idx_oauth_clients_owner ON oauth_clients(owner_id);

-- OAuth 2.0 Authorization Codes
CREATE TABLE oauth_authorization_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(128) UNIQUE NOT NULL,
    client_id UUID NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    code_challenge VARCHAR(128) DEFAULT '',
    code_challenge_method VARCHAR(10) DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_codes_code ON oauth_authorization_codes(code);
CREATE INDEX idx_oauth_codes_expires ON oauth_authorization_codes(expires_at);

-- OAuth 2.0 Access/Refresh Tokens
CREATE TABLE oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    access_token_hash VARCHAR(128) UNIQUE NOT NULL,
    refresh_token_hash VARCHAR(128) UNIQUE,
    client_id UUID NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    token_type VARCHAR(20) NOT NULL DEFAULT 'Bearer',
    expires_at TIMESTAMPTZ NOT NULL,
    refresh_expires_at TIMESTAMPTZ,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_tokens_access ON oauth_tokens(access_token_hash);
CREATE INDEX idx_oauth_tokens_refresh ON oauth_tokens(refresh_token_hash);
CREATE INDEX idx_oauth_tokens_client ON oauth_tokens(client_id);
CREATE INDEX idx_oauth_tokens_user ON oauth_tokens(user_id);

-- MCP Server Registry
CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    github_url TEXT NOT NULL,
    github_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    entry_command TEXT NOT NULL DEFAULT '',
    transport VARCHAR(20) NOT NULL DEFAULT 'stdio',
    icon_url TEXT DEFAULT '',
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    build_log TEXT DEFAULT '',
    env_mappings JSONB DEFAULT '{}',
    tool_definitions JSONB DEFAULT '[]',
    last_synced_at TIMESTAMPTZ,
    install_count INTEGER NOT NULL DEFAULT 0,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mcp_servers_owner ON mcp_servers(owner_id);
CREATE INDEX idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX idx_mcp_servers_public ON mcp_servers(is_public) WHERE is_public = TRUE;

-- User MCP Server Installations (which users have enabled which MCP servers)
CREATE TABLE mcp_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, mcp_server_id)
);

CREATE INDEX idx_mcp_installations_user ON mcp_installations(user_id);
CREATE INDEX idx_mcp_installations_server ON mcp_installations(mcp_server_id);

-- MCP Gateway Requests Log
CREATE TABLE mcp_gateway_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    mcp_server_id UUID REFERENCES mcp_servers(id) ON DELETE SET NULL,
    tool_name VARCHAR(255) NOT NULL,
    request_payload JSONB DEFAULT '{}',
    response_status VARCHAR(20) NOT NULL DEFAULT 'success',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mcp_gateway_log_user ON mcp_gateway_log(user_id, created_at DESC);
CREATE INDEX idx_mcp_gateway_log_server ON mcp_gateway_log(mcp_server_id, created_at DESC);
