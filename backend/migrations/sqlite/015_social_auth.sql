-- Additive external identity support. Provider tokens are never stored.
CREATE TABLE social_identities (
    provider VARCHAR(16) NOT NULL,
    subject TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id),
    email VARCHAR(255) NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (provider, subject),
    UNIQUE (user_id, provider)
);
CREATE TABLE social_auth_flows (
    state_hash CHAR(64) PRIMARY KEY,
    provider VARCHAR(16) NOT NULL,
    challenge CHAR(43) NOT NULL,
    link_user_id TEXT REFERENCES users(id),
    expires_at BIGINT NOT NULL
);
CREATE INDEX idx_social_auth_expiry ON social_auth_flows(expires_at);
