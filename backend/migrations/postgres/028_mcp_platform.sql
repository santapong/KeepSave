-- Resource-bound public OAuth for the separate, opt-in MCP platform.
CREATE TABLE mcp_oauth_clients (
    client_id TEXT PRIMARY KEY,
    harness TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO mcp_oauth_clients(client_id,harness,redirect_uri) VALUES
('keepsave-codex-linux-v1','codex','http://127.0.0.1:17701/callback'),
('keepsave-hermes-linux-v1','hermes','http://127.0.0.1:17702/callback');

CREATE TABLE mcp_oauth_requests (
    id UUID PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id),
    resource TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL,
    state TEXT NOT NULL,
    challenge TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE TABLE mcp_oauth_consents (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    session_id UUID NOT NULL REFERENCES session_tokens(id),
    client_id TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id),
    resource TEXT NOT NULL,
    scope TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX mcp_oauth_consents_parent ON mcp_oauth_consents(user_id,session_id);
CREATE TABLE mcp_oauth_codes (
    code_hash TEXT PRIMARY KEY,
    consent_id UUID NOT NULL REFERENCES mcp_oauth_consents(id),
    redirect_uri TEXT NOT NULL,
    challenge TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);
CREATE TABLE mcp_oauth_families (
    id UUID PRIMARY KEY,
    consent_id UUID NOT NULL REFERENCES mcp_oauth_consents(id),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE mcp_oauth_tokens (
    id UUID PRIMARY KEY,
    family_id UUID NOT NULL REFERENCES mcp_oauth_families(id),
    access_hash TEXT NOT NULL UNIQUE,
    refresh_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX mcp_oauth_tokens_family ON mcp_oauth_tokens(family_id);
